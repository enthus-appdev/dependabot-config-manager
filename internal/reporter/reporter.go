package reporter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/enthus-appdev/dependabot-config-manager/internal/detector"
)

// Report represents a synchronization report
type Report struct {
	Timestamp         time.Time          `json:"timestamp"`
	Organization      string             `json:"organization"`
	Summary           Summary            `json:"summary"`
	RepositoryDetails []RepositoryDetail `json:"repositories"`
	Errors            []Error            `json:"errors,omitempty"`
	Duration          string             `json:"duration"`
}

// Summary contains overall statistics
type Summary struct {
	TotalRepositories      int            `json:"total_repositories"`
	ProcessedRepositories  int            `json:"processed_repositories"`
	ConfiguredRepositories int            `json:"configured_repositories"`
	UpdatedRepositories    int            `json:"updated_repositories"`
	SkippedRepositories    int            `json:"skipped_repositories"`
	FailedRepositories     int            `json:"failed_repositories"`
	CoveragePercentage     float64        `json:"coverage_percentage"`
	EcosystemBreakdown     map[string]int `json:"ecosystem_breakdown"`
}

// RepositoryDetail contains details about a specific repository
type RepositoryDetail struct {
	Name               string               `json:"name"`
	Status             string               `json:"status"` // configured, updated, skipped, failed
	DetectedEcosystems []detector.Ecosystem `json:"detected_ecosystems,omitempty"`
	HasExistingConfig  bool                 `json:"has_existing_config"`
	ConfigUpdated      bool                 `json:"config_updated"`
	SkipReason         string               `json:"skip_reason,omitempty"`
	Error              string               `json:"error,omitempty"`
	URL                string               `json:"url"`
	Topics             []string             `json:"topics,omitempty"`
}

