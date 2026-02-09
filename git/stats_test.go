package git

import (
	"encoding/json"
	"gitsnap/options"
	"gitsnap/stats"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type statsTestSuite struct {
	suite.Suite
	remote     string
	clonePath  string
	outputPath string
}

func TestStatsTestSuite(t *testing.T) {
	suite.Run(t, new(statsTestSuite))
}

func (s *statsTestSuite) SetupTest() {
	s.remote = "https://github.com/apiirolab/dc-heacth.git"
	s.clonePath = cloneLocal(s.remote, "")
	var err error
	s.outputPath, err = os.MkdirTemp("", "stats-output-*.json")
	if err != nil {
		panic(err)
	}
}

func (s *statsTestSuite) TearDownTest() {
	err := os.RemoveAll(s.clonePath)
	if err != nil {
		panic(err)
	}
	// Clean up output file if it exists
	_ = os.Remove(s.outputPath)
}

func (s *statsTestSuite) TestStatsForRegularCommit() {
	outputFile := filepath.Join(os.TempDir(), "stats-test.json")
	defer func() { _ = os.Remove(outputFile) }()

	err := Snapshot(&options.Options{
		ClonePath:      s.clonePath,
		Revision:       "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:     outputFile,
		StatsOnly:      true,
		VerboseLogging: true,
	})
	s.Require().Nil(err)

	// Read and verify the output
	data, err := os.ReadFile(outputFile)
	s.Require().Nil(err)

	var result stats.CodeStats
	err = json.Unmarshal(data, &result)
	s.Require().Nil(err)

	// The repo is primarily Java
	s.Require().Contains(result.CountersByLanguage, "java", "should contain Java files")

	// Verify Java stats
	javaStats := result.CountersByLanguage["java"]
	s.Require().NotNil(javaStats)
	s.Require().Greater(javaStats.NumberOfFiles, 0, "should have Java files")
	s.Require().Greater(javaStats.LinesOfCode, float64(0), "should have lines of code")

	// Verify total file count and snapshot size
	s.Require().Greater(result.TotalFileCount, 0, "should have files")
	s.Require().Greater(result.SnapshotSizeInMb, 0, "should have non-zero snapshot size")
}

func (s *statsTestSuite) TestStatsForMasterBranch() {
	outputFile := filepath.Join(os.TempDir(), "stats-test-master.json")
	defer func() { _ = os.Remove(outputFile) }()

	err := Snapshot(&options.Options{
		ClonePath:      s.clonePath,
		Revision:       "master",
		OutputPath:     outputFile,
		StatsOnly:      true,
		VerboseLogging: false,
	})
	s.Require().Nil(err)

	data, err := os.ReadFile(outputFile)
	s.Require().Nil(err)

	var result stats.CodeStats
	err = json.Unmarshal(data, &result)
	s.Require().Nil(err)

	s.Require().Contains(result.CountersByLanguage, "java")
	s.Require().Greater(result.TotalFileCount, 0)
	s.Require().Greater(result.SnapshotSizeInMb, 0, "should have non-zero snapshot size")
}

func (s *statsTestSuite) TestStatsForNonExistentRevision() {
	outputFile := filepath.Join(os.TempDir(), "stats-test-invalid.json")
	defer func() { _ = os.Remove(outputFile) }()

	err := Snapshot(&options.Options{
		ClonePath:      s.clonePath,
		Revision:       "nonexistent-revision",
		OutputPath:     outputFile,
		StatsOnly:      true,
		VerboseLogging: false,
	})
	s.Require().NotNil(err, "should fail for non-existent revision")
}

func (s *statsTestSuite) TestStatsOutputFormat() {
	outputFile := filepath.Join(os.TempDir(), "stats-test-format.json")
	defer func() { _ = os.Remove(outputFile) }()

	err := Snapshot(&options.Options{
		ClonePath:      s.clonePath,
		Revision:       "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:     outputFile,
		StatsOnly:      true,
		VerboseLogging: false,
	})
	s.Require().Nil(err)

	data, err := os.ReadFile(outputFile)
	s.Require().Nil(err)

	var result stats.CodeStats
	err = json.Unmarshal(data, &result)
	s.Require().Nil(err)

	// Verify structure matches expected format
	s.Require().NotNil(result.CountersByLanguage, "countersByLanguage should not be nil")
	s.Require().GreaterOrEqual(result.TotalFileCount, 0, "totalFileCount should be >= 0")
	s.Require().GreaterOrEqual(result.SnapshotSizeInMb, 0, "snapshotSizeInMb should be >= 0")

	// For each language, verify the structure
	for lang, langStats := range result.CountersByLanguage {
		s.Require().NotNil(langStats, "language stats for %s should not be nil", lang)
		s.Require().GreaterOrEqual(langStats.NumberOfFiles, 1, "numberOfFiles for %s should be >= 1", lang)
		s.Require().GreaterOrEqual(langStats.LinesOfCode, float64(0), "linesOfCode for %s should be >= 0", lang)
	}
}
