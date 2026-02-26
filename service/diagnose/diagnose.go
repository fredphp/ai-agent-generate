// Package diagnose provides project diagnosis and auto-fix capabilities.
// It detects issues from project startup to runtime and fixes them automatically.
package diagnose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// IssueLevel represents the severity of an issue.
type IssueLevel string

const (
	LevelCritical IssueLevel = "critical" // Blocking, must fix
	LevelError    IssueLevel = "error"    // Important, should fix
	LevelWarning  IssueLevel = "warning"  // Minor, recommended to fix
	LevelInfo     IssueLevel = "info"     // Informational
)

// IssueCategory represents the category of an issue.
type IssueCategory string

const (
	CategoryConfig    IssueCategory = "config"    // Configuration issues
	CategoryDependency IssueCategory = "dependency" // Dependency issues
	CategoryBuild     IssueCategory = "build"     // Compilation issues
	CategoryRuntime   IssueCategory = "runtime"   // Runtime issues
	CategoryTest      IssueCategory = "test"      // Test issues
	CategoryLint      IssueCategory = "lint"      // Lint issues
	CategorySecurity  IssueCategory = "security"  // Security issues
)

// Issue represents a detected issue.
type Issue struct {
	ID          string        `json:"id"`
	Category    IssueCategory `json:"category"`
	Level       IssueLevel    `json:"level"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	File        string        `json:"file,omitempty"`
	Line        int           `json:"line,omitempty"`
	Column      int           `json:"column,omitempty"`
	Snippet     string        `json:"snippet,omitempty"`
	Suggestion  string        `json:"suggestion,omitempty"`
	RawOutput   string        `json:"raw_output,omitempty"`
	Fixed       bool          `json:"fixed"`
	FixResult   string        `json:"fix_result,omitempty"`
}

// DiagnosticResult represents the result of a diagnostic run.
type DiagnosticResult struct {
	ProjectPath    string    `json:"project_path"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Duration       string    `json:"duration"`
	TotalIssues    int       `json:"total_issues"`
	CriticalCount  int       `json:"critical_count"`
	ErrorCount     int       `json:"error_count"`
	WarningCount   int       `json:"warning_count"`
	FixedCount     int       `json:"fixed_count"`
	Issues         []Issue   `json:"issues"`
	BuildSuccess   bool      `json:"build_success"`
	TestSuccess    bool      `json:"test_success"`
	RunSuccess     bool      `json:"run_success"`
	Summary        string    `json:"summary"`
}

// Config holds diagnostic configuration.
type Config struct {
	ProjectPath    string
	Timeout        time.Duration
	CheckConfig    bool
	CheckDeps      bool
	CheckBuild     bool
	CheckTests     bool
	CheckRuntime   bool
	CheckLint      bool
	AutoFix        bool
	MaxFixAttempts int
	Verbose        bool
}

// Diagnoser performs project diagnosis.
type Diagnoser struct {
	config Config
	issues []Issue
}

// NewDiagnoser creates a new diagnoser.
func NewDiagnoser(config Config) *Diagnoser {
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.MaxFixAttempts == 0 {
		config.MaxFixAttempts = 3
	}
	return &Diagnoser{
		config: config,
		issues: make([]Issue, 0),
	}
}

