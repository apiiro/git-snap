package git

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/gobwas/glob"
	"github.com/shomali11/parallelizer"
	"gitsnap/options"
	"gitsnap/util"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	SHORT_SHA_LENGTH   = 7
	TARGET_PERMISSIONS = 0777
)

type repositoryProvider struct {
	repository      *git.Repository
	includePatterns []glob.Glob
	excludePatterns []glob.Glob
	opts            *options.Options
}

func Snapshot(opts *options.Options) (err error) {

	includePatterns, err := compileGlobs(opts.IncludePatterns)
	if err != nil {
		return fmt.Errorf("failed to compile include patterns '%v': %v", opts.IncludePatterns, err)
	}
	excludePatterns, err := compileGlobs(opts.ExcludePatterns)
	if err != nil {
		return fmt.Errorf("failed to compile exclude patterns '%v': %v", opts.ExcludePatterns, err)
	}

	provider := &repositoryProvider{
		includePatterns: includePatterns,
		excludePatterns: excludePatterns,
		opts:            opts,
	}
	provider.repository, err = git.PlainOpen(opts.ClonePath)
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_CLONE_GIT,
			InternalError: err,
		}
	}

	var commit *object.Commit
	commit, err = provider.getCommit(opts.Revision, opts.SupportShortSha)
	if err != nil {
		return fmt.Errorf("failed to get revision '%v': %v", opts.Revision, err)
	}

	log.Printf("snapshotting commit '%v' for revision '%v' at clone '%v'", commit.ID(), opts.Revision, opts.ClonePath)

	err = provider.snapshot(commit, opts.OutputPath)
	if err == nil {
		log.Printf("written files to target path '%v'", opts.OutputPath)
	}
	return err
}

func (provider *repositoryProvider) getCommit(commitish string, supportShortSha bool) (*object.Commit, error) {

	if len(commitish) == SHORT_SHA_LENGTH {
		if !supportShortSha {
			return nil, &util.ErrorWithCode{
				StatusCode:    util.ERROR_NO_SHORT_SHA,
				InternalError: fmt.Errorf("cannot parse short sha revision %v", commitish),
			}
		}
		return provider.getCommitFromShortSha(commitish)
	}

	hash, err := provider.repository.ResolveRevision(plumbing.Revision(commitish))
	if err != nil {
		return nil, &util.ErrorWithCode{
			StatusCode:    util.ERROR_NO_REVISION,
			InternalError: err,
		}
	}

	return provider.repository.CommitObject(*hash)
}

func (provider *repositoryProvider) getCommitFromShortSha(commitish string) (commit *object.Commit, err error) {
	// Manual implementation of short sha mapping, due to bug in go-git: https://github.com/go-git/go-git/issues/148
	var iter object.CommitIter
	iter, err = provider.repository.CommitObjects()
	if err != nil {
		return
	}
	err = iter.ForEach(func(iterated *object.Commit) error {
		sha := iterated.Hash.String()
		shortSha := sha[:SHORT_SHA_LENGTH]
		if shortSha == commitish {
			commit = iterated
			return storer.ErrStop
		}
		return nil
	})
	return
}

func compileGlobs(patterns []string) ([]glob.Glob, error) {
	globs := make([]glob.Glob, len(patterns))
	for i, pattern := range patterns {
		compiled, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		globs[i] = compiled
	}
	return globs, nil
}

func matches(filePath string, patterns []glob.Glob) bool {
	for _, pattern := range patterns {
		if pattern.Match(filePath) {
			return true
		}
	}
	return false
}

func (provider *repositoryProvider) verboseLog(format string, v ...interface{}) {
	if provider.opts.VerboseLogging {
		log.Printf(format, v...)
	}
}

func (provider *repositoryProvider) dumpFile(file *object.File, outputPath string) error {
	filePath := file.Name

	mode := file.Mode
	if !mode.IsFile() || mode.IsMalformed() || !mode.IsRegular() {
		provider.verboseLog("--- skipping '%v' - not regular file - mode: %v", filePath, mode)
		return nil
	}

	if provider.opts.MaxFileSizeBytes > 0 && file.Size >= provider.opts.MaxFileSizeBytes {
		log.Printf("--- skipping '%v' - file size is too large to snapshot - %v", filePath, file.Size)
		return nil
	}

	filePathToCheck := filePath
	if provider.opts.IgnoreCasePatterns {
		filePathToCheck = strings.ToLower(filePathToCheck)
	}

	if len(provider.includePatterns) > 0 && !matches(filePathToCheck, provider.includePatterns) {
		provider.verboseLog("--- skipping '%v' - not matching include patterns", filePath)
		return nil
	}

	if len(provider.excludePatterns) > 0 && matches(filePathToCheck, provider.excludePatterns) {
		provider.verboseLog("--- skipping '%v' - matching exclude patterns", filePath)
		return nil
	}

	if provider.opts.TextFilesOnly && util.NotTextExt(filepath.Ext(filePathToCheck)) {
		provider.verboseLog("--- skipping '%v' - not a text file", filePath)
		return nil
	}

	targetFilePath := filepath.Join(outputPath, filePath)
	targetDirectoryPath := filepath.Dir(targetFilePath)
	err := os.MkdirAll(targetDirectoryPath, TARGET_PERMISSIONS)
	if err != nil {
		return err
	}

	var contents string
	contents, err = file.Contents()
	if err != nil {
		return err
	}

	contentsBytes := []byte(contents)

	err = ioutil.WriteFile(targetFilePath, contentsBytes, TARGET_PERMISSIONS)
	if err != nil {
		return err
	}

	provider.verboseLog("+++ '%v' to '%v'", filePath, targetFilePath)

	if provider.opts.CreateHashMarkers {
		err = ioutil.WriteFile(fmt.Sprintf("%v.hash", targetFilePath), []byte(file.Hash.String()), TARGET_PERMISSIONS)
	}

	return err
}

func (provider *repositoryProvider) snapshot(commit *object.Commit, outputPath string) error {

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	queue := parallelizer.NewGroup(func(groupOptions *parallelizer.GroupOptions) {
		groupOptions.PoolSize = runtime.NumCPU()
		groupOptions.JobQueueSize = 1024
	})
	defer queue.Close()

	var internalError error
	err = forEachFile(tree.Files(), func(file *object.File) error {
		return queue.Add(func() {
			err := provider.dumpFile(file, outputPath)
			if err != nil {
				internalError = err
			}
		})
	})
	if err != nil {
		return fmt.Errorf("failed to iterate files of %v: %v", commit.Hash, err)
	}

	err = queue.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait on work queue: %v", err)
	}

	if internalError != nil {
		return fmt.Errorf("error in work queue processing: %v", internalError)
	}
	return nil
}

func forEachFile(iter *object.FileIter, cb func(*object.File) error) error {
	defer iter.Close()

	for {
		f, err := iter.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			if err.Error() == "object not found" {
				continue
			}

			return fmt.Errorf("failed to fetch next file: %v", err)
		}

		if err := cb(f); err != nil {
			if err == storer.ErrStop {
				return nil
			}

			return fmt.Errorf("error on file cb for %v: %v", f.Name, err)
		}
	}
}
