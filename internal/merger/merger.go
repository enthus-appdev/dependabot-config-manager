// Package merger handles merging of Dependabot configurations.
package merger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/enthus-appdev/dependabot-config-manager/internal/config"
	"github.com/enthus-appdev/dependabot-config-manager/internal/detector"
	"gopkg.in/yaml.v3"
)

// Merger merges organization configs with existing repository configs
type Merger struct {
	templates    map[string]config.DependabotConfig
	templatesDir string
}

// New creates a new config merger with templates
func New(templatesDir string) (*Merger, error) {
	m := &Merger{
		templates:    make(map[string]config.DependabotConfig),
		templatesDir: templatesDir,
	}

	if err := m.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return m, nil
}

// Merge combines org standard with existing config
func (m *Merger) Merge(existing *config.DependabotConfig, ecosystems []detector.Ecosystem) *config.DependabotConfig {
	if existing == nil {
		return m.createFromTemplates(ecosystems)
	}

	merged := &config.DependabotConfig{
		Version: 2,
		Updates: []config.DependabotUpdate{},
	}

	// Process each detected ecosystem
	for _, eco := range ecosystems {
		template, hasTemplate := m.templates[eco.Name]
		if !hasTemplate {
			continue
		}

		// For each directory in the ecosystem
		for _, dir := range eco.Directories {
			// Check if existing config has this ecosystem/directory
			existingUpdate := findUpdate(existing.Updates, eco.Type, dir)

			if existingUpdate != nil {
				// Merge with existing - preserve directory for root-only ecosystems
				for _, tmplUpdate := range template.Updates {
					mergedUpdate := m.mergeUpdate(*existingUpdate, tmplUpdate)
					// Always use "/" for root-only ecosystems
					if isRootOnlyEcosystem(eco.Type) {
						mergedUpdate.Directory = "/"
					} else {
						mergedUpdate.Directory = dir
					}
					merged.Updates = append(merged.Updates, mergedUpdate)
				}
			} else {
				// Use template
				for _, tmplUpdate := range template.Updates {
					newUpdate := tmplUpdate
					newUpdate.Directory = dir
					merged.Updates = append(merged.Updates, newUpdate)
				}
			}
		}
	}

	// Add any existing updates not covered by detected ecosystems
	for _, existingUpdate := range existing.Updates {
		found := false
		for _, mergedUpdate := range merged.Updates {
			// For root-only ecosystems, consider as found if ecosystem matches
			if isRootOnlyEcosystem(existingUpdate.PackageEcosystem) {
				if mergedUpdate.PackageEcosystem == existingUpdate.PackageEcosystem {
					found = true
					break
				}
			} else {
				// For others, check both ecosystem and directory
				if mergedUpdate.PackageEcosystem == existingUpdate.PackageEcosystem &&
					mergedUpdate.Directory == existingUpdate.Directory {
					found = true
					break
				}
			}
		}
		if !found {
			// Normalize directory for root-only ecosystems
			if isRootOnlyEcosystem(existingUpdate.PackageEcosystem) {
				existingUpdate.Directory = "/"
			}
			merged.Updates = append(merged.Updates, existingUpdate)
		}
	}

	// Sort updates for deterministic ordering
	sortUpdates(merged.Updates)

	return merged
}

// mergeUpdate merges an existing update with a template
func (m *Merger) mergeUpdate(existing, template config.DependabotUpdate) config.DependabotUpdate {
	merged := existing

	// Merge strategy:
	// - PRESERVE: directory, target-branch, vendor (keep repository-specific)
	// - MERGE: labels, reviewers, assignees, ignore rules
	// - REPLACE: schedule, PR limits, versioning strategy

	// Replace schedule with template
	merged.Schedule = template.Schedule

	// Replace PR limit
	if template.OpenPullRequestsLimit > 0 {
		merged.OpenPullRequestsLimit = template.OpenPullRequestsLimit
	}

	// Merge labels
	merged.Labels = mergeStringSlices(existing.Labels, template.Labels)

	// Merge reviewers
	merged.Reviewers = mergeStringSlices(existing.Reviewers, template.Reviewers)

	// Merge assignees
	merged.Assignees = mergeStringSlices(existing.Assignees, template.Assignees)

	// Replace versioning strategy
	if template.VersioningStrategy != "" {
		merged.VersioningStrategy = template.VersioningStrategy
	}

	// Deep merge groups
	if len(template.Groups) > 0 {
		if merged.Groups == nil {
			merged.Groups = make(map[string]config.GroupConfig)
		}
		for name, group := range template.Groups {
			merged.Groups[name] = group
		}
	}

	// Use template commit message if not set
	if merged.CommitMessage == nil && template.CommitMessage != nil {
		merged.CommitMessage = template.CommitMessage
	}

	return merged
}

