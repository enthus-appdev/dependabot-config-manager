// Package config defines the types and structures for Dependabot configuration management.
package config

// DependabotConfig represents the Dependabot configuration
type DependabotConfig struct {
	Version int                `yaml:"version"`
	Updates []DependabotUpdate `yaml:"updates"`
}

// DependabotUpdate represents an update configuration
type DependabotUpdate struct {
	PackageEcosystem      string                 `yaml:"package-ecosystem"`
	Directory             string                 `yaml:"directory"`
	Schedule              Schedule               `yaml:"schedule"`
	OpenPullRequestsLimit int                    `yaml:"open-pull-requests-limit,omitempty"`
	Labels                []string               `yaml:"labels,omitempty"`
	Reviewers             []string               `yaml:"reviewers,omitempty"`
	Assignees             []string               `yaml:"assignees,omitempty"`
	Milestone             int                    `yaml:"milestone,omitempty"`
	Groups                map[string]GroupConfig `yaml:"groups,omitempty"`
	VersioningStrategy    string                 `yaml:"versioning-strategy,omitempty"`
	CommitMessage         *CommitMessage         `yaml:"commit-message,omitempty"`
	TargetBranch          string                 `yaml:"target-branch,omitempty"`
	Vendor                bool                   `yaml:"vendor,omitempty"`
	Insecure              []string               `yaml:"insecure-external-code-execution,omitempty"`
	RebaseStrategy        string                 `yaml:"rebase-strategy,omitempty"`
	Ignore                []IgnoreConfig         `yaml:"ignore,omitempty"`
	Allow                 []AllowConfig          `yaml:"allow,omitempty"`
	Registries            []string               `yaml:"registries,omitempty"`
}

// Schedule represents update schedule
type Schedule struct {
	Interval string `yaml:"interval"`
	Day      string `yaml:"day,omitempty"`
	Time     string `yaml:"time,omitempty"`
	Timezone string `yaml:"timezone,omitempty"`
}

// GroupConfig represents dependency grouping
type GroupConfig struct {
	DependencyType  string   `yaml:"dependency-type,omitempty"`
	Patterns        []string `yaml:"patterns,omitempty"`
	ExcludePatterns []string `yaml:"exclude-patterns,omitempty"`
	UpdateTypes     []string `yaml:"update-types,omitempty"`
}

// CommitMessage represents commit message configuration
type CommitMessage struct {
	Prefix            string `yaml:"prefix,omitempty"`
	PrefixDevelopment string `yaml:"prefix-development,omitempty"`
	Include           string `yaml:"include,omitempty"`
}

// IgnoreConfig represents dependency ignore rules
type IgnoreConfig struct {
	DependencyName string   `yaml:"dependency-name,omitempty"`
	Versions       []string `yaml:"versions,omitempty"`
	UpdateTypes    []string `yaml:"update-types,omitempty"`
}

// AllowConfig represents dependency allow rules
type AllowConfig struct {
	DependencyName string `yaml:"dependency-name,omitempty"`
	DependencyType string `yaml:"dependency-type,omitempty"`
}

// Equal checks if two configs are equal
func (c *DependabotConfig) Equal(other *DependabotConfig) bool {
	if c == nil && other == nil {
		return true
	}
	if c == nil || other == nil {
		return false
	}
	if c.Version != other.Version {
		return false
	}
	if len(c.Updates) != len(other.Updates) {
		return false
	}

	// Simple comparison - in production would need deep comparison
	for i := range c.Updates {
		if !c.Updates[i].Equal(&other.Updates[i]) {
			return false
		}
	}
	return true
}

// Equal checks if two updates are equal
func (u *DependabotUpdate) Equal(other *DependabotUpdate) bool {
	if u.PackageEcosystem != other.PackageEcosystem {
		return false
	}
	if u.Directory != other.Directory {
		return false
	}
	if u.Schedule.Interval != other.Schedule.Interval {
		return false
	}
	if u.OpenPullRequestsLimit != other.OpenPullRequestsLimit {
		return false
	}
	// Additional comparisons would be needed for production
	return true
}
