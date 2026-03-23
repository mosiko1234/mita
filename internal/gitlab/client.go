package gitlab

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	gogitlab "github.com/xanzy/go-gitlab"
)

// Client wraps the GitLab API client.
type Client struct {
	gl          *gogitlab.Client
	baseURL     string
	insecureTLS bool
	username    string
	password    string
}

// Project represents a GitLab project.
type Project struct {
	ID            int
	Name          string
	PathWithNS    string
	Description   string
	DefaultBranch string
	HTTPURL       string
	SSHURL        string
}

// Branch represents a Git branch.
type Branch struct {
	Name   string
	Commit string
}

// NewClient creates a new GitLab API client.
func NewClient(baseURL, token string, insecureTLS bool) (*Client, error) {
	opts := []gogitlab.ClientOptionFunc{
		gogitlab.WithBaseURL(strings.TrimRight(baseURL, "/")),
	}

	if insecureTLS {
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		opts = append(opts, gogitlab.WithHTTPClient(httpClient))
	}

	var gl *gogitlab.Client
	var err error

	if token != "" {
		gl, err = gogitlab.NewClient(token, opts...)
	} else {
		// Create client without auth — will use basic auth for git operations
		gl, err = gogitlab.NewClient("", opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return &Client{
		gl:          gl,
		baseURL:     baseURL,
		insecureTLS: insecureTLS,
	}, nil
}

// NewClientWithBasicAuth creates a client authenticated with username/password.
func NewClientWithBasicAuth(baseURL, username, password string, insecureTLS bool) (*Client, error) {
	c, err := NewClient(baseURL, "", insecureTLS)
	if err != nil {
		return nil, err
	}
	c.username = username
	c.password = password

	// Re-create with basic auth
	opts := []gogitlab.ClientOptionFunc{
		gogitlab.WithBaseURL(strings.TrimRight(baseURL, "/")),
	}
	if insecureTLS {
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		opts = append(opts, gogitlab.WithHTTPClient(httpClient))
	}

	c.gl, err = gogitlab.NewBasicAuthClient(username, password, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gitlab basic auth client: %w", err)
	}

	return c, nil
}

// ListProjects returns all projects visible to the authenticated user.
func (c *Client) ListProjects(search string) ([]Project, error) {
	opt := &gogitlab.ListProjectsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 100},
		OrderBy:     gogitlab.Ptr("name"),
		Sort:        gogitlab.Ptr("asc"),
	}
	if search != "" {
		opt.Search = gogitlab.Ptr(search)
	}

	var allProjects []Project
	for {
		projects, resp, err := c.gl.Projects.ListProjects(opt)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}

		for _, p := range projects {
			allProjects = append(allProjects, Project{
				ID:            p.ID,
				Name:          p.Name,
				PathWithNS:    p.PathWithNamespace,
				Description:   p.Description,
				DefaultBranch: p.DefaultBranch,
				HTTPURL:       p.HTTPURLToRepo,
				SSHURL:        p.SSHURLToRepo,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allProjects, nil
}

// ListBranches returns all branches for a project.
func (c *Client) ListBranches(projectID int) ([]Branch, error) {
	opt := &gogitlab.ListBranchesOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 100},
	}

	var allBranches []Branch
	for {
		branches, resp, err := c.gl.Branches.ListBranches(projectID, opt)
		if err != nil {
			return nil, fmt.Errorf("list branches: %w", err)
		}

		for _, b := range branches {
			allBranches = append(allBranches, Branch{
				Name:   b.Name,
				Commit: b.Commit.ID,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allBranches, nil
}

// GetProject returns a single project by ID.
func (c *Client) GetProject(projectID int) (*Project, error) {
	p, _, err := c.gl.Projects.GetProject(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	return &Project{
		ID:            p.ID,
		Name:          p.Name,
		PathWithNS:    p.PathWithNamespace,
		Description:   p.Description,
		DefaultBranch: p.DefaultBranch,
		HTTPURL:       p.HTTPURLToRepo,
		SSHURL:        p.SSHURLToRepo,
	}, nil
}

// BaseURL returns the configured GitLab base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// IsInsecureTLS returns whether insecure TLS is enabled.
func (c *Client) IsInsecureTLS() bool {
	return c.insecureTLS
}

// Username returns the configured username (for basic auth git operations).
func (c *Client) Username() string {
	return c.username
}

// Password returns the configured password (for basic auth git operations).
func (c *Client) Password() string {
	return c.password
}
