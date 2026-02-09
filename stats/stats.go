package stats

import "math"

// CodeStats represents the repository statistics output
type CodeStats struct {
	CountersByLanguage map[string]*LanguageStats `json:"countersByLanguage"`
	TotalFileCount     int                       `json:"totalFileCount"`
	SnapshotSizeInMb   int                       `json:"snapshotSizeInMb"`
}

// LanguageStats represents statistics for a specific language
type LanguageStats struct {
	NumberOfFiles int     `json:"numberOfFiles"`
	LinesOfCode   float64 `json:"linesOfCode"`
}

// NewCodeStats creates a new CodeStats instance with initialized maps
func NewCodeStats() *CodeStats {
	return &CodeStats{
		CountersByLanguage: make(map[string]*LanguageStats),
		TotalFileCount:     0,
		SnapshotSizeInMb:   0,
	}
}

// AddFile adds a file's stats to the appropriate language bucket
func (cs *CodeStats) AddFile(language string, linesOfCode int, sizeBytes int64) {
	cs.TotalFileCount++

	if _, exists := cs.CountersByLanguage[language]; !exists {
		cs.CountersByLanguage[language] = &LanguageStats{
			NumberOfFiles: 0,
			LinesOfCode:   0,
		}
	}

	cs.CountersByLanguage[language].NumberOfFiles++
	cs.CountersByLanguage[language].LinesOfCode += float64(linesOfCode)
}

// SetSnapshotSize sets the total snapshot size in MB (rounded to nearest integer)
func (cs *CodeStats) SetSnapshotSize(totalBytes int64) {
	megabytes := float64(totalBytes) / (1024 * 1024)
	cs.SnapshotSizeInMb = int(math.Round(megabytes))
}

