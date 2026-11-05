package dezip

import "errors"

func normalizedOptions(options Options) (Options, error) {
	if options.MaxEntries < 0 {
		return Options{}, errors.New("max entries cannot be negative")
	}
	if options.MaxExpandedBytes < 0 {
		return Options{}, errors.New("maximum expanded bytes cannot be negative")
	}
	if options.StripComponents < 0 {
		return Options{}, errors.New("strip components cannot be negative")
	}
	if options.MaxEntries == 0 {
		options.MaxEntries = defaultMaxEntries
	}
	if options.MaxExpandedBytes == 0 {
		options.MaxExpandedBytes = defaultMaxExpandedBytes
	}
	return options, nil
}
