package dezip

import (
	"context"
	"errors"
	"fmt"
	"io"
)

func (state *unpackState) copy(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 128*1024)
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if int64(read) > state.options.MaxExpandedBytes-state.result.ExpandedBytes {
				return copied, fmt.Errorf("%w: limit is %d bytes", ErrLimitExceeded, state.options.MaxExpandedBytes)
			}
			written, writeErr := destination.Write(buffer[:read])
			state.result.ExpandedBytes += int64(written)
			copied += int64(written)
			if writeErr != nil {
				return copied, writeErr
			}
			if written != read {
				return copied, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return copied, nil
			}
			return copied, readErr
		}
	}
}

func (state *unpackState) notify(name string) {
	if state.options.OnProgress == nil {
		return
	}
	state.options.OnProgress(Progress{
		Name:          name,
		Entries:       state.result.Entries,
		Files:         state.result.Files,
		ExpandedBytes: state.result.ExpandedBytes,
	})
}
