package req

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

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
		case tar.TypeReg, 0:
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
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create parent directory for symlink %q: %w", path, err)
			}
			linkTarget := strings.TrimSpace(header.Linkname)
			if linkTarget == "" {
				return fmt.Errorf("empty symlink target for %q", header.Name)
			}
			if filepath.IsAbs(linkTarget) {
				return fmt.Errorf("absolute symlink target is not allowed for %q: %q", header.Name, header.Linkname)
			}
			if err := os.Symlink(linkTarget, path); err != nil {
				return fmt.Errorf("create symlink %q -> %q: %w", path, linkTarget, err)
			}
		case tar.TypeLink:
			linkTarget, err := safeRelativePath(header.Linkname)
			if err != nil {
				return fmt.Errorf("invalid hard link target %q for %q: %w", header.Linkname, header.Name, err)
			}
			if linkTarget == "." {
				return fmt.Errorf("invalid hard link target %q for %q", header.Linkname, header.Name)
			}
			targetPath := filepath.Join(dstDir, linkTarget)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("create parent directory for hard link %q: %w", path, err)
			}
			if err := os.Link(targetPath, path); err != nil {
				return fmt.Errorf("create hard link %q -> %q: %w", path, targetPath, err)
			}
		default:
			return fmt.Errorf("unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}
}
