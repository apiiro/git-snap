package options

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
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
		Name:     "shortsha",
		Value:    false,
		Usage:    "support short-sha Revision",
		Required: false,
	},
}

type Options struct {
	ClonePath       string
	Revision        string
	OutputPath      string
	IncludePatterns []string
	ExcludePatterns []string
	SupportShortSha bool
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
		ClonePath:       c.String("src"),
		Revision:        c.String("rev"),
		OutputPath:      c.String("out"),
		IncludePatterns: splitListFlag(c.String("include")),
		ExcludePatterns: splitListFlag(c.String("exclude")),
		SupportShortSha: c.Bool("shortsha"),
	}
	err := validateDirectory(opts.ClonePath, false)
	if err == nil {
		err = validateDirectory(opts.OutputPath, true)
	}
	return opts, err
}