// Error represents an error that occurred during processing
type Error struct {
	Repository string    `json:"repository"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

// Reporter handles report generation and output
type Reporter struct {
	startTime     time.Time
	report        *Report
	outputDir     string
	verboseOutput bool
}

// New creates a new reporter
func New(org, outputDir string, verbose bool) *Reporter {
	return &Reporter{
		startTime: time.Now(),
		report: &Report{
			Timestamp:    time.Now(),
			Organization: org,
			Summary: Summary{
				EcosystemBreakdown: make(map[string]int),
			},
			RepositoryDetails: []RepositoryDetail{},
			Errors:            []Error{},
		},
		outputDir:     outputDir,
		verboseOutput: verbose,
	}
}

// AddRepository adds a repository to the report
func (r *Reporter) AddRepository(repo *github.Repository, ecosystems []detector.Ecosystem, status string, skipReason string, err error) {
	detail := RepositoryDetail{
		Name:               repo.GetName(),
		Status:             status,
		DetectedEcosystems: ecosystems,
		URL:                repo.GetHTMLURL(),
		Topics:             repo.Topics,
		SkipReason:         skipReason,
	}

	if err != nil {
		detail.Error = err.Error()
		r.report.Errors = append(r.report.Errors, Error{
			Repository: repo.GetName(),
			Message:    err.Error(),
			Timestamp:  time.Now(),
		})
	}

	// Update ecosystem breakdown
	for _, eco := range ecosystems {
		r.report.Summary.EcosystemBreakdown[eco.Name]++
	}

	// Update summary counters
	r.report.Summary.TotalRepositories++

	switch status {
	case "configured":
		r.report.Summary.ConfiguredRepositories++
		detail.HasExistingConfig = true
	case "updated":
		r.report.Summary.UpdatedRepositories++
		detail.ConfigUpdated = true
	case "skipped":
		r.report.Summary.SkippedRepositories++
	case "failed":
		r.report.Summary.FailedRepositories++
	default:
		r.report.Summary.ProcessedRepositories++
	}

	r.report.RepositoryDetails = append(r.report.RepositoryDetails, detail)
}

// AddProcessedRepository adds a successfully processed repository
func (r *Reporter) AddProcessedRepository(repo *github.Repository, ecosystems []detector.Ecosystem, hasExisting, wasUpdated bool) {
	status := "configured"
	if wasUpdated {
		status = "updated"
	}

	r.AddRepository(repo, ecosystems, status, "", nil)
}

// AddSkippedRepository adds a skipped repository
func (r *Reporter) AddSkippedRepository(repo *github.Repository, reason string) {
	r.AddRepository(repo, nil, "skipped", reason, nil)
}

// AddFailedRepository adds a failed repository
func (r *Reporter) AddFailedRepository(repo *github.Repository, err error) {
	r.AddRepository(repo, nil, "failed", "", err)
}

// Finalize finalizes the report with calculated statistics
func (r *Reporter) Finalize() {
	r.report.Duration = time.Since(r.startTime).String()
	r.report.Summary.ProcessedRepositories = r.report.Summary.TotalRepositories -
		r.report.Summary.SkippedRepositories - r.report.Summary.FailedRepositories

	if r.report.Summary.TotalRepositories > 0 {
		r.report.Summary.CoveragePercentage = float64(r.report.Summary.ConfiguredRepositories+r.report.Summary.UpdatedRepositories) /
			float64(r.report.Summary.TotalRepositories) * 100
	}
}

// SaveReport saves the report to a file
func (r *Reporter) SaveReport(format string) error {
	r.Finalize()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02-150405")

	switch format {
	case "json":
		return r.saveJSON(timestamp)
	case "html":
		return r.saveHTML(timestamp)
	case "markdown":
		return r.saveMarkdown(timestamp)
	default:
		// Save all formats
		if err := r.saveJSON(timestamp); err != nil {
			return err
		}
		if err := r.saveHTML(timestamp); err != nil {
			return err
		}
		return r.saveMarkdown(timestamp)
	}
}

// saveJSON saves the report as JSON
func (r *Reporter) saveJSON(timestamp string) error {
	filename := filepath.Join(r.outputDir, fmt.Sprintf("dependabot-report-%s.json", timestamp))

	data, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	fmt.Printf("📊 Report saved to %s\n", filename)
	return nil
}

// saveMarkdown saves the report as Markdown
func (r *Reporter) saveMarkdown(timestamp string) error {
	filename := filepath.Join(r.outputDir, fmt.Sprintf("dependabot-report-%s.md", timestamp))

	var sb strings.Builder

	sb.WriteString("# Dependabot Configuration Report\n\n")
	sb.WriteString(fmt.Sprintf("**Organization:** %s\n", r.report.Organization))
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", r.report.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration:** %s\n\n", r.report.Duration))

	// Summary section
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Repositories:** %d\n", r.report.Summary.TotalRepositories))
	sb.WriteString(fmt.Sprintf("- **Configured:** %d\n", r.report.Summary.ConfiguredRepositories))
	sb.WriteString(fmt.Sprintf("- **Updated:** %d\n", r.report.Summary.UpdatedRepositories))
	sb.WriteString(fmt.Sprintf("- **Skipped:** %d\n", r.report.Summary.SkippedRepositories))
	sb.WriteString(fmt.Sprintf("- **Failed:** %d\n", r.report.Summary.FailedRepositories))
	sb.WriteString(fmt.Sprintf("- **Coverage:** %.1f%%\n\n", r.report.Summary.CoveragePercentage))

	// Ecosystem breakdown
	if len(r.report.Summary.EcosystemBreakdown) > 0 {
		sb.WriteString("## Ecosystem Distribution\n\n")
		sb.WriteString("| Ecosystem | Count |\n")
		sb.WriteString("|-----------|-------|\n")
		for eco, count := range r.report.Summary.EcosystemBreakdown {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", eco, count))
		}
		sb.WriteString("\n")
	}

	// Repository details
	sb.WriteString("## Repository Details\n\n")

	// Updated repositories
	updated := r.filterByStatus("updated")
	if len(updated) > 0 {
		sb.WriteString("### ✅ Updated Repositories\n\n")
		for _, repo := range updated {
			sb.WriteString(fmt.Sprintf("- [%s](%s)", repo.Name, repo.URL))
			if len(repo.DetectedEcosystems) > 0 {
				ecosystems := []string{}
				for _, eco := range repo.DetectedEcosystems {
					ecosystems = append(ecosystems, eco.Name)
				}
				sb.WriteString(fmt.Sprintf(" - %s", strings.Join(ecosystems, ", ")))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Failed repositories
	failed := r.filterByStatus("failed")
	if len(failed) > 0 {
		sb.WriteString("### ❌ Failed Repositories\n\n")
		for _, repo := range failed {
			sb.WriteString(fmt.Sprintf("- [%s](%s)", repo.Name, repo.URL))
			if repo.Error != "" {
				sb.WriteString(fmt.Sprintf(" - Error: %s", repo.Error))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Skipped repositories
	skipped := r.filterByStatus("skipped")
	if len(skipped) > 0 {
		sb.WriteString("### ⏭️ Skipped Repositories\n\n")
		for _, repo := range skipped {
			sb.WriteString(fmt.Sprintf("- [%s](%s)", repo.Name, repo.URL))
			if repo.SkipReason != "" {
				sb.WriteString(fmt.Sprintf(" - Reason: %s", repo.SkipReason))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Recommendations
	sb.WriteString("## Recommendations\n\n")

	if r.report.Summary.FailedRepositories > 0 {
		sb.WriteString("- ⚠️ Review failed repositories and resolve issues\n")
	}

	if r.report.Summary.CoveragePercentage < 80 {
		sb.WriteString("- 📈 Consider investigating skipped repositories to increase coverage\n")
	}

	if len(r.report.Summary.EcosystemBreakdown) > 5 {
		sb.WriteString("- 🎯 Consider creating specialized templates for frequently used ecosystems\n")
	}

	if err := ioutil.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown report: %w", err)
	}

	fmt.Printf("📝 Markdown report saved to %s\n", filename)
	return nil
}

// saveHTML saves the report as HTML
func (r *Reporter) saveHTML(timestamp string) error {
	filename := filepath.Join(r.outputDir, fmt.Sprintf("dependabot-report-%s.html", timestamp))

	html := r.generateHTML()

	if err := ioutil.WriteFile(filename, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}

	fmt.Printf("🌐 HTML report saved to %s\n", filename)
	return nil
}

// generateHTML generates an HTML report
func (r *Reporter) generateHTML() string {
	// Simple HTML template - in production, use html/template
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Dependabot Configuration Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        .summary { background: #f5f5f5; padding: 15px; border-radius: 5px; }
        .metric { display: inline-block; margin: 10px; padding: 10px; background: white; border-radius: 3px; }
        .success { color: green; }
        .warning { color: orange; }
        .error { color: red; }
        table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f5f5f5; }
    </style>
</head>
<body>
    <h1>Dependabot Configuration Report</h1>
    <p><strong>Organization:</strong> %s</p>
    <p><strong>Generated:</strong> %s</p>
    <div class="summary">
        <h2>Summary</h2>
        <div class="metric">Total: %d</div>
        <div class="metric success">Configured: %d</div>
        <div class="metric success">Updated: %d</div>
        <div class="metric warning">Skipped: %d</div>
        <div class="metric error">Failed: %d</div>
        <div class="metric">Coverage: %.1f%%</div>
    </div>
</body>
</html>`,
		r.report.Organization,
		r.report.Timestamp.Format(time.RFC3339),
		r.report.Summary.TotalRepositories,
		r.report.Summary.ConfiguredRepositories,
		r.report.Summary.UpdatedRepositories,
		r.report.Summary.SkippedRepositories,
		r.report.Summary.FailedRepositories,
		r.report.Summary.CoveragePercentage,
	)
}

