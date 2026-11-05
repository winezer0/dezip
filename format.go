package dezip

import (
	"context"
	"io"
	"path/filepath"

	"github.com/mholt/archives"
)

func identifyFormat(ctx context.Context, sourcePath string, source io.Reader) (archives.Format, io.Reader, error) {
	return archives.Identify(ctx, filepath.Base(sourcePath), source)
}
