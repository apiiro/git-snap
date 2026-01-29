package git

import (
	"encoding/json"
	"fmt"
	"gitsnap/options"
	"gitsnap/stats"
	"gitsnap/util"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gobwas/glob"
	"golang.org/x/net/html/charset"
)

// maxFileSizeBytes matches the complexity tool's default max file size (6 MB)
const maxFileSizeBytes = 6 * 1024 * 1024

// complexityToolExcludePatterns matches the complexity tool's default excludes
var complexityToolExcludePatterns = []string{
	"**/bin/**",
	"**/obj/**",
	"**/venv/**",
	"**/node_modules/**",
	"**/.idea/**",
	"**/.git/**",
	"**/site-packages/**",
	"**/vendor/**",
	"**/test_resources/**",
	"**/tests/**",
	"**/testing/**",
	"**/resources/**",
	"**/testdata/**",
	"**/simulation/**",
	"**/simulator/**",
	"**/automation/**",
}

// complexityToolExcludeSuffixes are file patterns to exclude (from complexity tool)
var complexityToolExcludeSuffixes = []string{
	"**/*test_resources.*",
	"**/*tests.*",
	"**/*spec.*",
	"**/*.min.js",
	"**/*.min.css",
	"**/*.bundle.js",
}

// getStatsExcludePatterns returns the combined exclusion patterns:
// git-snap's noisy directories + complexity tool's patterns
func getStatsExcludePatterns() []string {
	// Start with git-snap's noisy directory patterns
	patterns := util.NoisyDirectoryExclusionPatterns()

	// Add complexity tool patterns (some may overlap, but that's fine)
	patterns = append(patterns, complexityToolExcludePatterns...)
	patterns = append(patterns, complexityToolExcludeSuffixes...)

	return patterns
}

// Stats calculates repository statistics (LOC, file count, size per language)
// and outputs them as JSON to the specified output path.
// By default, uses the same exclusion patterns as the complexity tool.
// Use --stats-no-filter to skip all exclusions.
func Stats(opts *options.Options) error {
	repository, err := git.PlainOpen(opts.ClonePath)
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_CLONE_GIT,
			InternalError: err,
		}
	}

	hash, err := repository.ResolveRevision(plumbing.Revision(opts.Revision))
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_NO_REVISION,
			InternalError: fmt.Errorf("failed to get revision '%v': %v", opts.Revision, err),
		}
	}

	commit, err := repository.CommitObject(*hash)
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_NO_REVISION,
			InternalError: fmt.Errorf("failed to get commit for '%v': %v", opts.Revision, err),
		}
	}

	log.Printf("calculating stats for commit '%v' for revision '%v' at clone '%v'", commit.ID(), opts.Revision, opts.ClonePath)

	// Compile exclude patterns (use git-snap + complexity tool defaults unless --stats-no-filter is set)
	var excludePatterns []glob.Glob
	if !opts.StatsNoFilter {
		allPatterns := getStatsExcludePatterns()
		excludePatterns, err = compileStatsGlobs(allPatterns)
		if err != nil {
			return fmt.Errorf("failed to compile exclude patterns: %v", err)
		}
		if opts.VerboseLogging {
			log.Printf("%d exclude patterns (git-snap + complexity tool defaults)", len(allPatterns))
		}
	} else {
		if opts.VerboseLogging {
			log.Printf("--stats-no-filter: skipping all exclusion filters")
		}
	}

	tree, err := commit.Tree()
	if err != nil {
		return &util.ErrorWithCode{
			StatusCode:    util.ERROR_TREE_NOT_FOUND,
			InternalError: fmt.Errorf("failed to get tree of commit '%v': %v", commit.Hash, err),
		}
	}

	codeStats := stats.NewCodeStats()
	var totalSizeBytes int64

	treeWalker := object.NewTreeWalker(tree, true, nil)
	defer treeWalker.Close()

	for {
		name, entry, walkErr := treeWalker.Next()
		if walkErr == io.EOF {
			break
		}
		if walkErr != nil {
			return fmt.Errorf("failed to iterate files of %v: %v", commit.Hash, walkErr)
		}

		// 1. Only process files (skip directories) - no blob needed
		if !entry.Mode.IsFile() {
			continue
		}

		// 2. Check exclude patterns BEFORE getting blob (same order as snapshot)
		if !opts.StatsNoFilter && matchesGlob(name, excludePatterns) {
			if opts.VerboseLogging {
				log.Printf("skipping '%v' - excluded by patterns", name)
			}
			continue
		}

		// 3. Check extension BEFORE getting blob (same as snapshot's TextFilesOnly check)
		ext := filepath.Ext(name)
		language, found := stats.GetLanguageFromExtension(ext)
		if !found {
			// Skip files with unrecognized extensions
			if opts.VerboseLogging {
				log.Printf("skipping '%v' - unrecognized extension '%v'", name, ext)
			}
			continue
		}

		// 4. NOW get blob (only for files that passed pattern and extension checks)
		blob, err := object.GetBlob(repository.Storer, entry.Hash)
		if err != nil {
			if opts.VerboseLogging {
				log.Printf("warning: can't get blob %s: %s (skipping)", name, err)
			}
			continue
		}

		fileSize := blob.Size
		totalSizeBytes += fileSize

		// 5. Check file size (same as complexity tool default: 6 MB)
		if !opts.StatsNoFilter && fileSize > maxFileSizeBytes {
			if opts.VerboseLogging {
				log.Printf("skipping '%v' - file too large (%v MB)", name, fileSize/(1024*1024))
			}
			continue
		}

		// 6. Count lines of code (using same logic as complexity tool)
		linesOfCode, err := countLinesOfCode(blob, language)
		if err != nil {
			if opts.VerboseLogging {
				log.Printf("warning: failed to count lines for %s: %v (using 0)", name, err)
			}
			linesOfCode = 0
		}

		codeStats.AddFile(language, linesOfCode, fileSize)

		if opts.VerboseLogging {
			log.Printf("processed '%v': language=%v, loc=%v, size=%v", name, language, linesOfCode, fileSize)
		}
	}

	codeStats.SetSnapshotSize(totalSizeBytes)

	// Write JSON output
	jsonData, err := json.MarshalIndent(codeStats, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats to JSON: %v", err)
	}

	err = os.WriteFile(opts.OutputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write stats to '%v': %v", opts.OutputPath, err)
	}

	log.Printf("stats written to '%v': %d files, %d MB total", opts.OutputPath, codeStats.TotalFileCount, codeStats.SnapshotSizeInMb)
	return nil
}

