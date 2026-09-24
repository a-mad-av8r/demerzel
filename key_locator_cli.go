package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"gpt-load/internal/platform/encryption"
)

func dispatchKeyLocator(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("key-locator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dataDir := flags.String("data-dir", os.Getenv("DATA_DIR"), "existing application data directory")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stdout, "Usage: demerzel key-locator --data-dir PATH")
			return 0
		}
		fmt.Fprintln(stderr, "key-locator accepts only --data-dir PATH")
		return 1
	}
	if flags.NArg() != 0 || *dataDir == "" {
		fmt.Fprintln(stderr, "key-locator requires --data-dir PATH or DATA_DIR")
		return 1
	}
	locator, err := encryption.InstallationIdentifier(*dataDir)
	if err != nil {
		fmt.Fprintln(stderr, "key-locator requires an existing DATA_DIR")
		return 1
	}
	_, _ = fmt.Fprintln(stdout, locator)
	return 0
}
