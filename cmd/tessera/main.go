package main

import (
	"os"

	"github.com/balyakin/tessera/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
