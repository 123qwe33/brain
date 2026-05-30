package rms

import (
	"database/sql"
	"fmt"
)

type RMS struct {
	db *sql.DB
}

func New(db *sql.DB) *RMS {
	return &RMS{db: db}
}

func (r *RMS) Assert(label, nodeType, content string) error {
	fmt.Printf("RMS: Asserting node %s (type: %s, content: %s)\n", label, nodeType, content)
	return nil
}

func (r *RMS) Justify(nodeLabel, justifierLabel string) error {
	fmt.Printf("RMS: Justifying node %s with %s\n", nodeLabel, justifierLabel)
	return nil
}

func (r *RMS) Query() ([]string, error) {
	fmt.Println("RMS: Querying nodes with status 'IN'")
	return []string{}, nil
}

func (r *RMS) Invalidate(label string) error {
	fmt.Printf("RMS: Invalidating node %s\n", label)
	return nil
}
