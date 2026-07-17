package memoryagent

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/123qwe33/brain/internal/db"
	"github.com/123qwe33/brain/internal/rms"
)

// OpenAIAgent answers questions using any OpenAI-compatible API (e.g. Venice.ai).
type OpenAIAgent struct {
	cfg Config
}

func NewOpenAIAgent(cfg Config) *OpenAIAgent {
	return &OpenAIAgent{cfg: cfg}
}

func (a *OpenAIAgent) Ask(question string) (string, error) {
	database, err := db.InitDB(a.cfg.DB)
	if err != nil {
		return "", fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	data, err := rms.New(database).AllData()
	if err != nil {
		return "", fmt.Errorf("load graph: %w", err)
	}

	systemPrompt := buildSystemPrompt(data)

	if a.cfg.DryRun {
		return fmt.Sprintf("=== SYSTEM PROMPT ===\n%s\n=== USER MESSAGE ===\n%s\n=== (dry run — no API call made) ===", systemPrompt, question), nil
	}

	opts := []option.RequestOption{}
	if a.cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(a.cfg.APIKey))
	}
	if a.cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(a.cfg.BaseURL))
	}
	client := openai.NewClient(opts...)

	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: a.cfg.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(question),
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai api: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
