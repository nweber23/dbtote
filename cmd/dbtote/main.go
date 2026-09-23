package main

import (
	"os"

	"github.com/nweber23/dbtote/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}