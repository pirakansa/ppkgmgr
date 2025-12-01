package req

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// DecodeFile decodes srcPath according to encoding and writes the result to dstPath.
// Supported encodings include the empty string (no decoding) and "zstd".
func DecodeFile(encoding, srcPath, dstPath string) error {
	if err := ensureDir(dstPath); err != nil {
		return err
	}

	cleaned := strings.TrimSpace(strings.ToLower(encoding))
	switch cleaned {
	case "", "none":
		return copyFile(srcPath, dstPath)
	case "zstd":
		return decodeZstd(srcPath, dstPath)
	default:
		return fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		removeOnError(dstPath)
		return fmt.Errorf("copy file: %w", err)
	}

	if err := dst.Close(); err != nil {
		removeOnError(dstPath)
		return fmt.Errorf("close destination: %w", err)
	}

	return nil
}

func decodeZstd(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	decoder, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("init decoder: %w", err)
	}
	defer decoder.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	if _, err := io.Copy(dst, decoder); err != nil {
		dst.Close()
		removeOnError(dstPath)
		return fmt.Errorf("decode: %w", err)
	}

	if err := dst.Close(); err != nil {
		removeOnError(dstPath)
		return fmt.Errorf("close destination: %w", err)
	}

	return nil
}
