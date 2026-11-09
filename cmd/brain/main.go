package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/123qwe33/brain/internal/db"
	"github.com/123qwe33/brain/internal/rms"
	"github.com/spf13/cobra"
)

var (
	queryTypeFlag    string
	relationsInFlag  bool
	graphDepthFlag   int
	logNodeFlag      string
	logTypeFlag      string
	logLimitFlag     int
	searchAllFlag    bool
	assertConfidence float64

	rootCmd = &cobra.Command{
		Use:   "brain",
		Short: "Brain is a Reason Maintenance System CLI",
	}

	assertCmd = &cobra.Command{
		Use:   "assert [label] [type] [content]",
		Short: "Creates a node if it doesn't exist",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			var conf *float64
			if cmd.Flags().Changed("confidence") {
				conf = &assertConfidence
			}
			if err := r.Assert(args[0], args[1], args[2], conf); err != nil {
				log.Fatalf("Error asserting node: %v", err)
			}
		},
	}

	justifyCmd = &cobra.Command{
		Use:   "justify [node] [justifier...] -- creates an AND-group from all provided justifiers",
		Short: "Adds a justification linking a node to its justifier(s)",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Justify(args[0], args[1:]); err != nil {
				log.Fatalf("Error justifying node: %v", err)
			}
		},
	}

	queryCmd = &cobra.Command{
		Use:   "query [label]",
		Short: "Selects nodes where status = 'IN' or returns content of a specific node",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if len(args) == 0 {
				if queryTypeFlag != "" {
					nodes, err := r.QueryType(queryTypeFlag)
					if err != nil {
						log.Fatalf("Error querying nodes: %v", err)
					}
					for _, node := range nodes {
						fmt.Println(node)
					}
				} else {
					nodes, err := r.Query()
					if err != nil {
						log.Fatalf("Error querying nodes: %v", err)
					}
					for _, node := range nodes {
						fmt.Println(node)
					}
				}
			} else {
				var content string
				var err error
				if queryTypeFlag != "" {
					content, err = r.QueryLabelWithType(args[0], queryTypeFlag)
				} else {
					content, err = r.QueryLabel(args[0])
				}
				if err != nil {
					log.Fatalf("Error querying node content: %v", err)
				}
				if content != "" {
					fmt.Println(content)
				} else {
					os.Exit(1)
				}
			}
		},
	}

	invalidateCmd = &cobra.Command{
		Use:   "invalidate [label]",
		Short: "Invalidates a node",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Invalidate(args[0]); err != nil {
				log.Fatalf("Error invalidating node: %v", err)
			}
		},
	}

	linkCmd = &cobra.Command{
		Use:   "link [from] [to] [relation]",
		Short: "Creates a directed edge between two nodes",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Link(args[0], args[1], args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "Error linking nodes: %v\n", err)
				os.Exit(1)
			}
		},
	}

	unlinkCmd = &cobra.Command{
		Use:   "unlink [from] [to] [relation]",
		Short: "Removes a directed edge between two nodes",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Unlink(args[0], args[1], args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "Error unlinking nodes: %v\n", err)
				os.Exit(1)
			}
		},
	}

	relationsCmd = &cobra.Command{
		Use:   "relations [label]",
		Short: "Lists edges for a node (outgoing by default, incoming with --in)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			edges, err := r.Relations(args[0], relationsInFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching relations: %v\n", err)
				os.Exit(1)
			}
			for _, e := range edges {
				if relationsInFlag {
					fmt.Printf("%s -[%s]->\n", e.OtherLabel, e.Relation)
				} else {
					fmt.Printf("-[%s]-> %s\n", e.Relation, e.OtherLabel)
				}
			}
		},
	}

	graphCmd = &cobra.Command{
		Use:   "graph [label]",
		Short: "Returns a radial BFS context bubble as JSON",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			result, err := r.Graph(args[0], graphDepthFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error traversing graph: %v\n", err)
				os.Exit(1)
			}
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error serializing result: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(out))
		},
	}

	deleteCmd = &cobra.Command{
		Use:   "delete [label]",
		Short: "Hard-deletes a node and all its edges and justifications",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Delete(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting node: %v\n", err)
				os.Exit(1)
			}
		},
	}

	renameCmd = &cobra.Command{
		Use:   "rename [old-label] [new-label]",
		Short: "Renames a node label, preserving all edges and justifications",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Rename(args[0], args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error renaming node: %v\n", err)
				os.Exit(1)
			}
		},
	}

	logCmd = &cobra.Command{
		Use:   "log",
		Short: "Shows recent event log entries",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			events, err := r.Log(logNodeFlag, logTypeFlag, logLimitFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying event log: %v\n", err)
				os.Exit(1)
			}
			for _, e := range events {
				fmt.Printf("%d	%s	%s	node=%s", e.ID, e.OccurredAt, e.EventType, e.NodeLabel)
				if e.AgentLabel != "" {
					fmt.Printf("	agent=%s", e.AgentLabel)
				}
				if e.OldStatus != "" || e.NewStatus != "" {
					fmt.Printf("	status=%s->%s", e.OldStatus, e.NewStatus)
				}
				if e.OldConfidence != 0 || e.NewConfidence != 0 {
					fmt.Printf("	confidence=%.3f->%.3f", e.OldConfidence, e.NewConfidence)
				}
				if e.Detail != "" {
					fmt.Printf("	detail=%s", e.Detail)
				}
				fmt.Println()
			}
		},
	}

	mergeCmd = &cobra.Command{
		Use:   "merge [base] [ours] [theirs]",
		Short: "3-way merge driver for brain.db git conflicts",
		Long: `Merges three brain.db files by node label rather than row ID. Intended
to be called automatically by git when brain.db merge=brain is set in
.gitattributes.

Register the driver (once per machine):
  git config --global merge.brain.driver "brain merge %O %A %B"

Add to .gitattributes (once per repo):
  brain.db merge=brain

On conflict the node content is annotated with <<<<<<</ours/theirs markers
and needs_review is set to 1. Exits 0 on a clean merge, 1 when conflicts remain.`,
		Args: cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			conflicts, err := rms.Merge(args[0], args[1], args[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "brain merge error: %v\n", err)
				os.Exit(1)
			}
			if len(conflicts) > 0 {
				fmt.Fprintf(os.Stderr, "CONFLICT: %d node(s) have conflicting content:\n", len(conflicts))
				for _, c := range conflicts {
					fmt.Fprintf(os.Stderr, "  %s\n", c.Label)
				}
				os.Exit(1)
			}
		},
	}

	searchCmd = &cobra.Command{
		Use:   "search <text>",
		Short: "Search node labels and content",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			results, err := r.Search(args[0], searchAllFlag)
			if err != nil {
				log.Fatalf("Error searching: %v", err)
			}
			for _, res := range results {
				runes := []rune(res.Content)
				excerpt := res.Content
				if len(runes) > 80 {
					excerpt = string(runes[:80]) + "..."
				}
				if searchAllFlag {
					fmt.Printf("%s\t[%s]\t%s\t%s\n", res.Label, res.Type, res.Status, excerpt)
				} else {
					fmt.Printf("%s\t[%s]\t%s\n", res.Label, res.Type, excerpt)
				}
			}
		},
	}

	checkpointCmd = &cobra.Command{
		Use:   "checkpoint",
		Short: "Flush the WAL to the main database file before committing",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			r := getRMS()
			if err := r.Checkpoint(); err != nil {
				log.Fatalf("checkpoint failed: %v", err)
			}
			fmt.Println("checkpoint complete")
		},
	}
)

func init() {
	assertCmd.Flags().Float64Var(&assertConfidence, "confidence", 1.0, "Confidence score 0.0–1.0 (default 1.0 for new nodes, preserved for existing)")
	queryCmd.Flags().StringVarP(&queryTypeFlag, "type", "t", "", "Filter by node type")
	relationsCmd.Flags().BoolVar(&relationsInFlag, "in", false, "Show incoming edges instead of outgoing")
	graphCmd.Flags().IntVarP(&graphDepthFlag, "depth", "d", 1, "BFS traversal depth")
	logCmd.Flags().StringVar(&logNodeFlag, "node", "", "Filter by node label")
	logCmd.Flags().StringVar(&logTypeFlag, "type", "", "Filter by event type")
	logCmd.Flags().IntVar(&logLimitFlag, "limit", 20, "Limit number of events shown")
	searchCmd.Flags().BoolVar(&searchAllFlag, "all", false, "Include OUT nodes in results")

	rootCmd.AddCommand(assertCmd)
	rootCmd.AddCommand(justifyCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(invalidateCmd)
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(unlinkCmd)
	rootCmd.AddCommand(relationsCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(checkpointCmd)
}

func getRMS() *rms.RMS {
	database, err := db.InitDB("brain.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	return rms.New(database)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
