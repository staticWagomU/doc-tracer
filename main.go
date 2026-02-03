package main

import (
	"fmt"
	"os"

	"github.com/tomohiro-owada/doc-tracer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
