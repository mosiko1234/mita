package bundle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FindLFSObjects scans a bare repo's LFS storage and returns paths to all LFS objects.
func FindLFSObjects(bareRepoDir string) ([]string, error) {
	lfsDir := filepath.Join(bareRepoDir, "lfs", "objects")

	if _, err := os.Stat(lfsDir); os.IsNotExist(err) {
		return nil, nil // No LFS objects
	}

	var objects []string
	err := filepath.Walk(lfsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// LFS objects are stored in subdirectories by hash prefix
		rel, err := filepath.Rel(lfsDir, path)
		if err != nil {
			return err
		}
		objects = append(objects, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk LFS objects: %w", err)
	}

	return objects, nil
}

// CopyLFSObjects copies LFS objects from a bare repo to a staging directory.
func CopyLFSObjects(bareRepoDir, destDir string, objects []string) error {
	lfsDir := filepath.Join(bareRepoDir, "lfs", "objects")

	for _, obj := range objects {
		src := filepath.Join(lfsDir, obj)
		dst := filepath.Join(destDir, obj)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("create LFS dir: %w", err)
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy LFS object %s: %w", obj, err)
		}
	}

	return nil
}

// RestoreLFSObjects copies LFS objects from an extracted bundle into a bare repo.
func RestoreLFSObjects(lfsSourceDir, bareRepoDir string) error {
	lfsTargetDir := filepath.Join(bareRepoDir, "lfs", "objects")

	return filepath.Walk(lfsSourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(lfsSourceDir, path)
		if err != nil {
			return err
		}

		dst := filepath.Join(lfsTargetDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		return copyFile(path, dst)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}
