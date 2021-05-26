package git

import (
	"github.com/stretchr/testify/suite"
	"gitsnap/options"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type gitTestSuite struct {
	suite.Suite
	remote     string
	clonePath  string
	outputPath string
}

func TestGitTestSuite(t *testing.T) {
	suite.Run(t, new(gitTestSuite))
}

func cloneLocal(remote string) (clonePath string) {
	var err error
	clonePath, err = ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	proc := exec.Command("git", "clone", "--no-checkout", remote, clonePath)
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
	clonePath := cloneLocal(gitSuite.remote)
	gitSuite.clonePath = clonePath
	var err error
	gitSuite.outputPath, err = ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
}

func (gitSuite *gitTestSuite) TearDownTest() {
	err := os.RemoveAll(gitSuite.clonePath)
	if err != nil {
		panic(err)
	}
}

func (gitSuite *gitTestSuite) verifyOutputPath(
	expectedDirCount int,
	expectedFileCount int,
	expectedMinFileSize int,
	expectedMaxFileSize int,
) {
	fileCount, dirCount := 0, 0
	minFileSize, maxFileSize := 6*1024*1024, 0
	err := filepath.Walk(gitSuite.outputPath, func(path string, info fs.FileInfo, err error) error {
		gitSuite.NotNil(info, "missing info for %v", path)
		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
			gitSuite.True(info.Size() >= 0, "file at %v has invalid size", path)
			gitSuite.True(info.Mode().IsRegular())
			content, err := ioutil.ReadFile(path)
			gitSuite.Nil(err, "failed to read file at %v", path)
			fileSizeFromRead := len(content)
			gitSuite.EqualValues(info.Size(), fileSizeFromRead, "read different file size for %v", path)
			if fileSizeFromRead > maxFileSize {
				maxFileSize = fileSizeFromRead
			}
			if fileSizeFromRead < minFileSize {
				minFileSize = fileSizeFromRead
			}
		}
		return nil
	})
	gitSuite.Nil(err)
	gitSuite.EqualValues(expectedDirCount, dirCount, "unexpected dirs count")
	gitSuite.EqualValues(expectedFileCount, fileCount, "unexpected files count")
	gitSuite.EqualValues(expectedMinFileSize, minFileSize, "unexpected min file size")
	gitSuite.EqualValues(expectedMaxFileSize, maxFileSize, "unexpected max file size")
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
		28, 183,
		215, 47814,
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
		19, 102,
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
		28, 181 * 2,
		40, 47804,
	)
}
