package usb

import "fmt"

// USBDrive represents a detected removable USB drive.
type USBDrive struct {
	Path       string // Mount point (e.g., /Volumes/USB or E:\)
	Label      string // Volume label
	SizeBytes  int64  // Total capacity
	FreeBytes  int64  // Available space
	FileSystem string // e.g., exFAT, NTFS, FAT32
}

// String returns a human-readable representation of the drive.
func (d USBDrive) String() string {
	label := d.Label
	if label == "" {
		label = "Unnamed"
	}
	sizeMB := d.SizeBytes / (1024 * 1024)
	freeMB := d.FreeBytes / (1024 * 1024)
	return label + " (" + d.Path + ") " + formatSize(sizeMB) + " total, " + formatSize(freeMB) + " free [" + d.FileSystem + "]"
}

func formatSize(mb int64) string {
	if mb >= 1024 {
		gb := float64(mb) / 1024.0
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%d MB", mb)
}
