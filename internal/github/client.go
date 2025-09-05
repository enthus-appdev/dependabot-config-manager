// Package github provides a client for interacting with the GitHub API.
package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/enthus-appdev/dependabot-config-manager/internal/config"
	"github.com/enthus-appdev/dependabot-config-manager/internal/util"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

// Client wraps the GitHub client with our specific operations
type Client struct {
	client *github.Client
	org    string
}

// NewClient creates a new GitHub client
func NewClient(token, org string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		org:    org,
	}
}

// GetClient returns the underlying GitHub client
func (c *Client) GetClient() *github.Client {
	return c.client
}

// ListRepositories lists all repositories in the organization
func (c *Client) ListRepositories(ctx context.Context, excludeArchived bool) ([]*github.Repository, error) {
	var allRepos []*github.Repository

	opt := &github.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := c.client.Repositories.ListByOrg(ctx, c.org, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		for _, repo := range repos {
			if excludeArchived && repo.Archived != nil && *repo.Archived {
				continue
			}
			allRepos = append(allRepos, repo)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRepository gets a single repository
func (c *Client) GetRepository(ctx context.Context, name string) (*github.Repository, error) {
	repo, _, err := c.client.Repositories.Get(ctx, c.org, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repo, nil
}

// GetFileContent gets the content of a file from a repository
func (c *Client) GetFileContent(ctx context.Context, repo, path string) ([]byte, string, error) {
	fileContent, _, resp, err := c.client.Repositories.GetContents(ctx, c.org, repo, path, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, "", nil // File not found
		}
		return nil, "", fmt.Errorf("failed to get file content: %w", err)
	}

	if fileContent.Content == nil {
		return nil, "", fmt.Errorf("file content is nil")
	}

	content, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode content: %w", err)
	}

	sha := ""
	if fileContent.SHA != nil {
		sha = *fileContent.SHA
	}

	return content, sha, nil
}

// CreateOrUpdateFile creates or updates a file in a repository
func (c *Client) CreateOrUpdateFile(ctx context.Context, repo, path, message string, content []byte, sha string) error {
	// Get repository info to determine default branch
	repoInfo, _, err := c.client.Repositories.Get(ctx, c.org, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := "main"
	if repoInfo.DefaultBranch != nil {
		defaultBranch = *repoInfo.DefaultBranch
	}

	opts := &github.RepositoryContentFileOptions{
		Message: &message,
		Content: content,
		Branch:  github.String(defaultBranch),
	}

	if sha != "" {
		opts.SHA = &sha
	}

	_, _, err = c.client.Repositories.CreateFile(ctx, c.org, repo, path, opts)
	if err != nil {
		// If creation fails, try update
		if sha == "" {
			// Get current SHA
			currentContent, currentSHA, getErr := c.GetFileContent(ctx, repo, path)
			if getErr != nil {
				return fmt.Errorf("failed to create or update file: %w", err)
			}
			if currentContent != nil {
				opts.SHA = &currentSHA
				_, _, err = c.client.Repositories.UpdateFile(ctx, c.org, repo, path, opts)
			}
		}
	}

	return err
}

// CreatePullRequest creates a pull request for the Dependabot configuration
func (c *Client) CreatePullRequest(ctx context.Context, repo string, config *config.DependabotConfig, yamlIndent int) error {
	// Create a branch
	branchName := fmt.Sprintf("dependabot-config-%d", time.Now().Unix())

	// Get default branch
	repoInfo, _, err := c.client.Repositories.Get(ctx, c.org, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := "main"
	if repoInfo.DefaultBranch != nil {
		defaultBranch = *repoInfo.DefaultBranch
	}

	// Get reference of default branch
	ref, _, err := c.client.Git.GetRef(ctx, c.org, repo, "refs/heads/"+defaultBranch)
	if err != nil {
		return fmt.Errorf("failed to get reference: %w", err)
	}

	// Create new branch
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + branchName),
		Object: &github.GitObject{
			SHA: ref.Object.SHA,
		},
	}

	_, _, err = c.client.Git.CreateRef(ctx, c.org, repo, newRef)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Create or update the Dependabot config file on the new branch
	content, err := util.MarshalYAML(config, yamlIndent)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	message := "Add/Update Dependabot configuration"
	opts := &github.RepositoryContentFileOptions{
		Message: &message,
		Content: content,
		Branch:  &branchName,
	}

	// Check if file exists
	existingContent, sha, _ := c.GetFileContent(ctx, repo, ".github/dependabot.yml")
	if existingContent != nil {
		opts.SHA = &sha
		_, _, err = c.client.Repositories.UpdateFile(ctx, c.org, repo, ".github/dependabot.yml", opts)
	} else {
		_, _, err = c.client.Repositories.CreateFile(ctx, c.org, repo, ".github/dependabot.yml", opts)
	}

	if err != nil {
		return fmt.Errorf("failed to create/update file in branch: %w", err)
	}

	// Create pull request
	prTitle := "Configure Dependabot for dependency updates"
	prBody := generatePRBody(config)

	pr := &github.NewPullRequest{
		Title:               &prTitle,
		Head:                &branchName,
		Base:                &defaultBranch,
		Body:                &prBody,
		MaintainerCanModify: github.Bool(true),
	}

	_, _, err = c.client.PullRequests.Create(ctx, c.org, repo, pr)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	return nil
}

// GetExistingConfig retrieves the existing Dependabot configuration
func (c *Client) GetExistingConfig(ctx context.Context, repo string) (*config.DependabotConfig, error) {
	content, _, err := c.GetFileContent(ctx, repo, ".github/dependabot.yml")
	if err != nil {
		return nil, err
	}

	if content == nil {
		// Try alternative path
		content, _, err = c.GetFileContent(ctx, repo, ".github/dependabot.yaml")
		if err != nil {
			return nil, err
		}
	}

	if content == nil {
		return nil, nil // No existing config
	}

	var cfg config.DependabotConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse existing config: %w", err)
	}

	return &cfg, nil
}

// GetTreeSHA gets the SHA of the repository tree
func (c *Client) GetTreeSHA(ctx context.Context, repo string) (string, error) {
	// Get repository info to determine default branch
	repoInfo, _, err := c.client.Repositories.Get(ctx, c.org, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := "main"
	if repoInfo.DefaultBranch != nil {
		defaultBranch = *repoInfo.DefaultBranch
	}

	ref, _, err := c.client.Git.GetRef(ctx, c.org, repo, "refs/heads/"+defaultBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get ref for branch %s: %w", defaultBranch, err)
	}

	if ref.Object != nil && ref.Object.SHA != nil {
		return *ref.Object.SHA, nil
	}

	return "", fmt.Errorf("ref SHA is nil")
}

func generatePRBody(cfg *config.DependabotConfig) string {
	var ecosystems []string
	for _, update := range cfg.Updates {
		ecosystems = append(ecosystems, update.PackageEcosystem)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	unique := []string{}
	for _, eco := range ecosystems {
		if !seen[eco] {
			seen[eco] = true
			unique = append(unique, eco)
		}
	}

	body := `## Dependabot Configuration Update

This pull request adds or updates the Dependabot configuration for this repository.

### Configured Ecosystems
`

	for _, eco := range unique {
		body += fmt.Sprintf("- ✅ %s\n", eco)
	}

	body += `
### What This Does
- 🔄 Automatically creates pull requests for dependency updates
- 🔒 Helps identify and fix security vulnerabilities
- 📦 Keeps dependencies up-to-date with the latest versions
- 🏷️ Groups related dependencies for easier review

### Configuration Details
- **Update Schedule**: Weekly (Monday mornings)
- **PR Limit**: 10 open pull requests maximum
- **Dependency Grouping**: Enabled for better organization

### Next Steps
1. Review the configuration to ensure it matches your needs
2. Merge this PR to enable Dependabot
3. Dependabot will start creating PRs based on the schedule

---
*Generated by Dependabot Configuration Manager*`

	return body
}

