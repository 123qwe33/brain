package memoryagent

import "fmt"

// Config holds resolved configuration for a MemoryAgent.
type Config struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
	DB       string
	DryRun   bool
}

// MemoryAgent answers questions by consulting brain memory.
type MemoryAgent interface {
	Ask(question string) (string, error)
}

// DummyAgent echoes config values without making real LLM calls.
type DummyAgent struct {
	cfg Config
}

func NewDummyAgent(cfg Config) *DummyAgent {
	return &DummyAgent{cfg: cfg}
}

func (d *DummyAgent) Ask(question string) (string, error) {
	fmt.Printf("[brain-ask] provider=%s model=%s db=%s\n", d.cfg.Provider, d.cfg.Model, d.cfg.DB)
	return "A: (dummy) Memory lookup not yet implemented.", nil
}
