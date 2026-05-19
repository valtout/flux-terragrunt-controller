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

// NewClient creates a new git client.
func NewClient(repoURL, branch, filter string) *Client {
	return &Client{
		repoURL: repoURL,
		branch:  branch,
		filter:  filter,
	}
}

// HasChanges checks if there are any changes in the specified path within the repository
// by comparing the current commit with the last known commit.
// Returns the current commit SHA and true if changes exist.
func (c *Client) HasChanges(workDir, lastKnownCommit string) (string, bool, error) {
	clonePath, err := c.cloneRepo(workDir)
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(clonePath)

	currentCommit, err := c.getCurrentCommit(clonePath)
	if err != nil {
		return "", false, err
	}

	if lastKnownCommit == "" {
		// First reconcile, no previous commit to compare
		return currentCommit, false, nil
	}

	if lastKnownCommit == currentCommit {
		return currentCommit, false, nil
	}

	// Check if there are changes in the filter path
	hasChanges, err := c.hasPathChanges(clonePath, lastKnownCommit, currentCommit)
	if err != nil {
		return currentCommit, false, err
	}

	return currentCommit, hasChanges, nil
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

// hasPathChanges checks if there are changes in the specified path between two commits.
func (c *Client) hasPathChanges(repoPath, fromCommit, toCommit string) (bool, error) {
	// Use git diff to check for changes in the filter path
	cmd := exec.Command("git", "-C", repoPath, "diff", fromCommit, toCommit, "--", c.filter)
	output, err := cmd.Output()
	if err != nil {
		// If no changes, git returns non-zero but we still have output
	}

	diff := strings.TrimSpace(string(output))
	return len(diff) > 0, nil
}

// GetChangedFiles returns the list of changed files in the filter path between two commits.
func (c *Client) GetChangedFiles(workDir, fromCommit, toCommit string) ([]string, error) {
	clonePath, err := c.cloneRepo(workDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(clonePath)

	cmd := exec.Command("git", "-C", clonePath, "diff", "--name-only", fromCommit, toCommit, "--", c.filter)
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no changes found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}

	return result, nil
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