package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"mita/internal/crypto"
	"mita/internal/gitlab"
)

// ProgressFunc is called during export to report progress steps.
type ProgressFunc func(step string)

// Create packages a Git project into an .mita.zip bundle on the target path (USB).
// The progress callback is optional; pass nil to suppress output.
func Create(client *gitlab.Client, projectName, branch string, shallow bool, usbPath string, onProgress ProgressFunc) (string, error) {
	report := func(step string) {
		if onProgress != nil {
			onProgress(step)
		}
	}

	// Create temporary working directory
	tmpDir, err := os.MkdirTemp("", "mita-export-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Find the project
	report("Finding project...")
	projects, err := client.ListProjects(projectName)
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}

	var project *gitlab.Project
	for i, p := range projects {
		if p.Name == projectName || p.PathWithNS == projectName {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		return "", fmt.Errorf("project %q not found", projectName)
	}

	// Build clone URL with embedded credentials so git doesn't prompt interactively
	cloneURL := project.HTTPURL
	if client.Username() != "" && client.Password() != "" {
		cloneURL = gitlab.EmbedCredentialsInURL(cloneURL, client.Username(), client.Password())
	} else if client.Token() != "" {
		cloneURL = gitlab.EmbedTokenInURL(cloneURL, client.Token())
	}
	cloneDir := filepath.Join(tmpDir, "clone")
	bareDir := filepath.Join(tmpDir, "bare")
	stageDir := filepath.Join(tmpDir, "stage")

	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return "", fmt.Errorf("create stage dir: %w", err)
	}

	// Step 1: Clone
	report(fmt.Sprintf("Cloning %s...", projectName))
	cloneOpts := gitlab.CloneOptions{
		URL:         cloneURL,
		Branch:      branch,
		Destination: cloneDir,
		Shallow:     shallow,
		InsecureTLS: client.IsInsecureTLS(),
	}
	if err := gitlab.Clone(cloneOpts); err != nil {
		return "", fmt.Errorf("clone: %w", err)
	}

	// Step 2: Fetch LFS objects
	report("Fetching LFS objects...")
	gitlab.FetchLFS(cloneDir, client.IsInsecureTLS()) // ignore error if no LFS

	// Step 3: Get commit SHA
	commitSHA := getHeadCommit(cloneDir)

	// Step 4: Create bare repo and push
	report("Creating bare repository...")
	if err := gitlab.CreateBareRepo(cloneDir, bareDir); err != nil {
		return "", fmt.Errorf("create bare repo: %w", err)
	}

	// Step 5: Push LFS to bare
	gitlab.PushLFSToBare(cloneDir, bareDir) // ignore error if no LFS

	// Step 6: Create git bundle
	report("Creating git bundle...")
	bundleFile := filepath.Join(stageDir, "repo.bundle")
	if err := gitlab.CreateGitBundle(bareDir, bundleFile); err != nil {
		return "", fmt.Errorf("create git bundle: %w", err)
	}

	// Step 7: Copy LFS objects
	lfsObjects, _ := FindLFSObjects(bareDir)
	hasLFS := len(lfsObjects) > 0
	if hasLFS {
		report(fmt.Sprintf("Copying %d LFS objects...", len(lfsObjects)))
		lfsStageDir := filepath.Join(stageDir, "lfs")
		if err := CopyLFSObjects(bareDir, lfsStageDir, lfsObjects); err != nil {
			return "", fmt.Errorf("copy LFS objects: %w", err)
		}
	}

	// Step 8: Create manifest
	transferType := "shallow"
	if !shallow {
		transferType = "full"
	}

	bundleInfo, _ := os.Stat(bundleFile)
	var bundleSize int64
	if bundleInfo != nil {
		bundleSize = bundleInfo.Size()
	}

	manifest := NewManifest(projectName, client.BaseURL(), branch, commitSHA, transferType)
	manifest.HasLFS = hasLFS
	manifest.LFSObjectsCount = len(lfsObjects)
	manifest.BundleSizeBytes = bundleSize

	manifestPath := filepath.Join(stageDir, "manifest.json")
	if err := manifest.WriteToFile(manifestPath); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	// Step 9: Calculate checksums
	report("Calculating checksums...")
	checksumFiles := []string{"manifest.json", "repo.bundle"}
	if hasLFS {
		for _, obj := range lfsObjects {
			checksumFiles = append(checksumFiles, filepath.Join("lfs", obj))
		}
	}

	checksumPath := filepath.Join(stageDir, "checksum.sha256")
	if err := crypto.GenerateChecksumFile(stageDir, checksumFiles, checksumPath); err != nil {
		return "", fmt.Errorf("generate checksums: %w", err)
	}

	// Step 10: Create ZIP — try USB first, fallback to Desktop if read-only
	timestamp := time.Now().Format("20060102-150405")
	zipName := fmt.Sprintf("%s-%s.mita.zip", sanitizeName(projectName), timestamp)
	zipPath := filepath.Join(usbPath, zipName)

	// Check if USB is writable before attempting
	if !isWritable(usbPath) {
		// Fallback: write to ~/Desktop and let user copy manually
		home, _ := os.UserHomeDir()
		fallbackDir := filepath.Join(home, "Desktop")
		if _, err := os.Stat(fallbackDir); os.IsNotExist(err) {
			fallbackDir = home
		}
		zipPath = filepath.Join(fallbackDir, zipName)
		report(fmt.Sprintf("USB is read-only (NTFS on macOS?). Saving to: %s", fallbackDir))
	} else {
		report(fmt.Sprintf("Writing bundle to USB: %s", zipName))
	}

	if err := createZip(stageDir, zipPath); err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}

	// Step 11: Clean macOS hidden files from USB.
	report("Cleaning macOS metadata files from USB...")
	if !cleanMacOSJunk(usbPath) {
		report("WARNING: Could not clean macOS files (USB is read-only). Format USB as exFAT to fix this.")
	} else {
		// Check if .Spotlight-V100 survived cleanup
		spotlightCheck := filepath.Join(usbPath, ".Spotlight-V100")
		if _, err := os.Stat(spotlightCheck); err == nil {
			report("TIP: .Spotlight-V100 could not be removed. Grant Terminal 'Full Disk Access' in System Settings > Privacy & Security to fix this.")
		}
	}

	report("Export complete!")
	return zipPath, nil
}

