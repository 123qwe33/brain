package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

//go:embed viewer.html
var viewerHTML []byte

//go:embed d3.min.js
var d3JS []byte

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a local web viewer for the brain graph",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		r := getRMS()
		mux := http.NewServeMux()

		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(viewerHTML)
		})

		mux.HandleFunc("GET /d3.min.js", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(d3JS)
		})

		mux.HandleFunc("GET /api/data", func(w http.ResponseWriter, req *http.Request) {
			data, err := r.AllData()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
		})

		addr := fmt.Sprintf("localhost:%d", servePort)
		fmt.Printf("Brain viewer: http://%s\n", addr)
		log.Fatal(http.ListenAndServe(addr, mux))
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to serve on")
}
