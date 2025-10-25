package plugins

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func unpackZipArchive(path, dest string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open artifact archive: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if err := extractZipEntry(file, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(entry *zip.File, dest string) error {
	cleaned := filepath.Clean(entry.Name)
	if cleaned == "." || cleaned == "" {
		return nil
	}
	target := filepath.Join(dest, cleaned)
	if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
		return fmt.Errorf("artifact entry escapes destination: %s", entry.Name)
	}

	if entry.FileInfo().IsDir() {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("create artifact directory: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("prepare artifact path: %w", err)
	}

	reader, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open artifact entry: %w", err)
	}
	defer reader.Close()

	temp, err := os.CreateTemp(filepath.Dir(target), "entry-*.tmp")
	if err != nil {
		return fmt.Errorf("create artifact temp file: %w", err)
	}
	tempPath := temp.Name()
	if _, err := io.Copy(temp, reader); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return fmt.Errorf("write artifact entry: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("close artifact entry: %w", err)
	}
	if err := os.Rename(tempPath, target); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("finalize artifact entry: %w", err)
	}
	if mode := entry.Mode(); mode != 0 {
		os.Chmod(target, mode)
	}
	return nil
}
