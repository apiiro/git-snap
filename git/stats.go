package git

import (
	"fmt"
	"gitsnap/util"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/net/html/charset"
)

// statsMaxFileSizeBytes matches the complexity tool's default max file size (6 MB)
const statsMaxFileSizeBytes int64 = 6 * 1024 * 1024

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
