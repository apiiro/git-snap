package git

import (
	"errors"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
	"github.com/gobwas/glob"
	"gitsnap/options"
	"gitsnap/util"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	TARGET_PERMISSIONS = 0777
)

type repositoryProvider struct {
	repository      *git.Repository
	includePatterns []glob.Glob
	excludePatterns []glob.Glob
	opts            *options.Options
}

func Snapshot(opts *options.Options) (err error) {

	provider := &repositoryProvider{
		opts: opts,
	}

	provider.includePatterns, err = provider.compileGlobs(opts.IncludePatterns, "include")
	if err != nil {
		return fmt.Errorf("failed to compile include patterns '%v': %v", opts.IncludePatterns, err)
	}
	provider.excludePatterns, err = provider.compileGlobs(opts.ExcludePatterns, "exclude")
	if err != nil {
		return fmt.Errorf("failed to compile exclude patterns '%v': %v", opts.ExcludePatterns, err)
	}

	provider.repository, err = git.PlainOpen(opts.ClonePath)
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_CLONE_GIT,
			InternalError: err,
		}
	}

	var headCommit *object.Commit
	headCommit, err = provider.getCommit("HEAD")
	if err != nil {
		return fmt.Errorf("failed to resolved HEAD revision: %v", err)
	}
	if headCommit == nil {
		return nil
	}

	var commit *object.Commit
	commit, err = provider.getCommit(opts.Revision)
	if err != nil || commit == nil {
		return err
	}

	log.Printf("snapshotting commit '%v' for revision '%v' at clone '%v'", commit.ID(), opts.Revision, opts.ClonePath)

	var filesCount int
	var filesCountDryRun int
	if opts.SkipDoubleCheck {
		filesCount, err = provider.snapshot(commit, opts.OutputPath, false)
		if err != nil {
			return err
		}
	} else {
		filesCountDryRun, err = provider.snapshot(commit, opts.OutputPath, true)
		if err != nil {
			return err
		}

		filesCount, err = provider.snapshot(commit, opts.OutputPath, false)
		if err != nil {
			return err
		}
		if filesCount != filesCountDryRun {
			return &util.ErrorWithCode{
				StatusCode:    util.ERROR_FILES_DISCREPANCY,
				InternalError: fmt.Errorf("dryRun files count is %v , but snapshot files count is %v", filesCountDryRun, filesCount),
			}
		}

		filesCountDryRun, err = provider.snapshot(commit, opts.OutputPath, true)
		if err != nil {
			return err
		}
		if filesCount != filesCountDryRun {
			return &util.ErrorWithCode{
				StatusCode:    util.ERROR_FILES_DISCREPANCY,
				InternalError: fmt.Errorf("dryRun files count is %v , but snapshot files count is %v", filesCountDryRun, filesCount),
			}
		}
	}

	log.Printf("written %v files to target path '%v'", filesCount, opts.OutputPath)
	return nil
}

func (provider *repositoryProvider) getCommit(commitish string) (*object.Commit, error) {

	_, err := provider.repository.Head()
	if err == plumbing.ErrReferenceNotFound {
		log.Printf("repository is detected as empty -- nothing to do")
		return nil, nil
	}

	hash, err := provider.repository.ResolveRevision(plumbing.Revision(commitish))
	if err != nil {
		return nil, &util.ErrorWithCode{
			StatusCode:    util.ERROR_NO_REVISION,
			InternalError: fmt.Errorf("failed to get revision '%v': %v", commitish, err),
		}
	}

	return provider.repository.CommitObject(*hash)
}

func expandPatternsIfNeeded(patterns []string) []string {
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "*/") {
			patterns = append(patterns, strings.Replace(pattern, "*/", "", 1))
		}
		if strings.HasPrefix(pattern, "**/") {
			patterns = append(patterns, strings.Replace(pattern, "**/", "", 1))
		}
	}
	return patterns
}

func (provider *repositoryProvider) compileGlobs(patterns []string, title string) ([]glob.Glob, error) {
	patterns = expandPatternsIfNeeded(patterns)
	provider.verboseLog("%v %v patterns:\n%v", len(patterns), title, strings.Join(patterns, ", "))
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

	if !mode.IsFile() || mode.IsMalformed() || provider.isSymlink(filePath, mode) {
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

	skip := true
	hasIncludePatterns := len(provider.includePatterns) > 0
	if hasIncludePatterns && !matches(filePathToCheck, provider.includePatterns) {
		provider.verboseLog("--- skipping '%v' - not matching include patterns", filePath)
		return nil
	} else if hasIncludePatterns {
		skip = false
	}

	if len(provider.excludePatterns) > 0 && matches(filePathToCheck, provider.excludePatterns) && skip {
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
		return fmt.Errorf("failed to create target directory at '%v': %v", targetDirectoryPath, err)
	}

	var contents string
	err = retry.Do(
		func() error {
			var contentsErr error
			contents, contentsErr = file.Contents()
			return contentsErr
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get git file contents for '%v': %v", filePath, err)
	}

	contentsBytes := []byte(contents)

	err = ioutil.WriteFile(targetFilePath, contentsBytes, TARGET_PERMISSIONS)
	if err != nil {
		if strings.Contains(err.Error(), "file name too long") {
			return util.ErrorWithCode{
				StatusCode:    util.ERROR_PATH_TOO_LONG,
				InternalError: err,
			}
		}
		return fmt.Errorf("failed to write target file of '%v' to '%v': %v", filePath, targetFilePath, err)
	}

	provider.verboseLog("+++ '%v' to '%v'", filePath, targetFilePath)

	if provider.opts.CreateHashMarkers {
		targetHashFilePath := fmt.Sprintf("%v.hash", targetFilePath)
		err = ioutil.WriteFile(targetHashFilePath, []byte(file.Hash.String()), TARGET_PERMISSIONS)
		if err != nil {
			log.Printf("failed to write hash file of '%v' to '%v': %v", filePath, targetFilePath, err)
		}
	}

	return nil
}

func (provider *repositoryProvider) snapshot(commit *object.Commit, outputPath string, dryRun bool) (int, error) {

	tree, err := commit.Tree()
	if err != nil {
		return 0, fmt.Errorf("failed to get tree of commit '%v': %v", commit.Hash, err)
	}
	count := 0
	err = tree.Files().ForEach(func(file *object.File) error {
		count++
		if dryRun {
			return nil
		}
		return provider.dumpFile(file, outputPath)
	})
	if err != nil {
		if errors.Is(err, dotgit.ErrPackfileNotFound) {
			return 0, util.ErrorWithCode{
				StatusCode:    util.ERROR_BAD_CLONE_GIT,
				InternalError: err,
			}
		}
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return 0, util.ErrorWithCode{
				StatusCode:    util.ERROR_NO_REVISION,
				InternalError: err,
			}
		}
		return 0, fmt.Errorf("failed to iterate files of %v: %v", commit.Hash, err)
	}
	provider.verboseLog("iterated %v files for %v", count, commit.Hash)
	return count, nil
}

func (provider *repositoryProvider) isSymlink(filePath string, mode filemode.FileMode) bool {
	osMode, err := mode.ToOSFileMode()
	if err != nil {
		provider.verboseLog("failed to parse os file permissions for '%v': %v", filePath, err)
		return false
	}
	return osMode&os.ModeSymlink != 0
}
