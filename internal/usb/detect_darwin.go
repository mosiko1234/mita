//go:build darwin

package usb

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"howett.net/plist"
)

type diskutilOutput struct {
	AllDisksAndPartitions []diskEntry `plist:"AllDisksAndPartitions"`
}

type diskEntry struct {
	DeviceIdentifier string      `plist:"DeviceIdentifier"`
	Content          string      `plist:"Content"`
	Partitions       []partition `plist:"Partitions"`
}

type partition struct {
	DeviceIdentifier string `plist:"DeviceIdentifier"`
	MountPoint       string `plist:"MountPoint"`
	VolumeName       string `plist:"VolumeName"`
	Size             int64  `plist:"Size"`
	Content          string `plist:"Content"`
}

type diskInfo struct {
	Removable        string `plist:"RemovableMedia"`
	RemovableMediaOrExternalDevice string `plist:"RemovableMediaOrExternalDevice"`
	MountPoint       string `plist:"MountPoint"`
	VolumeName       string `plist:"VolumeName"`
	TotalSize        int64  `plist:"TotalSize"`
	DeviceNode       string `plist:"DeviceNode"`
	FilesystemType   string `plist:"FilesystemType"`
}

// DetectUSBDrives finds removable USB drives on macOS using diskutil.
func DetectUSBDrives() ([]USBDrive, error) {
	cmd := exec.Command("diskutil", "list", "-plist", "external")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diskutil list: %w", err)
	}

	var result diskutilOutput
	if _, err := plist.Unmarshal(out, &result); err != nil {
		return fallbackDetect()
	}

	var drives []USBDrive
	for _, disk := range result.AllDisksAndPartitions {
		for _, part := range disk.Partitions {
			if part.MountPoint == "" {
				continue
			}
			info, err := getDiskInfo(part.DeviceIdentifier)
			if err != nil {
				continue
			}
			freeBytes := getFreeSpace(part.MountPoint)
			drives = append(drives, USBDrive{
				Path:       part.MountPoint,
				Label:      part.VolumeName,
				SizeBytes:  info.TotalSize,
				FreeBytes:  freeBytes,
				FileSystem: info.FilesystemType,
			})
		}
	}

	return drives, nil
}

func getDiskInfo(deviceID string) (*diskInfo, error) {
	cmd := exec.Command("diskutil", "info", "-plist", deviceID)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var info diskInfo
	if _, err := plist.Unmarshal(out, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

func getFreeSpace(mountPoint string) int64 {
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

// fallbackDetect scans /Volumes for mounted drives when plist parsing fails.
func fallbackDetect() ([]USBDrive, error) {
	cmd := exec.Command("diskutil", "list", "external")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diskutil list external: %w", err)
	}

	var drives []USBDrive
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "/dev/disk") {
			devID := strings.TrimSpace(strings.Split(line, " ")[0])
			info, err := getDiskInfoText(devID)
			if err != nil || info.Path == "" {
				continue
			}
			drives = append(drives, *info)
		}
	}

	return drives, nil
}

func getDiskInfoText(devPath string) (*USBDrive, error) {
	cmd := exec.Command("diskutil", "info", devPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	drive := &USBDrive{}
	scanner := bytes.NewBuffer(out)
	for {
		line, err := scanner.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Mount Point":
			drive.Path = val
		case "Volume Name":
			drive.Label = val
		case "Disk Size":
			// Parse something like "31.5 GB (31457280000 Bytes)"
			if idx := strings.Index(val, "("); idx > 0 {
				numStr := strings.TrimRight(val[idx+1:], " Bytes)")
				if n, err := strconv.ParseInt(numStr, 10, 64); err == nil {
					drive.SizeBytes = n
				}
			}
		case "File System Personality":
			drive.FileSystem = val
		}
	}

	if drive.Path != "" {
		drive.FreeBytes = getFreeSpace(drive.Path)
	}

	return drive, nil
}
