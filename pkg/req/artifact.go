package req

import (
	"fmt"
	"strings"
)

const (
	encodingTarGzip = "tar+gzip"
	encodingTarXz   = "tar+xz"
)

// DecodeArtifactOptions describes how to decode or extract an artifact.
type DecodeArtifactOptions struct {
	Encoding   string
	SourcePath string
	OutputPath string
	OutputDir  string
	Extract    string
	Rename     string
}

// DecodeArtifact decodes an artifact and returns the resulting output path.
// For full archive extraction, the returned path is empty because multiple
// outputs may be produced.
func DecodeArtifact(options DecodeArtifactOptions) (string, error) {
	encoding := normalizeEncoding(options.Encoding)

	if IsArchiveEncoding(encoding) {
		if strings.TrimSpace(options.OutputDir) == "" {
			return "", fmt.Errorf("output directory is required for encoding %q", options.Encoding)
		}
		return extractArchive(encoding, options.SourcePath, options.OutputDir, options.Extract, options.Rename)
	}

	if strings.TrimSpace(options.OutputPath) == "" {
		return "", fmt.Errorf("output path is required for encoding %q", options.Encoding)
	}

	if err := DecodeFile(encoding, options.SourcePath, options.OutputPath); err != nil {
		return "", err
	}

	return options.OutputPath, nil
}

func normalizeEncoding(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

// IsArchiveEncoding reports whether encoding is an archive type supported by
// artifact extraction.
func IsArchiveEncoding(encoding string) bool {
	switch normalizeEncoding(encoding) {
	case encodingTarGzip, encodingTarXz:
		return true
	default:
		return false
	}
}
