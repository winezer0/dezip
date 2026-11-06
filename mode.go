package dezip

import (
	"fmt"
	"strings"
)

// Mode selects whether and how an already downloaded file is processed.
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeZip  Mode = "zip"
	ModeTar  Mode = "tar"
	ModeGzip Mode = "gzip"
	ModeSkip Mode = "skip"
)

// ParseExtractAs converts a DASM-style extract_as value to a Mode. Empty values
// mean skip; "tar.gz"/"tar.xz" and other tar compression labels map to tar;
// "gzip" selects a raw gzip stream.
func ParseExtractAs(value string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "skip":
		return ModeSkip, nil
	case string(ModeAuto):
		return ModeAuto, nil
	case string(ModeZip):
		return ModeZip, nil
	case string(ModeTar), "tar.gz", "tar.xz", "tar.bz2", "tar.lz", "tar.lz4", "tar.br", "tar.sz", "tar.s2", "tar.zz", "tar.zst":
		return ModeTar, nil
	case string(ModeGzip):
		return ModeGzip, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidMode, value)
	}
}

func normalizedMode(mode Mode) (Mode, error) {
	return ParseExtractAs(string(mode))
}
