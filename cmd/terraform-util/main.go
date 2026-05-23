package main

import (
	"os"

	"github.com/sofushn/terraform-util-cli/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
