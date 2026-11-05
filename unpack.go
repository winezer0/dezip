package dezip

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mholt/archives"
)

// Unpack identifies and unpacks an archive or compressed stream into destination.
// Archives are extracted as directories. A raw compressed stream is written as one
// file in destination, using Options.OutputName or the input name without its
// compression extension. The destination must not already exist. Work is performed
// in a sibling staging directory and published by rename only after success.
func Unpack(ctx context.Context, sourcePath, destination string, options Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if sourcePath == "" {
		return Result{}, errors.New("source path is required")
	}
	opts, err := normalizedOptions(options)
	if err != nil {
		return Result{}, err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return Result{}, fmt.Errorf("open source: %w", err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return Result{}, fmt.Errorf("stat source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Result{}, errors.New("source must be a regular file")
	}

	format, stream, err := identifyFormat(ctx, sourcePath, source)
	if err != nil {
		return Result{}, fmt.Errorf("identify source format: %w", err)
	}

	dest, stage, err := prepareDestination(destination)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)

	result := Result{
		Format:    strings.TrimPrefix(format.Extension(), "."),
		OutputDir: dest,
	}
	state := &unpackState{
		root:    stage,
		options: opts,
		result:  &result,
		paths:   newPathTracker(),
	}

	if extractor, ok := format.(archives.Extractor); ok {
		if err := extractor.Extract(ctx, stream, state.extractEntry); err != nil {
			return result, fmt.Errorf("extract %s: %w", result.Format, err)
		}
		if err := state.finishLinks(ctx); err != nil {
			return result, err
		}
	} else if decompressor, ok := format.(archives.Decompressor); ok {
		if err := state.decompress(ctx, format.Extension(), decompressor, stream, sourcePath); err != nil {
			return result, fmt.Errorf("decompress %s: %w", result.Format, err)
		}
	} else {
		return result, fmt.Errorf("%w: %s", ErrUnsupportedFormat, result.Format)
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	if _, err := os.Lstat(dest); err == nil {
		return result, ErrDestinationExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("check destination: %w", err)
	}
	if err := os.Rename(stage, dest); err != nil {
		return result, fmt.Errorf("publish unpacked output: %w", err)
	}
	return result, nil
}
