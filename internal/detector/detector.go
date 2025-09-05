package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v50/github"
)

// Ecosystem represents a detected ecosystem with its confidence
type Ecosystem struct {
	Name        string
	Type        string
	Directories []string
	Confidence  float64
}

// Detector detects package ecosystems in a repository
type Detector struct {
	client *github.Client
	org    string
}

// New creates a new ecosystem detector
func New(client *github.Client, org string) *Detector {
	return &Detector{
		client: client,
		org:    org,
	}
}

// Detect analyzes repository files to identify ecosystems
func (d *Detector) Detect(ctx context.Context, repo string) ([]Ecosystem, error) {
	tree, _, err := d.client.Git.GetTree(ctx, d.org, repo, "HEAD", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	ecosystems := make(map[string]*Ecosystem)

	indicators := map[string][]indicator{
		"npm": {
			{file: "package-lock.json", confidence: 1.0},
			{file: "yarn.lock", confidence: 1.0},
			{file: "pnpm-lock.yaml", confidence: 1.0},
			{file: "package.json", confidence: 0.8},
		},
		"gomod": {
			{file: "go.sum", confidence: 1.0},
			{file: "go.mod", confidence: 0.9},
		},
		"pip": {
			{file: "poetry.lock", confidence: 1.0},
			{file: "Pipfile.lock", confidence: 1.0},
			{file: "requirements.txt", confidence: 0.8},
			{file: "setup.py", confidence: 0.7},
			{file: "pyproject.toml", confidence: 0.9},
		},
		"docker": {
			{file: "Dockerfile", confidence: 0.9},
			{file: "docker-compose.yml", confidence: 0.8},
			{file: "docker-compose.yaml", confidence: 0.8},
			{file: "Dockerfile.*", confidence: 0.9},
		},
		"maven": {
			{file: "pom.xml", confidence: 0.9},
		},
		"gradle": {
			{file: "gradle.lock", confidence: 1.0},
			{file: "build.gradle", confidence: 0.8},
			{file: "build.gradle.kts", confidence: 0.8},
		},
		"bundler": {
			{file: "Gemfile.lock", confidence: 1.0},
			{file: "Gemfile", confidence: 0.8},
		},
		"cargo": {
			{file: "Cargo.lock", confidence: 1.0},
			{file: "Cargo.toml", confidence: 0.8},
		},
		"composer": {
			{file: "composer.lock", confidence: 1.0},
			{file: "composer.json", confidence: 0.8},
		},
		"nuget": {
			{file: "packages.config", confidence: 0.8},
			{file: "*.csproj", confidence: 0.7},
			{file: "*.fsproj", confidence: 0.7},
			{file: "*.vbproj", confidence: 0.7},
		},
		"github-actions": {
			{file: ".github/workflows/*.yml", confidence: 0.9},
			{file: ".github/workflows/*.yaml", confidence: 0.9},
		},
		"terraform": {
			{file: "*.tf", confidence: 0.8},
			{file: ".terraform.lock.hcl", confidence: 1.0},
		},
		"elm": {
			{file: "elm.json", confidence: 0.9},
			{file: "elm-package.json", confidence: 0.8},
		},
		"gitsubmodule": {
			{file: ".gitmodules", confidence: 0.9},
		},
		"pub": {
			{file: "pubspec.yaml", confidence: 0.9},
			{file: "pubspec.lock", confidence: 1.0},
		},
		"hex": {
			{file: "mix.exs", confidence: 0.9},
			{file: "mix.lock", confidence: 1.0},
		},
	}

	for _, entry := range tree.Entries {
		if entry.Type != nil && *entry.Type == "blob" && entry.Path != nil {
			path := *entry.Path
			dir := extractDirectory(path)

			for ecosystem, files := range indicators {
				for _, ind := range files {
					if matchesPattern(path, ind.file) {
						if _, exists := ecosystems[ecosystem]; !exists {
							ecosystems[ecosystem] = &Ecosystem{
								Name:        ecosystem,
								Type:        ecosystem,
								Directories: []string{},
								Confidence:  ind.confidence,
							}
						} else if ind.confidence > ecosystems[ecosystem].Confidence {
							ecosystems[ecosystem].Confidence = ind.confidence
						}

						// Some ecosystems always scan from root directory
						directory := dir
						switch ecosystem {
						case "docker", "github-actions", "terraform", "gitsubmodule":
							directory = "/"
						}

						ecosystems[ecosystem].Directories = appendUnique(
							ecosystems[ecosystem].Directories, directory,
						)
					}
				}
			}
		}
	}

	result := make([]Ecosystem, 0, len(ecosystems))
	for _, eco := range ecosystems {
		result = append(result, *eco)
	}

	// Sort by confidence (highest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Confidence > result[i].Confidence {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// HasExclusionTopic checks if repository has exclusion topics
func (d *Detector) HasExclusionTopic(ctx context.Context, repo *github.Repository) bool {
	excludeTags := []string{"no-dependabot", "skip-dependabot", "exclude-dependabot"}

	for _, topic := range repo.Topics {
		for _, exclude := range excludeTags {
			if topic == exclude {
				return true
			}
		}
	}
	return false
}

type indicator struct {
	file       string
	confidence float64
}

func extractDirectory(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return "/"
	}
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	return dir
}

func matchesPattern(path, pattern string) bool {
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
		matched, _ = filepath.Match(pattern, path)
		return matched
	}
	return filepath.Base(path) == pattern || path == pattern
}

func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}
