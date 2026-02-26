// Package executor provides command execution utilities.
package executor

import (
        "bytes"
        "context"
        "fmt"
        "os"
        "os/exec"
        "strings"
        "time"
)

// Errors
var (
        ErrCommandEmpty    = fmt.Errorf("command cannot be empty")
        ErrCommandNotFound = fmt.Errorf("command not found")
        ErrTimeout         = fmt.Errorf("command timed out")
        ErrCancelled       = fmt.Errorf("command cancelled")
)

// Result represents execution result.
type Result struct {
        Command   string
        ExitCode  int
        Stdout    string
        Stderr    string
        Combined  string
        Duration  time.Duration
        TimedOut  bool
        Cancelled bool
        Success   bool
        PID       int
}

// Options holds execution options.
type Options struct {
        WorkingDir string
        Env        map[string]string
        Timeout    time.Duration
        Shell      bool
        Input      string
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
        return Options{
                Shell:   true,
                Timeout: 60 * time.Second,
        }
}

// Executor handles command execution.
type Executor struct {
        defaultOptions Options
}

// NewExecutor creates a new executor.
func NewExecutor(opts ...Options) *Executor {
        options := DefaultOptions()
        if len(opts) > 0 {
                options = opts[0]
        }
        return &Executor{defaultOptions: options}
}

// Execute executes a command.
func (e *Executor) Execute(ctx context.Context, command string) (*Result, error) {
        return e.ExecuteWithOptions(ctx, command, e.defaultOptions)
}

// ExecuteWithOptions executes with custom options.
func (e *Executor) ExecuteWithOptions(ctx context.Context, command string, opts Options) (*Result, error) {
        if command == "" {
                return nil, ErrCommandEmpty
        }

        result := &Result{
                Command:  command,
                ExitCode: -1,
        }

        var cmd *exec.Cmd
        if opts.Shell {
                cmd = exec.CommandContext(ctx, "sh", "-c", command)
        } else {
                parts := strings.Fields(command)
                if len(parts) == 0 {
                        return nil, ErrCommandEmpty
                }
                cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
        }

        if opts.WorkingDir != "" {
                cmd.Dir = opts.WorkingDir
        }

        cmd.Env = os.Environ()
        for k, v := range opts.Env {
                cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
        }

        var stdoutBuf, stderrBuf, combinedBuf bytes.Buffer
        cmd.Stdout = ioMultiWriter(&stdoutBuf, &combinedBuf)
        cmd.Stderr = ioMultiWriter(&stderrBuf, &combinedBuf)

        if opts.Input != "" {
                cmd.Stdin = strings.NewReader(opts.Input)
        }

        start := time.Now()
        err := cmd.Run()
        result.Duration = time.Since(start)

        result.Stdout = stdoutBuf.String()
        result.Stderr = stderrBuf.String()
        result.Combined = combinedBuf.String()

        if cmd.Process != nil {
                result.PID = cmd.Process.Pid
        }

        if err != nil {
                if ctx.Err() == context.DeadlineExceeded {
                        result.TimedOut = true
                        return result, ErrTimeout
                }
                if ctx.Err() == context.Canceled {
                        result.Cancelled = true
                        return result, ErrCancelled
                }
                if exitErr, ok := err.(*exec.ExitError); ok {
                        result.ExitCode = exitErr.ExitCode()
                        return result, nil
                }
                return result, err
        }

        result.ExitCode = 0
        result.Success = true
        return result, nil
}

// Run executes and returns stdout.
func (e *Executor) Run(command string) (string, error) {
        ctx := context.Background()
        if e.defaultOptions.Timeout > 0 {
                var cancel context.CancelFunc
                ctx, cancel = context.WithTimeout(ctx, e.defaultOptions.Timeout)
                defer cancel()
        }
        result, err := e.Execute(ctx, command)
        if err != nil {
                return "", err
        }
        return result.Stdout, nil
}

// RunInDir executes in a directory.
func (e *Executor) RunInDir(command, dir string) (*Result, error) {
        opts := e.defaultOptions
        opts.WorkingDir = dir
        ctx := context.Background()
        return e.ExecuteWithOptions(ctx, command, opts)
}

// RunWithTimeout executes with timeout.
func (e *Executor) RunWithTimeout(command string, timeout time.Duration) (*Result, error) {
        ctx, cancel := context.WithTimeout(context.Background(), timeout)
        defer cancel()
        return e.Execute(ctx, command)
}

// RunStream executes with streaming output.
func (e *Executor) RunStream(ctx context.Context, command string, handler func(line string)) (*Result, error) {
        opts := e.defaultOptions

        cmd := exec.CommandContext(ctx, "sh", "-c", command)
        if opts.WorkingDir != "" {
                cmd.Dir = opts.WorkingDir
        }

        stdoutPipe, _ := cmd.StdoutPipe()
        stderrPipe, _ := cmd.StderrPipe()

        result := &Result{Command: command, ExitCode: -1}

        if err := cmd.Start(); err != nil {
                return nil, err
        }

        if cmd.Process != nil {
                result.PID = cmd.Process.Pid
        }

        // Read output in goroutines
        go func() {
                buf := make([]byte, 1024)
                for {
                        n, err := stdoutPipe.Read(buf)
                        if n > 0 {
                                output := string(buf[:n])
                                result.Stdout += output
                                if handler != nil {
                                        for _, line := range strings.Split(output, "\n") {
                                                if line != "" {
                                                        handler(line)
                                                }
                                        }
                                }
                        }
                        if err != nil {
                                break
                        }
                }
        }()

        go func() {
                buf := make([]byte, 1024)
                for {
                        n, err := stderrPipe.Read(buf)
                        if n > 0 {
                                result.Stderr += string(buf[:n])
                        }
                        if err != nil {
                                break
                        }
                }
        }()

        err := cmd.Wait()
        start := time.Now()

        if cmd.ProcessState != nil {
                result.ExitCode = cmd.ProcessState.ExitCode()
                result.Success = result.ExitCode == 0
        }
        result.Duration = time.Since(start)

        if err != nil {
                if ctx.Err() == context.DeadlineExceeded {
                        result.TimedOut = true
                        return result, ErrTimeout
                }
                if ctx.Err() == context.Canceled {
                        result.Cancelled = true
                        return result, ErrCancelled
                }
        }

        return result, nil
}

// IsCommandAvailable checks if command exists.
func IsCommandAvailable(command string) bool {
        _, err := exec.LookPath(command)
        return err == nil
}

// GetCommandPath returns command path.
func GetCommandPath(command string) (string, error) {
        return exec.LookPath(command)
}

// Helper
func ioMultiWriter(writers ...*bytes.Buffer) *multiWriter {
        return &multiWriter{writers: writers}
}

type multiWriter struct {
        writers []*bytes.Buffer
}

func (m *multiWriter) Write(p []byte) (n int, err error) {
        for _, w := range m.writers {
                w.Write(p)
        }
        return len(p), nil
}
