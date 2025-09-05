package detector

import (
	"context"
	"testing"

	"github.com/google/go-github/v50/github"
)

func TestDetector_matchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			path:     "package.json",
			pattern:  "package.json",
			expected: true,
		},
		{
			name:     "wildcard match",
			path:     ".github/workflows/test.yml",
			pattern:  ".github/workflows/*.yml",
			expected: true,
		},
		{
			name:     "no match",
			path:     "README.md",
			pattern:  "package.json",
			expected: false,
		},
		{
			name:     "nested path exact match",
			path:     "src/package.json",
			pattern:  "package.json",
			expected: true,
		},
		{
			name:     "dockerfile wildcard",
			path:     "Dockerfile.prod",
			pattern:  "Dockerfile.*",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPattern(tt.path, tt.pattern); got != tt.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestDetector_extractDirectory(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "root file",
			path:     "package.json",
			expected: "/",
		},
		{
			name:     "nested file",
			path:     "src/main.go",
			expected: "/src",
		},
		{
			name:     "deeply nested",
			path:     "src/internal/app/main.go",
			expected: "/src/internal/app",
		},
		{
			name:     "github workflows",
			path:     ".github/workflows/test.yml",
			expected: "/.github/workflows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDirectory(tt.path); got != tt.expected {
				t.Errorf("extractDirectory(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDetector_appendUnique(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected []string
	}{
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "new",
			expected: []string{"new"},
		},
		{
			name:     "unique item",
			slice:    []string{"a", "b"},
			item:     "c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "duplicate item",
			slice:    []string{"a", "b", "c"},
			item:     "b",
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.slice, tt.item)
			if len(got) != len(tt.expected) {
				t.Errorf("appendUnique() returned slice of length %d, want %d", len(got), len(tt.expected))
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("appendUnique()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestDetector_HasExclusionTopic(t *testing.T) {
	tests := []struct {
		name     string
		topics   []string
		expected bool
	}{
		{
			name:     "has no-dependabot topic",
			topics:   []string{"javascript", "no-dependabot", "frontend"},
			expected: true,
		},
		{
			name:     "has skip-dependabot topic",
			topics:   []string{"backend", "skip-dependabot"},
			expected: true,
		},
		{
			name:     "has exclude-dependabot topic",
			topics:   []string{"exclude-dependabot"},
			expected: true,
		},
		{
			name:     "no exclusion topics",
			topics:   []string{"javascript", "frontend", "react"},
			expected: false,
		},
		{
			name:     "empty topics",
			topics:   []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detector{}
			repo := &github.Repository{
				Topics: tt.topics,
			}
			
			if got := d.HasExclusionTopic(context.Background(), repo); got != tt.expected {
				t.Errorf("HasExclusionTopic() = %v, want %v", got, tt.expected)
			}
		})
	}
}