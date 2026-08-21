package main

import (
	"context"
	"fmt"
	"os"

	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/cli"
	"github.com/launchbynttdata/launch-ado-automatic-versioner/internal/cli/exitcodes"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitcodes.CodeFor(err))
	}
}
