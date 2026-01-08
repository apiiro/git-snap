package git

import (
	"bufio"
	"fmt"
	"gitsnap/options"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type gitTestSuite struct {
	suite.Suite
	remote            string
	clonePath         string
	outputPath        string
	filteredClonePath string
}

func TestGitTestSuite(t *testing.T) {
	suite.Run(t, new(gitTestSuite))
}

func cloneLocal(remote string, filter string) (clonePath string) {
	var err error
	clonePath, err = os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}

	var proc *exec.Cmd
	if len(filter) > 0 {
		proc = exec.Command("git", "clone", "--no-checkout", filter, remote, clonePath)
	} else {
		proc = exec.Command("git", "clone", "--no-checkout", remote, clonePath)
	}
	err = proc.Start()
	if err != nil {
		panic(err)
	}
	err = proc.Wait()
	if err != nil {
		panic(err)
	}
	return
}

func (gitSuite *gitTestSuite) SetupTest() {
	gitSuite.remote = "https://github.com/apiirolab/dc-heacth.git"
	gitSuite.clonePath = cloneLocal(gitSuite.remote, "")
	gitSuite.filteredClonePath = cloneLocal(gitSuite.remote, "--filter=blob:limit=1k")
	var err error
	gitSuite.outputPath, err = os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
}

func (gitSuite *gitTestSuite) TearDownTest() {
	err := os.RemoveAll(gitSuite.clonePath)
	if err != nil {
		panic(err)
	}
	err = os.RemoveAll(gitSuite.filteredClonePath)
	if err != nil {
		panic(err)
	}
}

func (gitSuite *gitTestSuite) verifyOutputPathAux(
	expectedDirCount int,
	expectedFileCount int,
	expectedMinFileSize int,
	expectedMaxFileSize int,
	outputPath string,
) {
	fileCount, dirCount := 0, 0
	minFileSize, maxFileSize := int64(6*1024*1024), int64(0)
	err := filepath.Walk(outputPath, func(path string, info fs.FileInfo, err error) error {
		gitSuite.NotNil(info, "missing info for %v", path)
		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
			fileSize := info.Size()
			gitSuite.True(fileSize >= 0, "file at %v has invalid size", path)
			gitSuite.True(info.Mode().IsRegular())
			content, err := os.ReadFile(path)
			gitSuite.Nil(err, "failed to read file at %v", path)
			fileSizeFromRead := int64(len(content))
			gitSuite.EqualValues(fileSize, fileSizeFromRead, "read different file size for %v", path)
			if fileSizeFromRead > maxFileSize {
				maxFileSize = fileSizeFromRead
			}
			if fileSizeFromRead < minFileSize {
				minFileSize = fileSizeFromRead
			}
		}
		return nil
	})
	gitSuite.Require().Nil(err)
	gitSuite.Require().EqualValues(expectedDirCount, dirCount, "unexpected dirs count")
	gitSuite.Require().EqualValues(expectedFileCount, fileCount, "unexpected files count")
	gitSuite.Require().EqualValues(expectedMinFileSize, minFileSize, "unexpected min file size")
	gitSuite.Require().EqualValues(expectedMaxFileSize, maxFileSize, "unexpected max file size")
}

func (gitSuite *gitTestSuite) verifyOutputPath(
	expectedDirCount int,
	expectedFileCount int,
	expectedMinFileSize int,
	expectedMaxFileSize int,
) {
	gitSuite.verifyOutputPathAux(expectedDirCount, expectedFileCount, expectedMinFileSize, expectedMaxFileSize, gitSuite.outputPath)
}

