package main

import (
	"gitsnap/git"
	"gitsnap/options"
	"gitsnap/util"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

const VERSION = "1.6.8"

func main() {
	cli.AppHelpTemplate =
		`NAME:
   {{.Name}} - {{.Version}} - {{.Usage}}

USAGE:
   {{.Name}}{{range .Flags}}{{if and (not (eq .Name "help")) (not (eq .Name "version")) }} {{if .Required}}--{{.Name}} value{{end}}{{end}}{{end}} [optional flags]

OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}
EXIT CODES:
	0   Success
	101 Some file name is too long
	201	Clone path is invalid (fs-wise)
	202	Clone path is invalid (git-wise)
	203	Output path is invalid
	204	Short sha is not supported
	205	Provided revision could not be found
	206 Double check for files discrepancy failed
	1	Any other error
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
