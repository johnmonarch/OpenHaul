package main

import (
	"os"

	"github.com/openhaulguard/openhaulguard/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
