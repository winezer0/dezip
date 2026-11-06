// Package dezip safely unpacks common archive and compression formats.
package dezip

import "errors"

const (
	defaultMaxEntries       = 100_000
	defaultMaxExpandedBytes = int64(1 << 30)
)

var (
	// ErrDestinationExists indicates that the requested output directory already exists.
	ErrDestinationExists = errors.New("destination already exists")
	// ErrInvalidMode indicates that an extraction mode is not supported.
	ErrInvalidMode = errors.New("invalid extraction mode")
	// ErrModeMismatch indicates that the input format does not match the requested mode.
	ErrModeMismatch = errors.New("input format does not match extraction mode")
	// ErrUnsupportedFormat indicates that the input is not a supported archive or compressed stream.
	ErrUnsupportedFormat = errors.New("unsupported archive or compression format")
	// ErrUnsafeEntry indicates that an archive entry cannot be written safely.
	ErrUnsafeEntry = errors.New("unsafe archive entry")
	// ErrLimitExceeded indicates that an archive exceeded a configured expansion limit.
	ErrLimitExceeded = errors.New("archive expansion limit exceeded")
)

// Options controls unpacking. Zero limits use the documented defaults.
type Options struct {
	// MaxEntries limits archive members. The default is 100,000.
	MaxEntries int
	// MaxExpandedBytes limits total bytes written. The default is 1 GiB.
	MaxExpandedBytes int64
	// StripComponents removes this many leading path components from archive entries.
	StripComponents int
	// OutputName overrides the output file name when the input is a compressed stream,
	// rather than an archive. It must be a single file name.
	OutputName string
	// ExpectedFile requires this relative regular file to exist in the completed output.
	// For a compressed stream, it must match the single decompressed file name.
	ExpectedFile string
	// AllowSymlinks enables safe, relative symbolic links. Symlinks are rejected by default.
	AllowSymlinks bool
	// OnProgress is called after each extracted file or link.
	OnProgress func(Progress)
}

// Progress describes completed work. ExpandedBytes counts bytes written so far.
type Progress struct {
	Name          string
	Entries       int
	Files         int
	ExpandedBytes int64
}

// Result describes the completed processing operation. Skipped results retain
// the original file at SourcePath. OutputFile is set for decompressed streams or
// when Options.ExpectedFile selects a file from an archive.
type Result struct {
	Mode          Mode
	SourcePath    string
	Skipped       bool
	Format        string
	OutputDir     string
	OutputFile    string
	Entries       int
	Files         int
	ExpandedBytes int64
}
