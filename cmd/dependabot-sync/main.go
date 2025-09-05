package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/google/go-github/v50/github"
	"github.com/your-org/dependabot-config-manager/internal/config"
	"github.com/your-org/dependabot-config-manager/internal/detector"
	"github.com/your-org/dependabot-config-manager/internal/merger"
	githubClient "github.com/your-org/dependabot-config-manager/internal/github"
	"github.com/your-org/dependabot-config-manager/internal/reporter"
	"github.com/your-org/dependabot-config-manager/internal/util"
)

// Version is the application version
var Version = "1.0.0"

type options struct {
	token            string
	org              string
	dryRun           bool
	createPR         bool
	repositories     []string
	excludeArchived  bool
	excludeTopics    []string
	configDir        string
	reportDir        string
	reportFormat     string
	concurrency      int
	verbose          bool
	version          bool
	yamlIndent       int
}

func main() {
	opts := parseFlags()
	
	if opts.version {
		fmt.Printf("dependabot-sync version %s\n", Version)
		os.Exit(0)
	}
	
	if err := validateOptions(opts); err != nil {
		log.Fatalf("❌ Invalid options: %v", err)
	}
	
	ctx := context.Background()
	
	// Create GitHub client
	client := githubClient.NewClient(opts.token, opts.org)
	
	// Create detector
	det := detector.New(client.GetClient(), opts.org)
	
	// Create merger
	mrg, err := merger.New(opts.configDir)
	if err != nil {
		log.Fatalf("❌ Failed to initialize merger: %v", err)
	}
	
	// Create reporter
	rep := reporter.New(opts.org, opts.reportDir, opts.verbose)
	
	// Create synchronizer
	syncer := &Synchronizer{
		client:          client,
		detector:        det,
		merger:          mrg,
		reporter:        rep,
		options:         opts,
		semaphore:       make(chan struct{}, opts.concurrency),
		wg:              &sync.WaitGroup{},
	}
	
	// Run synchronization
	if err := syncer.Run(ctx); err != nil {
		log.Fatalf("❌ Synchronization failed: %v", err)
	}
	
	// Save report
	if err := rep.SaveReport(opts.reportFormat); err != nil {
		log.Printf("⚠️  Failed to save report: %v", err)
	}
	
	// Print summary
	rep.PrintSummary()
}

// Synchronizer orchestrates the synchronization process
type Synchronizer struct {
	client    *githubClient.Client
	detector  *detector.Detector
	merger    *merger.Merger
	reporter  *reporter.Reporter
	options   *options
	semaphore chan struct{}
	wg        *sync.WaitGroup
	mu        sync.Mutex
}

// Run executes the synchronization process
func (s *Synchronizer) Run(ctx context.Context) error {
	fmt.Printf("🔄 Starting Dependabot configuration sync for organization: %s\n", s.options.org)
	
	if s.options.dryRun {
		fmt.Println("🔍 Running in DRY-RUN mode - no changes will be made")
	}
	
	// Get repositories
	repos, err := s.getRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repositories: %w", err)
	}
	
	fmt.Printf("📚 Found %d repositories to process\n", len(repos))
	
	// Process repositories concurrently
	for _, repo := range repos {
		s.wg.Add(1)
		go s.processRepository(ctx, repo)
	}
	
	// Wait for all processing to complete
	s.wg.Wait()
	
	return nil
}

// getRepositories gets the list of repositories to process
func (s *Synchronizer) getRepositories(ctx context.Context) ([]*github.Repository, error) {
	if len(s.options.repositories) > 0 {
		// Get specific repositories
		var repos []*github.Repository
		for _, name := range s.options.repositories {
			repo, err := s.client.GetRepository(ctx, name)
			if err != nil {
				log.Printf("⚠️  Failed to get repository %s: %v", name, err)
				continue
			}
			repos = append(repos, repo)
		}
		return repos, nil
	}
	
	// Get all organization repositories
	return s.client.ListRepositories(ctx, s.options.excludeArchived)
}

