package req

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
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

// ExtractArchive extracts a supported archive into dstDir.
// If extractPath is provided, only that file/directory is moved into dstDir,
// optionally renamed when rename is non-empty. The returned string is the
// resulting output path for extractPath mode; for full extraction it returns "".
func ExtractArchive(encoding, srcPath, dstDir, extractPath, rename string) (string, error) {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", fmt.Errorf("create destination directory: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ppkgmgr-extract-*")
	if err != nil {
		return "", fmt.Errorf("create temp extraction directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	switch strings.TrimSpace(strings.ToLower(encoding)) {
	case "tar+gzip":
		if err := decodeTarGzip(srcPath, tmpDir); err != nil {
			return "", err
		}
	case "tar+xz":
		if err := decodeTarXz(srcPath, tmpDir); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encoding)
	}

	cleanExtract := strings.TrimSpace(extractPath)
	if cleanExtract == "" || cleanExtract == "." {
		if err := moveDirectoryContents(tmpDir, dstDir); err != nil {
			return "", fmt.Errorf("move extracted contents: %w", err)
		}
		return "", nil
	}

	cleanedPath, err := safeRelativePath(cleanExtract)
	if err != nil {
		return "", fmt.Errorf("invalid extract path %q: %w", cleanExtract, err)
	}

	sourcePath := filepath.Join(tmpDir, cleanedPath)
	if _, err := os.Stat(sourcePath); err != nil {
		return "", fmt.Errorf("extract path %q not found in archive: %w", cleanExtract, err)
	}

	targetName := filepath.Base(cleanedPath)
	if strings.TrimSpace(rename) != "" {
		targetName = sanitizeOutputName(rename)
	}

	destinationPath := filepath.Join(dstDir, targetName)
	if err := movePath(sourcePath, destinationPath); err != nil {
		return "", fmt.Errorf("move extracted path: %w", err)
	}

	return destinationPath, nil
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

func decodeTarGzip(srcPath, dstDir string) error {
	source, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer source.Close()

	gzr, err := gzip.NewReader(source)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer gzr.Close()

	if err := extractTarStream(tar.NewReader(gzr), dstDir); err != nil {
		return fmt.Errorf("extract tar+gzip: %w", err)
	}

	return nil
}

func decodeTarXz(srcPath, dstDir string) error {
	source, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer source.Close()

	xzr, err := xz.NewReader(source)
	if err != nil {
		return fmt.Errorf("open xz reader: %w", err)
	}

	if err := extractTarStream(tar.NewReader(xzr), dstDir); err != nil {
		return fmt.Errorf("extract tar+xz: %w", err)
	}

	return nil
}

func extractTarStream(reader *tar.Reader, dstDir string) error {
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		rel, err := safeRelativePath(header.Name)
		if err != nil {
			return fmt.Errorf("invalid tar entry %q: %w", header.Name, err)
		}
		if rel == "." {
			continue
		}

		path := filepath.Join(dstDir, rel)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, fs.FileMode(header.Mode).Perm()); err != nil {
				return fmt.Errorf("create directory %q: %w", path, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", path, err)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(header.Mode).Perm())
			if err != nil {
				return fmt.Errorf("create file %q: %w", path, err)
			}
			if _, err := io.Copy(file, reader); err != nil {
				file.Close()
				return fmt.Errorf("write file %q: %w", path, err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close file %q: %w", path, err)
			}
		default:
			return fmt.Errorf("unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}
}

func safeRelativePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return ".", nil
	}
	if filepath.IsAbs(cleaned) {
		return "", errors.New("absolute paths are not allowed")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal is not allowed")
	}
	return cleaned, nil
}

func sanitizeOutputName(name string) string {
	trimmed := strings.TrimSpace(name)
	if filepath.IsAbs(trimmed) {
		trimmed = strings.TrimPrefix(trimmed, filepath.VolumeName(trimmed))
		trimmed = strings.TrimLeft(trimmed, "/\\")
	}
	return filepath.Clean(trimmed)
}

func moveDirectoryContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read extracted directory: %w", err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if err := movePath(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func movePath(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create destination parent: %w", err)
	}
	_ = os.RemoveAll(dstPath)

	if err := os.Rename(srcPath, dstPath); err == nil {
		return nil
	}

	info, err := os.Lstat(srcPath)
	if err != nil {
		return fmt.Errorf("stat source path: %w", err)
	}
	if info.IsDir() {
		if err := copyDirectory(srcPath, dstPath, info.Mode().Perm()); err != nil {
			return err
		}
	} else {
		if err := copyRegularFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(srcPath); err != nil {
		return fmt.Errorf("cleanup source path: %w", err)
	}
	return nil
}

func copyDirectory(srcDir, dstDir string, perm fs.FileMode) error {
	if err := os.MkdirAll(dstDir, perm); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat source entry: %w", err)
		}
		if info.IsDir() {
			if err := copyDirectory(srcPath, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
			continue
		}
		if err := copyRegularFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
			return err
		}
	}

	return nil
}

func copyRegularFile(srcPath, dstPath string, perm fs.FileMode) error {
	source, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()

	destination, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return fmt.Errorf("copy file: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close destination file: %w", err)
	}
	return nil
}