func (gitSuite *gitTestSuite) verifyIndexFile(
	expectedFileCount int,
	indexFile string,
) {
	file, err := os.Open(indexFile)
	gitSuite.Require().Nil(err)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			gitSuite.T().Logf("warning: failed to close index file: %v", closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	fileCount := 0
	i := 0
	for scanner.Scan() {
		i++
		lineText := scanner.Text()
		fields := strings.Split(lineText, "\t")

		if i == 1 {
			if len(fields) != 3 || fields[0] != "Path" || fields[1] != "BlobId" || fields[2] != "IsFile" {
				gitSuite.Require().Fail("Index file header line is incorrect", lineText)
				return
			}

			continue
		}

		fileStat, err := os.Stat(gitSuite.outputPath + "/" + fields[0])

		if err == nil {
			gitSuite.Require().Equal(fileStat.IsDir(), fields[2] != "true")
		}

		fileCount++
	}

	gitSuite.Require().Nil(err)
	gitSuite.Require().EqualValues(expectedFileCount, fileCount, "unexpected files count")
}

func (gitSuite *gitTestSuite) TestSnapshotForRegularCommit() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 181,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotForRegularCommitOnTextFilesOnly() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     true,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 181,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotForRegularCommitOnTextFilesOnlyWithExclude() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{"**/*.html"},
		VerboseLogging:    true,
		TextFilesOnly:     true,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 180,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotForRegularCommitWithSkipDoubleCheck() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
		SkipDoubleCheck:   true,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 181,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotNonExistingRevision() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "wat",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.NotNil(err)
}

