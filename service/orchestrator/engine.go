// Package orchestrator coordinates the AI development workflow.
package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Interfaces
type FileService interface {
	ReadFile(path string) (string, error)
	WriteFile(path, content string) error
	FileExists(path string) bool
}

type PromptService interface {
	SetMode(mode string) PromptService
	SetInstruction(instruction string) PromptService
	AddFile(path, content string, isMain bool) PromptService
	Build() (string, error)
}

type LLMService interface {
	Chat(ctx context.Context, prompt string) (string, error)
}

type CommandService interface {
	ExecuteInDir(ctx context.Context, command, dir string) (int, string, string, error)
}

type Logger interface {
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
}

// Types
type Mode string

const (
	ModeRefactor Mode = "refactor"
	ModeFix      Mode = "fix"
	ModeGenerate Mode = "generate"
)

type Config struct {
	MaxRetries  int
	BuildVerify bool
	Logger      Logger
}

func DefaultConfig() Config {
	return Config{MaxRetries: 3, BuildVerify: true, Logger: &defaultLogger{}}
}

type Request struct {
	Mode        Mode
	Files       []string
	Instruction string
	WorkDir     string
}

type Result struct {
	Success      bool
	FilesWritten []string
	Output       string
	Explanation  string
	Attempts     int
	Duration     time.Duration
	Error        error
}

type CodeBlock struct {
	Language string
	Code     string
	Filename string
}

// Engine orchestrates the workflow.
type Engine struct {
	file   FileService
	prompt PromptService
	llm    LLMService
	exec   CommandService
	config Config
}

func NewEngine(file FileService, prompt PromptService, llm LLMService, exec CommandService, config Config) *Engine {
	return &Engine{file: file, prompt: prompt, llm: llm, exec: exec, config: config}
}

func (e *Engine) Execute(ctx context.Context, req *Request) *Result {
	start := time.Now()
	result := &Result{Attempts: 0}

	e.logInfo("Starting %s operation on %d file(s)", req.Mode, len(req.Files))

	for attempt := 1; attempt <= e.config.MaxRetries; attempt++ {
		result.Attempts = attempt
		e.logInfo("Attempt %d/%d", attempt, e.config.MaxRetries)

		// Read files
		fileContents, err := e.readFiles(req.Files)
		if err != nil {
			result.Error = fmt.Errorf("read files: %w", err)
			e.logError("Failed to read files: %v", err)
			continue
		}

		// Build prompt
		prompt, err := e.buildPrompt(req, fileContents)
		if err != nil {
			result.Error = fmt.Errorf("build prompt: %w", err)
			e.logError("Failed to build prompt: %v", err)
			continue
		}

		// Call LLM
		response, err := e.llm.Chat(ctx, prompt)
		if err != nil {
			result.Error = fmt.Errorf("LLM call: %w", err)
			e.logError("LLM call failed: %v", err)
			if !e.isRetryable(err) {
				break
			}
			continue
		}
		e.logInfo("LLM response received (%d chars)", len(response))

		// Parse code blocks
		codeBlocks := e.parseCodeBlocks(response)
		if len(codeBlocks) == 0 {
			result.Error = fmt.Errorf("no code blocks found in response")
			e.logError("No code blocks found")
			continue
		}
		e.logInfo("Parsed %d code block(s)", len(codeBlocks))

		// Write files
		written, err := e.writeFiles(req.Files, codeBlocks)
		if err != nil {
			result.Error = fmt.Errorf("write files: %w", err)
			e.logError("Failed to write files: %v", err)
			continue
		}
		result.FilesWritten = written

		// Verify build
		if e.config.BuildVerify && req.WorkDir != "" {
			if err := e.verifyBuild(ctx, req.WorkDir); err != nil {
				result.Error = fmt.Errorf("build failed: %w", err)
				e.logError("Build verification failed: %v", err)
				req.Instruction = e.appendBuildError(req.Instruction, err)
				continue
			}
			e.logInfo("Build verification passed")
		}

		result.Success = true
		result.Output = response
		result.Explanation = e.extractExplanation(response)
		result.Error = nil
		break
	}

	result.Duration = time.Since(start)
	return result
}

