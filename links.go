package dezip

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func (state *unpackState) finishLinks(ctx context.Context) error {
	pending := append([]pendingLink(nil), state.hardlinks...)
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := make([]pendingLink, 0, len(pending))
		progressed := false
		for _, link := range pending {
			source, err := joinWithin(state.root, link.target)
			if err != nil {
				return err
			}
			info, err := os.Lstat(source)
			if errors.Is(err, os.ErrNotExist) {
				remaining = append(remaining, link)
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect hard link target %q: %w", link.target, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%w: hard link %q does not target a regular file", ErrUnsafeEntry, link.name)
			}
			destination, err := joinWithin(state.root, link.name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			if err := state.copyFile(ctx, source, destination, safeMode(link.mode)); err != nil {
				return fmt.Errorf("copy hard link %q: %w", link.name, err)
			}
			state.notify(link.name)
			progressed = true
		}
		if !progressed && len(remaining) > 0 {
			return fmt.Errorf("%w: unresolved hard link target %q", ErrUnsafeEntry, remaining[0].target)
		}
		pending = remaining
	}

	for _, link := range state.symlinks {
		if err := ctx.Err(); err != nil {
			return err
		}
		destination, err := joinWithin(state.root, link.name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(filepath.FromSlash(link.target), destination); err != nil {
			return fmt.Errorf("create symbolic link %q: %w", link.name, err)
		}
	}
	return nil
}

func (state *unpackState) copyFile(ctx context.Context, source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := state.copy(ctx, output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destination, mode)
}