// filterByStatus filters repositories by status
func (r *Reporter) filterByStatus(status string) []RepositoryDetail {
	var filtered []RepositoryDetail
	for _, repo := range r.report.RepositoryDetails {
		if repo.Status == status {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

// PrintSummary prints a summary to stdout
func (r *Reporter) PrintSummary() {
	r.Finalize()

	fmt.Println("\n📊 Synchronization Summary")
	fmt.Println("═══════════════════════════")
	fmt.Printf("Total Repositories: %d\n", r.report.Summary.TotalRepositories)
	fmt.Printf("✅ Configured: %d\n", r.report.Summary.ConfiguredRepositories)
	fmt.Printf("🔄 Updated: %d\n", r.report.Summary.UpdatedRepositories)
	fmt.Printf("⏭️  Skipped: %d\n", r.report.Summary.SkippedRepositories)
	fmt.Printf("❌ Failed: %d\n", r.report.Summary.FailedRepositories)
	fmt.Printf("📈 Coverage: %.1f%%\n", r.report.Summary.CoveragePercentage)
	fmt.Printf("⏱️  Duration: %s\n", r.report.Duration)

	if len(r.report.Summary.EcosystemBreakdown) > 0 {
		fmt.Println("\n🔧 Detected Ecosystems:")
		for eco, count := range r.report.Summary.EcosystemBreakdown {
			fmt.Printf("  - %s: %d repositories\n", eco, count)
		}
	}

	if len(r.report.Errors) > 0 {
		fmt.Printf("\n⚠️  %d errors occurred during synchronization\n", len(r.report.Errors))
	}
}
