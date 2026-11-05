package dezip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func prepareDestination(destination string) (string, string, error) {
	if destination == "" {
		return "", "", errors.New("destination path is required")
	}
	dest, err := filepath.Abs(destination)
	if err != nil {
		return "", "", fmt.Errorf("resolve destination: %w", err)
	}
	dest = filepath.Clean(dest)
	if dest == filepath.VolumeName(dest)+string(filepath.Separator) {
		return "", "", errors.New("destination cannot be a filesystem root")
	}
	if _, err := os.Lstat(dest); err == nil {
		return "", "", ErrDestinationExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("check destination: %w", err)
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", fmt.Errorf("create destination parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".dezip-*")
	if err != nil {
		return "", "", fmt.Errorf("create staging directory: %w", err)
	}
	return dest, stage, nil
}
