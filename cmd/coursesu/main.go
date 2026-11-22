package main

import (
	"os"

	"github.com/NicolasGuilloux/superu-mcp/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
