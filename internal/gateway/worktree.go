package gateway

import (
	"fmt"
	"os/exec"
	"strings"
)

// WorktreeManager handles git worktree operations for story isolation.
type WorktreeManager struct {
	root string // e.g., ".loom/worktrees"
}

// NewWorktreeManager creates a new WorktreeManager with the given root path.
func NewWorktreeManager(root string) *WorktreeManager {
	if root == "" {
		root = ".loom/worktrees"
	}
	return &WorktreeManager{root: root}
}

// CreateWorktree creates a git worktree at {root}/{storyID} and checks out
// a new branch feature/story-{storyID}-{slug}.
// Returns the worktree path and branch name, or an error.
func (wm *WorktreeManager) CreateWorktree(repoPath, storyID, branchName string) (worktreePath, actualBranch string, err error) {
	if branchName == "" {
		branchName = fmt.Sprintf("feature/story-%s", storyID)
	}

	worktreePath = fmt.Sprintf("%s/%s", wm.root, storyID)

	// git worktree add -b {branch} {path}
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git worktree add: %w\nOutput: %s", err, string(out))
	}

	return worktreePath, branchName, nil
}

// RemoveWorktree removes a git worktree at the given path.
func (wm *WorktreeManager) RemoveWorktree(repoPath, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\nOutput: %s", err, string(out))
	}
	return nil
}

// GenerateBranchName creates a branch name from a story ID and title.
func GenerateBranchName(storyID, title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	var clean []rune
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean = append(clean, r)
		}
	}
	slug = string(clean)
	// Truncate to keep it reasonable
	if len(slug) > 40 {
		slug = slug[:40]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = storyID
	}
	return fmt.Sprintf("feature/story-%s-%s", storyID, slug)
}