// cleanMacOSJunk removes hidden macOS metadata files from a USB drive.
//
// On macOS, .Spotlight-V100 is protected by SIP (SF_RESTRICTED flag) and CANNOT
// be deleted by any user-space process — not even root. This is a kernel-level
// restriction. The directory is created immediately on mount by diskarbitrationd.
//
// What we CAN clean: .fseventsd, .Trashes, .DS_Store, ._ resource forks,
// .TemporaryItems, .VolumeIcon.icns — these are all deletable.
//
// .Spotlight-V100 on a freshly formatted drive is an empty directory and should
// not cause issues with classified-side scanners.
func cleanMacOSJunk(usbPath string) bool {
	if runtime.GOOS != "darwin" {
		return true
	}

	if !isWritable(usbPath) {
		return false
	}

	// Disable Spotlight indexing on this volume to prevent index files
	exec.Command("mdutil", "-d", usbPath).Run()
	exec.Command("mdutil", "-i", "off", usbPath).Run()

	// Merge ._ resource fork files into their parent files
	exec.Command("dot_clean", "-m", usbPath).Run()

	// Remove deletable directories
	for _, name := range []string{".fseventsd", ".Trashes", ".TemporaryItems"} {
		os.RemoveAll(filepath.Join(usbPath, name))
	}

	// Remove deletable files
	for _, name := range []string{".DS_Store", ".VolumeIcon.icns", ".com.apple.timemachine.donotpresent", ".metadata_never_index"} {
		os.Remove(filepath.Join(usbPath, name))
	}

	// Recursively remove .DS_Store and ._ resource fork files
	filepath.Walk(usbPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if name == ".DS_Store" || strings.HasPrefix(name, "._") {
			os.Remove(path)
		}
		return nil
	})

	// Try to remove .Spotlight-V100 — requires Full Disk Access for Terminal.
	// First try normal rm, then try via osascript with admin privileges.
	spotlightPath := filepath.Join(usbPath, ".Spotlight-V100")
	if _, err := os.Stat(spotlightPath); err == nil {
		// Try 1: direct removal (works if Terminal has Full Disk Access)
		if os.RemoveAll(spotlightPath) != nil {
			// Try 2: sudo rm via osascript (shows password prompt)
			script := fmt.Sprintf(
				`do shell script "rm -rf %q && rm -rf %q" with administrator privileges`,
				spotlightPath,
				filepath.Join(usbPath, ".Trashes"),
			)
			exec.Command("osascript", "-e", script).Run()
		}
	}

	// Final pass: clean any ._ resource forks and .fseventsd that may have been
	// recreated by macOS after the Spotlight deletion above
	for _, name := range []string{".fseventsd", ".Trashes", ".TemporaryItems"} {
		os.RemoveAll(filepath.Join(usbPath, name))
	}
	filepath.Walk(usbPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if name == ".DS_Store" || strings.HasPrefix(name, "._") {
			os.Remove(path)
		}
		return nil
	})

	return true
}

// findDeviceForMount finds the disk identifier (e.g. "disk2s2") for a mount point.
func findDeviceForMount(mountPoint string) string {
	cmd := exec.Command("mount")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, mountPoint) {
			fields := strings.Fields(line)
			if len(fields) >= 1 && strings.HasPrefix(fields[0], "/dev/") {
				return strings.TrimPrefix(fields[0], "/dev/")
			}
		}
	}
	return ""
}

// isWritable tests whether a path is writable by creating and removing a temp file.
func isWritable(dir string) bool {
	testPath := filepath.Join(dir, ".mita-write-test")
	f, err := os.Create(testPath)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testPath)
	return true
}

func getHeadCommit(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func createZip(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(writer, f)
		return err
	})
}
