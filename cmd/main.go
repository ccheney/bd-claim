// Package main provides the bd-claim CLI entrypoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ccheney/bd-claim/internal/application"
	"github.com/ccheney/bd-claim/internal/domain"
	"github.com/ccheney/bd-claim/internal/infrastructure"
)

// Version is set at build time.
var Version = "dev"

// osExit is a variable for testing; defaults to os.Exit.
var osExit = os.Exit

// stdout is the default output writer.
var stdout io.Writer = os.Stdout

// stderr is the default error writer.
var stderr io.Writer = os.Stderr

type arrayFlag []string

func (a *arrayFlag) String() string {
	return fmt.Sprintf("%v", *a)
}

func (a *arrayFlag) Set(value string) error {
	*a = append(*a, value)
	return nil
}

type config struct {
	agent          string
	labels         arrayFlag
	excludeLabels  arrayFlag
	minPriority    int
	onlyUnassigned bool
	workspace      string
	dbPath         string
	dryRun         bool
	jsonOutput     bool
	pretty         bool
	human          bool
	timeoutMs      int
	logLevel       string
	showVersion    bool
	skipVersionCheck bool
}

func main() {
	exitCode := runApp(os.Args[1:])
	osExit(exitCode)
}

func runApp(args []string) int {
	cfg, err := parseFlagsFromArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error parsing flags: %s\n", err.Error())
		return 1
	}

	if cfg.showVersion {
		fmt.Fprintf(stdout, "bd-claim version %s\n", Version)
		return 0
	}

	result := run(cfg)
	return outputResult(cfg, result)
}

func parseFlagsFromArgs(args []string) (config, error) {
	cfg := config{}
	fs := flag.NewFlagSet("bd-claim", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Suppress default usage output

	fs.StringVar(&cfg.agent, "agent", "", "Agent name (required)")
	fs.Var(&cfg.labels, "label", "Include issues with this label (repeatable)")
	fs.Var(&cfg.excludeLabels, "exclude-label", "Exclude issues with this label (repeatable)")
	fs.IntVar(&cfg.minPriority, "min-priority", -1, "Minimum priority level (0=low, 1=medium, 2=high)")
	fs.BoolVar(&cfg.onlyUnassigned, "only-unassigned", false, "Only consider unassigned issues")
	fs.StringVar(&cfg.workspace, "workspace", "", "Override workspace root path")
	fs.StringVar(&cfg.dbPath, "db", "", "Override database path")
	fs.BoolVar(&cfg.dryRun, "dry-run", false, "Show which issue would be claimed without updating")
	fs.BoolVar(&cfg.jsonOutput, "json", true, "Output in JSON format (default)")
	fs.BoolVar(&cfg.pretty, "pretty", false, "Pretty-print JSON output")
	fs.BoolVar(&cfg.human, "human", false, "Human-friendly output")
	fs.IntVar(&cfg.timeoutMs, "timeout-ms", 3000, "Database busy timeout in milliseconds")
	fs.StringVar(&cfg.logLevel, "log-level", "error", "Log level (debug, info, warn, error)")
	fs.BoolVar(&cfg.showVersion, "version", false, "Show version")
	fs.BoolVar(&cfg.skipVersionCheck, "skip-version-check", false, "Skip database version compatibility check")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	return cfg, nil
}

func run(cfg config) application.ClaimIssueResult {
	// Validate agent name
	if cfg.agent == "" {
		return errorResult("", domain.ErrCodeInvalidArgument, "--agent flag is required")
	}

	agent, err := domain.NewAgentName(cfg.agent)
	if err != nil {
		return errorResult(cfg.agent, domain.ErrCodeInvalidArgument, err.Error())
	}

	// Set up logger
	logLevel := parseLogLevel(cfg.logLevel)
	logger := infrastructure.NewJSONLogger(logLevel)

	// Discover workspace
	workspaceAdapter := infrastructure.NewWorkspaceDiscoveryAdapter()

	var dbPath string
	var workspaceRoot string
	if cfg.dbPath != "" {
		dbPath = cfg.dbPath
		logger.Debug("workspace_discovery", map[string]interface{}{
			"cwd":            "",
			"workspace_root": "",
			"db_path":        dbPath,
			"source":         "override",
		})
	} else {
		cwd := cfg.workspace
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return errorResult(cfg.agent, domain.ErrCodeUnexpected, "failed to get working directory: "+err.Error())
			}
		}

		var err error
		workspaceRoot, err = workspaceAdapter.FindWorkspaceRoot(cwd)
		if err != nil {
			return handleDomainError(cfg.agent, err)
		}

		dbPath, err = workspaceAdapter.FindBeadsDbPath(workspaceRoot)
		if err != nil {
			return handleDomainError(cfg.agent, err)
		}

		logger.Debug("workspace_discovery", map[string]interface{}{
			"cwd":            cwd,
			"workspace_root": workspaceRoot,
			"db_path":        dbPath,
			"source":         "auto",
		})
	}

	// Set up repository
	repo, err := infrastructure.NewSQLiteIssueRepository(dbPath, cfg.timeoutMs)
	if err != nil {
		return handleDomainError(cfg.agent, err)
	}
	defer repo.Close()

	// Check version compatibility
	if !cfg.skipVersionCheck {
		if err := repo.CheckVersionCompatibility(context.Background()); err != nil {
			logger.Warn("version_check_failed", map[string]interface{}{
				"error":       err.Error(),
				"min_version": infrastructure.MinCompatibleBdVersion,
			})
			return handleDomainError(cfg.agent, err)
		}
	}

	// Build filters
	filters := domain.NewClaimFilters()
	filters.OnlyUnassigned = cfg.onlyUnassigned
	filters.IncludeLabels = cfg.labels
	filters.ExcludeLabels = cfg.excludeLabels
	if cfg.minPriority >= 0 {
		p := domain.Priority(cfg.minPriority)
		filters.MinPriority = &p
	}

	// Set up use case
	clock := infrastructure.NewSystemClock()
	useCase := application.NewClaimIssueUseCase(repo, clock, logger)

	// Execute
	req := application.ClaimIssueRequest{
		Agent:     agent,
		Filters:   filters,
		DryRun:    cfg.dryRun,
		TimeoutMs: cfg.timeoutMs,
	}

	return useCase.Execute(context.Background(), req)
}