func (gitSuite *gitTestSuite) TestSnapshotForShortSha() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca7420",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 181,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotForMainBranchName() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "master",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		30, 185,
		7, 47814,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotForBranchName() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "remotes/origin/lfx",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 181,
		215, 47582,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithIncludePattern() {
	err := Snapshot(&options.Options{
		ClonePath:  gitSuite.clonePath,
		Revision:   "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath: gitSuite.outputPath,
		IncludePatterns: []string{
			"**/*.java",
		},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		20, 167,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithIncludePatterns() {
	err := Snapshot(&options.Options{
		ClonePath:  gitSuite.clonePath,
		Revision:   "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath: gitSuite.outputPath,
		IncludePatterns: []string{
			"**/*.java",
			"pom.xml",
		},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		20, 168,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithExcludePattern() {
	err := Snapshot(&options.Options{
		ClonePath:       gitSuite.clonePath,
		Revision:        "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:      gitSuite.outputPath,
		IncludePatterns: []string{},
		ExcludePatterns: []string{
			"**/*.java",
		},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		11, 14,
		416, 18821,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithExcludePatterns() {
	err := Snapshot(&options.Options{
		ClonePath:       gitSuite.clonePath,
		Revision:        "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:      gitSuite.outputPath,
		IncludePatterns: []string{},
		ExcludePatterns: []string{
			"*.java",
			"*/pom.xml",
		},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		11, 13,
		416, 13270,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithAllPatterns() {
	err := Snapshot(&options.Options{
		ClonePath:  gitSuite.clonePath,
		Revision:   "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath: gitSuite.outputPath,
		IncludePatterns: []string{
			"**/*.java",
		},
		ExcludePatterns: []string{
			"**/VO/**",
		},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: false,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		20, 167,
		215, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithMarkers() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: true,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		28, 181*2,
		40, 47804,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotWithMissingBlobs() {
	err := Snapshot(&options.Options{
		ClonePath:         gitSuite.filteredClonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		VerboseLogging:    true,
		TextFilesOnly:     false,
		CreateHashMarkers: true,
		MaxFileSizeBytes:  6 * 1024 * 1024,
	})
	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(
		18, 88,
		40, 1020,
	)
}

func (gitSuite *gitTestSuite) TestSnapshotForRegularCommitMultipleTimesAndConcurrently() {
	N := 10
	for i := 0; i < N; i++ {
		barrier := make(chan interface{}, N)
		for j := 0; j < N; j++ {
			innerJ := j
			go func() {
				outputPath := filepath.Join(gitSuite.outputPath, fmt.Sprintf("%v_%v", i, innerJ))
				err := Snapshot(&options.Options{
					ClonePath:         gitSuite.clonePath,
					Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
					OutputPath:        outputPath,
					IncludePatterns:   []string{},
					ExcludePatterns:   []string{},
					VerboseLogging:    false,
					TextFilesOnly:     false,
					CreateHashMarkers: false,
					MaxFileSizeBytes:  6 * 1024 * 1024,
				})
				gitSuite.Nil(err)
				gitSuite.verifyOutputPathAux(
					28, 181,
					215, 47804,
					outputPath,
				)
				barrier <- nil
			}()
		}
		for j := 0; j < N; j++ {
			<-barrier
		}
	}
}

func (gitSuite *gitTestSuite) TestSnapshotWithInvalidIndexPath() {
	err := Snapshot(&options.Options{
		ClonePath:             gitSuite.filteredClonePath,
		Revision:              "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:            gitSuite.outputPath,
		IncludePatterns:       []string{},
		ExcludePatterns:       []string{},
		VerboseLogging:        true,
		TextFilesOnly:         false,
		CreateHashMarkers:     true,
		MaxFileSizeBytes:      6 * 1024 * 1024,
		OptionalIndexFilePath: "non/existing/file/path/index.csv",
	})
	gitSuite.NotNil(err)
}

func (gitSuite *gitTestSuite) TestSnapshotWithIndexPath() {
	indexFilePath := gitSuite.outputPath + "/" + "__index.csv"

	err := Snapshot(&options.Options{
		ClonePath:       gitSuite.clonePath,
		Revision:        "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:      gitSuite.outputPath,
		IncludePatterns: []string{},
		ExcludePatterns: []string{
			"*.java",
			"*/pom.xml",
		},
		VerboseLogging:        true,
		TextFilesOnly:         false,
		CreateHashMarkers:     false,
		MaxFileSizeBytes:      6 * 1024 * 1024,
		OptionalIndexFilePath: indexFilePath,
	})

	gitSuite.Nil(err)
	gitSuite.verifyIndexFile(40, indexFilePath)
}

func (gitSuite *gitTestSuite) TestIndexFileDoesNotContainNewlines() {
	indexFilePath := gitSuite.outputPath + "/" + "__index_newline_test.csv"

	err := Snapshot(&options.Options{
		ClonePath:             gitSuite.clonePath,
		Revision:              "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:            gitSuite.outputPath,
		OptionalIndexFilePath: indexFilePath,
	})

	gitSuite.Nil(err)

	file, err := os.Open(indexFilePath)
	gitSuite.Require().Nil(err)

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := strings.Split(scanner.Text(), "\t")
		gitSuite.Require().Equal(3, len(fields), "Line %d should have exactly 3 fields", lineNumber)
		if lineNumber > 1 {
			gitSuite.Require().False(strings.ContainsAny(fields[0], "\n\r"), "Path contains newline on line %d", lineNumber)
		}
	}
	if closeErr := file.Close(); closeErr != nil {
		gitSuite.T().Logf("warning: failed to close paths file: %v", closeErr)
	}
	gitSuite.Require().Nil(scanner.Err())
}

func (gitSuite *gitTestSuite) TestSnapshotWithPathsFile() {
	filesDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}

	filePath := filepath.Join(filesDir, "file.txt")

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

	if err != nil {
		gitSuite.Nil(err)
		return
	}

	data := []byte("src/main/java/com/dchealth/VO/DataElementFormat.java")
	_, err = file.Write(data)

	if err != nil {
		err = fmt.Errorf("Failed to write paths file: '%v'", err)
		fmt.Println(err)
		gitSuite.Nil(err)
		return
	}

	if closeErr := file.Close(); closeErr != nil {
		gitSuite.T().Logf("warning: failed to close paths file: %v", closeErr)
	}

	err = Snapshot(&options.Options{
		ClonePath:         gitSuite.clonePath,
		Revision:          "2ca742044ba451d00c6854a465fdd4280d9ad1f5",
		OutputPath:        gitSuite.outputPath,
		IncludePatterns:   []string{},
		ExcludePatterns:   []string{},
		PathsFileLocation: filePath,
	})

	gitSuite.Nil(err)
	gitSuite.verifyOutputPath(7, 1, 1696, 1696)
}