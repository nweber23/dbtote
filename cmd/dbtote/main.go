package main

import (
	"os"

	"github.com/nweber23/dbtote/internal/cli"

	_ "github.com/nweber23/dbtote/internal/db/mysql"
	_ "github.com/nweber23/dbtote/internal/db/postgres"
	_ "github.com/nweber23/dbtote/internal/storage/local"
)

func main() {
	os.Exit(cli.Execute())
}
