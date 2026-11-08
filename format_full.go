//go:build dezip_full

package dezip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archives"
)

const fullFormatsEnabled = true

func identifyFormatForBuild(ctx context.Context, sourcePath string, source *os.File) (*identifiedFormat, error) {
	format, stream, err := archives.Identify(ctx, filepath.Base(sourcePath), source)
	if err != nil {
		if errors.Is(err, archives.NoMatch) {
			return nil, fmt.Errorf("%w: %w", ErrNoMatch, err)
		}
		return nil, err
	}
	if format == nil {
		return nil, fmt.Errorf("%w: no archive format identified", ErrNoMatch)
	}

	identified := &identifiedFormat{extension: format.Extension()}
	if extractor, ok := format.(archives.Extractor); ok {
		identified.extract = func(ctx context.Context, handle func(context.Context, archiveEntry) error) error {
			return extractor.Extract(ctx, stream, func(ctx context.Context, file archives.FileInfo) error {
				return handle(ctx, archiveEntry{
					NameInArchive: file.NameInArchive,
					LinkTarget:    file.LinkTarget,
					mode:          file.Mode(),
					directory:     file.IsDir(),
					open: func() (io.ReadCloser, error) {
						return file.Open()
					},
				})
			})
		}
	}
	if decompressor, ok := format.(archives.Decompressor); ok {
		identified.openReader = func() (io.ReadCloser, error) {
			return decompressor.OpenReader(stream)
		}
	}
	return identified, nil
}
