package memoryagent

import (
	"fmt"
	"strings"

	"github.com/123qwe33/brain/internal/rms"
)

// RememberAgent extracts structured nodes and links from a plaintext statement
// and writes them to the knowledge graph.
type RememberAgent interface {
	Remember(statement string) (RememberResult, error)
}

type NodeToAssert struct {
	Label   string `json:"label"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type LinkToCreate struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

type RememberResult struct {
	Nodes []NodeToAssert `json:"nodes"`
	Links []LinkToCreate `json:"links"`
}

func buildRememberSystemPrompt(existingNodes []rms.FullNode) string {
	var b strings.Builder
	b.WriteString("You are a memory storage assistant. Given a statement, extract structured memory nodes and links to store in a knowledge graph.\n\n")

	b.WriteString("EXISTING NODES (do not re-create these, only reference their labels in links):\n")
	for _, n := range existingNodes {
		b.WriteString(fmt.Sprintf("%s (%s)\n", n.Label, n.Type))
	}

	b.WriteString(`
RULES:
- Labels must be lowercase kebab-case (e.g. decision-use-postgres)
- Types should be one of: goal, decision, fact, constraint, task, person, risk, assumption, concept — or a sensible new type
- Content should be a single clear sentence
- Only create links to labels that exist in the node list above OR in your new nodes list
- Output valid JSON only — no explanation, no markdown, no code fences

OUTPUT SCHEMA:
{
  "nodes": [{"label": "...", "type": "...", "content": "..."}],
  "links": [{"from": "...", "to": "...", "relation": "..."}]
}

If nothing meaningful should be stored, return: {"nodes": [], "links": []}`)
	return b.String()
}

func validateResult(result RememberResult, existingLabels map[string]bool) error {
	// Build a set of all labels: existing + new nodes being asserted
	allLabels := make(map[string]bool, len(existingLabels)+len(result.Nodes))
	for k := range existingLabels {
		allLabels[k] = true
	}
	for _, n := range result.Nodes {
		allLabels[n.Label] = true
	}

	var hallucinated []string
	for _, l := range result.Links {
		if !allLabels[l.From] {
			hallucinated = append(hallucinated, l.From)
		}
		if !allLabels[l.To] {
			hallucinated = append(hallucinated, l.To)
		}
	}
	if len(hallucinated) > 0 {
		return fmt.Errorf("links reference unknown labels: %s", strings.Join(hallucinated, ", "))
	}
	return nil
}
