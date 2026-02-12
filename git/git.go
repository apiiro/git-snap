package git

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"gitsnap/options"
	"gitsnap/stats"
	"gitsnap/util"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
	"github.com/gobwas/glob"
)

const (
	TARGET_PERMISSIONS   = 0777
	DISCREPANCY_ATTEMPTS = 3
	DISCREPANCY_DELAY    = 3 * time.Second
)

type repositoryProvider struct {
	repository      *git.Repository
	includePatterns []glob.Glob
	excludePatterns []glob.Glob
	fileListToSnap  map[string]bool
	opts            *options.Options
	// Stats mode fields
	statsCollector *stats.CodeStats
	createdDirs    map[string]bool
}

func Snapshot(opts *options.Options) (err error) {

	provider := &repositoryProvider{
		opts:           opts,
		fileListToSnap: map[string]bool{},
		createdDirs:    map[string]bool{},
	}

	err = loadFilePathsList(opts, provider)
	if err != nil {
		return err
	}

	provider.includePatterns, err = provider.compileGlobs(opts.IncludePatterns, "include")
	if err != nil {
		return fmt.Errorf("failed to compile include patterns '%v': %v", opts.IncludePatterns, err)
	}

	// For stats mode, add complexity tool patterns to exclude patterns
	excludePatterns := opts.ExcludePatterns
	if opts.StatsOnly {
		excludePatterns = append(excludePatterns, getStatsExcludePatterns()...)
	}
	provider.excludePatterns, err = provider.compileGlobs(excludePatterns, "exclude")
	if err != nil {
		return fmt.Errorf("failed to compile exclude patterns '%v': %v", excludePatterns, err)
	}

	// Initialize stats collector if in stats mode
	if opts.StatsOnly {
		provider.statsCollector = stats.NewCodeStats()
	}

	provider.repository, err = git.PlainOpen(opts.ClonePath)
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_CLONE_GIT,
			InternalError: err,
		}
	}

	// Helper function to get commit with validation
	getCommitWithValidation := func() (*object.Commit, error) {
		_, _ = provider.getCommit("HEAD")

		var commit *object.Commit
		commit, err = provider.getCommit(opts.Revision)
		if err != nil || commit == nil {
			return nil, err
		}
		return commit, nil
	}

	var commit *object.Commit
	commit, err = getCommitWithValidation()
	if err != nil {
		return err
	}

	if opts.StatsOnly {
		log.Printf("calculating stats for commit '%v' for revision '%v' at clone '%v'", commit.ID(), opts.Revision, opts.ClonePath)
	} else {
		log.Printf("snapshotting commit '%v' for revision '%v' at clone '%v'", commit.ID(), opts.Revision, opts.ClonePath)
	}

	var totalCount, writtenCount int
	if opts.SkipDoubleCheck || opts.StatsOnly {
		// Stats mode doesn't need discrepancy detection
		totalCount, writtenCount, err = provider.snapshot(provider.repository, commit, opts.OutputPath, opts.OptionalIndexFilePath, opts.IndexOnly, false)
		if err != nil {
			return err
		}
	} else {
		// Retry logic for discrepancy detection
		for attempt := 1; attempt <= DISCREPANCY_ATTEMPTS; attempt++ {
			// Re-get commit for each attempt after the first
			if attempt > 1 {
				log.Printf("waiting %v before retry attempt %d", DISCREPANCY_DELAY, attempt)
				time.Sleep(DISCREPANCY_DELAY)

				log.Printf("re-acquiring commit for retry attempt %d", attempt)
				commit, err = getCommitWithValidation()
				if err != nil {
					return err
				}
			}

			// Dry run - only counts total entries, skips filtering
			totalCountDryRun, _, err := provider.snapshot(provider.repository, commit, opts.OutputPath, opts.OptionalIndexFilePath, opts.IndexOnly, true)
			if err != nil {
				return err
			}

			// Actual run - counts both total and filtered
			totalCount, writtenCount, err = provider.snapshot(provider.repository, commit, opts.OutputPath, opts.OptionalIndexFilePath, opts.IndexOnly, false)
			if err != nil {
				return err
			}

			// Check for discrepancy using total counts (not filtered)
			if totalCount == totalCountDryRun {
				// Success - no discrepancy
				break
			}

			log.Printf("discrepancy detected on attempt %d: dryRun total count is %v, but snapshot total count is %v", attempt, totalCountDryRun, totalCount)

			// If this was the last attempt, return error
			if attempt == DISCREPANCY_ATTEMPTS {
				return &util.ErrorWithCode{
					StatusCode:    util.ERROR_FILES_DISCREPANCY,
					InternalError: fmt.Errorf("discrepancy persists after %d attempts: dryRun total count is %v, but snapshot total count is %v", DISCREPANCY_ATTEMPTS, totalCountDryRun, totalCount),
				}
			}
		}
	}

	// Handle stats output
	if opts.StatsOnly {
		return provider.writeStatsOutput(opts.OutputPath)
	}

	log.Printf("written %v files (out of %v total) to target path '%v'", writtenCount, totalCount, opts.OutputPath)
	return nil
}

