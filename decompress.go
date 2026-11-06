package dezip

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

func (state *unpackState) decompress(ctx context.Context, extension string, decompressor archives.Decompressor, source io.Reader, sourcePath string) error {
	name := state.options.OutputName
	if name == "" {
		base := filepath.Base(sourcePath)
		if !strings.HasSuffix(strings.ToLower(base), strings.ToLower(extension)) || len(base) <= len(extension) {
			return errors.New("raw compressed input needs OutputName when its filename does not end in the detected extension")
		}
		name = base[:len(base)-len(extension)]
	}
	if err := validateOutputName(name); err != nil {
		return err
	}
	destination, err := joinWithin(state.root, name)
	if err != nil {
		return err
	}
	reader, err := decompressor.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := state.copy(ctx, output, reader)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	state.result.OutputFile = name
	state.result.Entries = 1
	state.result.Files = 1
	state.notify(name)
	return nil
}
