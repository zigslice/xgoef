// Command xgoef scaffolds and stages XGo + CEF applications.
package main

import (
	"os"

	"github.com/zigslice/xgoef/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
