// Command carto is the Cartographer core: the CLI, and later the MCP server and the hooks.
package main

import (
	"os"

	"github.com/nathanaday/cartographer/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
