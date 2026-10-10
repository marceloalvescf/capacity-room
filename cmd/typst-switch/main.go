// Command typst-switch stands in for typst in the worker of the end-to-end
// stack. It runs the real typst with the same arguments and environment,
// unless the switch file exists, which makes every compile fail.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// The worker runs typst with an empty environment, so both paths are fixed.
const (
	realTypst  = "/usr/local/bin/typst"
	switchFile = "/switch/fail"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Stat, syscall.Exec))
}

func run(args []string, stderr io.Writer, stat func(string) (fs.FileInfo, error), exec func(string, []string, []string) error) int {
	if len(args) > 0 && args[0] == "compile" {
		_, err := stat(switchFile)
		if err == nil {
			_, _ = fmt.Fprintln(stderr, "typst-switch: compile refused by the switch")
			return 1
		}
		if !errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(stderr, "typst-switch: %v\n", err)
			return 1
		}
	}
	err := exec(realTypst, append([]string{realTypst}, args...), os.Environ())
	_, _ = fmt.Fprintf(stderr, "typst-switch: %v\n", err)
	return 1
}
