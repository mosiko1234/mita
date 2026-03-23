package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	wroteToUSB := false
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
		wroteToUSB = true
	}

	if err := createZip(stageDir, zipPath); err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}

	// Step 11: Clean macOS hidden files from USB so classified-side scanners don't block it
	if wroteToUSB {
		report("Cleaning macOS hidden files from USB...")
		cleanMacOSJunk(usbPath)
	}

	report("Export complete!")
	return zipPath, nil
}

// cleanMacOSJunk removes hidden macOS metadata files/directories from a USB drive.
// These files (Spotlight indexes, FSEvents, .DS_Store, resource forks, etc.) can cause
// issues when the USB is scanned by classified-network security gateways.
func cleanMacOSJunk(usbPath string) {
	// Directories to remove recursively
	junkDirs := []string{
		".Spotlight-V100",
		".fseventsd",
		".Trashes",
		".TemporaryItems",
	}
	for _, name := range junkDirs {
		path := filepath.Join(usbPath, name)
		os.RemoveAll(path)
	}

	// Individual files to remove
	junkFiles := []string{
		".DS_Store",
		".VolumeIcon.icns",
		".com.apple.timemachine.donotpresent",
	}
	for _, name := range junkFiles {
		path := filepath.Join(usbPath, name)
		os.Remove(path)
	}

	// Remove .DS_Store and ._ resource fork files recursively
	filepath.Walk(usbPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors (permission denied etc.)
		}
		name := info.Name()
		if name == ".DS_Store" || strings.HasPrefix(name, "._") {
			os.Remove(path)
		}
		return nil
	})
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
