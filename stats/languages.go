package stats

import "strings"

// Language type alias for readability
type Language = string

var languageToExtensions = map[Language][]string{
	"java":       {"java"},
	"csharp":     {"cs", "cshtml"},
	"node":       {"js", "jsx", "ts", "tsx"},
	"python":     {"py", "py3", "py2"},
	"kotlin":     {"kt", "kts", "ktm"},
	"c":          {"c", "h"},
	"cpp":        {"cpp", "cxx", "cc", "hpp", "hh", "txx", "tpp"},
	"objectivec": {"m", "mm"},
	"swift":      {"swift"},
	"ruby":       {"rb"},
	"go":         {"go"},
	"rust":       {"rs"},
	"scala":      {"scala", "sc"},
	"php":        {"php", "phtml", "php3", "php4", "php5", "php7", "phps", "pht", "phar"},
	"fortran":    {"f", "for", "f77", "f90", "f95", "f03", "f03p", "f08", "f08p", "f15", "f20", "f18", "f2k", "f2003", "f2008", "f2015", "f2018", "fpp", "ftn", "f05", "F", "FOR", "F77", "F90", "F95", "F03", "F08", "F15", "F18", "F2K", "F2003", "F2015", "F2008", "F2018", "FPP", "FTN"},
}

var extensionToLanguage = make(map[string]Language)

func init() {
	for language, extensions := range languageToExtensions {
		for _, extension := range extensions {
			extensionToLanguage[extension] = language
		}
	}
}

// GetLanguageFromExtension returns the language for a given file extension.
// The extension should include the leading dot (e.g., ".java").
// Returns the language name and true if found, or empty string and false if not recognized.
func GetLanguageFromExtension(ext string) (Language, bool) {
	if len(ext) == 0 {
		return "", false
	}
	// Remove the leading dot if present
	ext = strings.TrimPrefix(ext, ".")
	language, found := extensionToLanguage[ext]
	return language, found
}

