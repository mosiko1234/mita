package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mita/internal/config"
	"mita/internal/crypto"
	"mita/internal/gitlab"
)

// ScanForBundles finds all .mita.zip files on a USB drive path.
func ScanForBundles(usbPath string) ([]string, error) {
	var bundles []string

	entries, err := os.ReadDir(usbPath)
	if err != nil {
		return nil, fmt.Errorf("read USB directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".mita.zip") {
			bundles = append(bundles, entry.Name())
		}
	}

	return bundles, nil
}

// Extract extracts a bundle, verifies integrity, and pushes to target GitLab.
func Extract(bundleName, usbPath string, mapping config.MappingEntry, client *gitlab.Client, cfg *config.Config) error {
	zipPath := filepath.Join(usbPath, bundleName)

	// Create temp dir for extraction
	tmpDir, err := os.MkdirTemp("", "mita-import-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extracted")

	// Step 1: Extract ZIP
	fmt.Printf("Extracting %s...\n", bundleName)
	if err := extractZip(zipPath, extractDir); err != nil {
		return fmt.Errorf("extract zip: %w", err)
	}

	// Step 2: Verify checksums
	fmt.Println("Verifying integrity...")
	checksumFile := filepath.Join(extractDir, "checksum.sha256")
	if err := crypto.VerifyChecksumFile(checksumFile, extractDir); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}
	fmt.Println("Integrity verified OK")

	// Step 3: Read manifest
	manifest, err := ReadManifest(filepath.Join(extractDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	fmt.Printf("Source: %s (branch: %s, type: %s)\n", manifest.SourceProject, manifest.Branch, manifest.TransferType)

	// Step 4: Clone from bundle into bare repo
	bareDir := filepath.Join(tmpDir, "bare-repo")
	bundleFile := filepath.Join(extractDir, "repo.bundle")

	fmt.Println("Creating repository from bundle...")
	if err := cloneFromBundle(bundleFile, bareDir); err != nil {
		return fmt.Errorf("clone from bundle: %w", err)
	}

	// Step 5: Restore LFS objects
	lfsDir := filepath.Join(extractDir, "lfs")
	if manifest.HasLFS {
		fmt.Printf("Restoring %d LFS objects...\n", manifest.LFSObjectsCount)
		if err := RestoreLFSObjects(lfsDir, bareDir); err != nil {
			return fmt.Errorf("restore LFS objects: %w", err)
		}
	}

	// Step 6: Push to target GitLab
	remoteURL := gitlab.BuildRemoteURL(
		cfg.TargetGitLabURL,
		mapping.TargetGroup,
		mapping.TargetProject,
		cfg.TargetUsername,
		cfg.TargetPassword,
	)

	fmt.Printf("Pushing to %s/%s...\n", mapping.TargetGroup, mapping.TargetProject)
	pushOpts := gitlab.PushOptions{
		RepoDir:     bareDir,
		RemoteURL:   remoteURL,
		InsecureTLS: cfg.InsecureTLS,
	}

	if err := gitlab.PushAll(pushOpts); err != nil {
		return fmt.Errorf("push to target: %w", err)
	}

	// Step 7: Push LFS
	if manifest.HasLFS {
		fmt.Println("Pushing LFS objects...")
		gitlab.PushLFS(bareDir, remoteURL, cfg.InsecureTLS)
	}

	return nil
}

func cloneFromBundle(bundlePath, destDir string) error {
	cmd := newGitCommand("clone", "--bare", bundlePath, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone from bundle: %w", err)
	}

	return nil
}

func newGitCommand(args ...string) *exec.Cmd {
	return exec.Command("git", args...)
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Security: prevent zip slip
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
