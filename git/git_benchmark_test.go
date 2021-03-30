// +build bench

package git

import (
	"fmt"
	"gitsnap/options"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func timed(op func()) (elapsedSeconds float64) {
	start := time.Now()
	op()
	return time.Since(start).Seconds()
}

func gitArchiveArgs(clonePath string, commitish string, outputFilePath string) []string {
	args := []string{
		"archive", commitish, "--format=tar",
	}
	if outputFilePath != "" {
		args = append(args, fmt.Sprintf("--output=%v", outputFilePath))
	}
	return args
}

func withTempDir(op func(string)) {
	dirPath, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	os.MkdirAll(dirPath, 0777)
	defer os.RemoveAll(dirPath)
	op(dirPath)
}

func runCommand(workingDirectory string, name string, arg ...string) {
	executable, err := exec.LookPath(name)
	if err != nil {
		panic(err)
	}
	cmd := &exec.Cmd{
		Path: executable,
		Args: append([]string{name}, arg...),
		Dir:  workingDirectory,
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("command [%v] failed to execute: %v\nS%v", cmd.Args, err, string(output))
	}
}

func gitArchive(clonePath string, commitish string) {
	log.Printf("> Running archive")
	withTempDir(func(outputPath string) {
		runCommand(clonePath, "git", gitArchiveArgs(clonePath, commitish, filepath.Join(outputPath, "out.tar"))...)
	})
}

func gitArchiveAndDecompress(clonePath string, commitish string) {
	log.Printf("> Running archive & decompress")
	withTempDir(func(outputPath string) {
		runCommand(clonePath, "bash", "-c", fmt.Sprintf("git %v | tar -x -C %v", strings.Join(gitArchiveArgs(clonePath, commitish, ""), " "), outputPath))
	})
}

func gitWorktreeCheckout(clonePath string, commitish string) {
	log.Printf("> Running worktree checkout")
	withTempDir(func(outputPath string) {
		runCommand(clonePath, "git", "--work-tree", outputPath, "checkout", commitish, "-f", "-q", "--", "./")
	})
}

func benchmark(remote string) {
	clonePath := cloneLocal(remote)

	archiveSec := timed(func() {
		gitArchive(clonePath, "master")
	})
	archiveAndDecompressSec := timed(func() {
		gitArchiveAndDecompress(clonePath, "master")
	})
	worktreeSec := timed(func() {
		gitWorktreeCheckout(clonePath, "master")
	})

	snapshotSec := timed(func() {
		withTempDir(func(outputPath string) {
			log.Printf("> Running snapshot")
			err := Snapshot(&options.Options{
				ClonePath:       clonePath,
				Revision:        "master",
				OutputPath:      outputPath,
				IncludePatterns: []string{},
				ExcludePatterns: []string{},
				SupportShortSha: false,
			})
			if err != nil {
				panic(err)
			}
		})
	})

	snapshotWithPatternSec := timed(func() {
		withTempDir(func(outputPath string) {
			log.Printf("> Running snapshot")
			err := Snapshot(&options.Options{
				ClonePath:          clonePath,
				Revision:           "master",
				OutputPath:         outputPath,
				IncludePatterns:    []string{"*.java"},
				ExcludePatterns:    []string{},
				VerboseLogging:     false,
				SupportShortSha:    true,
				TextFilesOnly:      false,
				CreateHashMarkers:  false,
				IgnoreCasePatterns: false,
				MaxFileSizeBytes:   0,
			})
			if err != nil {
				panic(err)
			}
		})
	})

	log.Printf("Benchmark results:\nArchive: %v sec\nArchive & Decompress: %v sec\nWorktree: %v sec\nSnapshot: %v sec\nSnapshot w Pattern: %v sec", archiveSec, archiveAndDecompressSec, worktreeSec, snapshotSec, snapshotWithPatternSec)
}

func TestBenchmark(t *testing.T) {
	remotes := []string{
		"https://github.com/apiirolab/EVO-Exchange-BE-2019",
		"https://github.com/apiirolab/elasticsearch.git",
	}
	for _, remote := range remotes {
		log.Printf("Benchmarking %v -->", remote)
		benchmark(remote)
	}
}
