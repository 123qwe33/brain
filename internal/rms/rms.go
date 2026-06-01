package rms

import (
	"database/sql"
	"fmt"
	"time"
)

type RMS struct {
	db *sql.DB
}

func New(db *sql.DB) *RMS {
	return &RMS{db: db}
}

func (r *RMS) appendEvent(eventType, nodeLabel, agentLabel, oldStatus, newStatus string, oldConf, newConf float64, detail string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = r.db.Exec(
		`INSERT INTO events (occurred_at, event_type, node_label, agent_label, old_status, new_status, old_confidence, new_confidence, detail)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now, eventType, nodeLabel, agentLabel, oldStatus, newStatus, oldConf, newConf, detail,
	)
}

func (r *RMS) Assert(label, nodeType, content string) error {
	var id int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&id)
	if err == sql.ErrNoRows {
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := r.db.Exec("INSERT INTO nodes (label, type, content, status, confidence, created_at) VALUES (?, ?, ?, 'IN', 1.0, ?)", label, nodeType, content, now)
		if err != nil {
			return err
		}
		newID, _ := res.LastInsertId()
		r.appendEvent("node_asserted", label, "", "", "IN", 0, 1.0, "")
		return r.recomputeConfidence(int(newID))
	} else if err != nil {
		return err
	}

	_, err = r.db.Exec("UPDATE nodes SET type = ?, content = ? WHERE id = ?", nodeType, content, id)
	if err != nil {
		return err
	}
	r.appendEvent("node_updated", label, "", "", "", 0, 0, "content updated")
	return r.Reevaluate(id)
}

func (r *RMS) Justify(nodeLabel string, justifierLabels []string) error {
	var nodeID int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", nodeLabel).Scan(&nodeID)
	if err != nil {
		return fmt.Errorf("node %s not found: %v", nodeLabel, err)
	}

	var justifierIDs []int
	for _, label := range justifierLabels {
		var jID int
		err = r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&jID)
		if err != nil {
			return fmt.Errorf("justifier %s not found: %v", label, err)
		}
		justifierIDs = append(justifierIDs, jID)
	}

	var groupID int
	err = r.db.QueryRow("SELECT COALESCE(MAX(group_id), 0) + 1 FROM justifications WHERE node_id = ?", nodeID).Scan(&groupID)
	if err != nil {
		return err
	}

	for _, jID := range justifierIDs {
		_, err = r.db.Exec("INSERT INTO justifications (node_id, group_id, justifier_id) VALUES (?, ?, ?)", nodeID, groupID, jID)
		if err != nil {
			return err
		}
	}

	r.appendEvent("justified", nodeLabel, "", "", "", 0, 0, fmt.Sprintf("group %d added with %d justifier(s)", groupID, len(justifierIDs)))
	return r.Reevaluate(nodeID)
}

func (r *RMS) Invalidate(label string) error {
	var id int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&id)
	if err != nil {
		return fmt.Errorf("node %s not found: %v", label, err)
	}

	_, err = r.db.Exec("UPDATE nodes SET status = 'OUT', confidence = 0.0 WHERE id = ?", id)
	if err != nil {
		return err
	}
	r.appendEvent("node_invalidated", label, "", "", "OUT", 0, 0, "explicit invalidation")

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

	rows, err := r.db.Query("SELECT group_id, justifier_id FROM justifications WHERE node_id = ?", id)
	if err != nil {
		return err
	}
	defer rows.Close()

	groups := make(map[int][]int)
	for rows.Next() {
		var gID, jID int
		if err := rows.Scan(&gID, &jID); err != nil {
			return err
		}
		groups[gID] = append(groups[gID], jID)
	}

	if len(groups) == 0 {
		return nil // Bedrock node, leave status unchanged
	}

	newStatus := "OUT"
	newConfidence := 0.0
	for _, justifierIDs := range groups {
		allInGroupIn := true
		groupMinConf := 1.0
		for _, jID := range justifierIDs {
			var status string
			var conf float64
			err := r.db.QueryRow("SELECT status, COALESCE(confidence, 1.0) FROM nodes WHERE id = ?", jID).Scan(&status, &conf)
			if err != nil || status != "IN" {
				allInGroupIn = false
				break
			}
			if conf < groupMinConf {
				groupMinConf = conf
			}
		}
		if allInGroupIn {
			candidate := groupMinConf * 0.95
			if candidate > newConfidence {
				newConfidence = candidate
			}
			newStatus = "IN"
		}
	}

	var needsReview int
	_ = r.db.QueryRow("SELECT COALESCE(needs_review, 0) FROM nodes WHERE id = ?", id).Scan(&needsReview)
	if needsReview == 1 && newStatus == "IN" {
		newConfidence *= 0.8
	}

	var oldConfidence float64
	_ = r.db.QueryRow("SELECT COALESCE(confidence, 1.0) FROM nodes WHERE id = ?", id).Scan(&oldConfidence)

	if newStatus != currentStatus {
		var label string
		_ = r.db.QueryRow("SELECT label FROM nodes WHERE id = ?", id).Scan(&label)
		_, err = r.db.Exec("UPDATE nodes SET status = ?, confidence = ? WHERE id = ?", newStatus, newConfidence, id)
		if err != nil {
			return err
		}
		r.appendEvent("status_cascaded", label, "", currentStatus, newStatus, oldConfidence, newConfidence, "")
		return r.triggerDependents(id)
	}

	if newConfidence != oldConfidence {
		_, _ = r.db.Exec("UPDATE nodes SET confidence = ? WHERE id = ?", newConfidence, id)
	}

	return nil
}

func (r *RMS) recomputeConfidence(id int) error {
	return r.Reevaluate(id)
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
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	_, err = r.db.Exec("INSERT INTO edges (from_id, to_id, relation) VALUES (?, ?, ?)", fromID, toID, relation)
	if err == nil {
		r.appendEvent("linked", from, "", "", "", 0, 0, fmt.Sprintf("%s -[%s]-> %s", from, relation, to))
	}
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

type EventEntry struct {
	ID            int     `json:"id"`
	OccurredAt    string  `json:"occurred_at"`
	EventType     string  `json:"event_type"`
	NodeLabel     string  `json:"node_label"`
	AgentLabel    string  `json:"agent_label,omitempty"`
	OldStatus     string  `json:"old_status,omitempty"`
	NewStatus     string  `json:"new_status,omitempty"`
	OldConfidence float64 `json:"old_confidence,omitempty"`
	NewConfidence float64 `json:"new_confidence,omitempty"`
	Detail        string  `json:"detail,omitempty"`
}

func (r *RMS) Log(nodeLabel string, eventType string, limit int) ([]EventEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT id, occurred_at, event_type, node_label,
COALESCE(agent_label,''), COALESCE(old_status,''), COALESCE(new_status,''),
COALESCE(old_confidence,0), COALESCE(new_confidence,0), COALESCE(detail,'')
FROM events WHERE 1=1`
	args := make([]any, 0, 3)
	if nodeLabel != "" {
		query += " AND node_label = ?"
		args = append(args, nodeLabel)
	}
	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EventEntry
	for rows.Next() {
		var e EventEntry
		if err := rows.Scan(
			&e.ID,
			&e.OccurredAt,
			&e.EventType,
			&e.NodeLabel,
			&e.AgentLabel,
			&e.OldStatus,
			&e.NewStatus,
			&e.OldConfidence,
			&e.NewConfidence,
			&e.Detail,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

type Edge struct {
	Relation   string
	OtherLabel string
}

type GraphNode struct {
	Label  string `json:"label"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type GraphResult struct {
	Center       string      `json:"center"`
	DepthReached int         `json:"depth_reached"`
	Nodes        []GraphNode `json:"nodes"`
	Edges        []GraphEdge `json:"edges"`
}

type GraphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

func (r *RMS) Graph(startLabel string, maxDepth int) (*GraphResult, error) {
	var startID int
	err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", startLabel).Scan(&startID)
	if err != nil {
		return nil, fmt.Errorf("start node %s not found: %v", startLabel, err)
	}

	visited := make(map[int]bool)
	seenEdges := make(map[[3]string]bool)
	var resultNodes []GraphNode
	var resultEdges []GraphEdge
	depthReached := 0

	type queueItem struct {
		id    int
		depth int
	}
	queue := []queueItem{{id: startID, depth: 0}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if visited[item.id] {
			continue
		}
		visited[item.id] = true

		var node GraphNode
		err := r.db.QueryRow("SELECT label, type, status FROM nodes WHERE id = ?", item.id).Scan(&node.Label, &node.Type, &node.Status)
		if err != nil {
			return nil, err
		}
		resultNodes = append(resultNodes, node)

		if item.depth > depthReached {
			depthReached = item.depth
		}

		if item.depth >= maxDepth {
			continue
		}

		// Outgoing edges
		rows, err := r.db.Query("SELECT n.label, e.relation, e.to_id FROM edges e JOIN nodes n ON n.id = e.to_id WHERE e.from_id = ?", item.id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var toLabel, relation string
			var toID int
			if err := rows.Scan(&toLabel, &relation, &toID); err != nil {
				rows.Close()
				return nil, err
			}
			key := [3]string{node.Label, relation, toLabel}
			if !seenEdges[key] {
				seenEdges[key] = true
				resultEdges = append(resultEdges, GraphEdge{From: node.Label, To: toLabel, Relation: relation})
			}
			if !visited[toID] {
				queue = append(queue, queueItem{id: toID, depth: item.depth + 1})
			}
		}
		rows.Close()

		// Incoming edges
		rows, err = r.db.Query("SELECT n.label, e.relation, e.from_id FROM edges e JOIN nodes n ON n.id = e.from_id WHERE e.to_id = ?", item.id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var fromLabel, relation string
			var fromID int
			if err := rows.Scan(&fromLabel, &relation, &fromID); err != nil {
				rows.Close()
				return nil, err
			}
			key := [3]string{fromLabel, relation, node.Label}
			if !seenEdges[key] {
				seenEdges[key] = true
				resultEdges = append(resultEdges, GraphEdge{From: fromLabel, To: node.Label, Relation: relation})
			}
			if !visited[fromID] {
				queue = append(queue, queueItem{id: fromID, depth: item.depth + 1})
			}
		}
		rows.Close()
	}

	return &GraphResult{
		Center:       startLabel,
		DepthReached: depthReached,
		Nodes:        resultNodes,
		Edges:        resultEdges,
	}, nil
}

func (r *RMS) Relations(label string, incoming bool) ([]Edge, error) {
	var id int
	if err := r.db.QueryRow("SELECT id FROM nodes WHERE label = ?", label).Scan(&id); err != nil {
		return nil, fmt.Errorf("node %s not found: %v", label, err)
	}

	var rows *sql.Rows
	var err error
	if incoming {
		rows, err = r.db.Query("SELECT n.label, e.relation FROM edges e JOIN nodes n ON n.id = e.from_id WHERE e.to_id = ?", id)
	} else {
		rows, err = r.db.Query("SELECT n.label, e.relation FROM edges e JOIN nodes n ON n.id = e.to_id WHERE e.from_id = ?", id)
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
