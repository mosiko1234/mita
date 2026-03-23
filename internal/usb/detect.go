package usb

import (
	"fmt"
	"os"
	"path/filepath"
)

// USBDrive represents a detected removable USB drive.
type USBDrive struct {
	Path       string // Mount point (e.g., /Volumes/USB or E:\)
	Label      string // Volume label
	SizeBytes  int64  // Total capacity
	FreeBytes  int64  // Available space
	FileSystem string // e.g., exFAT, NTFS, FAT32
	ReadOnly   bool   // true if the drive is mounted read-only
}

// String returns a human-readable representation of the drive.
func (d USBDrive) String() string {
	label := d.Label
	if label == "" {
		label = "Unnamed"
	}
	sizeMB := d.SizeBytes / (1024 * 1024)
	freeMB := d.FreeBytes / (1024 * 1024)
	roTag := ""
	if d.ReadOnly {
		roTag = " [READ-ONLY]"
	}
	return label + " (" + d.Path + ") " + formatSize(sizeMB) + " total, " + formatSize(freeMB) + " free [" + d.FileSystem + "]" + roTag
}

// IsWritable tests whether the drive is writable.
func (d USBDrive) IsWritable() bool {
	return !d.ReadOnly
}

// checkWritable tests if a directory is writable by creating/removing a temp file.
func checkWritable(dir string) bool {
	testPath := filepath.Join(dir, ".mita-write-test")
	f, err := os.Create(testPath)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testPath)
	return true
}

func formatSize(mb int64) string {
	if mb >= 1024 {
		gb := float64(mb) / 1024.0
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%d MB", mb)
}
