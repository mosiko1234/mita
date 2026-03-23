//go:build windows

package usb

import (
	"fmt"
	"syscall"
	"unsafe"
)

const DRIVE_REMOVABLE = 2

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	getLogicalDrives      = kernel32.NewProc("GetLogicalDrives")
	getDriveTypeW         = kernel32.NewProc("GetDriveTypeW")
	getVolumeInformationW = kernel32.NewProc("GetVolumeInformationW")
	getDiskFreeSpaceExW   = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// DetectUSBDrives finds removable USB drives on Windows.
func DetectUSBDrives() ([]USBDrive, error) {
	mask, _, err := getLogicalDrives.Call()
	if mask == 0 {
		return nil, fmt.Errorf("GetLogicalDrives: %w", err)
	}

	var drives []USBDrive

	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}

		letter := string(rune('A'+i)) + ":\\"
		rootPath, _ := syscall.UTF16PtrFromString(letter)

		driveType, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(rootPath)))
		if driveType != DRIVE_REMOVABLE {
			continue
		}

		label := getVolumeLabel(rootPath)
		fsName := getFileSystem(rootPath)
		totalBytes, freeBytes := getDiskSpace(rootPath)

		drives = append(drives, USBDrive{
			Path:       letter,
			Label:      label,
			SizeBytes:  totalBytes,
			FreeBytes:  freeBytes,
			FileSystem: fsName,
		})
	}

	return drives, nil
}

func getVolumeLabel(rootPath *uint16) string {
	var volumeName [256]uint16
	getVolumeInformationW.Call(
		uintptr(unsafe.Pointer(rootPath)),
		uintptr(unsafe.Pointer(&volumeName[0])),
		uintptr(len(volumeName)),
		0, 0, 0, 0, 0,
	)
	return syscall.UTF16ToString(volumeName[:])
}

func getFileSystem(rootPath *uint16) string {
	var fsName [256]uint16
	getVolumeInformationW.Call(
		uintptr(unsafe.Pointer(rootPath)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&fsName[0])),
		uintptr(len(fsName)),
	)
	return syscall.UTF16ToString(fsName[:])
}

func getDiskSpace(rootPath *uint16) (int64, int64) {
	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(rootPath)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	return totalBytes, freeBytesAvailable
}
