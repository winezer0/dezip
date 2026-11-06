package dezip

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

// Unpack identifies and unpacks an archive or compressed stream into destination.
// Archives are extracted as directories. A raw compressed stream is written as one
// file in destination, using Options.OutputName or the input name without its
// compression extension. The destination must not already exist. Work is performed
// in a sibling staging directory and published by rename only after success.
func Unpack(ctx context.Context, sourcePath, destination string, options Options) (Result, error) {
	result, err := unpack(ctx, sourcePath, destination, options, ModeAuto, false)
	result.Mode = ""
	result.SourcePath = ""
	result.Skipped = false
	return result, err
}

// Process applies a DASM-style extraction mode to an already downloaded file.
// ModeSkip returns the source path without copying or extracting it. ModeAuto
// unpacks recognized archives and compressed streams, while an ordinary file
// with an unrecognized extension is kept as-is. ModeGzip only accepts a raw
// gzip stream. A zero mode follows DASM's empty extract_as behavior and means
// ModeSkip.
func Process(ctx context.Context, sourcePath, destination string, mode Mode, options Options) (Result, error) {
	mode, err := normalizedMode(mode)
	if err != nil {
		return Result{}, err
	}
	if sourcePath == "" {
		return Result{}, errors.New("source path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	if mode == ModeSkip {
		if options.ExpectedFile != "" {
			return Result{}, errors.New("expected file cannot be used with skip mode")
		}
		path, err := regularSourcePath(sourcePath)
		if err != nil {
			return Result{}, err
		}
		return Result{Mode: mode, SourcePath: path, Skipped: true}, nil
	}

	return unpack(ctx, sourcePath, destination, options, mode, true)
}

func unpack(ctx context.Context, sourcePath, destination string, options Options, mode Mode, skipOrdinaryFiles bool) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if sourcePath == "" {
		return Result{}, errors.New("source path is required")
	}
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve source path: %w", err)
	}
	opts, err := normalizedOptions(options)
	if err != nil {
		return Result{}, err
	}
	if opts.ExpectedFile != "" && (!filepath.IsLocal(opts.ExpectedFile) || filepath.Clean(opts.ExpectedFile) == ".") {
		return Result{}, fmt.Errorf("%w: expected file must be a relative file path", ErrUnsafeEntry)
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
		if mode == ModeAuto && skipOrdinaryFiles && opts.ExpectedFile == "" && !hasKnownArchiveExtension(sourcePath) && errors.Is(err, archives.NoMatch) && ctx.Err() == nil {
			return Result{Mode: mode, SourcePath: sourcePath, Skipped: true}, nil
		}
		return Result{}, fmt.Errorf("identify source format: %w", err)
	}
	if format == nil {
		if mode == ModeAuto && skipOrdinaryFiles && opts.ExpectedFile == "" && !hasKnownArchiveExtension(sourcePath) && ctx.Err() == nil {
			return Result{Mode: mode, SourcePath: sourcePath, Skipped: true}, nil
		}
		return Result{}, ErrUnsupportedFormat
	}
	if mode != ModeAuto && !formatMatchesMode(format, mode) {
		return Result{}, fmt.Errorf("%w: requested %s, detected %s", ErrModeMismatch, mode, format.Extension())
	}
	if _, ok := format.(archives.Extractor); !ok {
		if _, ok := format.(archives.Decompressor); !ok {
			return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format.Extension())
		}
	}

	dest, stage, err := prepareDestination(destination)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)

	result := Result{
		Mode:       mode,
		SourcePath: sourcePath,
		Format:     strings.TrimPrefix(format.Extension(), "."),
		OutputDir:  dest,
	}
	state := &unpackState{
		root:    stage,
		options: opts,
		result:  &result,
		paths:   newPathTracker(),
	}

	rawStream := false
	if extractor, ok := format.(archives.Extractor); ok {
		if err := extractor.Extract(ctx, stream, state.extractEntry); err != nil {
			return result, fmt.Errorf("extract %s: %w", result.Format, err)
		}
		if err := state.finishLinks(ctx); err != nil {
			return result, err
		}
	} else if decompressor, ok := format.(archives.Decompressor); ok {
		rawStream = true
		if err := state.decompress(ctx, format.Extension(), decompressor, stream, sourcePath); err != nil {
			return result, fmt.Errorf("decompress %s: %w", result.Format, err)
		}
	} else {
		return result, fmt.Errorf("%w: %s", ErrUnsupportedFormat, result.Format)
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	if opts.ExpectedFile != "" {
		if rawStream {
			if filepath.Clean(opts.ExpectedFile) != filepath.Clean(result.OutputFile) {
				return result, fmt.Errorf("expected file %q does not match decompressed output %q", opts.ExpectedFile, result.OutputFile)
			}
		} else {
			expectedFile, err := resolveExpectedOutputFile(stage, opts.ExpectedFile)
			if err != nil {
				return result, err
			}
			result.OutputFile = expectedFile
		}
	}
	if _, err := os.Lstat(dest); err == nil {
		return result, ErrDestinationExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("check destination: %w", err)
	}
	if err := os.Rename(stage, dest); err != nil {
		return result, fmt.Errorf("publish unpacked output: %w", err)
	}
	if result.OutputFile != "" {
		result.OutputFile = filepath.Join(dest, result.OutputFile)
	}
	return result, nil
}

func regularSourcePath(sourcePath string) (string, error) {
	path, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", fmt.Errorf("resolve source path: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("source must be a regular file")
	}
	return filepath.Clean(path), nil
}

func hasKnownArchiveExtension(sourcePath string) bool {
	name := strings.ToLower(filepath.Base(sourcePath))
	for _, extension := range []string{
		".zip", ".tar", ".tar.gz", ".tgz", ".rar", ".7z",
		".br", ".bz2", ".gz", ".gzip", ".lz4", ".lz", ".mz", ".sz", ".s2",
		".xz", ".zz", ".zst",
	} {
		if strings.HasSuffix(name, extension) {
			return true
		}
	}
	return false
}

func formatMatchesMode(format archives.Format, mode Mode) bool {
	extension := strings.ToLower(strings.TrimPrefix(format.Extension(), "."))
	switch mode {
	case ModeZip:
		_, ok := format.(archives.Extractor)
		return ok && extension == "zip"
	case ModeTar:
		_, ok := format.(archives.Extractor)
		return ok && (extension == "tar" || extension == "tgz" || strings.HasPrefix(extension, "tar."))
	case ModeGzip:
		_, ok := format.(archives.Decompressor)
		return ok && (extension == "gz" || extension == "gzip")
	default:
		return true
	}
}
