//go:build !dezip_full

package dezip

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const maxZipSymlinkTargetBytes = 32 * 1024

const fullFormatsEnabled = false

func identifyFormatForBuild(ctx context.Context, sourcePath string, source *os.File) (*identifiedFormat, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := source.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat ZIP source: %w", err)
	}
	reader, err := zip.NewReader(source, info.Size())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoMatch, err)
	}

	format := &identifiedFormat{extension: "zip"}
	format.extract = func(ctx context.Context, handle func(context.Context, archiveEntry) error) error {
		for _, file := range reader.File {
			if err := ctx.Err(); err != nil {
				return err
			}
			mode := file.Mode()
			linkTarget := ""
			if mode&fs.ModeSymlink != 0 {
				var err error
				linkTarget, err = readZipSymlink(file)
				if err != nil {
					return fmt.Errorf("read ZIP symlink %q: %w", file.Name, err)
				}
			}
			entry := archiveEntry{
				NameInArchive: file.Name,
				LinkTarget:    linkTarget,
				mode:          mode,
				directory:     file.FileInfo().IsDir(),
				open:          file.Open,
			}
			if err := handle(ctx, entry); err != nil {
				return err
			}
		}
		return nil
	}
	return format, nil
}

func readZipSymlink(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	target, err := io.ReadAll(io.LimitReader(reader, maxZipSymlinkTargetBytes+1))
	if err != nil {
		return "", err
	}
	if len(target) > maxZipSymlinkTargetBytes {
		return "", fmt.Errorf("%w: symlink target exceeds %d bytes", ErrUnsafeEntry, maxZipSymlinkTargetBytes)
	}
	return string(target), nil
}
