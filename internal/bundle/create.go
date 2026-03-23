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

// Create packages a Git project into an .mita.zip bundle on the target path (USB).
func Create(client *gitlab.Client, projectName, branch string, shallow bool, usbPath string) (string, error) {
	// Create temporary working directory
	tmpDir, err := os.MkdirTemp("", "mita-export-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Find the project
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
	fmt.Printf("Cloning %s...\n", projectName)
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
	fmt.Println("Fetching LFS objects...")
	gitlab.FetchLFS(cloneDir, client.IsInsecureTLS()) // ignore error if no LFS

	// Step 3: Get commit SHA
	commitSHA := getHeadCommit(cloneDir)

	// Step 4: Create bare repo and push
	fmt.Println("Creating bare repository...")
	if err := gitlab.CreateBareRepo(cloneDir, bareDir); err != nil {
		return "", fmt.Errorf("create bare repo: %w", err)
	}

	// Step 5: Push LFS to bare
	gitlab.PushLFSToBare(cloneDir, bareDir) // ignore error if no LFS

	// Step 6: Create git bundle
	fmt.Println("Creating git bundle...")
	bundleFile := filepath.Join(stageDir, "repo.bundle")
	if err := gitlab.CreateGitBundle(bareDir, bundleFile); err != nil {
		return "", fmt.Errorf("create git bundle: %w", err)
	}

	// Step 7: Copy LFS objects
	lfsObjects, _ := FindLFSObjects(bareDir)
	hasLFS := len(lfsObjects) > 0
	if hasLFS {
		fmt.Printf("Copying %d LFS objects...\n", len(lfsObjects))
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
	fmt.Println("Calculating checksums...")
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

	// Step 10: Create ZIP
	timestamp := time.Now().Format("20060102-150405")
	zipName := fmt.Sprintf("%s-%s.mita.zip", sanitizeName(projectName), timestamp)
	zipPath := filepath.Join(usbPath, zipName)

	fmt.Printf("Creating bundle: %s\n", zipName)
	if err := createZip(stageDir, zipPath); err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}

	return zipPath, nil
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
