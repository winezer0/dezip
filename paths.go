package dezip

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type entryKind uint8

const (
	kindDirectory entryKind = iota + 1
	kindRegular
	kindHardlink
	kindSymlink
)

type pathTracker struct {
	entries          map[string]entryKind
	descendantCounts map[string]int
	caseInsensitive  bool
}

func newPathTracker() pathTracker {
	return pathTracker{
		entries:          make(map[string]entryKind),
		descendantCounts: make(map[string]int),
		caseInsensitive:  runtime.GOOS == "windows" || runtime.GOOS == "darwin",
	}
}

func (tracker *pathTracker) key(name string) string {
	if tracker.caseInsensitive {
		return strings.ToLower(name)
	}
	return name
}

func (tracker *pathTracker) add(name string, kind entryKind) (bool, error) {
	key := tracker.key(name)
	if previous, exists := tracker.entries[key]; exists {
		if previous == kindDirectory && kind == kindDirectory {
			return true, nil
		}
		return false, fmt.Errorf("%w: duplicate path %q", ErrUnsafeEntry, name)
	}
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		if previous, exists := tracker.entries[tracker.key(parent)]; exists && previous != kindDirectory {
			return false, fmt.Errorf("%w: %q is nested under a non-directory entry", ErrUnsafeEntry, name)
		}
	}
	if kind != kindDirectory && tracker.descendantCounts[key] > 0 {
		return false, fmt.Errorf("%w: %q conflicts with a directory entry", ErrUnsafeEntry, name)
	}
	tracker.entries[key] = kind
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		tracker.descendantCounts[tracker.key(parent)]++
	}
	return false, nil
}

func normalizeArchiveName(name string, stripComponents int) (string, bool, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", false, fmt.Errorf("%w: path contains a NUL byte", ErrUnsafeEntry)
	}
	name = strings.ReplaceAll(name, `\`, "/")
	if path.IsAbs(name) {
		return "", false, fmt.Errorf("%w: absolute path %q", ErrUnsafeEntry, name)
	}
	parts := make([]string, 0, strings.Count(name, "/")+1)
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", false, fmt.Errorf("%w: path traversal in %q", ErrUnsafeEntry, name)
		}
		if err := validatePathComponent(part); err != nil {
			return "", false, fmt.Errorf("%w: %q: %v", ErrUnsafeEntry, name, err)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 || len(parts) <= stripComponents {
		return "", true, nil
	}
	return strings.Join(parts[stripComponents:], "/"), false, nil
}

func normalizeSymlinkTarget(name, target string) (string, error) {
	if target == "" || strings.ContainsRune(target, '\x00') {
		return "", fmt.Errorf("%w: symbolic link %q has an invalid target", ErrUnsafeEntry, name)
	}
	target = strings.ReplaceAll(target, `\`, "/")
	if path.IsAbs(target) || strings.Contains(target, ":") {
		return "", fmt.Errorf("%w: symbolic link %q has an absolute target", ErrUnsafeEntry, name)
	}
	for _, part := range strings.Split(target, "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if err := validatePathComponent(part); err != nil {
			return "", fmt.Errorf("%w: symbolic link %q has an invalid target", ErrUnsafeEntry, name)
		}
	}
	resolved := path.Clean(path.Join(path.Dir(name), target))
	if resolved == ".." || strings.HasPrefix(resolved, "../") || path.IsAbs(resolved) || resolved == name {
		return "", fmt.Errorf("%w: symbolic link %q escapes or refers to itself", ErrUnsafeEntry, name)
	}
	return target, nil
}

func validateOutputName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: output name must be a single file name", ErrUnsafeEntry)
	}
	if err := validatePathComponent(name); err != nil {
		return fmt.Errorf("%w: output name: %v", ErrUnsafeEntry, err)
	}
	return nil
}

func validatePathComponent(component string) error {
	if strings.ContainsAny(component, `<>:"|?*`) {
		return errors.New("contains a character unsafe on Windows")
	}
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return errors.New("ends with a dot or space")
	}
	for _, r := range component {
		if r < 0x20 || r == 0x7f {
			return errors.New("contains a control character")
		}
	}
	base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return errors.New("uses a reserved Windows name")
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return errors.New("uses a reserved Windows name")
	}
	return nil
}

func safeMode(mode fs.FileMode) fs.FileMode {
	permissions := mode.Perm() & 0o755
	if permissions == 0 {
		return 0o644
	}
	return permissions
}

func joinWithin(root, relative string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: path %q escapes destination", ErrUnsafeEntry, relative)
	}
	return target, nil
}
