package main

import (
	"fmt"
	"os"

	"ovw/cmd/ovw"
)

func main() {
	if err := ovw.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
