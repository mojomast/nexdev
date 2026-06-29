package main

import (
	"fmt"
	"os"

	"github.com/mojomast/nexdev/internal/cli"
)

// Version is set during build time
var Version = "dev"

func main() {
	if err := cli.Execute(Version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
