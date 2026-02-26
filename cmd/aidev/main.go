// Package main provides the AI Dev Agent CLI tool.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ai-dev-agent/service/executor"
	"ai-dev-agent/service/filesystem"
	"ai-dev-agent/service/llm"
	"ai-dev-agent/service/orchestrator"
	"ai-dev-agent/service/prompt"
)

var Version = "1.0.0"

type Config struct {
	APIKey     string
	Model      string
	MaxRetries int
	Timeout    time.Duration
	Verbose    bool
	DryRun     bool
	NoBackup   bool
	WorkDir    string
}

type Command struct {
	Type        string
	Files       []string
	Instruction string
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	if args[0] == "--help" || args[0] == "-h" {
		printUsage()
		os.Exit(0)
	}
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("aidev v%s\n", Version)
		os.Exit(0)
	}

	config, cmd, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted...")
		cancel()
	}()

	if err := run(ctx, config, cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (*Config, *Command, error) {
	config := &Config{Model: "glm-4-flash", MaxRetries: 3, Timeout: 120 * time.Second}
	cmd := &Command{}
	i := 0

	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			break
		}
		switch arg {
		case "-k", "--api-key":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("missing value for %s", arg)
			}
			config.APIKey = args[i+1]
			i += 2
		case "-m", "--model":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("missing value for %s", arg)
			}
			config.Model = args[i+1]
			i += 2
		case "--retries":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("missing value for %s", arg)
			}
			fmt.Sscanf(args[i+1], "%d", &config.MaxRetries)
			i += 2
		case "--timeout":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("missing value for %s", arg)
			}
			config.Timeout, _ = time.ParseDuration(args[i+1])
			i += 2
		case "-V", "--verbose":
			config.Verbose = true
			i++
		case "--dry-run":
			config.DryRun = true
			i++
		case "--no-backup":
			config.NoBackup = true
			i++
		case "-w", "--workdir":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("missing value for %s", arg)
			}
			config.WorkDir = args[i+1]
			i += 2
		default:
			return nil, nil, fmt.Errorf("unknown flag: %s", arg)
		}
	}

	if i >= len(args) {
		return nil, nil, fmt.Errorf("no command specified")
	}

	cmd.Type = args[i]
	i++

	switch cmd.Type {
	case "refactor", "fix", "generate", "explain", "review", "test":
	default:
		return nil, nil, fmt.Errorf("unknown command: %s", cmd.Type)
	}

	for i < len(args) {
		if args[i] == "--" || args[i] == "-i" {
			i++
			if i < len(args) {
				cmd.Instruction = strings.Join(args[i:], " ")
			}
			break
		}
		cmd.Files = append(cmd.Files, args[i])
		i++
	}

	if len(cmd.Files) == 0 && cmd.Type != "generate" {
		return nil, nil, fmt.Errorf("no target files specified")
	}

	if config.APIKey == "" {
		config.APIKey = os.Getenv("GLM_API_KEY")
		if config.APIKey == "" {
			config.APIKey = os.Getenv("ZHIPUAI_API_KEY")
		}
		if config.APIKey == "" {
			return nil, nil, fmt.Errorf("API key required (GLM_API_KEY or -k flag)")
		}
	}

	if config.WorkDir == "" {
		config.WorkDir, _ = os.Getwd()
	}

	return config, cmd, nil
}

func run(ctx context.Context, config *Config, cmd *Command) error {
	services, err := initServices(config)
	if err != nil {
		return fmt.Errorf("init services: %w", err)
	}

	engine := orchestrator.NewEngine(
		services.file,
		services.prompt,
		services.llm,
		services.exec,
		orchestrator.Config{MaxRetries: config.MaxRetries, BuildVerify: !config.DryRun, Logger: newLogger(config.Verbose)},
	)

	var result *orchestrator.Result
	switch cmd.Type {
	case "refactor":
		result = engine.Refactor(ctx, cmd.Files, cmd.Instruction, config.WorkDir)
	case "fix":
		result = engine.Fix(ctx, cmd.Files, cmd.Instruction, config.WorkDir)
	case "generate":
		result = engine.Generate(ctx, cmd.Files, cmd.Instruction, config.WorkDir)
	case "explain", "review", "test":
		result = engine.Refactor(ctx, cmd.Files, cmd.Instruction, config.WorkDir)
	default:
		return fmt.Errorf("unsupported command: %s", cmd.Type)
	}

	printResult(result, config.Verbose)
	if !result.Success {
		return result.Error
	}
	return nil
}

type services struct {
	file   *fileAdapter
	prompt *promptAdapter
	llm    *llmAdapter
	exec   *execAdapter
}