func errorResult(agent string, code domain.ClaimErrorCode, message string) application.ClaimIssueResult {
	return application.ClaimIssueResult{
		Status: "error",
		Agent:  agent,
		Issue:  nil,
		Error: &application.ClaimErrorDTO{
			Code:    string(code),
			Message: message,
		},
	}
}

func handleDomainError(agent string, err error) application.ClaimIssueResult {
	if claimErr, ok := err.(*domain.ClaimFailed); ok {
		return application.ClaimIssueResult{
			Status: "error",
			Agent:  agent,
			Issue:  nil,
			Error: &application.ClaimErrorDTO{
				Code:    string(claimErr.ErrorCode),
				Message: claimErr.Message,
			},
		}
	}
	return errorResult(agent, domain.ErrCodeUnexpected, err.Error())
}

func outputResult(cfg config, result application.ClaimIssueResult) int {
	if cfg.human {
		return outputHuman(result)
	}
	return outputJSON(cfg, result)
}

func outputJSON(cfg config, result application.ClaimIssueResult) int {
	var data []byte
	var err error

	if cfg.pretty {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}

	if err != nil {
		fmt.Fprintf(stderr, "failed to marshal result: %s\n", err.Error())
		return 1
	}

	fmt.Fprintln(stdout, string(data))

	if result.Status == "error" {
		return 1
	}
	return 0
}

func outputHuman(result application.ClaimIssueResult) int {
	if result.Status == "error" {
		fmt.Fprintf(stdout, "Error: [%s] %s\n", result.Error.Code, result.Error.Message)
		return 1
	}

	if result.Issue == nil {
		fmt.Fprintf(stdout, "No issue available for agent '%s'\n", result.Agent)
		return 0
	}

	fmt.Fprintf(stdout, "Claimed issue %s: %s\n", result.Issue.ID, result.Issue.Title)
	fmt.Fprintf(stdout, "  Status: %s\n", result.Issue.Status)
	fmt.Fprintf(stdout, "  Assignee: %s\n", *result.Issue.Assignee)
	fmt.Fprintf(stdout, "  Priority: %d\n", result.Issue.Priority)
	if len(result.Issue.Labels) > 0 {
		fmt.Fprintf(stdout, "  Labels: %v\n", result.Issue.Labels)
	}
	return 0
}

func parseLogLevel(level string) infrastructure.LogLevel {
	switch level {
	case "debug":
		return infrastructure.LogLevelDebug
	case "info":
		return infrastructure.LogLevelInfo
	case "warn":
		return infrastructure.LogLevelWarn
	case "error":
		return infrastructure.LogLevelError
	default:
		return infrastructure.LogLevelError
	}
}
