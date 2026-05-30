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
	queryTypeFlag   string
	relationsInFlag bool
	graphDepthFlag  int

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
			if err := r.Assert(args[0], args[1], args[2]); err != nil {
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
)

func init() {
	queryCmd.Flags().StringVarP(&queryTypeFlag, "type", "t", "", "Filter by node type")
	relationsCmd.Flags().BoolVar(&relationsInFlag, "in", false, "Show incoming edges instead of outgoing")
	graphCmd.Flags().IntVarP(&graphDepthFlag, "depth", "d", 1, "BFS traversal depth")

	rootCmd.AddCommand(assertCmd)
	rootCmd.AddCommand(justifyCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(invalidateCmd)
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(relationsCmd)
	rootCmd.AddCommand(graphCmd)
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