func initServices(config *Config) (*services, error) {
	fileMgr, err := filesystem.NewManager(filesystem.Config{RootDir: config.WorkDir, BackupEnabled: !config.NoBackup})
	if err != nil {
		return nil, fmt.Errorf("filesystem: %w", err)
	}

	llmClient, err := llm.NewClient(llm.Config{APIKey: config.APIKey, Model: config.Model, Timeout: config.Timeout, MaxRetries: config.MaxRetries})
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}

	execMgr := executor.NewExecutor(executor.DefaultOptions())

	return &services{
		file:   &fileAdapter{mgr: fileMgr},
		prompt: &promptAdapter{builder: prompt.NewBuilder(prompt.DefaultConfig())},
		llm:    &llmAdapter{client: llmClient},
		exec:   &execAdapter{exec: execMgr},
	}, nil
}

type fileAdapter struct{ mgr *filesystem.Manager }

func (a *fileAdapter) ReadFile(path string) (string, error) {
	content, err := a.mgr.ReadFile(path)
	if err != nil {
		return "", err
	}
	return content.Content, nil
}
func (a *fileAdapter) WriteFile(path, content string) error {
	_, err := a.mgr.WriteFile(path, content, true)
	return err
}
func (a *fileAdapter) FileExists(path string) bool { return a.mgr.FileExists(path) }

type promptAdapter struct {
	builder *prompt.Builder
	mode    string
	inst    string
	files   map[string]string
}

func (a *promptAdapter) SetMode(mode string) orchestrator.PromptService {
	a.mode = mode
	return a
}
func (a *promptAdapter) SetInstruction(instruction string) orchestrator.PromptService {
	a.inst = instruction
	return a
}
func (a *promptAdapter) AddFile(path, content string, isMain bool) orchestrator.PromptService {
	if a.files == nil {
		a.files = make(map[string]string)
	}
	a.files[path] = content
	return a
}
func (a *promptAdapter) Build() (string, error) {
	b := prompt.NewBuilder(prompt.DefaultConfig())
	b.SetMode(prompt.InstructionMode(a.mode))
	b.SetInstruction(a.inst)
	for p, c := range a.files {
		b.AddFile(p, c, true)
	}
	result, err := b.Build()
	if err != nil {
		return "", err
	}
	if len(result.Messages) == 0 {
		return "", fmt.Errorf("no messages in prompt")
	}
	return result.Messages[len(result.Messages)-1].Content, nil
}

type llmAdapter struct{ client *llm.Client }

func (a *llmAdapter) Chat(ctx context.Context, prompt string) (string, error) {
	return a.client.SimpleChat(ctx, prompt)
}

type execAdapter struct{ exec *executor.Executor }

func (a *execAdapter) ExecuteInDir(ctx context.Context, command, dir string) (int, string, string, error) {
	result, err := a.exec.RunInDir(command, dir)
	if result == nil {
		return -1, "", "", err
	}
	return result.ExitCode, result.Stdout, result.Stderr, err
}

type logger struct{ verbose bool }

func newLogger(verbose bool) *logger { return &logger{verbose: verbose} }
func (l *logger) Info(format string, args ...interface{}) {
	fmt.Printf("  %s\n", fmt.Sprintf(format, args...))
}
func (l *logger) Error(format string, args ...interface{}) {
	fmt.Printf("  ❌ %s\n", fmt.Sprintf(format, args...))
}
func (l *logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		fmt.Printf("  🐛 %s\n", fmt.Sprintf(format, args...))
	}
}

func printResult(result *orchestrator.Result, verbose bool) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if result.Success {
		fmt.Println("  ✅ Operation completed successfully!")
	} else {
		fmt.Println("  ❌ Operation failed!")
	}
	if len(result.FilesWritten) > 0 {
		fmt.Println("\n  Files changed:")
		for _, f := range result.FilesWritten {
			fmt.Printf("    📝 %s\n", f)
		}
	}
	fmt.Printf("\n  Attempts: %d\n", result.Attempts)
	fmt.Printf("  Duration: %v\n", result.Duration)
	if verbose && result.Explanation != "" {
		fmt.Printf("\n  Explanation:\n    %s\n", truncate(result.Explanation, 200))
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func printUsage() {
	fmt.Println(`AI Dev Agent - AI-powered code assistant

Usage:
  aidev <command> <files...> [flags] [-- instruction]

Commands:
  refactor    Refactor code
  fix         Fix bugs
  generate    Generate code
  explain     Explain code
  review      Review code
  test        Generate tests

Examples:
  aidev refactor server/handler.go
  aidev fix server/auth.go -- "Fix nil pointer"
  aidev generate api/user.go -- "Generate CRUD handlers"

Flags:
  -k, --api-key <key>     GLM API key
  -m, --model <name>      Model name (default: glm-4-flash)
      --retries <n>       Max retries (default: 3)
      --timeout <dur>     Timeout (default: 2m)
  -V, --verbose           Verbose output
      --dry-run           Don't write files
      --no-backup         Don't create backups
  -w, --workdir <dir>     Working directory

Environment:
  GLM_API_KEY             API key (required)`)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func findGoModRoot(start string) string {
	dir := filepath.Dir(start)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
