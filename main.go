package main

import (
	"git-snap/git"
	"git-snap/options"
	"github.com/urfave/cli/v2"
	"log"
	"os"
)

const VERSION = "1.0"

func main() {
	cli.AppHelpTemplate =
		`NAME:
   {{.Name}} - {{.Version}} - {{.Usage}}

USAGE:
   {{.Name}}{{range .Flags}}{{if and (not (eq .Name "help")) (not (eq .Name "version")) }} {{if .Required}}--{{.Name}} value{{else}}[--{{.Name}} value]{{end}}{{end}}{{end}}

OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}
`

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	app := &cli.App{
		Name:    "git-snap",
		Usage:   "Create a git revision snapshot for an existing repository clone.\n                    Symbolic link files will be omitted.",
		Flags:   options.Flags,
		Version: VERSION,
		Action: func(ctx *cli.Context) error {
			opts, err := options.ParseOptions(ctx)
			if err != nil {
				return err
			}
			err = git.Snapshot(opts)
			if err != nil {
				log.Printf("Completed successfully at %v", opts.OutputPath)
			}
			return err
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