// processRepository processes a single repository
func (s *Synchronizer) processRepository(ctx context.Context, repo *github.Repository) {
	defer s.wg.Done()
	
	// Acquire semaphore
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()
	
	repoName := repo.GetName()
	
	if s.options.verbose {
		fmt.Printf("🔍 Processing repository: %s\n", repoName)
	}
	
	// Check exclusion topics
	if s.detector.HasExclusionTopic(ctx, repo) {
		s.reporter.AddSkippedRepository(repo, "has exclusion topic")
		if s.options.verbose {
			fmt.Printf("⏭️  Skipping %s: has exclusion topic\n", repoName)
		}
		return
	}
	
	// Detect ecosystems
	ecosystems, err := s.detector.Detect(ctx, repoName)
	if err != nil {
		s.reporter.AddFailedRepository(repo, err)
		log.Printf("❌ Failed to detect ecosystems in %s: %v", repoName, err)
		return
	}
	
	if len(ecosystems) == 0 {
		s.reporter.AddSkippedRepository(repo, "no supported ecosystems detected")
		if s.options.verbose {
			fmt.Printf("⏭️  Skipping %s: no supported ecosystems\n", repoName)
		}
		return
	}
	
	// Get existing configuration
	existingConfig, err := s.client.GetExistingConfig(ctx, repoName)
	if err != nil {
		s.reporter.AddFailedRepository(repo, err)
		log.Printf("❌ Failed to get existing config for %s: %v", repoName, err)
		return
	}
	
	// Merge configurations
	mergedConfig := s.merger.Merge(existingConfig, ecosystems)
	
	// Check if update is needed
	if existingConfig != nil && existingConfig.Equal(mergedConfig) {
		s.reporter.AddProcessedRepository(repo, ecosystems, true, false)
		if s.options.verbose {
			fmt.Printf("✅ %s: already configured\n", repoName)
		}
		return
	}
	
	// Apply configuration (if not dry run)
	if !s.options.dryRun {
		if err := s.applyConfiguration(ctx, repoName, mergedConfig); err != nil {
			s.reporter.AddFailedRepository(repo, err)
			log.Printf("❌ Failed to apply config to %s: %v", repoName, err)
			return
		}
	}
	
	s.reporter.AddProcessedRepository(repo, ecosystems, existingConfig != nil, true)
	
	action := "would be updated"
	if !s.options.dryRun {
		if s.options.createPR {
			action = "PR created"
		} else {
			action = "updated"
		}
	}
	
	fmt.Printf("✅ %s: %s (ecosystems: ", repoName, action)
	for i, eco := range ecosystems {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(eco.Name)
	}
	fmt.Println(")")
}

// applyConfiguration applies the configuration to a repository
func (s *Synchronizer) applyConfiguration(ctx context.Context, repoName string, cfg *config.DependabotConfig) error {
	if s.options.createPR {
		return s.client.CreatePullRequest(ctx, repoName, cfg, s.options.yamlIndent)
	}
	
	// Direct commit to main branch
	content, err := util.MarshalYAML(cfg, s.options.yamlIndent)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// Get existing file SHA if it exists
	_, sha, _ := s.client.GetFileContent(ctx, repoName, ".github/dependabot.yml")
	
	message := "Configure Dependabot for dependency updates"
	if sha != "" {
		message = "Update Dependabot configuration"
	}
	
	return s.client.CreateOrUpdateFile(ctx, repoName, ".github/dependabot.yml", message, content, sha)
}

// parseFlags parses command-line flags
func parseFlags() *options {
	opts := &options{}
	
	flag.StringVar(&opts.token, "token", os.Getenv("GITHUB_TOKEN"), "GitHub personal access token (or set GITHUB_TOKEN env var)")
	flag.StringVar(&opts.org, "org", os.Getenv("GITHUB_ORG"), "GitHub organization name (or set GITHUB_ORG env var)")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "Perform a dry run without making changes")
	flag.BoolVar(&opts.createPR, "create-pr", false, "Create pull requests instead of direct commits")
	flag.BoolVar(&opts.excludeArchived, "exclude-archived", true, "Exclude archived repositories")
	flag.StringVar(&opts.configDir, "config-dir", "./configs", "Directory containing configuration templates")
	flag.StringVar(&opts.reportDir, "report-dir", "./reports", "Directory for saving reports")
	flag.StringVar(&opts.reportFormat, "report-format", "all", "Report format: json, html, markdown, or all")
	flag.IntVar(&opts.concurrency, "concurrency", 10, "Number of concurrent repository operations")
	flag.BoolVar(&opts.verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&opts.version, "version", false, "Show version information")
	flag.IntVar(&opts.yamlIndent, "yaml-indent", 2, "Number of spaces for YAML indentation")
	
	// Custom flag for repositories list
	var reposList string
	flag.StringVar(&reposList, "repos", "", "Comma-separated list of specific repositories to process")
	
	// Custom flag for exclude topics
	var excludeTopics string
	flag.StringVar(&excludeTopics, "exclude-topics", "no-dependabot,skip-dependabot", "Comma-separated list of topics that exclude a repository")
	
	flag.Parse()
	
	// Parse repositories list
	if reposList != "" {
		opts.repositories = parseCSV(reposList)
	}
	
	// Parse exclude topics
	if excludeTopics != "" {
		opts.excludeTopics = parseCSV(excludeTopics)
	}
	
	return opts
}

// validateOptions validates the provided options
func validateOptions(opts *options) error {
	if opts.token == "" {
		return fmt.Errorf("GitHub token is required (use -token flag or GITHUB_TOKEN env var)")
	}
	
	if opts.org == "" {
		return fmt.Errorf("GitHub organization is required (use -org flag or GITHUB_ORG env var)")
	}
	
	if opts.concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	
	if opts.concurrency > 50 {
		return fmt.Errorf("concurrency should not exceed 50 to avoid rate limiting")
	}
	
	// Check config directory exists
	if _, err := os.Stat(opts.configDir); os.IsNotExist(err) {
		return fmt.Errorf("config directory does not exist: %s", opts.configDir)
	}
	
	// Validate report format
	validFormats := map[string]bool{
		"json":     true,
		"html":     true,
		"markdown": true,
		"all":      true,
	}
	
	if !validFormats[opts.reportFormat] {
		return fmt.Errorf("invalid report format: %s (must be json, html, markdown, or all)", opts.reportFormat)
	}
	
	return nil
}

// parseCSV parses a comma-separated string into a slice
func parseCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	
	var result []string
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}