package git

import (
	"fmt"
	"git-snap/options"
	"git-snap/parallel"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/gobwas/glob"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

const (
	MAX_FSIZE_MB       int64 = 6
	MAX_FSIZE_BYTES          = MAX_FSIZE_MB * 1024 * 1024
	SHORT_SHA_LENGTH         = 7
	TARGET_PERMISSIONS       = 0777
)

type repositoryProvider struct {
	repository      *git.Repository
	includePatterns []glob.Glob
	excludePatterns []glob.Glob
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

func (provider *repositoryProvider) dumpFile(commit *object.Commit, file *object.File, outputPath string) error {
	mode := file.Mode
	if !mode.IsFile() || mode.IsMalformed() || !mode.IsRegular() {
		return nil
	}

	filePath := file.Name

	if file.Size >= MAX_FSIZE_BYTES {
		log.Printf("file size is too large to snapshot - %v at %v/%v", file.Size, commit.ID(), filePath)
		return nil
	}

	if len(provider.includePatterns) > 0 && !matches(filePath, provider.includePatterns) {
		return nil
	}

	if len(provider.excludePatterns) > 0 && matches(filePath, provider.excludePatterns) {
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
	err = ioutil.WriteFile(targetFilePath, []byte(contents), TARGET_PERMISSIONS)
	return err
}

func (provider *repositoryProvider) snapshot(commit *object.Commit, outputPath string) error {

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	queue := parallel.CreateJobQueue(1024, runtime.NumCPU())
	defer queue.Close()

	var internalError error
	err = tree.Files().ForEach(func(file *object.File) error {
		return queue.Add(func() {
			err := provider.dumpFile(commit, file, outputPath)
			if err != nil {
				internalError = err
			}
		})
	})
	if err != nil {
		return err
	}

	err = queue.Wait()
	if err != nil {
		return err
	}

	if internalError != nil {
		return internalError
	}
	return nil
}
