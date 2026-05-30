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
	var id int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&id)
	if err == sql.ErrNoRows {
		_, err := r.db.Exec("INSERT INTO nodes (label, type, content, status) VALUES (?, ?, ?, 'IN')", label, nodeType, content)
		return err
	} else if err != nil {
		return err
	}

	_, err = r.db.Exec("UPDATE nodes SET type = ?, content = ? WHERE id = ?", nodeType, content, id)
	if err != nil {
		return err
	}
	return r.Reevaluate(id)
}

func (r *RMS) Justify(nodeLabel, justifierLabel string) error {
	var nodeID, justifierID int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", nodeLabel).Scan(&nodeID)
	if err != nil {
		return fmt.Errorf("node %s not found: %v", nodeLabel, err)
	}
	err = r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", justifierLabel).Scan(&justifierID)
	if err != nil {
		return fmt.Errorf("justifier %s not found: %v", justifierLabel, err)
	}

	_, err = r.db.Exec("INSERT INTO justifications (node_id, justifier_id) VALUES (?, ?)", nodeID, justifierID)
	if err != nil {
		return err
	}

	return r.Reevaluate(nodeID)
}

func (r *RMS) Invalidate(label string) error {
	var id int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&id)
	if err != nil {
		return fmt.Errorf("node %s not found: %v", label, err)
	}

	_, err = r.db.Exec("UPDATE nodes SET status = 'OUT' WHERE id = ?", id)
	if err != nil {
		return err
	}

	return r.triggerDependents(id)
}

func (r *RMS) Query() ([]string, error) {
	rows, err := r.db.Query("SELECT label FROM nodes WHERE status = 'IN'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, nil
}

func (r *RMS) QueryLabel(label string) (string, error) {
	var content string
	err := r.db.QueryRow("SELECT content FROM nodes WHERE label = ? AND status = 'IN'", label).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return content, nil
}

func (r *RMS) Reevaluate(id int) error {
	var currentStatus string
	err := r.db.QueryRow("SELECT status FROM nodes WHERE id = ?", id).Scan(&currentStatus)
	if err != nil {
		return err
	}

	// Check justifications
	rows, err := r.db.Query("SELECT justifier_id FROM justifications WHERE node_id = ?", id)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasJustifications := false
	anyJustifierIn := false

	for rows.Next() {
		hasJustifications = true
		var justifierID int
		if err := rows.Scan(&justifierID); err != nil {
			return err
		}

		var justifierStatus string
		err := r.db.QueryRow("SELECT status FROM nodes WHERE id = ?", justifierID).Scan(&justifierStatus)
		if err != nil {
			continue // Should we handle this error?
		}

		if justifierStatus == "IN" {
			anyJustifierIn = true
			break
		}
	}

	newStatus := currentStatus
	if hasJustifications {
		if anyJustifierIn {
			newStatus = "IN"
		} else {
			newStatus = "OUT"
		}
	}

	if newStatus != currentStatus {
		_, err = r.db.Exec("UPDATE nodes SET status = ? WHERE id = ?", newStatus, id)
		if err != nil {
			return err
		}
		return r.triggerDependents(id)
	}

	return nil
}

func (r *RMS) triggerDependents(id int) error {
	rows, err := r.db.Query("SELECT node_id FROM justifications WHERE justifier_id = ?", id)
	if err != nil {
		return err
	}
	defer rows.Close()

	var dependentIDs []int
	for rows.Next() {
		var depID int
		if err := rows.Scan(&depID); err != nil {
			return err
		}
		dependentIDs = append(dependentIDs, depID)
	}

	for _, depID := range dependentIDs {
		if err := r.Reevaluate(depID); err != nil {
			return err
		}
	}
	return nil
}
