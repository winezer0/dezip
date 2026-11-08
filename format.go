package dezip

import (
	"context"
	"io"
	"io/fs"
	"os"
)

type archiveEntry struct {
	NameInArchive string
	LinkTarget    string
	mode          fs.FileMode
	directory     bool
	open          func() (io.ReadCloser, error)
}

func (entry archiveEntry) Mode() fs.FileMode { return entry.mode }

func (entry archiveEntry) IsDir() bool { return entry.directory }

func (entry archiveEntry) Open() (io.ReadCloser, error) { return entry.open() }

type identifiedFormat struct {
	extension  string
	extract    func(context.Context, func(context.Context, archiveEntry) error) error
	openReader func() (io.ReadCloser, error)
}

func (format *identifiedFormat) Extension() string { return format.extension }

func identifyFormat(ctx context.Context, sourcePath string, source *os.File) (*identifiedFormat, error) {
	return identifyFormatForBuild(ctx, sourcePath, source)
}