// compileStatsGlobs compiles glob patterns for stats exclusion
func compileStatsGlobs(patterns []string) ([]glob.Glob, error) {
	patterns = expandStatsPatterns(patterns)
	globs := make([]glob.Glob, len(patterns))
	for i, pattern := range patterns {
		compiled, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern '%v': %v", pattern, err)
		}
		globs[i] = compiled
	}
	return globs, nil
}

// expandStatsPatterns expands patterns that start with */ or **/ to also match without prefix
func expandStatsPatterns(patterns []string) []string {
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

// matchesGlob checks if a file path matches any of the glob patterns
func matchesGlob(filePath string, patterns []glob.Glob) bool {
	for _, pattern := range patterns {
		if pattern.Match(filePath) {
			return true
		}
	}
	return false
}

// countLinesOfCode counts lines of code using the same logic as the complexity tool:
// - Auto-detects character encoding (same as complexity tool)
// - Normalizes line endings (\r\n -> \n)
// - Skips blank lines
// - Skips single-line comments (// and #)
// - Skips multi-line comment blocks (/* */, python """, ruby =begin/=end)
func countLinesOfCode(blob *object.Blob, language string) (int, error) {
	reader, err := blob.Reader()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = reader.Close()
	}()

	// Read blob content
	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}

	// Auto-detect encoding and decode (same as complexity tool)
	// Complexity tool returns error on decode failure, so we do the same
	encoding, _, _ := charset.DetermineEncoding(contentBytes, "")
	decodedBytes, err := encoding.NewDecoder().Bytes(contentBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to decode file: %v", err)
	}

	// Normalize line endings and split into lines
	content := strings.ReplaceAll(string(decodedBytes), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	linesOfCode := 0
	expectEndingComment := ""

	for _, line := range lines {
		// Handle multi-line comment continuation
		if len(expectEndingComment) > 0 {
			endCommentIndex := strings.Index(line, expectEndingComment)
			if endCommentIndex == -1 {
				// Still in comment block
				continue
			}
			// Comment block ended on this line
			line = strings.TrimSpace(line[endCommentIndex+len(expectEndingComment):])
			expectEndingComment = ""
		}

		cleanLine := strings.TrimSpace(line)

		// Skip blank lines
		if len(cleanLine) == 0 {
			continue
		}

		// Skip single-line comments
		if strings.HasPrefix(cleanLine, "//") || strings.HasPrefix(cleanLine, "#") {
			continue
		}

		// Check for multi-line comment start
		const pythonMultilineString = "\"\"\""
		postCommentLine := ""

		if isStartOfMultiLineComment(cleanLine) {
			expectEndingComment = "*/"
			commentIndex := strings.Index(cleanLine, "/*")
			postCommentLine = strings.TrimSpace(cleanLine[2+commentIndex:])
			cleanLine = strings.TrimSpace(cleanLine[:commentIndex])
		} else if language == "python" && strings.Contains(cleanLine, pythonMultilineString) {
			expectEndingComment = pythonMultilineString
			commentIndex := strings.Index(cleanLine, pythonMultilineString)
			postCommentLine = strings.TrimSpace(cleanLine[len(pythonMultilineString)+commentIndex:])
			cleanLine = strings.TrimSpace(cleanLine[:commentIndex])
		} else if language == "ruby" && strings.HasPrefix(cleanLine, "=begin") {
			expectEndingComment = "=end"
			continue
		} else if language == "ruby" && strings.HasPrefix(cleanLine, "<<-DOC") {
			expectEndingComment = "DOC"
			continue
		}

		// Check if comment ends on the same line
		if len(postCommentLine) > 0 {
			endCommentIndex := strings.Index(postCommentLine, expectEndingComment)
			if endCommentIndex == -1 {
				// Comment continues to next line
				if len(cleanLine) == 0 {
					continue
				}
			} else {
				// Comment block ended on this line
				expectEndingComment = ""
			}
		}

		// Skip if line is now empty after removing comment start
		if len(cleanLine) == 0 {
			continue
		}

		linesOfCode++
	}

	return linesOfCode, nil
}

// isStartOfMultiLineComment checks if a line starts a /* */ comment block
// (same logic as complexity tool)
func isStartOfMultiLineComment(cleanLine string) bool {
	_, after, found := strings.Cut(cleanLine, "/*")

	if !found {
		return false
	}

	if len(after) > 1 {
		nextChar := after[0:1]
		// Avoid false positives for regex patterns like /*. or strings containing /*
		if strings.ContainsAny(nextChar, "'\".") {
			return false
		}
	}
	return true
}
