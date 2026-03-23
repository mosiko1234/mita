package crypto

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CalculateFileSHA256 computes the SHA-256 hash of a file.
func CalculateFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GenerateChecksumFile creates a checksum.sha256 file for all files in a directory.
// The format matches sha256sum output: <hash>  <relative-path>
func GenerateChecksumFile(dir string, files []string, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create checksum file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, file := range files {
		fullPath := filepath.Join(dir, file)
		hash, err := CalculateFileSHA256(fullPath)
		if err != nil {
			return fmt.Errorf("hash %s: %w", file, err)
		}
		fmt.Fprintf(w, "%s  %s\n", hash, file)
	}

	return w.Flush()
}

// VerifyChecksumFile reads a checksum.sha256 file and verifies all entries against files in dir.
func VerifyChecksumFile(checksumPath, dir string) error {
	f, err := os.Open(checksumPath)
	if err != nil {
		return fmt.Errorf("open checksum file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid checksum line: %s", line)
		}

		expectedHash := parts[0]
		relPath := parts[1]
		fullPath := filepath.Join(dir, relPath)

		actualHash, err := CalculateFileSHA256(fullPath)
		if err != nil {
			return fmt.Errorf("verify %s: %w", relPath, err)
		}

		if actualHash != expectedHash {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", relPath, expectedHash, actualHash)
		}
	}

	return scanner.Err()
}
