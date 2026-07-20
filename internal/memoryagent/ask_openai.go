package memoryagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/123qwe33/brain/internal/db"
	"github.com/123qwe33/brain/internal/rms"
)

// OpenAIAskAgent answers questions using brain memory via an OpenAI-compatible API.
type OpenAIAskAgent struct {
	cfg Config
}

func NewOpenAIAskAgent(cfg Config) *OpenAIAskAgent {
	return &OpenAIAskAgent{cfg: cfg}
}

func (a *OpenAIAskAgent) Ask(question string) (string, error) {
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

func buildAskSystemPrompt(nodes []rms.FullNode) string {
	var b strings.Builder
	b.WriteString("You are a codebase memory assistant. Answer the user's question using only the memory nodes below.\n")
	b.WriteString("Each node was extracted from a previous coding session on this codebase.\n\n")
	b.WriteString("MEMORY NODES:\n")
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("- %s (%s): %s\n", n.Label, n.Type, n.Content))
	}
	b.WriteString("\nIf the nodes do not contain enough information to answer, say so briefly. Be concise.")
	return b.String()
}
