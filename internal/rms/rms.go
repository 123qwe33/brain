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

func (r *RMS) Link(from, to, relation string) error {
	var fromID, toID int
	if err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", from).Scan(&fromID); err != nil {
		return fmt.Errorf("node %s not found: %v", from, err)
	}
	if err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", to).Scan(&toID); err != nil {
		return fmt.Errorf("node %s not found: %v", to, err)
	}

	var existing int
	err := r.db.QueryRow("SELECT id FROM edges WHERE from_id = ? AND to_id = ? AND relation = ?", fromID, toID, relation).Scan(&existing)
	if err == nil {
		return nil // already exists
	}
	if err != sql.ErrNoRows {
		return err
	}

	_, err = r.db.Exec("INSERT INTO edges (from_id, to_id, relation) VALUES (?, ?, ?)", fromID, toID, relation)
	return err
}

func (r *RMS) QueryType(nodeType string) ([]string, error) {
	rows, err := r.db.Query("SELECT label FROM nodes WHERE status = 'IN' AND type = ?", nodeType)
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

func (r *RMS) QueryLabelWithType(label, nodeType string) (string, error) {
	var content string
	err := r.db.QueryRow("SELECT content FROM nodes WHERE label = ? AND status = 'IN' AND type = ?", label, nodeType).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return content, nil
}

type Edge struct {
	Relation    string
	OtherLabel  string
}

func (r *RMS) Relations(label string, incoming bool) ([]Edge, error) {
	var id int
	if err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&id); err != nil {
		return nil, fmt.Errorf("node %s not found: %v", label, err)
	}

	var rows *sql.Rows
	var err error
	if incoming {
		rows, err = r.db.Query(`
			SELECT n.label, e.relation FROM edges e
			JOIN nodes n ON n.id = e.from_id
			WHERE e.to_id = ?`, id)
	} else {
		rows, err = r.db.Query(`
			SELECT n.label, e.relation FROM edges e
			JOIN nodes n ON n.id = e.to_id
			WHERE e.from_id = ?`, id)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.OtherLabel, &e.Relation); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, nil
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
