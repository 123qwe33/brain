package memoryagent

import (
	"context"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/123qwe33/brain/internal/db"
	"github.com/123qwe33/brain/internal/rms"
)

// AnthropicAgent answers questions by loading the full brain graph and sending
// a single Claude API request with the graph embedded in the system prompt.
type AnthropicAgent struct {
	cfg Config
}

func NewAnthropicAgent(cfg Config) *AnthropicAgent {
	return &AnthropicAgent{cfg: cfg}
}

func (a *AnthropicAgent) Ask(question string) (string, error) {
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

func buildSystemPrompt(data *rms.AllDataResult) string {
	var b strings.Builder
	b.WriteString("You are a memory assistant. Answer the user's question using only the following knowledge graph data. Be concise and plaintext only.\n\n")
	b.WriteString("NODES:\n")
	for _, n := range data.Nodes {
		b.WriteString(fmt.Sprintf("%s (%s): %s\n", n.Label, n.Type, n.Content))
	}
	b.WriteString("\nEDGES:\n")
	for _, e := range data.Edges {
		b.WriteString(fmt.Sprintf("%s -[%s]-> %s\n", e.From, e.Relation, e.To))
	}
	if len(data.Justifications) > 0 {
		b.WriteString("\nJUSTIFICATIONS:\n")
		for _, j := range data.Justifications {
			b.WriteString(fmt.Sprintf("%s is supported by: [%s] (group %d)\n", j.NodeLabel, strings.Join(j.Justifiers, ", "), j.GroupID))
		}
	}
	return b.String()
}
