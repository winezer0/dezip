package dezip

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mholt/archives"
)

type pendingLink struct {
	name   string
	target string
	mode   fs.FileMode
}

type unpackState struct {
	root    string
	options Options
	result  *Result
	paths   pathTracker

	hardlinks []pendingLink
	symlinks  []pendingLink
}

func (state *unpackState) extractEntry(ctx context.Context, entry archives.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state.result.Entries++
	if state.result.Entries > state.options.MaxEntries {
		return fmt.Errorf("%w: more than %d entries", ErrLimitExceeded, state.options.MaxEntries)
	}

	name, skip, err := normalizeArchiveName(entry.NameInArchive, state.options.StripComponents)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	mode := entry.Mode()
	if entry.IsDir() {
		duplicate, err := state.paths.add(name, kindDirectory)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		destination, err := joinWithin(state.root, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(destination, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", name, err)
		}
		return nil
	}

	if mode&fs.ModeSymlink != 0 {
		if !state.options.AllowSymlinks {
			return fmt.Errorf("%w: symbolic link %q is disabled", ErrUnsafeEntry, name)
		}
		target, err := normalizeSymlinkTarget(name, entry.LinkTarget)
		if err != nil {
			return err
		}
		if _, err := state.paths.add(name, kindSymlink); err != nil {
			return err
		}
		state.symlinks = append(state.symlinks, pendingLink{name: name, target: target, mode: mode})
		state.result.Files++
		state.notify(name)
		return nil
	}

	if !mode.IsRegular() {
		return fmt.Errorf("%w: unsupported file type at %q", ErrUnsafeEntry, name)
	}
	if entry.LinkTarget != "" {
		target, skipTarget, err := normalizeArchiveName(entry.LinkTarget, state.options.StripComponents)
		if err != nil {
			return fmt.Errorf("hard link %q: %w", name, err)
		}
		if skipTarget {
			return fmt.Errorf("%w: hard link %q points outside the stripped archive root", ErrUnsafeEntry, name)
		}
		if _, err := state.paths.add(name, kindHardlink); err != nil {
			return err
		}
		state.hardlinks = append(state.hardlinks, pendingLink{name: name, target: target, mode: mode})
		state.result.Files++
		return nil
	}

	if _, err := state.paths.add(name, kindRegular); err != nil {
		return err
	}
	destination, err := joinWithin(state.root, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create parent for %q: %w", name, err)
	}
	if err := state.writeEntry(ctx, destination, entry, safeMode(mode)); err != nil {
		return fmt.Errorf("write %q: %w", name, err)
	}
	state.result.Files++
	state.notify(name)
	return nil
}

func (state *unpackState) writeEntry(ctx context.Context, destination string, entry archives.FileInfo, mode fs.FileMode) error {
	reader, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open archive member: %w", err)
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = reader.Close()
		return err
	}
	_, copyErr := state.copy(ctx, output, reader)
	readerErr := reader.Close()
	outputErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if readerErr != nil {
		return readerErr
	}
	if outputErr != nil {
		return outputErr
	}
	if err := os.Chmod(destination, mode); err != nil {
		return err
	}
	return nil
}
