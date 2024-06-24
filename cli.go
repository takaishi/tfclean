package tfclean

import (
	"context"
	"github.com/alecthomas/kong"
)

type CLI struct {
	Tfstate string `help:"Terraform state file"`
	Dir     string `arg:"" required:"" help:"Directory to clean"`
}

func RunCLI(ctx context.Context, args []string) error {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		return err
	}
	_, err = parser.Parse(args)
	if err != nil {
		return err
	}

	app := New(&cli)
	return app.Run(ctx)
}