// Run performs full project diagnosis.
func (d *Diagnoser) Run(ctx context.Context) (*DiagnosticResult, error) {
	startTime := time.Now()
	result := &DiagnosticResult{
		ProjectPath: d.config.ProjectPath,
		StartTime:   startTime,
	}

	// Change to project directory
	originalDir, _ := os.Getwd()
	if err := os.Chdir(d.config.ProjectPath); err != nil {
		return nil, fmt.Errorf("failed to change to project directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Run diagnostic checks
	if d.config.CheckConfig {
		d.checkConfig(ctx)
	}

	if d.config.CheckDeps {
		d.checkDependencies(ctx)
	}

	if d.config.CheckBuild {
		result.BuildSuccess = d.checkBuild(ctx)
	}

	if d.config.CheckLint {
		d.checkLint(ctx)
	}

	if d.config.CheckTests {
		result.TestSuccess = d.checkTests(ctx)
	}

	if d.config.CheckRuntime {
		result.RunSuccess = d.checkRuntime(ctx)
	}

	// Compile results
	endTime := time.Now()
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime).String()
	result.Issues = d.issues
	result.TotalIssues = len(d.issues)

	for _, issue := range d.issues {
		switch issue.Level {
		case LevelCritical:
			result.CriticalCount++
		case LevelError:
			result.ErrorCount++
		case LevelWarning:
			result.WarningCount++
		}
		if issue.Fixed {
			result.FixedCount++
		}
	}

	result.Summary = d.generateSummary()

	return result, nil
}

// checkConfig checks project configuration files.
func (d *Diagnoser) checkConfig(ctx context.Context) {
	// Check go.mod
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		d.addIssue(Issue{
			ID:          "config-go-mod-missing",
			Category:    CategoryConfig,
			Level:       LevelCritical,
			Title:       "go.mod file missing",
			Description: "Project is not a Go module. Run 'go mod init' to initialize.",
			Suggestion:  "Run: go mod init <module-name>",
		})
	} else {
		// Parse go.mod for issues
		content, err := os.ReadFile("go.mod")
		if err == nil {
			d.analyzeGoMod(string(content))
		}
	}

	// Check for common config files
	configFiles := []string{
		".env.example",
		"config.yaml",
		"config.json",
		"Dockerfile",
		"docker-compose.yml",
	}

	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			if d.config.Verbose {
				fmt.Printf("✓ Found config file: %s\n", file)
			}
		}
	}
}

// analyzeGoMod analyzes go.mod content for issues.
func (d *Diagnoser) analyzeGoMod(content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Check for replace directives that might cause issues
		if strings.Contains(line, "replace") && strings.Contains(line, "=>") {
			if d.config.Verbose {
				fmt.Printf("ℹ Found replace directive: %s\n", strings.TrimSpace(line))
			}
		}
	}
}

// checkDependencies checks project dependencies.
func (d *Diagnoser) checkDependencies(ctx context.Context) {
	// Run go mod verify
	cmd := exec.CommandContext(ctx, "go", "mod", "verify")
	output, err := cmd.CombinedOutput()
	if err != nil {
		d.addIssue(Issue{
			ID:          "dep-verify-failed",
			Category:    CategoryDependency,
			Level:       LevelWarning,
			Title:       "Dependency verification failed",
			Description: string(output),
			Suggestion:  "Run 'go mod tidy' and 'go mod download' to fix dependencies",
			RawOutput:   string(output),
		})
	}

	// Check for unused dependencies
	cmd = exec.CommandContext(ctx, "go", "mod", "tidy", "-v")
	output, _ = cmd.CombinedOutput()
	if strings.Contains(string(output), "unused") {
		d.addIssue(Issue{
			ID:          "dep-unused",
			Category:    CategoryDependency,
			Level:       LevelInfo,
			Title:       "Unused dependencies detected",
			Description: "Some dependencies are not used in the code",
			Suggestion:  "Run 'go mod tidy' to clean up",
			RawOutput:   string(output),
		})
	}
}

// checkBuild checks if the project builds successfully.
func (d *Diagnoser) checkBuild(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "go", "build", "-v", "./...")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		issues := d.parseBuildErrors(string(output))
		for _, issue := range issues {
			d.addIssue(issue)
		}
		return false
	}

	if d.config.Verbose {
		fmt.Println("✓ Build successful")
	}
	return true
}

