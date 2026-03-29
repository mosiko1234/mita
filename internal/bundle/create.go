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
// Uses /bin/rm like the Python script (not Go os.RemoveAll).
func cleanMacOSJunk(usbPath string) bool {
	if runtime.GOOS != "darwin" {
		return true
	}

	if !isWritable(usbPath) {
		return false
	}

	exec.Command("mdutil", "-i", "off", usbPath).Run()

	hiddenFiles := globAllHidden(usbPath)
	if len(hiddenFiles) > 0 {
		args := append([]string{"-rf"}, hiddenFiles...)
		exec.Command("rm", args...).Run()
	}

	return true
}

// CleanAndEject removes ALL hidden files from the USB then unmounts it.
// This is a direct Go translation of the proven Python cleanup script (clean.py):
//
//	mdutil('-i', 'off', dok_path)
//	hidden_files = list(Path(dok_path).glob('**/.*'))
//	rm('-rf', hidden_files)
//	diskutil['unmount', dok_path]()
//
// IMPORTANT: Only unmount, do NOT eject. The Python script does unmount only,
// and eject causes "unable to mount device" on the classified-side machine.
func CleanAndEject(usbPath string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("unmount is only supported on macOS")
	}

	// Step 1: mdutil -i off
	exec.Command("mdutil", "-i", "off", usbPath).Run()

	// Step 2: Collect all hidden files — Path(dok_path).glob('**/.*')
	hiddenFiles := globAllHidden(usbPath)

	// Step 3: rm -rf <all hidden files> — single call to /bin/rm
	if len(hiddenFiles) > 0 {
		args := append([]string{"-rf"}, hiddenFiles...)
		exec.Command("rm", args...).Run()
	}

	// Step 4: diskutil unmount (NOT eject)
	cmd := exec.Command("diskutil", "unmount", usbPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unmount failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	outStr := strings.TrimSpace(string(out))
	if !strings.Contains(strings.ToLower(outStr), "unmounted") {
		return fmt.Errorf("unmount may have failed: %s", outStr)
	}

	return nil
}

// globAllHidden collects all paths starting with "." under usbPath.
// Equivalent to Python's Path(dok_path).glob('**/.*')
func globAllHidden(usbPath string) []string {
	var hidden []string

	filepath.Walk(usbPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == usbPath {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			hidden = append(hidden, path)
			if info.IsDir() {
				return filepath.SkipDir // don't recurse into hidden dirs, rm -rf handles it
			}
		}
		return nil
	})

	return hidden
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
