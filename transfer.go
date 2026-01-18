package granular

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// Export writes the entire cache contents to a tar archive.
// The archive can be imported later with Import().
func (c *Cache) Export(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tw := tar.NewWriter(w)
	defer tw.Close()

	// Walk the cache root and add all files
	baseDir := c.root
	return afero.Walk(c.fs, baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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
			defer file.Close()
			_, err = io.Copy(tw, file)
			return err
		}

		return nil
	})
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

		// Validate path (security: prevent path traversal)
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		targetPath := filepath.Join(baseDir, header.Name)

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

			file, err := c.fs.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			file.Close()
		}
	}

	return nil
}
