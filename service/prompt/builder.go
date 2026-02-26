// Package prompt provides prompt building utilities.
package prompt

import (
        "encoding/json"
        "fmt"
        "path/filepath"
        "regexp"
        "sort"
        "strings"
)

// InstructionMode defines the type of instruction.
type InstructionMode string

const (
        ModeRefactor  InstructionMode = "refactor"
        ModeFix       InstructionMode = "fix"
        ModeGenerate  InstructionMode = "generate"
        ModeExplain   InstructionMode = "explain"
        ModeReview    InstructionMode = "review"
        ModeTest      InstructionMode = "test"
)

// Role defines the message role.
type Role string

const (
        RoleSystem    Role = "system"
        RoleUser      Role = "user"
        RoleAssistant Role = "assistant"
)

// Message represents a chat message.
type Message struct {
        Role    Role   `json:"role"`
        Content string `json:"content"`
}

// PromptResult represents the prompt result.
type PromptResult struct {
        Version  string    `json:"version"`
        Mode     string    `json:"mode"`
        Messages []Message `json:"messages"`
}

// Config holds builder configuration.
type Config struct {
        MaxTotalTokens  int
        MaxOutputTokens int
}

// DefaultConfig returns default config.
func DefaultConfig() Config {
        return Config{
                MaxTotalTokens:  128000,
                MaxOutputTokens: 4096,
        }
}

// ModeTemplates contains system prompts.
var ModeTemplates = map[string]string{
        "refactor": `You are an expert software architect. Refactor the provided code according to the instructions.
Return the complete refactored code in a markdown code block.

Rules:
- Preserve exact functionality
- Follow best practices
- Improve readability
- Add comments where helpful`,

        "fix": `You are an expert software engineer. Fix the bugs in the provided code.
Return the fixed code in a markdown code block.

Rules:
- Identify root causes
- Make minimal targeted fixes
- Preserve existing functionality
- Add proper error handling`,

        "generate": `You are an expert software developer. Generate code according to the specifications.
Return the generated code in a markdown code block.

Rules:
- Follow exact requirements
- Use appropriate patterns
- Write clean, maintainable code
- Include error handling`,

        "explain": `You are an expert software educator. Explain the provided code in detail.
Return your explanation in clear, structured text.`,

        "review": `You are an expert code reviewer. Review the provided code.
Identify issues, suggest improvements, and rate the code quality.`,

        "test": `You are an expert test engineer. Generate comprehensive tests for the provided code.
Return the test code in a markdown code block.`,
}

// Builder builds prompts.
type Builder struct {
        config      Config
        mode        string
        instruction string
        files       map[string]string
        constraints []string
}

// NewBuilder creates a new builder.
func NewBuilder(config Config) *Builder {
        return &Builder{
                config: config,
                files:  make(map[string]string),
        }
}

// SetMode sets the mode.
func (b *Builder) SetMode(mode string) *Builder {
        b.mode = mode
        return b
}

// SetInstruction sets the instruction.
func (b *Builder) SetInstruction(instruction string) *Builder {
        b.instruction = instruction
        return b
}

// AddFile adds a file.
func (b *Builder) AddFile(path, content string, isMain bool) *Builder {
        b.files[path] = content
        return b
}

// AddConstraint adds a constraint.
func (b *Builder) AddConstraint(constraint string) *Builder {
        b.constraints = append(b.constraints, constraint)
        return b
}

// Build builds the prompt.
func (b *Builder) Build() (*PromptResult, error) {
        var messages []Message

        // System message
        systemPrompt := b.getSystemPrompt()
        messages = append(messages, Message{
                Role:    RoleSystem,
                Content: systemPrompt,
        })

        // User message
        userPrompt := b.buildUserPrompt()
        messages = append(messages, Message{
                Role:    RoleUser,
                Content: userPrompt,
        })

        return &PromptResult{
                Version:  "1.0",
                Mode:     b.mode,
                Messages: messages,
        }, nil
}

func (b *Builder) getSystemPrompt() string {
        if prompt, ok := ModeTemplates[b.mode]; ok {
                return prompt
        }
        return ModeTemplates["generate"]
}

func (b *Builder) buildUserPrompt() string {
        var sb strings.Builder

        // Instruction
        if b.instruction != "" {
                sb.WriteString(fmt.Sprintf("## Task: %s\n\n", strings.Title(b.mode)))
                sb.WriteString(fmt.Sprintf("### Instruction:\n%s\n\n", b.instruction))
        }

        // Constraints
        if len(b.constraints) > 0 {
                sb.WriteString("### Constraints:\n")
                for _, c := range b.constraints {
                        sb.WriteString(fmt.Sprintf("- %s\n", c))
                }
                sb.WriteString("\n")
        }

        // Files
        if len(b.files) > 0 {
                sb.WriteString("### Files:\n")

                // Sort files for consistent ordering
                paths := make([]string, 0, len(b.files))
                for p := range b.files {
                        paths = append(paths, p)
                }
                sort.Strings(paths)

                for _, path := range paths {
                        content := b.files[path]
                        lang := detectLanguage(path)
                        sb.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n```%s\n%s\n```\n", path, lang, content))
                }
        }

        sb.WriteString("\nProvide your response with code in markdown code blocks (```language\\ncode\\n```).")

        return sb.String()
}

// ToJSON returns JSON representation.
func (r *PromptResult) ToJSON() (string, error) {
        data, err := json.MarshalIndent(r, "", "  ")
        return string(data), err
}

// Helper functions
func detectLanguage(path string) string {
        ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))

        langMap := map[string]string{
                "go": "go", "py": "python", "js": "javascript", "ts": "typescript",
                "tsx": "typescript", "jsx": "javascript", "java": "java", "kt": "kotlin",
                "rs": "rust", "c": "c", "cpp": "cpp", "cs": "csharp",
                "rb": "ruby", "php": "php", "swift": "swift", "scala": "scala",
        }

        if lang, ok := langMap[ext]; ok {
                return lang
        }
        return ""
}

// ExtractCodeBlocks extracts code blocks from response.
func ExtractCodeBlocks(response string) []CodeBlock {
        blocks := []CodeBlock{}
        re := regexp.MustCompile("```(\\w*)\n?([\\s\\S]*?)```")
        matches := re.FindAllStringSubmatch(response, -1)

        for _, match := range matches {
                blocks = append(blocks, CodeBlock{
                        Language: match[1],
                        Code:     strings.TrimSpace(match[2]),
                })
        }
        return blocks
}

// CodeBlock represents a code block.
type CodeBlock struct {
        Language string `json:"language"`
        Code     string `json:"code"`
}

// ExtractExplanation extracts explanation text.
func ExtractExplanation(response string) string {
        re := regexp.MustCompile("```[\\s\\S]*?```")
        return strings.TrimSpace(re.ReplaceAllString(response, ""))
}
