package ai

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/darkspock/gosnag/internal/config"
)

// Provider abstracts AI chat completions across different providers.
type Provider interface {
	// Chat sends a prompt and returns a response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Name returns the provider identifier (e.g. "openai", "groq", "bedrock").
	Name() string
}

// ChatRequest is the input for an AI chat call.
type ChatRequest struct {
	SystemPrompt string
	Messages     []Message
	MaxTokens    int
	Temperature  float64
	JSON         bool // request structured JSON output
}

// Message is a single message in the chat.
type Message struct {
	Role    string // "user", "assistant"
	Content string
}

// ChatResponse is the output from an AI chat call.
type ChatResponse struct {
	Content     string
	InputTokens int
	OutputTokens int
}

// PromptHash returns a SHA-256 hash of the request for caching.
func (r ChatRequest) PromptHash() string {
	h := sha256.New()
	h.Write([]byte(r.SystemPrompt))
	for _, m := range r.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// NewProvider creates a provider from global config.
func NewProvider(cfg *config.Config) Provider {
	switch cfg.AIProvider {
	case "openai":
		return newOpenAICompatible(openAIConfig{
			apiKey:  cfg.AIAPIKey,
			model:   defaultString(cfg.AIModel, "gpt-4o-mini"),
			baseURL: defaultString(cfg.AIBaseURL, "https://api.openai.com/v1"),
			name:    "openai",
		})
	case "groq":
		return newOpenAICompatible(openAIConfig{
			apiKey:  cfg.AIAPIKey,
			model:   defaultString(cfg.AIModel, "llama-3.3-70b-versatile"),
			baseURL: defaultString(cfg.AIBaseURL, "https://api.groq.com/openai/v1"),
			name:    "groq",
		})
	case "bedrock":
		return newBedrockProvider(cfg.AIBedrockRegion, cfg.AIBedrockModelID)
	default:
		return nil
	}
}

// IsConfigured returns true if an AI provider is set.
func IsConfigured(cfg *config.Config) bool {
	return cfg.AIProvider != ""
}

func defaultString(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
