package main

import (
	"errors"
	"os"

	"github.com/guilherme-grimm/basego/internal/cli"
)

func main() {
	err := cli.Run(os.Args, os.Stdout, os.Stderr)
	switch {
	case err == nil:
		return
	case errors.Is(err, cli.ErrUsage):
		os.Exit(2)
	default:
		os.Exit(1)
	}
}
