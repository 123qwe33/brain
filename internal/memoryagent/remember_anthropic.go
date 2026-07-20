package memoryagent

import (
	"context"
	"encoding/json"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/123qwe33/brain/internal/db"
	"github.com/123qwe33/brain/internal/rms"
)

// AnthropicRememberAgent extracts structured memory from a statement using Claude.
type AnthropicRememberAgent struct {
	cfg Config
}

func NewAnthropicRememberAgent(cfg Config) *AnthropicRememberAgent {
	return &AnthropicRememberAgent{cfg: cfg}
}

func (a *AnthropicRememberAgent) Remember(statement string) (RememberResult, error) {
	database, err := db.InitDB(a.cfg.DB)
	if err != nil {
		return RememberResult{}, fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	data, err := rms.New(database).AllData()
	if err != nil {
		return RememberResult{}, fmt.Errorf("load graph: %w", err)
	}

	existingLabels := make(map[string]bool, len(data.Nodes))
	for _, n := range data.Nodes {
		existingLabels[n.Label] = true
	}

	systemPrompt := buildRememberSystemPrompt(data.Nodes)

	if a.cfg.DryRun {
		fmt.Printf("=== SYSTEM PROMPT ===\n%s\n=== USER MESSAGE ===\n%s\n=== (dry run — no API call made) ===\n", systemPrompt, statement)
		return RememberResult{}, nil
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
			anthropic.NewUserMessage(anthropic.NewTextBlock(statement)),
		},
	})
	if err != nil {
		return RememberResult{}, fmt.Errorf("claude api: %w", err)
	}

	var raw string
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			raw = t.Text
			break
		}
	}
	if raw == "" {
		return RememberResult{}, fmt.Errorf("no text content in response")
	}

	raw = stripFences(raw)
	var result RememberResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return RememberResult{}, fmt.Errorf("parse response JSON: %w\nraw: %s", err, raw)
	}

	if err := validateResult(result, existingLabels); err != nil {
		return RememberResult{}, err
	}

	r := rms.New(database)
	for _, n := range result.Nodes {
		if err := r.Assert(n.Label, n.Type, n.Content); err != nil {
			return RememberResult{}, fmt.Errorf("assert %s: %w", n.Label, err)
		}
	}
	for _, l := range result.Links {
		if err := r.Link(l.From, l.To, l.Relation); err != nil {
			return RememberResult{}, fmt.Errorf("link %s->%s: %w", l.From, l.To, err)
		}
	}

	return result, nil
}
