// Package llm provides LLM client implementations for various AI providers.
package llm

import (
        "bytes"
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "io"
        "net/http"
        "strings"
        "time"
)

// Errors
var (
        ErrEmptyAPIKey     = errors.New("api key cannot be empty")
        ErrEmptyMessages   = errors.New("messages cannot be empty")
        ErrRequestFailed   = errors.New("request failed")
        ErrResponseParse   = errors.New("failed to parse response")
        ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// APIError represents an API error.
type APIError struct {
        Code       interface{} `json:"code"`    // Can be string or int
        Message    string      `json:"message"`
        HTTPStatus int         `json:"-"`
}

func (e *APIError) Error() string {
        return fmt.Sprintf("API error: code=%v, message=%s", e.Code, e.Message)
}

// Message represents a chat message.
type Message struct {
        Role    string `json:"role"`
        Content string `json:"content"`
}

// ChatCompletionRequest represents a chat request.
type ChatCompletionRequest struct {
        Model    string    `json:"model"`
        Messages []Message `json:"messages"`
}

// ChatCompletionResponse represents a chat response.
type ChatCompletionResponse struct {
        ID      string `json:"id"`
        Model   string `json:"model"`
        Choices []struct {
                Message struct {
                        Role    string `json:"role"`
                        Content string `json:"content"`
                } `json:"message"`
                FinishReason string `json:"finish_reason"`
        } `json:"choices"`
        Usage struct {
                TotalTokens int `json:"total_tokens"`
        } `json:"usage"`
        Error *APIError `json:"error,omitempty"`
}

// Config holds client configuration.
type Config struct {
        APIKey     string
        BaseURL    string
        Model      string
        Timeout    time.Duration
        MaxRetries int
}

// Client is the LLM client.
type Client struct {
        config     Config
        httpClient *http.Client
}

// NewClient creates a new LLM client.
func NewClient(config Config) (*Client, error) {
        if config.APIKey == "" {
                return nil, ErrEmptyAPIKey
        }
        if config.BaseURL == "" {
                config.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
        }
        if config.Model == "" {
                config.Model = "glm-4-flash"
        }
        if config.Timeout == 0 {
                config.Timeout = 60 * time.Second
        }
        if config.MaxRetries == 0 {
                config.MaxRetries = 3
        }

        return &Client{
                config:     config,
                httpClient: &http.Client{Timeout: config.Timeout},
        }, nil
}

// ChatCompletion sends a chat request.
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
        req.Model = c.config.Model

        body, _ := json.Marshal(req)
        httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
        httpReq.Header.Set("Content-Type", "application/json")
        httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

        httpResp, err := c.httpClient.Do(httpReq)
        if err != nil {
                return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
        }
        defer httpResp.Body.Close()

        respBody, _ := io.ReadAll(httpResp.Body)

        var response ChatCompletionResponse
        if err := json.Unmarshal(respBody, &response); err != nil {
                return nil, fmt.Errorf("%w: %v", ErrResponseParse, err)
        }

        if response.Error != nil {
                response.Error.HTTPStatus = httpResp.StatusCode
                return nil, response.Error
        }

        return &response, nil
}

// SimpleChat sends a simple chat request.
func (c *Client) SimpleChat(ctx context.Context, prompt string) (string, error) {
        resp, err := c.ChatCompletion(ctx, ChatCompletionRequest{
                Messages: []Message{{Role: "user", Content: prompt}},
        })
        if err != nil {
                return "", err
        }
        if len(resp.Choices) == 0 {
                return "", fmt.Errorf("no choices in response")
        }
        return resp.Choices[0].Message.Content, nil
}

// SimpleChatWithSystem sends a chat with system prompt.
func (c *Client) SimpleChatWithSystem(ctx context.Context, system, user string) (string, error) {
        resp, err := c.ChatCompletion(ctx, ChatCompletionRequest{
                Messages: []Message{
                        {Role: "system", Content: system},
                        {Role: "user", Content: user},
                },
        })
        if err != nil {
                return "", err
        }
        if len(resp.Choices) == 0 {
                return "", fmt.Errorf("no choices in response")
        }
        return resp.Choices[0].Message.Content, nil
}

// StreamChunk represents a streaming chunk.
type StreamChunk struct {
        Choices []struct {
                Delta struct {
                        Content string `json:"content"`
                } `json:"delta"`
                FinishReason string `json:"finish_reason"`
        } `json:"choices"`
}

// StreamCallback is callback for streaming.
type StreamCallback func(chunk string) error

// ChatCompletionStream sends a streaming request.
func (c *Client) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, callback StreamCallback) error {
        req.Model = c.config.Model

        body, _ := json.Marshal(req)
        httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
        httpReq.Header.Set("Content-Type", "application/json")
        httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
        httpReq.Header.Set("Accept", "text/event-stream")

        httpResp, err := c.httpClient.Do(httpReq)
        if err != nil {
                return fmt.Errorf("%w: %v", ErrRequestFailed, err)
        }
        defer httpResp.Body.Close()

        buf := make([]byte, 4096)
        for {
                n, err := httpResp.Body.Read(buf)
                if err != nil && err != io.EOF {
                        break
                }
                if n > 0 {
                        data := string(buf[:n])
                        lines := strings.Split(data, "\n")
                        for _, line := range lines {
                                line = strings.TrimSpace(line)
                                if strings.HasPrefix(line, "data: ") {
                                        data := strings.TrimPrefix(line, "data: ")
                                        if data == "[DONE]" {
                                                return nil
                                        }
                                        var chunk StreamChunk
                                        if json.Unmarshal([]byte(data), &chunk) == nil {
                                                if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
                                                        if err := callback(chunk.Choices[0].Delta.Content); err != nil {
                                                                return err
                                                        }
                                                }
                                        }
                                }
                        }
                }
                if err == io.EOF {
                        break
                }
        }
        return nil
}
