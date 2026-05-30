package main

import (
	"fmt"
	"log"

	"github.com/123qwe33/brain/internal/db"
)

func main() {
	dbPath := "brain.db"
	
	// Remove if exists to start fresh for verification if needed, 
	// but the task just says "Create brain.db", so I'll just init it.
	
	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	fmt.Println("Database initialized successfully at", dbPath)

	// Verify PRAGMAs
	var journalMode string
	err = database.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		log.Fatalf("Failed to query journal_mode: %v", err)
	}
	fmt.Printf("Journal Mode: %s\n", journalMode)

	var foreignKeys int
	err = database.QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys)
	if err != nil {
		log.Fatalf("Failed to query foreign_keys: %v", err)
	}
	fmt.Printf("Foreign Keys: %d\n", foreignKeys)

	// List tables
	rows, err := database.Query("SELECT name FROM sqlite_master WHERE type='table';")
	if err != nil {
		log.Fatalf("Failed to query tables: %v", err)
	}
	defer rows.Close()

	fmt.Println("Tables:")
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s\n", name)
	}
}
