package stats

import "math"

// CodeStats represents the repository statistics output
type CodeStats struct {
	CountersByLanguage map[string]*LanguageStats `json:"countersByLanguage"`
	TotalFileCount     int                       `json:"totalFileCount"`
	SnapshotSizeInMb   int                       `json:"snapshotSizeInMb"`
	totalSizeBytes     int64
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

// AddFile adds a file's stats to the appropriate language bucket and accumulates total size
func (cs *CodeStats) AddFile(language string, linesOfCode int, sizeBytes int64) {
	cs.TotalFileCount++
	cs.totalSizeBytes += sizeBytes

	if _, exists := cs.CountersByLanguage[language]; !exists {
		cs.CountersByLanguage[language] = &LanguageStats{
			NumberOfFiles: 0,
			LinesOfCode:   0,
		}
	}

	cs.CountersByLanguage[language].NumberOfFiles++
	cs.CountersByLanguage[language].LinesOfCode += float64(linesOfCode)
}

// Finalize calculates derived fields (e.g. snapshot size in MB) from accumulated data
func (cs *CodeStats) Finalize() {
	megabytes := float64(cs.totalSizeBytes) / (1024 * 1024)
	cs.SnapshotSizeInMb = int(math.Round(megabytes))
}