func loadFilePathsList(opts *options.Options, provider *repositoryProvider) error {
	if opts.PathsFileLocation != "" {
		file, err := os.Open(opts.PathsFileLocation)
		if err != nil {
			return fmt.Errorf("failed to read paths file from location: '%v', error: '%v'", opts.PathsFileLocation, err)
		}

		reader := csv.NewReader(file)
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				log.Printf("warning: failed to close paths file: %v", closeErr)
			}
		}()

		lines, err := reader.ReadAll()
		if err != nil {
			return fmt.Errorf("failed to read paths file from location: '%v', error: '%v'", opts.PathsFileLocation, err)
		}
		for i := range lines {
			path := lines[i][0]
			if !utf8.ValidString(path) {
				provider.verboseLog("skipping invalid UTF-8 path found in the file paths file: %s", lines[i][0])
				continue
			}
			provider.fileListToSnap[path] = true
		}
	}
	return nil
}

func (provider *repositoryProvider) getCommit(commitish string) (*object.Commit, error) {

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

// shouldIncludeFile checks if a file should be included based on cheap filters (no blob access needed).
func (provider *repositoryProvider) shouldIncludeFile(name string, entry *object.TreeEntry) bool {
	filePath := name
	mode := entry.Mode

	if !mode.IsFile() || mode.IsMalformed() || provider.isSymlink(filePath, mode) {
		provider.verboseLog("--- skipping '%v' - not regular file - mode: %v", filePath, mode)
		return false
	}

	if !utf8.ValidString(filePath) {
		provider.verboseLog("--- skipping '%v' - file path is not a valid UTF-8 string", filePath)
		return false
	}

	filePathToCheck := filePath
	if provider.opts.IgnoreCasePatterns {
		filePathToCheck = strings.ToLower(filePathToCheck)
	}

	if !isFileInList(provider, filePathToCheck) {
		provider.verboseLog("--- skipping '%v' - not matching file list", filePath)
		return false
	}

	skip := true
	hasIncludePatterns := len(provider.includePatterns) > 0
	if hasIncludePatterns && !matches(filePathToCheck, provider.includePatterns) {
		provider.verboseLog("--- skipping '%v' - not matching include patterns", filePath)
		return false
	} else if hasIncludePatterns {
		skip = false
	}

	if len(provider.excludePatterns) > 0 && matches(filePathToCheck, provider.excludePatterns) && skip {
		provider.verboseLog("--- skipping '%v' - matching exclude patterns", filePath)
		return false
	}

	if provider.opts.TextFilesOnly && util.NotTextExt(filepath.Ext(filePathToCheck)) {
		provider.verboseLog("--- skipping '%v' - not a text file", filePath)
		return false
	}

	fileName := filepath.Base(filePath)

	if len(fileName) > 255 || len(filePath) > 4095 {
		log.Printf("--- skipping '%v' - file name is too long to snapshot", filePath)
		return false
	}

	return true
}

// shouldWriteFile checks if a file should be included in the snapshot based on filters.
// Returns (shouldWrite, file, error) - file is non-nil only when shouldWrite is true.
// This performs blob access for the size check, so use shouldIncludeFile when blob access is not needed.
func (provider *repositoryProvider) shouldWriteFile(repository *git.Repository, name string, entry *object.TreeEntry) (bool, *object.File, error) {
	if !provider.shouldIncludeFile(name, entry) {
		return false, nil, nil
	}

	blob, err := object.GetBlob(repository.Storer, entry.Hash)
	if err != nil {
		return false, nil, err
	}

	file := object.NewFile(name, entry.Mode, blob)

	if provider.opts.MaxFileSizeBytes > 0 && file.Size >= provider.opts.MaxFileSizeBytes {
		log.Printf("--- skipping '%v' - file size is too large to snapshot - %v", name, file.Size)
		return false, nil, nil
	}

	return true, file, nil
}

// ensureDir creates the directory if it hasn't been created already (cached).
func (provider *repositoryProvider) ensureDir(dirPath string) error {
	if provider.createdDirs[dirPath] {
		return nil
	}
	err := os.MkdirAll(dirPath, TARGET_PERMISSIONS)
	if err != nil {
		return err
	}
	provider.createdDirs[dirPath] = true
	return nil
}

// writeFile writes the file contents to the target path. Called only after shouldWriteFile returns true.
// Returns (written, error) - written is true only if the file was actually written to disk.
func (provider *repositoryProvider) writeFile(file *object.File, filePath string, outputPath string) (bool, error) {
	targetFilePath := filepath.Join(outputPath, filePath)
	targetDirectoryPath := filepath.Dir(targetFilePath)

	err := provider.ensureDir(targetDirectoryPath)
	if err != nil {
		if errors.Is(err, syscall.ENAMETOOLONG) {
			log.Printf("--- skipping '%v' - path component too long for filesystem (ENAMETOOLONG)", targetDirectoryPath)
			return false, nil // Skipped, not written
		}
		if errors.Is(err, syscall.EINVAL) {
			log.Printf("--- skipping '%v' - path contains invalid characters (EINVAL)", targetDirectoryPath)
			return false, nil // Skipped, not written
		}
		return false, fmt.Errorf("failed to create target directory at '%v': %v", targetDirectoryPath, err)
	}

	reader, err := file.Reader()
	if err != nil {
		return false, fmt.Errorf("failed to get git file reader for '%v': %v", filePath, err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			log.Printf("warning: failed to close git file reader for '%v': %v", filePath, closeErr)
		}
	}()

	f, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, TARGET_PERMISSIONS)
	if err != nil {
		if errors.Is(err, syscall.ENAMETOOLONG) {
			log.Printf("--- skipping '%v' - path component too long for filesystem (ENAMETOOLONG)", targetFilePath)
			return false, nil // Skipped, not written
		}
		if errors.Is(err, syscall.EINVAL) {
			log.Printf("--- skipping '%v' - path contains invalid characters (EINVAL)", targetFilePath)
			return false, nil // Skipped, not written
		}
		return false, fmt.Errorf("failed to create target file '%v': %v", targetFilePath, err)
	}
	_, err = io.Copy(f, reader)
	if err != nil {
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("warning: failed to close target file after write error '%v': %v", targetFilePath, closeErr)
		}
		return false, fmt.Errorf("failed to write file contents for '%v' to '%v': %v", filePath, targetFilePath, err)
	}
	if closeErr := f.Close(); closeErr != nil {
		return false, fmt.Errorf("failed to close target file '%v': %v", targetFilePath, closeErr)
	}

	provider.verboseLog("+++ '%v' to '%v'", filePath, targetFilePath)

	if provider.opts.CreateHashMarkers {
		targetHashFilePath := fmt.Sprintf("%v.hash", targetFilePath)
		err = os.WriteFile(targetHashFilePath, []byte(file.Hash.String()), TARGET_PERMISSIONS)
		if err != nil {
			log.Printf("failed to write hash file of '%v' to '%v': %v", filePath, targetFilePath, err)
		}
	}

	return true, nil // Successfully written
}