// parseBuildErrors parses build error output into issues.
func (d *Diagnoser) parseBuildErrors(output string) []Issue {
	var issues []Issue

	// Parse Go compiler errors
	// Format: file.go:line:column: error message
	errorPattern := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.+)$`)
	// Parse undefined errors
	undefinedPattern := regexp.MustCompile(`undefined:\s*(\w+)`)
	// Parse import errors
	importPattern := regexp.MustCompile(`could not import\s+(.+)`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for standard error format
		if matches := errorPattern.FindStringSubmatch(line); matches != nil {
			issue := Issue{
				Category:  CategoryBuild,
				Level:     LevelError,
				File:      matches[1],
				Line:      parseInt(matches[2]),
				Column:    parseInt(matches[3]),
				RawOutput: line,
			}

			errMsg := matches[4]

			// Categorize the error
			if strings.Contains(errMsg, "undefined") {
				if undefMatches := undefinedPattern.FindStringSubmatch(errMsg); undefMatches != nil {
					issue.ID = fmt.Sprintf("build-undefined-%s", undefMatches[1])
					issue.Title = fmt.Sprintf("Undefined identifier: %s", undefMatches[1])
					issue.Description = errMsg
					issue.Suggestion = fmt.Sprintf("Check if '%s' is defined or imported correctly", undefMatches[1])
				}
			} else if strings.Contains(errMsg, "could not import") {
				if importMatches := importPattern.FindStringSubmatch(errMsg); importMatches != nil {
					issue.ID = fmt.Sprintf("build-import-%s", sanitizeID(importMatches[1]))
					issue.Title = fmt.Sprintf("Import failed: %s", importMatches[1])
					issue.Description = errMsg
					issue.Suggestion = "Check if the package exists and run 'go mod tidy'"
				}
			} else if strings.Contains(errMsg, "declared but not used") {
				issue.ID = fmt.Sprintf("build-unused-%s-%d", sanitizeID(matches[1]), issue.Line)
				issue.Title = "Unused variable/declaration"
				issue.Description = errMsg
				issue.Level = LevelWarning
				issue.Suggestion = "Remove unused declaration or use the variable"
			} else if strings.Contains(errMsg, "cannot use") {
				issue.ID = fmt.Sprintf("build-type-mismatch-%s-%d", sanitizeID(matches[1]), issue.Line)
				issue.Title = "Type mismatch"
				issue.Description = errMsg
				issue.Suggestion = "Check type compatibility"
			} else {
				issue.ID = fmt.Sprintf("build-error-%s-%d", sanitizeID(matches[1]), issue.Line)
				issue.Title = "Build error"
				issue.Description = errMsg
			}

			issues = append(issues, issue)
		} else if strings.Contains(line, "error") || strings.Contains(line, "Error") {
			// Generic error line
			issue := Issue{
				ID:          fmt.Sprintf("build-generic-%d", len(issues)),
				Category:    CategoryBuild,
				Level:       LevelError,
				Title:       "Build error",
				Description: line,
				RawOutput:   line,
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

// checkLint runs linter checks.
func (d *Diagnoser) checkLint(ctx context.Context) {
	// Check if golangci-lint is available
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		// Fallback to go vet
		cmd := exec.CommandContext(ctx, "go", "vet", "./...")
		output, err := cmd.CombinedOutput()
		if err != nil {
			issues := d.parseVetErrors(string(output))
			for _, issue := range issues {
				d.addIssue(issue)
			}
		}
		return
	}

	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--timeout", "5m", "--issues-exit-code", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		issues := d.parseLintErrors(string(output))
		for _, issue := range issues {
			d.addIssue(issue)
		}
	}
}

// parseVetErrors parses go vet output.
func (d *Diagnoser) parseVetErrors(output string) []Issue {
	var issues []Issue

	errorPattern := regexp.MustCompile(`^([^:]+):(\d+):\s*(.+)$`)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := errorPattern.FindStringSubmatch(line); matches != nil {
			issue := Issue{
				ID:          fmt.Sprintf("vet-%s-%s", sanitizeID(matches[1]), matches[2]),
				Category:    CategoryLint,
				Level:       LevelWarning,
				Title:       "go vet issue",
				Description: matches[3],
				File:        matches[1],
				Line:        parseInt(matches[2]),
				RawOutput:   line,
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

// parseLintErrors parses golangci-lint output.
func (d *Diagnoser) parseLintErrors(output string) []Issue {
	var issues []Issue

	// Format: file.go:line:column: message (linter)
	errorPattern := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.+)\s+\((\w+)\)$`)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := errorPattern.FindStringSubmatch(line); matches != nil {
			linter := matches[5]
			level := LevelWarning
			if linter == "errcheck" || linter == "staticcheck" {
				level = LevelError
			}

			issue := Issue{
				ID:          fmt.Sprintf("lint-%s-%s-%d", linter, sanitizeID(matches[1]), parseInt(matches[2])),
				Category:    CategoryLint,
				Level:       level,
				Title:       fmt.Sprintf("[%s] %s", linter, matches[4]),
				Description: matches[4],
				File:        matches[1],
				Line:        parseInt(matches[2]),
				Column:      parseInt(matches[3]),
				RawOutput:   line,
				Suggestion:  d.getLintSuggestion(linter, matches[4]),
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

// getLintSuggestion returns suggestion for a linter issue.
func (d *Diagnoser) getLintSuggestion(linter, message string) string {
	suggestions := map[string]string{
		"errcheck":    "Check and handle the returned error",
		"govet":       "Follow the suggested fix from go vet",
		"staticcheck": "Apply the suggested fix from staticcheck",
		"ineffassign": "Remove or use the assigned variable",
		"deadcode":    "Remove unused code",
		"unused":      "Remove or use the unused code",
		"gosec":       "Review and fix the security issue",
	}

	if suggestion, ok := suggestions[linter]; ok {
		return suggestion
	}
	return "Review and fix the issue"
}

// checkTests runs tests and captures failures.
func (d *Diagnoser) checkTests(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-json", "./...")
	output, err := cmd.CombinedOutput()

	if err != nil {
		issues := d.parseTestErrors(string(output))
		for _, issue := range issues {
			d.addIssue(issue)
		}
		return false
	}

	if d.config.Verbose {
		fmt.Println("✓ Tests passed")
	}
	return true
}

// parseTestErrors parses test output for failures.
func (d *Diagnoser) parseTestErrors(output string) []Issue {
	var issues []Issue

	type TestEvent struct {
		Time    string `json:"Time"`
		Action  string `json:"Action"`
		Package string `json:"Package"`
		Test    string `json:"Test"`
		Output  string `json:"Output"`
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Action == "fail" && event.Test != "" {
			issue := Issue{
				ID:          fmt.Sprintf("test-fail-%s-%s", sanitizeID(event.Package), sanitizeID(event.Test)),
				Category:    CategoryTest,
				Level:       LevelError,
				Title:       fmt.Sprintf("Test failed: %s", event.Test),
				Description: fmt.Sprintf("Test '%s' in package '%s' failed", event.Test, event.Package),
				RawOutput:   event.Output,
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

// checkRuntime attempts to run the project and capture errors.
func (d *Diagnoser) checkRuntime(ctx context.Context) bool {
	// Find main package
	mainFile := d.findMainFile()
	if mainFile == "" {
		// No main file, skip runtime check
		return true
	}

	// Build and run with timeout
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "go", "run", mainFile)
	output, err := cmd.CombinedOutput()

	if err != nil && runCtx.Err() != context.DeadlineExceeded {
		// Process exited with error (not timeout)
		issues := d.parseRuntimeErrors(string(output))
		for _, issue := range issues {
			d.addIssue(issue)
		}
		return false
	}

	return true
}

// findMainFile finds the main.go file.
func (d *Diagnoser) findMainFile() string {
	// Check common locations
	locations := []string{
		"main.go",
		"cmd/main.go",
		"cmd/server/main.go",
		"cmd/app/main.go",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Search for main.go
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, "main.go") {
			locations = append(locations, path)
		}
		return nil
	})

	if len(locations) > 0 {
		return locations[0]
	}
	return ""
}

// parseRuntimeErrors parses runtime error output.
func (d *Diagnoser) parseRuntimeErrors(output string) []Issue {
	var issues []Issue

	// Panic pattern
	panicPattern := regexp.MustCompile(`panic:\s*(.+)`)
	if matches := panicPattern.FindStringSubmatch(output); matches != nil {
		issue := Issue{
			ID:          "runtime-panic",
			Category:    CategoryRuntime,
			Level:       LevelCritical,
			Title:       "Runtime panic",
			Description: matches[1],
			RawOutput:   output,
			Suggestion:  "Review the panic stack trace and fix the root cause",
		}
		issues = append(issues, issue)
	}

	// Nil pointer
	nilPattern := regexp.MustCompile(`nil pointer dereference`)
	if nilPattern.MatchString(output) {
		issue := Issue{
			ID:          "runtime-nil-pointer",
			Category:    CategoryRuntime,
			Level:       LevelCritical,
			Title:       "Nil pointer dereference",
			Description: "The program attempted to access a nil pointer",
			RawOutput:   output,
			Suggestion:  "Add nil checks before accessing pointers",
		}
		issues = append(issues, issue)
	}

	// Index out of range
	indexPattern := regexp.MustCompile(`index out of range`)
	if indexPattern.MatchString(output) {
		issue := Issue{
			ID:          "runtime-index-out-of-range",
			Category:    CategoryRuntime,
			Level:       LevelCritical,
			Title:       "Index out of range",
			Description: "Array/slice index out of bounds",
			RawOutput:   output,
			Suggestion:  "Add bounds checking before accessing array/slice elements",
		}
		issues = append(issues, issue)
	}

	return issues
}

// addIssue adds an issue to the list.
func (d *Diagnoser) addIssue(issue Issue) {
	d.issues = append(d.issues, issue)
}

// generateSummary generates a summary of the diagnostic.
func (d *Diagnoser) generateSummary() string {
	var parts []string

	if len(d.issues) == 0 {
		return "No issues found. Project is healthy!"
	}

	parts = append(parts, fmt.Sprintf("Found %d issue(s):", len(d.issues)))

	if d.config.CheckConfig {
		configIssues := d.countByCategory(CategoryConfig)
		if configIssues > 0 {
			parts = append(parts, fmt.Sprintf("  - Config: %d", configIssues))
		}
	}

	if d.config.CheckDeps {
		depIssues := d.countByCategory(CategoryDependency)
		if depIssues > 0 {
			parts = append(parts, fmt.Sprintf("  - Dependencies: %d", depIssues))
		}
	}

	if d.config.CheckBuild {
		buildIssues := d.countByCategory(CategoryBuild)
		if buildIssues > 0 {
			parts = append(parts, fmt.Sprintf("  - Build: %d", buildIssues))
		}
	}

	if d.config.CheckLint {
		lintIssues := d.countByCategory(CategoryLint)
		if lintIssues > 0 {
			parts = append(parts, fmt.Sprintf("  - Lint: %d", lintIssues))
		}
	}

	if d.config.CheckTests {
		testIssues := d.countByCategory(CategoryTest)
		if testIssues > 0 {
			parts = append(parts, fmt.Sprintf("  - Tests: %d", testIssues))
		}
	}

	if d.config.CheckRuntime {
		runtimeIssues := d.countByCategory(CategoryRuntime)
		if runtimeIssues > 0 {
			parts = append(parts, fmt.Sprintf("  - Runtime: %d", runtimeIssues))
		}
	}

	return strings.Join(parts, "\n")
}

// countByCategory counts issues by category.
func (d *Diagnoser) countByCategory(category IssueCategory) int {
	count := 0
	for _, issue := range d.issues {
		if issue.Category == category {
			count++
		}
	}
	return count
}

// GetIssues returns all detected issues.
func (d *Diagnoser) GetIssues() []Issue {
	return d.issues
}

// GetIssuesByFile returns issues grouped by file.
func (d *Diagnoser) GetIssuesByFile() map[string][]Issue {
	result := make(map[string][]Issue)
	for _, issue := range d.issues {
		if issue.File != "" {
			result[issue.File] = append(result[issue.File], issue)
		}
	}
	return result
}

// GetFixableIssues returns issues that can be fixed automatically.
func (d *Diagnoser) GetFixableIssues() []Issue {
	var fixable []Issue
	for _, issue := range d.issues {
		if issue.File != "" && issue.Level != LevelInfo {
			fixable = append(fixable, issue)
		}
	}
	return fixable
}

// Helper functions

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

func sanitizeID(s string) string {
	// Replace non-alphanumeric characters with dash
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return strings.Trim(re.ReplaceAllString(s, "-"), "-")
}
