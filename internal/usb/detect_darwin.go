//go:build darwin

package usb

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DetectUSBDrives finds removable USB drives on macOS using diskutil.
func DetectUSBDrives() ([]USBDrive, error) {
	// Get list of external disks (text output — simpler and more reliable)
	cmd := exec.Command("diskutil", "list", "external")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diskutil list external: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	// Find all partition identifiers from the diskutil list output.
	// Lines look like:
	//   1:               Windows_NTFS Transcend               60.5 GB    disk2s1
	// Partition IDs are "disk<N>s<M>" — we need the LAST "s" to split disk from partition.
	var partIDs []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		lastField := fields[len(fields)-1]
		// Partition identifiers: "disk2s1", "disk3s2", etc.
		if strings.HasPrefix(lastField, "disk") {
			idx := strings.LastIndex(lastField, "s")
			if idx < 0 || idx == len(lastField)-1 {
				continue
			}
			suffix := lastField[idx+1:]
			if _, err := strconv.Atoi(suffix); err == nil {
				// Also verify the part before "s" ends with a digit (disk number)
				diskPart := lastField[4:idx] // remove "disk" prefix
				if _, err := strconv.Atoi(diskPart); err == nil {
					partIDs = append(partIDs, lastField)
				}
			}
		}
	}

	var drives []USBDrive
	for _, partID := range partIDs {
		drive, err := getPartitionInfo(partID)
		if err != nil || drive == nil || drive.Path == "" {
			continue
		}
		drives = append(drives, *drive)
	}

	return drives, nil
}

// getPartitionInfo uses `diskutil info` (text output) to get details about a partition.
func getPartitionInfo(partID string) (*USBDrive, error) {
	cmd := exec.Command("diskutil", "info", partID)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	drive := &USBDrive{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		switch key {
		case "Mount Point":
			drive.Path = val
		case "Volume Name":
			drive.Label = val
		case "Total Size":
			drive.SizeBytes = parseSizeBytes(val)
		case "Disk Size":
			if drive.SizeBytes == 0 {
				drive.SizeBytes = parseSizeBytes(val)
			}
		case "Volume Free Space", "Container Free Space":
			drive.FreeBytes = parseSizeBytes(val)
		case "File System Personality":
			if drive.FileSystem == "" {
				drive.FileSystem = val
			}
		case "Type (Bundle)":
			if drive.FileSystem == "" {
				drive.FileSystem = val
			}
		}
	}

	// Fallback: get free space via df if diskutil didn't provide it
	if drive.FreeBytes == 0 && drive.Path != "" {
		drive.FreeBytes = getFreeSpaceDf(drive.Path)
	}

	// Skip unmounted partitions
	if drive.Path == "" {
		return nil, nil
	}

	return drive, nil
}

// parseSizeBytes extracts byte count from diskutil output like:
// "60.5 GB (60472324096 Bytes)" or "60472324096"
func parseSizeBytes(s string) int64 {
	// Look for "(NNN Bytes)" pattern
	if idx := strings.Index(s, "("); idx >= 0 {
		inner := s[idx+1:]
		if end := strings.Index(inner, " Byte"); end > 0 {
			numStr := strings.TrimSpace(inner[:end])
			if n, err := strconv.ParseInt(numStr, 10, 64); err == nil {
				return n
			}
		}
	}

	// Try plain number
	if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return n
	}

	return 0
}

// getFreeSpaceDf gets free space using the df command as fallback.
func getFreeSpaceDf(mountPoint string) int64 {
	cmd := exec.Command("df", "-k", mountPoint)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0
	}
	available, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0
	}
	return available * 1024 // df -k reports in 1K blocks
}
