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

// Note: Version and Revision are set via ldflags during build
// The tfclean package variables are set directly via ldflags,
// so no init() function is needed here

func main() {
	ctx := context.TODO()
	ctx, stop := signal.NotifyContext(ctx, []os.Signal{os.Interrupt}...)
	defer stop()
	if err := tfclean.RunCLI(ctx, os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}
