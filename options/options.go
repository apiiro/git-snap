package options

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"gitsnap/util"
	"os"
	"path"
	"strings"
)

var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:     "src",
		Aliases:  []string{"s"},
		Usage:    "path to existing git clone as source directory, may contain no more than .git directory, current git state doesn't affect the command",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "rev",
		Aliases:  []string{"r"},
		Usage:    "commit-ish Revision",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "out",
		Aliases:  []string{"o"},
		Usage:    "output directory. will be created if does not exist",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "include",
		Aliases:  []string{"i"},
		Value:    "",
		Usage:    "patterns of file paths to include, comma delimited, may contain any glob pattern",
		Required: false,
	},
	&cli.StringFlag{
		Name:     "exclude",
		Aliases:  []string{"e"},
		Value:    "",
		Usage:    "patterns of file paths to exclude, comma delimited, may contain any glob pattern",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "verbose",
		Aliases:  []string{"vv"},
		Value:    false,
		Usage:    "verbose logging",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "text-only",
		Value:    false,
		Usage:    "include only text files",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "hash-markers",
		Value:    false,
		Usage:    "create also hint files mirroring the hash of original files at <path>.hash",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "ignore-case",
		Value:    false,
		Usage:    "ignore case when checking path against inclusion patterns",
		Required: false,
	},
	&cli.IntFlag{
		Name:     "max-size",
		Value:    6,
		Usage:    "maximal file size, in MB",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "no-double-check",
		Value:    false,
		Usage:    "disable files discrepancy double check",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "include-noise-dirs",
		Value:    false,
		Usage:    "don't filter out noisy directory names in paths (bin, node_modules etc)",
		Required: false,
	},
}

type Options struct {
	ClonePath          string
	Revision           string
	OutputPath         string
	IncludePatterns    []string
	ExcludePatterns    []string
	VerboseLogging     bool
	TextFilesOnly      bool
	CreateHashMarkers  bool
	IgnoreCasePatterns bool
	MaxFileSizeBytes   int64
	SkipDoubleCheck    bool
	IncludeNoiseDirs   bool
}

func splitListFlag(flag string) []string {
	if len(flag) == 0 {
		return []string{}
	}
	return strings.Split(flag, ",")
}

func validateDirectory(dirPath string, createIfNotExist bool) error {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		if !createIfNotExist {
			return fmt.Errorf("directory does not exist at %v", dirPath)
		}
		err = os.MkdirAll(dirPath, 0777)
		if err != nil {
			return fmt.Errorf("failed to create directory at %v: %w", dirPath, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("directory error at %v: %w", dirPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("directory is actually a file at %v", dirPath)
	}
	return nil
}

func ParseOptions(c *cli.Context) (*Options, error) {
	opts := &Options{
		ClonePath:          c.String("src"),
		Revision:           c.String("rev"),
		OutputPath:         c.String("out"),
		IncludePatterns:    splitListFlag(c.String("include")),
		ExcludePatterns:    splitListFlag(c.String("exclude")),
		VerboseLogging:     c.Bool("verbose"),
		TextFilesOnly:      c.Bool("text-only"),
		CreateHashMarkers:  c.Bool("hash-markers"),
		IgnoreCasePatterns: c.Bool("ignore-case"),
		MaxFileSizeBytes:   int64(c.Int("max-size")) * 1024 * 1024,
		SkipDoubleCheck:    c.Bool("no-double-check"),
		IncludeNoiseDirs:   c.Bool("include-noise-dirs"),
	}

	err := validateDirectory(opts.ClonePath, false)
	if err != nil {
		return nil, &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_CLONE_PATH,
			InternalError: fmt.Errorf("clone at '%v' is missing or invalid: %v", opts.ClonePath, err),
		}
	}

	err = validateDirectory(path.Join(opts.ClonePath, ".git"), false)
	if err != nil {
		return nil, &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_CLONE_PATH,
			InternalError: fmt.Errorf(".git at '%v' is missing or invalid: %v", opts.ClonePath, err),
		}
	}

	err = validateDirectory(opts.OutputPath, true)
	if err != nil {
		return nil, &util.ErrorWithCode{
			StatusCode:    util.ERROR_BAD_OUTPUT_PATH,
			InternalError: err,
		}
	}

	if !opts.IncludeNoiseDirs {
		opts.ExcludePatterns = union(util.NoisyDirectoryExclusionPatterns(), opts.ExcludePatterns)
	}

	return opts, nil
}

func union(s1 []string, s2 []string) []string {
	if len(s1) == 0 {
		return s2
	}
	if len(s2) == 0 {
		return s1
	}
	unified := make([]string, len(s1)+len(s2))
	i := 0
	for _, item := range s1 {
		unified[i] = item
		i++
	}
	for _, item := range s2 {
		unified[i] = item
		i++
	}
	return unified
}
