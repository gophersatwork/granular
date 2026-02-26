package granular

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// validateArchivePath checks that a path from an archive entry is safe to extract.
// It rejects path traversal attempts (absolute paths, ".." components) and ensures
// the resolved path stays within the target directory.
func validateArchivePath(name, baseDir string) (string, error) {
	// Reject absolute paths
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path in archive: %s", name)
	}

	// Clean the path and reject any ".." components
	cleaned := filepath.Clean(name)
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path traversal in archive: %s", name)
	}

	// Join with base and verify the result is within baseDir
	target := filepath.Join(baseDir, cleaned)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %s: %w", name, err)
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base dir: %w", err)
	}

	// Ensure target is within base directory
	if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) && absTarget != absBase {
		return "", fmt.Errorf("path escapes cache directory: %s", name)
	}

	return target, nil
}

// Export writes the entire cache contents to a tar archive.
// The archive can be imported later with Import().
func (c *Cache) Export(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Require Lstater to detect symlinks. Without it, afero.Walk follows symlinks
	// via Stat, which could leak files outside the cache directory into the archive.
	lstater, ok := c.fs.(afero.Lstater)
	if !ok {
		return fmt.Errorf("filesystem does not support Lstat; export requires symlink detection to prevent data leakage")
	}

	tw := tar.NewWriter(w)

	// Walk the cache root and add all files.
	// Uses Lstat to avoid following symlinks that could leak files outside the cache.
	baseDir := c.root
	walkErr := afero.Walk(c.fs, baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent leaking files outside the cache.
		// afero.Walk uses Stat which follows symlinks, so we re-check with Lstat.
		linfo, _, lErr := lstater.LstatIfPossible(path)
		if lErr == nil && linfo.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Get relative path for archive
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Write file contents
		if !info.IsDir() {
			file, err := c.fs.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			closeErr := file.Close()
			return errors.Join(copyErr, closeErr)
		}

		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	// Close tar writer explicitly to flush the footer and catch write errors.
	// A deferred Close() would discard this error, producing a silently corrupt archive.
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to finalize tar archive: %w", err)
	}

	return nil
}

// Import reads a tar archive and populates the cache.
// Existing entries with the same keys will be overwritten.
func (c *Cache) Import(r io.Reader) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tr := tar.NewReader(r)
	baseDir := c.root

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Reject symlinks and other non-regular types from archive
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return fmt.Errorf("symlinks and hardlinks not allowed in archive: %s", header.Name)
		}

		// Validate path (security: prevent path traversal, absolute paths, symlinks)
		targetPath, err := validateArchivePath(header.Name, baseDir)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := c.fs.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := c.fs.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Use atomic write (tmp + rename) to avoid partial files on crash
			tmpPath := targetPath + ".tmp." + randomSuffix()
			file, err := c.fs.Create(tmpPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				_ = c.fs.Remove(tmpPath)
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			file.Close()
			if err := c.fs.Rename(tmpPath, targetPath); err != nil {
				_ = c.fs.Remove(tmpPath)
				return fmt.Errorf("failed to rename temp file %s: %w", targetPath, err)
			}
		}
	}

	// Verify imported manifests by re-computing output hashes.
	// This detects corruption or tampering in the archive.
	var walkErr error
	var corruptedKeys []string
	for keyHash, m := range c.manifests(&walkErr, &corruptedKeys) {
		if err := c.verifyOutputHash(m); err != nil {
			// Remove the corrupted entry
			_ = c.removeByHash(keyHash)
			return fmt.Errorf("imported entry %s failed integrity check: %w", keyHash, err)
		}
	}

	c.cleanupCorrupted(corruptedKeys)

	return walkErr
}
