package gitlab

import (
	"fmt"
	"os"
	"os/exec"
)

// CloneOptions configures a git clone operation.
type CloneOptions struct {
	URL         string
	Branch      string
	Destination string
	Shallow     bool // if true, clone with --depth 1
	InsecureTLS bool
}

// Clone clones a Git repository from GitLab.
func Clone(opts CloneOptions) error {
	args := []string{"clone"}

	if opts.Shallow {
		args = append(args, "--depth", "1")
	}

	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}

	args = append(args, opts.URL, opts.Destination)

	cmd := exec.Command("git", args...)

	// Build environment: disable interactive prompts + optional insecure TLS
	env := os.Environ()
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	if opts.InsecureTLS {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, string(out))
	}

	return nil
}

// FetchLFS fetches all LFS objects for a cloned repository.
func FetchLFS(repoDir string, insecureTLS bool) error {
	cmd := exec.Command("git", "lfs", "fetch", "--all")
	cmd.Dir = repoDir

	env := os.Environ()
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	if insecureTLS {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git lfs fetch: %w\n%s", err, string(out))
	}

	return nil
}

// CreateBareRepo initializes a bare repository and pushes all refs to it.
func CreateBareRepo(sourceDir, bareDir string) error {
	// Init bare repo
	out, err := exec.Command("git", "init", "--bare", bareDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init --bare: %w\n%s", err, string(out))
	}

	// Add bare repo as remote and push all
	cmd := exec.Command("git", "remote", "add", "bare", bareDir)
	cmd.Dir = sourceDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git remote add bare: %w", err)
	}

	cmd = exec.Command("git", "push", "bare", "--all")
	cmd.Dir = sourceDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push --all to bare: %w\n%s", err, string(out))
	}

	cmd = exec.Command("git", "push", "bare", "--tags")
	cmd.Dir = sourceDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push --tags to bare: %w\n%s", err, string(out))
	}

	return nil
}

// CreateGitBundle creates a git bundle file from a bare repository.
func CreateGitBundle(bareDir, bundlePath string) error {
	cmd := exec.Command("git", "bundle", "create", bundlePath, "--all")
	cmd.Dir = bareDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git bundle create: %w\n%s", err, string(out))
	}

	return nil
}

// PushLFSToBare pushes all LFS objects to a bare repository.
func PushLFSToBare(sourceDir, bareDir string) error {
	cmd := exec.Command("git", "lfs", "push", "--all", bareDir)
	cmd.Dir = sourceDir

	if err := cmd.Run(); err != nil {
		// LFS push might fail if no LFS objects, that's OK
		return nil
	}

	return nil
}
