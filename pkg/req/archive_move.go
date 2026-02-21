package req

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

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
