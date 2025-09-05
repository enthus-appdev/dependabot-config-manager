package config

import (
	"testing"
)

func TestDependabotConfig_Equal(t *testing.T) {
	tests := []struct {
		name     string
		config1  *DependabotConfig
		config2  *DependabotConfig
		expected bool
	}{
		{
			name:     "both nil",
			config1:  nil,
			config2:  nil,
			expected: true,
		},
		{
			name: "one nil",
			config1: &DependabotConfig{
				Version: 2,
			},
			config2:  nil,
			expected: false,
		},
		{
			name: "different versions",
			config1: &DependabotConfig{
				Version: 1,
			},
			config2: &DependabotConfig{
				Version: 2,
			},
			expected: false,
		},
		{
			name: "same basic config",
			config1: &DependabotConfig{
				Version: 2,
				Updates: []DependabotUpdate{
					{
						PackageEcosystem: "npm",
						Directory:        "/",
						Schedule: Schedule{
							Interval: "weekly",
						},
					},
				},
			},
			config2: &DependabotConfig{
				Version: 2,
				Updates: []DependabotUpdate{
					{
						PackageEcosystem: "npm",
						Directory:        "/",
						Schedule: Schedule{
							Interval: "weekly",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "different updates length",
			config1: &DependabotConfig{
				Version: 2,
				Updates: []DependabotUpdate{
					{
						PackageEcosystem: "npm",
						Directory:        "/",
					},
				},
			},
			config2: &DependabotConfig{
				Version: 2,
				Updates: []DependabotUpdate{
					{
						PackageEcosystem: "npm",
						Directory:        "/",
					},
					{
						PackageEcosystem: "docker",
						Directory:        "/",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config1.Equal(tt.config2); got != tt.expected {
				t.Errorf("DependabotConfig.Equal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDependabotUpdate_Equal(t *testing.T) {
	tests := []struct {
		name     string
		update1  DependabotUpdate
		update2  DependabotUpdate
		expected bool
	}{
		{
			name: "same updates",
			update1: DependabotUpdate{
				PackageEcosystem:      "npm",
				Directory:             "/",
				Schedule:              Schedule{Interval: "weekly"},
				OpenPullRequestsLimit: 10,
			},
			update2: DependabotUpdate{
				PackageEcosystem:      "npm",
				Directory:             "/",
				Schedule:              Schedule{Interval: "weekly"},
				OpenPullRequestsLimit: 10,
			},
			expected: true,
		},
		{
			name: "different ecosystem",
			update1: DependabotUpdate{
				PackageEcosystem: "npm",
				Directory:        "/",
			},
			update2: DependabotUpdate{
				PackageEcosystem: "docker",
				Directory:        "/",
			},
			expected: false,
		},
		{
			name: "different directory",
			update1: DependabotUpdate{
				PackageEcosystem: "npm",
				Directory:        "/",
			},
			update2: DependabotUpdate{
				PackageEcosystem: "npm",
				Directory:        "/src",
			},
			expected: false,
		},
		{
			name: "different schedule",
			update1: DependabotUpdate{
				PackageEcosystem: "npm",
				Directory:        "/",
				Schedule:         Schedule{Interval: "daily"},
			},
			update2: DependabotUpdate{
				PackageEcosystem: "npm",
				Directory:        "/",
				Schedule:         Schedule{Interval: "weekly"},
			},
			expected: false,
		},
		{
			name: "different PR limit",
			update1: DependabotUpdate{
				PackageEcosystem:      "npm",
				Directory:             "/",
				OpenPullRequestsLimit: 5,
			},
			update2: DependabotUpdate{
				PackageEcosystem:      "npm",
				Directory:             "/",
				OpenPullRequestsLimit: 10,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.update1.Equal(&tt.update2); got != tt.expected {
				t.Errorf("DependabotUpdate.Equal() = %v, want %v", got, tt.expected)
			}
		})
	}
}