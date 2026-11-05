package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/winezer0/dezip"
)

func main() {
	os.Exit(exitCode())
}

func exitCode() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("dezip", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("src", "", "archive or compressed file to unpack")
	destination := flags.String("dest", "", "new output directory")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dezip -src <archive> -dest <new-directory>")
		fmt.Fprintln(stderr, "   or: dezip <archive> <new-directory>")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if positional := flags.Args(); len(positional) > 0 {
		if *source != "" || *destination != "" {
			return errors.New("use positional arguments or -src/-dest flags, not both")
		}
		if len(positional) != 2 {
			flags.Usage()
			return errors.New("expected an input file and output directory")
		}
		*source, *destination = positional[0], positional[1]
	}
	if *source == "" || *destination == "" {
		flags.Usage()
		return errors.New("both -src and -dest are required")
	}

	result, err := dezip.Unpack(ctx, *source, *destination, dezip.Options{})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Unpacked %s: %d entries, %d files, %d bytes -> %s\n",
		result.Format, result.Entries, result.Files, result.ExpandedBytes, result.OutputDir)
	return err
}