func (e *Engine) readFiles(files []string) (map[string]string, error) {
	contents := make(map[string]string)
	for _, path := range files {
		content, err := e.file.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		contents[path] = content
	}
	return contents, nil
}

func (e *Engine) buildPrompt(req *Request, files map[string]string) (string, error) {
	builder := e.prompt.SetMode(string(req.Mode)).SetInstruction(req.Instruction)
	for path, content := range files {
		builder = builder.AddFile(path, content, true)
	}
	return builder.Build()
}

func (e *Engine) parseCodeBlocks(response string) []CodeBlock {
	blocks := []CodeBlock{}
	re := regexp.MustCompile("```(\\w*)\n?([\\s\\S]*?)```")
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		code := strings.TrimSpace(match[2])
		if code != "" {
			blocks = append(blocks, CodeBlock{Language: match[1], Code: code})
		}
	}
	return blocks
}

func (e *Engine) writeFiles(files []string, blocks []CodeBlock) ([]string, error) {
	written := []string{}
	for i, block := range blocks {
		var targetPath string
		if i < len(files) {
			targetPath = files[i]
		} else if block.Filename != "" {
			targetPath = block.Filename
		} else {
			continue
		}
		if err := e.file.WriteFile(targetPath, block.Code); err != nil {
			return written, fmt.Errorf("%s: %w", targetPath, err)
		}
		written = append(written, targetPath)
		e.logInfo("Wrote: %s", targetPath)
	}
	return written, nil
}

func (e *Engine) verifyBuild(ctx context.Context, workDir string) error {
	exitCode, _, stderr, err := e.exec.ExecuteInDir(ctx, "go build ./...", workDir)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("%s", stderr)
	}
	return nil
}

func (e *Engine) appendBuildError(instruction string, buildErr error) string {
	return fmt.Sprintf("%s\n\nPrevious attempt failed:\n%s\nPlease fix the code.", instruction, buildErr.Error())
}

func (e *Engine) extractExplanation(response string) string {
	re := regexp.MustCompile("```[\\s\\S]*?```")
	return strings.TrimSpace(re.ReplaceAllString(response, ""))
}

func (e *Engine) isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "connection") ||
		strings.Contains(msg, "rate limit") || strings.Contains(msg, "503") || strings.Contains(msg, "502")
}

func (e *Engine) Refactor(ctx context.Context, files []string, instruction, workDir string) *Result {
	return e.Execute(ctx, &Request{Mode: ModeRefactor, Files: files, Instruction: instruction, WorkDir: workDir})
}

func (e *Engine) Fix(ctx context.Context, files []string, instruction, workDir string) *Result {
	return e.Execute(ctx, &Request{Mode: ModeFix, Files: files, Instruction: instruction, WorkDir: workDir})
}

func (e *Engine) Generate(ctx context.Context, files []string, instruction, workDir string) *Result {
	return e.Execute(ctx, &Request{Mode: ModeGenerate, Files: files, Instruction: instruction, WorkDir: workDir})
}

func (e *Engine) logInfo(format string, args ...interface{}) {
	if e.config.Logger != nil {
		e.config.Logger.Info(format, args...)
	}
}

func (e *Engine) logError(format string, args ...interface{}) {
	if e.config.Logger != nil {
		e.config.Logger.Error(format, args...)
	}
}

type defaultLogger struct{}

func (l *defaultLogger) Info(format string, args ...interface{})  { fmt.Printf("[INFO] %s\n", fmt.Sprintf(format, args...)) }
func (l *defaultLogger) Error(format string, args ...interface{}) { fmt.Printf("[ERROR] %s\n", fmt.Sprintf(format, args...)) }
func (l *defaultLogger) Debug(format string, args ...interface{}) { fmt.Printf("[DEBUG] %s\n", fmt.Sprintf(format, args...)) }