func isFileInList(provider *repositoryProvider, filePathToCheck string) bool {
	_, inFileList := provider.fileListToSnap[filePathToCheck]
	return inFileList || len(provider.fileListToSnap) == 0
}

func addEntryToIndexFile(indexFile *csv.Writer, name string, entry *object.TreeEntry) error {
	if indexFile != nil && utf8.ValidString(name) && !strings.ContainsAny(name, "\n\r") {
		record := []string{name, entry.Hash.String(), strconv.FormatBool(entry.Mode.IsFile())}
		err := indexFile.Write(record)
		if err != nil {
			return err
		}
	}
	return nil
}

// snapshot returns (totalCount, writtenCount, error)
// totalCount: all file entries in the tree (used for discrepancy detection)
// writtenCount: files that were actually written to disk (used for logging)
func (provider *repositoryProvider) snapshot(repository *git.Repository, commit *object.Commit, outputPath string, optionalIndexFilePath string, indexOnly bool, dryRun bool) (int, int, error) {

	tree, err := commit.Tree()
	if err != nil {
		return 0, 0, &util.ErrorWithCode{
			StatusCode:    util.ERROR_TREE_NOT_FOUND,
			InternalError: fmt.Errorf("failed to get tree of commit '%v': %v", commit.Hash, err),
		}
	}
	totalCount := 0
	writtenCount := 0

	treeWalker := object.NewTreeWalker(tree, true, nil)
	defer treeWalker.Close()

	var indexOutputFile *csv.Writer = nil
	if optionalIndexFilePath != "" && !dryRun {
		locIndexOutputFile, err := os.Create(optionalIndexFilePath)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create index file '%v': %v", optionalIndexFilePath, err)
		}

		csvWriter := csv.NewWriter(locIndexOutputFile)
		csvWriter.Comma = '\t'
		err = csvWriter.Write([]string{"Path", "BlobId", "IsFile"})
		if err != nil {
			return 0, 0, fmt.Errorf("failed to write file headers '%v': %v", optionalIndexFilePath, err)
		}

		defer func() {
			if closeErr := locIndexOutputFile.Close(); closeErr != nil {
				log.Printf("warning: failed to close index output file: %v", closeErr)
			}
		}()

		indexOutputFile = csvWriter
	}

	for {
		name, entry, walkErr := treeWalker.Next()
		if walkErr == io.EOF {
			if indexOutputFile != nil {
				indexOutputFile.Flush()
				if flushErr := indexOutputFile.Error(); flushErr != nil {
					return 0, 0, fmt.Errorf("failed to flush index file '%v': %v", optionalIndexFilePath, flushErr)
				}
			}
			return totalCount, writtenCount, nil
		}

		if walkErr != nil {
			return 0, 0, fmt.Errorf("failed to iterate files of %v: %v", commit.Hash, err)
		}

		// Count all entries (files + directories) for discrepancy detection
		totalCount++

		// For dry run, only count - skip all processing
		if dryRun {
			continue
		}

		// Process files: apply filters and write
		if entry.Mode.IsFile() {
			if indexOnly {
				// Fast path: skip blob access entirely, only apply cheap filters
				if !provider.shouldIncludeFile(name, &entry) {
					continue
				}
			} else {
				shouldWrite, file, err := provider.shouldWriteFile(repository, name, &entry)
				if err != nil {
					if errors.Is(err, plumbing.ErrObjectNotFound) {
						log.Printf("Can't get blob %s: %s (ignoring - possible partial clone)", name, err)
						continue
					} else if errors.Is(err, dotgit.ErrPackfileNotFound) {
						return 0, 0, &util.ErrorWithCode{
							StatusCode:    util.ERROR_BAD_CLONE_GIT,
							InternalError: err,
						}
					} else {
						return 0, 0, fmt.Errorf("failed to check file %s: %v", name, err)
					}
				}

				if !shouldWrite {
					continue
				}

				// Stats mode: calculate LOC and collect stats instead of writing files
				if provider.statsCollector != nil {
					if err := provider.processFileForStats(file, name); err != nil {
						provider.verboseLog("warning: failed to process stats for %s: %v", name, err)
					}
					continue
				}

				written, err := provider.writeFile(file, name, outputPath)
				if err != nil {
					return 0, 0, fmt.Errorf("failed to write file %s: %v", name, err)
				}
				if written {
					writtenCount++
				}
			}
		}

		// Add to index file (includes both files and directories)
		err = addEntryToIndexFile(indexOutputFile, name, &entry)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to write to index file for '%v': %v", name, err)
		}
	}
}