// createFromTemplates creates a new config from templates
func (m *Merger) createFromTemplates(ecosystems []detector.Ecosystem) *config.DependabotConfig {
	cfg := &config.DependabotConfig{
		Version: 2,
		Updates: []config.DependabotUpdate{},
	}

	for _, eco := range ecosystems {
		template, hasTemplate := m.templates[eco.Name]
		if !hasTemplate {
			// Create a basic config if no template exists
			for _, dir := range eco.Directories {
				cfg.Updates = append(cfg.Updates, config.DependabotUpdate{
					PackageEcosystem:      eco.Type,
					Directory:             dir,
					Schedule:              config.Schedule{Interval: "weekly"},
					OpenPullRequestsLimit: 10,
					Labels:                []string{"dependencies"},
				})
			}
			continue
		}

		// Use template for each directory
		for _, dir := range eco.Directories {
			for _, tmplUpdate := range template.Updates {
				update := tmplUpdate
				update.Directory = dir
				cfg.Updates = append(cfg.Updates, update)
			}
		}
	}

	// Sort updates for deterministic ordering
	sortUpdates(cfg.Updates)

	return cfg
}

// loadTemplates loads configuration templates from the configs directory
func (m *Merger) loadTemplates() error {
	// Load ecosystem-specific templates
	ecosystems := []string{"npm", "golang", "python", "docker", "maven", "gradle", "bundler", "cargo", "composer", "nuget", "github-actions"}

	for _, eco := range ecosystems {
		templatePath := filepath.Join(m.templatesDir, eco, "default.yml")
		data, err := os.ReadFile(templatePath)
		if err != nil {
			continue // Template not found, skip
		}

		var tmpl config.DependabotConfig
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return fmt.Errorf("failed to parse %s template: %w", eco, err)
		}

		// Map ecosystem names
		ecosystemName := eco
		if eco == "golang" {
			ecosystemName = "gomod"
		}

		m.templates[ecosystemName] = tmpl
	}

	return nil
}

// Helper functions

func findUpdate(updates []config.DependabotUpdate, ecosystem, directory string) *config.DependabotUpdate {
	// For ecosystems that always use root, check for any directory
	rootOnlyEcosystems := map[string]bool{
		"github-actions": true,
		"docker":         true,
		"terraform":      true,
		"gitsubmodule":   true,
	}

	for i := range updates {
		if updates[i].PackageEcosystem == ecosystem {
			// For root-only ecosystems, match regardless of directory
			if rootOnlyEcosystems[ecosystem] {
				return &updates[i]
			}
			// For others, match exact directory
			if updates[i].Directory == directory {
				return &updates[i]
			}
		}
	}
	return nil
}

func isRootOnlyEcosystem(ecosystem string) bool {
	rootOnlyEcosystems := map[string]bool{
		"github-actions": true,
		"docker":         true,
		"terraform":      true,
		"gitsubmodule":   true,
	}
	return rootOnlyEcosystems[ecosystem]
}

func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range a {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	for _, item := range b {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// sortUpdates sorts the updates array for deterministic ordering
func sortUpdates(updates []config.DependabotUpdate) {
	sort.Slice(updates, func(i, j int) bool {
		// First, sort by package ecosystem
		if updates[i].PackageEcosystem != updates[j].PackageEcosystem {
			return updates[i].PackageEcosystem < updates[j].PackageEcosystem
		}
		// Then by directory
		return updates[i].Directory < updates[j].Directory
	})
}

