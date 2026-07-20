package memoryagent

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/123qwe33/brain/internal/db"
	"github.com/123qwe33/brain/internal/rms"
)

// AnthropicAskAgent answers questions using brain memory via the Claude API.
type AnthropicAskAgent struct {
	cfg Config
}

func NewAnthropicAskAgent(cfg Config) *AnthropicAskAgent {
	return &AnthropicAskAgent{cfg: cfg}
}

func (a *AnthropicAskAgent) Ask(question string) (string, error) {
	database, err := db.InitDB(a.cfg.DB)
	if err != nil {
		return "", fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	data, err := rms.New(database).AllData()
	if err != nil {
		return "", fmt.Errorf("load graph: %w", err)
	}

	var inNodes []rms.FullNode
	for _, n := range data.Nodes {
		if n.Status == "IN" {
			inNodes = append(inNodes, n)
		}
	}

	if len(inNodes) == 0 {
		return "No prior knowledge stored for this codebase.", nil
	}

	systemPrompt := buildAskSystemPrompt(inNodes)

	if a.cfg.DryRun {
		fmt.Printf("=== SYSTEM PROMPT ===\n%s\n=== USER MESSAGE ===\n%s\n=== (dry run — no API call made) ===\n", systemPrompt, question)
		return "", nil
	}

	opts := []option.RequestOption{}
	if a.cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(a.cfg.APIKey))
	}
	if a.cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(a.cfg.BaseURL))
	}
	client := anthropic.NewClient(opts...)

	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(a.cfg.Model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude api: %w", err)
	}

	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			return t.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}