func (provider *repositoryProvider) isSymlink(filePath string, mode filemode.FileMode) bool {
	osMode, err := mode.ToOSFileMode()
	if err != nil {
		provider.verboseLog("failed to parse os file permissions for '%v': %v", filePath, err)
		return false
	}
	return osMode&os.ModeSymlink != 0
}

// processFileForStats processes a file for stats collection.
// Checks language extension, calculates LOC, and adds to stats collector.
func (provider *repositoryProvider) processFileForStats(file *object.File, name string) error {
	// Check if extension maps to a known language
	ext := filepath.Ext(name)
	language, found := stats.GetLanguageFromExtension(ext)
	if !found {
		provider.verboseLog("--- skipping '%v' for stats - unrecognized extension '%v'", name, ext)
		return nil
	}

	// Check file size (stats mode uses fixed 6MB limit like complexity tool)
	if file.Size > statsMaxFileSizeBytes {
		provider.verboseLog("--- skipping '%v' for stats - file too large (%v MB)", name, file.Size/(1024*1024))
		return nil
	}

	// Count lines of code
	linesOfCode, err := countLinesOfCode(&file.Blob, language)
	if err != nil {
		return err
	}

	provider.statsCollector.AddFile(language, linesOfCode, file.Size)

	provider.verboseLog("processed '%v': language=%v, loc=%v, size=%v", name, language, linesOfCode, file.Size)
	return nil
}

// writeStatsOutput writes the collected stats to the output path as JSON.
func (provider *repositoryProvider) writeStatsOutput(outputPath string) error {
	provider.statsCollector.Finalize()
	jsonData, err := json.MarshalIndent(provider.statsCollector, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats to JSON: %v", err)
	}

	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write stats to '%v': %v", outputPath, err)
	}

	log.Printf("stats written to '%v': %d files, %d MB total", outputPath, provider.statsCollector.TotalFileCount, provider.statsCollector.SnapshotSizeInMb)
	return nil
}
