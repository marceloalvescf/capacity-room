package main

import (
	"os"

	"github.com/marceloalvescf/k8s-capacity-report/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
