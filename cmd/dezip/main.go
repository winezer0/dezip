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
	destination := flags.String("dest", "", "new output directory (required when extracting)")
	extractAs := flags.String("as", "auto", "processing mode: auto, zip, tar, gzip, skip (aliases: tar.gz, tar.xz)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dezip -src <file> [-dest <new-directory>] [-as auto|zip|tar|gzip|skip]")
		fmt.Fprintln(stderr, "   or: dezip <file> [new-directory] [-as auto|zip|tar|gzip|skip]")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	mode, err := dezip.ParseExtractAs(*extractAs)
	if err != nil {
		return err
	}
	if positional := flags.Args(); len(positional) > 0 {
		if *source != "" || *destination != "" {
			return errors.New("use positional arguments or -src/-dest flags, not both")
		}
		if len(positional) < 1 || len(positional) > 2 || (len(positional) == 1 && mode != dezip.ModeAuto && mode != dezip.ModeSkip) {
			flags.Usage()
			return errors.New("expected an input file and, when extracting, an output directory")
		}
		*source = positional[0]
		if len(positional) == 2 {
			*destination = positional[1]
		}
	}
	if *source == "" || ((*destination == "") && mode != dezip.ModeAuto && mode != dezip.ModeSkip) {
		flags.Usage()
		return errors.New("-src is required and -dest is required when extracting")
	}

	result, err := dezip.Process(ctx, *source, *destination, mode, dezip.Options{})
	if err != nil {
		return err
	}
	if result.Skipped {
		_, err = fmt.Fprintf(stdout, "Kept file without extraction: %s\n", result.SourcePath)
		return err
	}
	_, err = fmt.Fprintf(stdout, "Unpacked %s: %d entries, %d files, %d bytes -> %s\n",
		result.Format, result.Entries, result.Files, result.ExpandedBytes, result.OutputDir)
	return err
}
