package merger

import (
	"testing"

	"github.com/your-org/dependabot-config-manager/internal/config"
	"github.com/your-org/dependabot-config-manager/internal/detector"
)

func TestMerger_mergeStringSlices(t *testing.T) {
	tests := []struct {
		name     string
		slice1   []string
		slice2   []string
		expected []string
	}{
		{
			name:     "both empty",
			slice1:   []string{},
			slice2:   []string{},
			expected: []string{},
		},
		{
			name:     "first empty",
			slice1:   []string{},
			slice2:   []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "second empty",
			slice1:   []string{"a", "b"},
			slice2:   []string{},
			expected: []string{"a", "b"},
		},
		{
			name:     "no duplicates",
			slice1:   []string{"a", "b"},
			slice2:   []string{"c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "with duplicates",
			slice1:   []string{"a", "b", "c"},
			slice2:   []string{"b", "c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "all duplicates",
			slice1:   []string{"a", "b"},
			slice2:   []string{"a", "b"},
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeStringSlices(tt.slice1, tt.slice2)
			
			if len(got) != len(tt.expected) {
				t.Errorf("mergeStringSlices() returned %d items, want %d", len(got), len(tt.expected))
				return
			}
			
			// Check each item exists in result (order doesn't matter for merged slices)
			for _, exp := range tt.expected {
				found := false
				for _, g := range got {
					if g == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("mergeStringSlices() missing expected item %q", exp)
				}
			}
		})
	}
}

func TestMerger_mergeUpdate(t *testing.T) {
	m := &Merger{}
	
	existing := config.DependabotUpdate{
		PackageEcosystem: "npm",
		Directory:        "/",
		Schedule: config.Schedule{
			Interval: "daily",
		},
		OpenPullRequestsLimit: 5,
		Labels:                []string{"dependencies", "custom"},
		Reviewers:             []string{"user1"},
		TargetBranch:          "develop",
		Vendor:                true,
	}
	
	template := config.DependabotUpdate{
		PackageEcosystem: "npm",
		Directory:        "/src",
		Schedule: config.Schedule{
			Interval: "weekly",
			Day:      "monday",
			Time:     "04:00",
		},
		OpenPullRequestsLimit: 10,
		Labels:                []string{"automated", "npm"},
		Reviewers:             []string{"security-team"},
		VersioningStrategy:    "increase",
		Groups: map[string]config.GroupConfig{
			"dev-dependencies": {
				DependencyType: "development",
			},
		},
	}
	
	merged := m.mergeUpdate(existing, template)
	
	// Check merge strategy results
	if merged.Schedule.Interval != "weekly" {
		t.Errorf("Schedule should be replaced with template, got %v", merged.Schedule.Interval)
	}
	
	if merged.OpenPullRequestsLimit != 10 {
		t.Errorf("PR limit should be replaced with template, got %d", merged.OpenPullRequestsLimit)
	}
	
	if merged.Directory != "/" {
		t.Errorf("Directory should be preserved from existing, got %v", merged.Directory)
	}
	
	if merged.TargetBranch != "develop" {
		t.Errorf("Target branch should be preserved from existing, got %v", merged.TargetBranch)
	}
	
	if !merged.Vendor {
		t.Errorf("Vendor should be preserved from existing")
	}
	
	// Check merged labels contains both
	expectedLabels := map[string]bool{
		"dependencies": true,
		"custom":       true,
		"automated":    true,
		"npm":          true,
	}
	
	for _, label := range merged.Labels {
		if !expectedLabels[label] {
			t.Errorf("Unexpected label %q in merged result", label)
		}
		delete(expectedLabels, label)
	}
	
	if len(expectedLabels) > 0 {
		t.Errorf("Missing expected labels in merged result")
	}
	
	// Check groups were added
	if len(merged.Groups) != 1 {
		t.Errorf("Groups should be merged from template, got %d groups", len(merged.Groups))
	}
}

func TestMerger_createFromTemplates(t *testing.T) {
	m := &Merger{
		templates: map[string]config.DependabotConfig{
			"npm": {
				Version: 2,
				Updates: []config.DependabotUpdate{
					{
						PackageEcosystem:      "npm",
						Schedule:              config.Schedule{Interval: "weekly"},
						OpenPullRequestsLimit: 10,
						Labels:                []string{"dependencies", "npm"},
					},
				},
			},
			"docker": {
				Version: 2,
				Updates: []config.DependabotUpdate{
					{
						PackageEcosystem:      "docker",
						Schedule:              config.Schedule{Interval: "monthly"},
						OpenPullRequestsLimit: 5,
						Labels:                []string{"dependencies", "docker"},
					},
				},
			},
		},
	}
	
	ecosystems := []detector.Ecosystem{
		{
			Name:        "npm",
			Type:        "npm",
			Directories: []string{"/", "/frontend"},
			Confidence:  1.0,
		},
		{
			Name:        "docker",
			Type:        "docker",
			Directories: []string{"/"},
			Confidence:  0.9,
		},
	}
	
	cfg := m.createFromTemplates(ecosystems)
	
	if cfg.Version != 2 {
		t.Errorf("Config version should be 2, got %d", cfg.Version)
	}
	
	// Should have 3 updates total (2 for npm directories, 1 for docker)
	if len(cfg.Updates) != 3 {
		t.Errorf("Should have 3 updates, got %d", len(cfg.Updates))
	}
	
	// Count updates by ecosystem
	npmCount := 0
	dockerCount := 0
	for _, update := range cfg.Updates {
		switch update.PackageEcosystem {
		case "npm":
			npmCount++
		case "docker":
			dockerCount++
		}
	}
	
	if npmCount != 2 {
		t.Errorf("Should have 2 npm updates, got %d", npmCount)
	}
	
	if dockerCount != 1 {
		t.Errorf("Should have 1 docker update, got %d", dockerCount)
	}
}