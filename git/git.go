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
	"sync"
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
	mutex           *sync.Mutex
}

func Snapshot(opts *options.Options) (err error) {

	includePatterns, err := compileGlobs(opts.IncludePatterns)
	if err != nil {
		return err
	}
	excludePatterns, err := compileGlobs(opts.ExcludePatterns)
	if err != nil {
		return err
	}

	provider := &repositoryProvider{
		includePatterns: includePatterns,
		excludePatterns: excludePatterns,
		opts:            opts,
		mutex:           &sync.Mutex{},
	}
	provider.repository, err = git.PlainOpen(opts.ClonePath)
	if err != nil {
		return
	}

	var commit *object.Commit
	commit, err = provider.getCommit(opts.Revision, opts.SupportShortSha)
	if err != nil {
		return err
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
			return nil, fmt.Errorf("cannot parse short sha revision")
		}
		return provider.getCommitFromShortSha(commitish)
	}

	hash, err := provider.repository.ResolveRevision(plumbing.Revision(commitish))
	if err != nil {
		return nil, err
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

func (provider *repositoryProvider) dumpFile(commit *object.Commit, file *object.File, outputPath string) error {
	filePath := file.Name

	mode := file.Mode
	if !mode.IsFile() || mode.IsMalformed() || !mode.IsRegular() {
		provider.verboseLog("--- skipping '%v' - not regular file - mode: %v", filePath, mode)
		return nil
	}

	if file.Size >= provider.opts.MaxFileSizeBytes {
		log.Printf("--- skipping '%v' - file size is too large to snapshot - %v", filePath, file.Size)
		return nil
	}

	if len(provider.includePatterns) > 0 && !matches(filePath, provider.includePatterns) {
		provider.verboseLog("--- skipping '%v' - not matching include patterns", filePath)
		return nil
	}

	if len(provider.excludePatterns) > 0 && matches(filePath, provider.excludePatterns) {
		provider.verboseLog("--- skipping '%v' - matching exclude patterns", filePath)
		return nil
	}

	if provider.opts.TextFilesOnly && util.NotTextExt(filepath.Ext(filePath)) {
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
	contents, err = provider.getFileContents(file)
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

func (provider *repositoryProvider) getFileContents(file *object.File) (string, error) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	return file.Contents()
}

func (provider *repositoryProvider) snapshot(commit *object.Commit, outputPath string) error {

	queue := parallelizer.NewGroup(func(groupOptions *parallelizer.GroupOptions) {
		groupOptions.PoolSize = runtime.NumCPU()
		groupOptions.JobQueueSize = 1024
	})
	defer queue.Close()

	err := provider.iterateFiles(commit, outputPath)

	if err != nil {
		return err
	}

	err = queue.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (provider *repositoryProvider) iterateFiles(commit *object.Commit, outputPath string) error {
	tree, err := commit.Tree()
	if err != nil {
		return err
	}
	iter := tree.Files()
	defer iter.Close()
	for {
		var file *object.File
		provider.mutex.Lock()
		file, err = iter.Next()
		provider.mutex.Unlock()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if err = provider.dumpFile(commit, file, outputPath); err != nil {
			if err == storer.ErrStop {
				return nil
			}

			return err
		}
	}
}
