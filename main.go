package main

import (
	"github.com/urfave/cli/v2"
	"gitsnap/git"
	"gitsnap/options"
	"gitsnap/util"
	"log"
	"os"
)

const VERSION = "1.5.8"

func main() {
	cli.AppHelpTemplate =
		`NAME:
   git-snap - 1.5.8 - Create a git revision snapshot for an existing repository clone. Symbolic link files will be omitted.

USAGE:
   git-snap --src value --rev value --out value        [optional flags]

OPTIONS:
   --src value, -s value      path to existing git clone as source directory, may contain no more than .git directory, current git state doesn't affect the command
   --rev value, -r value      commit-ish Revision
   --out value, -o value      output directory. will be created if does not exist
   --include value, -i value  patterns of file paths to include, comma delimited, may contain any glob pattern
   --exclude value, -e value  patterns of file paths to exclude, comma delimited, may contain any glob pattern
   --verbose, --vv            verbose logging (default: false)
   --text-only                include only text files (default: false)
   --hash-markers             create also hint files mirroring the hash of original files at <path>.hash (default: false)
   --ignore-case              ignore case when checking path against inclusion patterns (default: false)
   --max-size value           maximal file size, in MB (default: 6)
   --help, -h                 show help (default: false)
   --version, -v              print the version (default: false)

EXIT CODES:
  0   Success
  201  Clone path is invalid (fs-wise)
  202  Clone path is invalid (git-wise)
  203  Output path is invalid
  204  Short sha is not supported
  205  Provided revision could not be found
  1    Any other error
`

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)
	app := &cli.App{
		Name:    "git-snap",
		Usage:   "Create a git revision snapshot for an existing repository clone. Symbolic link files will be omitted.",
		Flags:   options.Flags,
		Version: VERSION,
		Action: func(ctx *cli.Context) error {
			opts, err := options.ParseOptions(ctx)
			if err != nil {
				return err
			}
			err = git.Snapshot(opts)
			if err == nil {
				log.Printf("Completed successfully at %v", opts.OutputPath)
			}
			return err
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Printf("failed: %v", err)
		if errorWithCode, isWithCode := err.(*util.ErrorWithCode); isWithCode {
			os.Exit(errorWithCode.StatusCode)
		}
		os.Exit(1)
	}
}
