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

// Result describes the completed unpack operation.
type Result struct {
	Format        string
	OutputDir     string
	Entries       int
	Files         int
	ExpandedBytes int64
}
