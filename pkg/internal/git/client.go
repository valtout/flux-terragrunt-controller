package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client provides git operations for checking repository changes.
type Client struct {
	repoURL string
	branch  string
	filter  string
}

// NewClient creates a new git client for a single filter.
func NewClient(repoURL, branch, filter string) *Client {
	return &Client{
		repoURL: repoURL,
		branch:  branch,
		filter:  filter,
	}
}

// MultiClient provides git operations for checking repository changes across multiple filters.
type MultiClient struct {
	repoURL  string
	branch   string
	filters  []string
	clients  []*Client
}

// NewMultiClient creates a new git client that handles multiple filters.
func NewMultiClient(repoURL, branch string, filters []string) *MultiClient {
	clients := make([]*Client, len(filters))
	for i, f := range filters {
		clients[i] = NewClient(repoURL, branch, f)
	}
	return &MultiClient{
		repoURL: repoURL,
		branch:  branch,
		filters: filters,
		clients: clients,
	}
}

// cloneRepo clones the repository to a temporary directory.
func (c *Client) cloneRepo(workDir string) (string, error) {
	tempDir, err := os.MkdirTemp(workDir, "git-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cmd := exec.Command("git", "clone", "--branch", c.branch, "--depth", "1", c.repoURL, tempDir)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to clone repo: %w", err)
	}

	return tempDir, nil
}

// getCurrentCommit returns the current commit SHA of the cloned repository.
func (c *Client) getCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current commit: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetChangedFiles returns the list of changed files in the filter path between two commits.
func (c *Client) GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error) {
	clonePath, err := c.cloneRepo(workDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(clonePath)

	return c.getChangedFilesInternal(clonePath, fromCommit, toCommit)
}

// GetChangedFilesForFilter is used by MultiClient to check a specific filter.
func (c *Client) GetChangedFilesForFilter(workDir, fromCommit, toCommit, filter string) ([]string, error) {
	clonePath, err := c.cloneRepo(workDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(clonePath)

	cmd := exec.Command("git", "-C", clonePath, "diff", "--name-only", fromCommit, toCommit, "--", filter)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	return parseFileList(string(output)), nil
}

func (c *Client) getChangedFilesInternal(repoPath, fromCommit, toCommit string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "diff", "--name-only", fromCommit, toCommit, "--", c.filter)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	return parseFileList(string(output)), nil
}

func parseFileList(output string) []string {
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

// CloneAtCommit clones the repository and checks out the specific commit.
func (c *Client) CloneAtCommit(workDir, commitSHA string) (string, error) {
	clonePath := filepath.Join(workDir, "git-clone")
	if err := os.RemoveAll(clonePath); err != nil {
		return "", fmt.Errorf("failed to clean clone path: %w", err)
	}

	// Clone shallowly
	cmd := exec.Command("git", "clone", "--branch", c.branch, "--depth", "100", c.repoURL, clonePath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repo: %w", err)
	}

	// Fetch the specific commit if not in shallow history
	fetchCmd := exec.Command("git", "-C", clonePath, "fetch", "--depth", "100", c.repoURL, commitSHA)
	if err := fetchCmd.Run(); err != nil {
		// Fallback to full clone
		os.RemoveAll(clonePath)
		fullCmd := exec.Command("git", "clone", c.repoURL, clonePath)
		if err := fullCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to clone repo: %w", err)
		}
	}

	// Checkout the specific commit
	checkoutCmd := exec.Command("git", "-C", clonePath, "checkout", commitSHA)
	if err := checkoutCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to checkout commit: %w", err)
	}

	return clonePath, nil
}

// HasChanges checks if there are any changes in the specified path within the repository.
func (c *MultiClient) HasChanges(workDir, lastKnownCommit string) (string, bool, error) {
	client := c.clients[0]
	clonePath, err := client.cloneRepo(workDir)
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(clonePath)

	currentCommit, err := client.getCurrentCommit(clonePath)
	if err != nil {
		return "", false, err
	}

	if lastKnownCommit == "" || lastKnownCommit == currentCommit {
		return currentCommit, false, nil
	}

	for _, cli := range c.clients {
		hasChanges, err := cli.hasPathChangesInternal(clonePath, lastKnownCommit, currentCommit)
		if err != nil {
			return currentCommit, false, err
		}
		if hasChanges {
			return currentCommit, true, nil
		}
	}

	return currentCommit, false, nil
}

func (c *Client) hasPathChangesInternal(repoPath, fromCommit, toCommit string) (bool, error) {
	cmd := exec.Command("git", "-C", repoPath, "diff", fromCommit, toCommit, "--", c.filter)
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no changes found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		// Some git versions exit with 0 even with changes
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// GetChangedFiles returns all changed files across all filters.
func (c *MultiClient) GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error) {
	var allFiles []string
	for _, cli := range c.clients {
		files, err := cli.GetChangedFiles(workDir, fromCommit, toCommit)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}
	return allFiles, nil
}

// GetChangedFilesForFilter returns changed files for a specific filter.
func (c *MultiClient) GetChangedFilesForFilter(workDir, fromCommit, toCommit, filter string) ([]string, error) {
	return c.clients[0].GetChangedFilesForFilter(workDir, fromCommit, toCommit, filter)
}

// CloneAtCommit clones the repository and checks out the specific commit.
func (c *MultiClient) CloneAtCommit(workDir, commitSHA string) (string, error) {
	client := c.clients[0]
	return client.CloneAtCommit(workDir, commitSHA)
}