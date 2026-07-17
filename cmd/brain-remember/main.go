package main

import (
	"fmt"
	"os"

	"github.com/123qwe33/brain/internal/memoryagent"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	flagProvider string
	flagModel    string
	flagAPIKey   string
	flagBaseURL  string
	flagDB       string
	flagDryRun   bool

	rootCmd = &cobra.Command{
		Use:   "brain-remember [statement]",
		Short: "Store a plaintext statement as structured memory nodes and links",
		Args:  cobra.ExactArgs(1),
		Run:   run,
	}
)

func init() {
	rootCmd.Flags().StringVar(&flagProvider, "provider", "", "LLM provider (anthropic|openai)")
	rootCmd.Flags().StringVar(&flagModel, "model", "", "Model name")
	rootCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")
	rootCmd.Flags().StringVar(&flagBaseURL, "base-url", "", "Base URL for openai-compatible endpoints")
	rootCmd.Flags().StringVar(&flagDB, "db", "", "Path to brain.db")
	rootCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the system prompt and statement without calling the API or writing anything")
}

func run(cmd *cobra.Command, args []string) {
	_ = godotenv.Load()

	cfg := resolveConfig(cmd)

	var agent memoryagent.RememberAgent
	switch cfg.Provider {
	case "anthropic":
		agent = memoryagent.NewAnthropicRememberAgent(cfg)
	case "openai":
		agent = memoryagent.NewOpenAIRememberAgent(cfg)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown provider %q (use anthropic or openai)\n", cfg.Provider)
		os.Exit(1)
	}

	result, err := agent.Remember(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.DryRun {
		return
	}

	fmt.Printf("Stored %d node(s), %d link(s).\n", len(result.Nodes), len(result.Links))
	for _, n := range result.Nodes {
		fmt.Printf("  + %s (%s)\n", n.Label, n.Type)
	}
	for _, l := range result.Links {
		fmt.Printf("  + link: %s -[%s]-> %s\n", l.From, l.Relation, l.To)
	}
}

// resolveConfig applies precedence: flag > env > default.
func resolveConfig(cmd *cobra.Command) memoryagent.Config {
	cfg := memoryagent.Config{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-6",
		DB:       "./brain.db",
	}

	if v := os.Getenv("BRAIN_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("BRAIN_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("BRAIN_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("BRAIN_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("BRAIN_DB"); v != "" {
		cfg.DB = v
	}

	if cmd.Flags().Changed("provider") {
		cfg.Provider = flagProvider
	}
	if cmd.Flags().Changed("model") {
		cfg.Model = flagModel
	}
	if cmd.Flags().Changed("api-key") {
		cfg.APIKey = flagAPIKey
	}
	if cmd.Flags().Changed("base-url") {
		cfg.BaseURL = flagBaseURL
	}
	if cmd.Flags().Changed("db") {
		cfg.DB = flagDB
	}
	if cmd.Flags().Changed("dry-run") {
		cfg.DryRun = flagDryRun
	}

	return cfg
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
