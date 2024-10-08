package main

import (
	"context"
	"github.com/takaishi/tfclean"
	"log"
	"os"
	"os/signal"
)

var Version = "dev"
var Revision = "HEAD"

func init() {
	tfclean.Version = Version
	tfclean.Revision = Revision
}

func main() {
	ctx := context.TODO()
	ctx, stop := signal.NotifyContext(ctx, []os.Signal{os.Interrupt}...)
	defer stop()
	if err := tfclean.RunCLI(ctx, os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}
