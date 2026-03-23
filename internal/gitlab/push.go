package gitlab

import (
	"fmt"
	"os"
	"os/exec"
)

// PushOptions configures a git push operation to a target GitLab.
type PushOptions struct {
	RepoDir     string
	RemoteURL   string
	InsecureTLS bool
}

// PushAll pushes all branches and tags to the remote.
func PushAll(opts PushOptions) error {
	env := os.Environ()
	if opts.InsecureTLS {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	// Add remote
	cmd := exec.Command("git", "remote", "add", "target", opts.RemoteURL)
	cmd.Dir = opts.RepoDir
	cmd.Env = env
	// Ignore error if remote already exists
	cmd.Run()

	// Push all branches
	cmd = exec.Command("git", "push", "target", "--all")
	cmd.Dir = opts.RepoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push --all: %w", err)
	}

	// Push all tags
	cmd = exec.Command("git", "push", "target", "--tags")
	cmd.Dir = opts.RepoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push --tags: %w", err)
	}

	return nil
}

// PushLFS pushes all LFS objects to the remote.
func PushLFS(repoDir, remoteURL string, insecureTLS bool) error {
	env := os.Environ()
	if insecureTLS {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	cmd := exec.Command("git", "lfs", "push", "--all", "target")
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		// LFS push might fail if no LFS objects
		return nil
	}

	return nil
}

// BuildRemoteURL constructs a GitLab HTTP URL with embedded credentials.
func BuildRemoteURL(baseURL, group, project, username, password string) string {
	// Remove protocol prefix
	host := baseURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(host) > len(prefix) && host[:len(prefix)] == prefix {
			host = host[len(prefix):]
			break
		}
	}
	// Remove trailing slash
	for len(host) > 0 && host[len(host)-1] == '/' {
		host = host[:len(host)-1]
	}

	path := group + "/" + project + ".git"
	if username != "" && password != "" {
		return fmt.Sprintf("https://%s:%s@%s/%s", username, password, host, path)
	}
	return fmt.Sprintf("https://%s/%s", host, path)
}
