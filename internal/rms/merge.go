package rms

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/123qwe33/brain/internal/db"
)

type nodeRec struct {
	label, nodeType, status, content string
	confidence                        float64
	needsReview                       int
	createdAt, agentLabel             string
}

type edgeKey struct{ from, relation, to string }

type justGroupKey struct {
	nodeLabel  string
	justifiers string // sorted justifier labels joined with "|"
}

type justGroupEntry struct {
	nodeLabel  string
	justifiers []string
}

type eventRec struct {
	occurredAt, eventType, nodeLabel string
	agentLabel, oldStatus, newStatus string
	oldConfidence, newConfidence      float64
	detail                            string
}

// MergeConflict describes a node whose content differed between ours and theirs.
type MergeConflict struct {
	Label, OursContent, TheirsContent string
}

func readNodes(d *sql.DB) (map[string]nodeRec, error) {
	rows, err := d.Query(`SELECT label, type, status,
		COALESCE(content,''), COALESCE(confidence,1.0),
		COALESCE(needs_review,0), COALESCE(created_at,''), COALESCE(agent_label,'')
		FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]nodeRec)
	for rows.Next() {
		var n nodeRec
		if err := rows.Scan(&n.label, &n.nodeType, &n.status, &n.content,
			&n.confidence, &n.needsReview, &n.createdAt, &n.agentLabel); err != nil {
			return nil, err
		}
		m[n.label] = n
	}
	return m, rows.Err()
}

func readEdges(d *sql.DB) (map[edgeKey]bool, error) {
	rows, err := d.Query(`SELECT n1.label, e.relation, n2.label
		FROM edges e
		JOIN nodes n1 ON n1.id = e.from_id
		JOIN nodes n2 ON n2.id = e.to_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[edgeKey]bool)
	for rows.Next() {
		var k edgeKey
		if err := rows.Scan(&k.from, &k.relation, &k.to); err != nil {
			return nil, err
		}
		m[k] = true
	}
	return m, rows.Err()
}

func readJustifications(d *sql.DB) (map[justGroupKey]justGroupEntry, error) {
	rows, err := d.Query(`SELECT n.label, j.group_id, jn.label
		FROM justifications j
		JOIN nodes n  ON n.id  = j.node_id
		JOIN nodes jn ON jn.id = j.justifier_id
		ORDER BY n.label, j.group_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawKey struct {
		nodeLabel string
		groupID   int
	}
	raw := make(map[rawKey][]string)
	var order []rawKey

	for rows.Next() {
		var nodeLabel, justLabel string
		var gID int
		if err := rows.Scan(&nodeLabel, &gID, &justLabel); err != nil {
			return nil, err
		}
		k := rawKey{nodeLabel, gID}
		if _, ok := raw[k]; !ok {
			order = append(order, k)
		}
		raw[k] = append(raw[k], justLabel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(map[justGroupKey]justGroupEntry)
	for _, k := range order {
		justs := raw[k]
		sorted := make([]string, len(justs))
		copy(sorted, justs)
		sort.Strings(sorted)
		jk := justGroupKey{nodeLabel: k.nodeLabel, justifiers: strings.Join(sorted, "|")}
		if _, dup := result[jk]; !dup {
			result[jk] = justGroupEntry{nodeLabel: k.nodeLabel, justifiers: justs}
		}
	}
	return result, nil
}

func readEvents(d *sql.DB) ([]eventRec, error) {
	rows, err := d.Query(`SELECT occurred_at, event_type, node_label,
		COALESCE(agent_label,''), COALESCE(old_status,''), COALESCE(new_status,''),
		COALESCE(old_confidence,0), COALESCE(new_confidence,0), COALESCE(detail,'')
		FROM events ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []eventRec
	for rows.Next() {
		var e eventRec
		if err := rows.Scan(&e.occurredAt, &e.eventType, &e.nodeLabel,
			&e.agentLabel, &e.oldStatus, &e.newStatus,
			&e.oldConfidence, &e.newConfidence, &e.detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nodeChanged(a, b nodeRec) bool {
	return a.nodeType != b.nodeType || a.status != b.status ||
		a.content != b.content || a.confidence != b.confidence ||
		a.needsReview != b.needsReview
}

func conflictMarkers(ours, theirs string) string {
	return "<<<<<<< ours\n" + ours + "\n=======\n" + theirs + "\n>>>>>>> theirs"
}

func allNodeKeys(maps ...map[string]nodeRec) []string {
	seen := make(map[string]bool)
	for _, m := range maps {
		for k := range m {
			seen[k] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func writeMerged(path string, nodes map[string]nodeRec, edges []edgeKey, justs []justGroupEntry, events []eventRec, conflictCount int) error {
	database, err := db.InitDB(path)
	if err != nil {
		return err
	}

	labelToID := make(map[string]int64)

	// Sort node labels for deterministic insert order
	labels := make([]string, 0, len(nodes))
	for l := range nodes {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	for _, label := range labels {
		n := nodes[label]
		res, err := database.Exec(
			`INSERT INTO nodes (label, type, status, content, confidence, needs_review, created_at, agent_label)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.label, n.nodeType, n.status, n.content, n.confidence, n.needsReview, n.createdAt, n.agentLabel)
		if err != nil {
			database.Close()
			return fmt.Errorf("insert node %s: %w", label, err)
		}
		id, _ := res.LastInsertId()
		labelToID[label] = id
	}

	for _, e := range edges {
		if _, err := database.Exec("INSERT INTO edges (from_id, to_id, relation) VALUES (?, ?, ?)",
			labelToID[e.from], labelToID[e.to], e.relation); err != nil {
			database.Close()
			return fmt.Errorf("insert edge %s -[%s]-> %s: %w", e.from, e.relation, e.to, err)
		}
	}

	nodeGroupCounter := make(map[string]int)
	for _, jg := range justs {
		nodeGroupCounter[jg.nodeLabel]++
		gID := nodeGroupCounter[jg.nodeLabel]
		nodeID := labelToID[jg.nodeLabel]
		for _, justLabel := range jg.justifiers {
			if _, err := database.Exec("INSERT INTO justifications (node_id, group_id, justifier_id) VALUES (?, ?, ?)",
				nodeID, gID, labelToID[justLabel]); err != nil {
				database.Close()
				return fmt.Errorf("insert justification for %s: %w", jg.nodeLabel, err)
			}
		}
	}

	for _, e := range events {
		if _, err := database.Exec(
			`INSERT INTO events (occurred_at, event_type, node_label, agent_label,
			 old_status, new_status, old_confidence, new_confidence, detail)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.occurredAt, e.eventType, e.nodeLabel, e.agentLabel,
			e.oldStatus, e.newStatus, e.oldConfidence, e.newConfidence, e.detail); err != nil {
			database.Close()
			return fmt.Errorf("insert event: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	detail := "clean"
	if conflictCount > 0 {
		detail = fmt.Sprintf("%d conflict(s)", conflictCount)
	}
	if _, err := database.Exec(
		`INSERT INTO events (occurred_at, event_type, node_label, detail) VALUES (?, 'merge', '', ?)`,
		now, detail); err != nil {
		database.Close()
		return fmt.Errorf("insert merge event: %w", err)
	}

	_, _ = database.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	database.Close()
	os.Remove(path + "-wal")
	os.Remove(path + "-shm")
	return nil
}

// Merge performs a 3-way semantic merge of brain.db files, matching nodes by
// label rather than row ID. The merged result is written to oursPath. Returns
// the list of content conflicts; on conflict the node content embeds git-style
// markers and needs_review is set to 1.
func Merge(basePath, oursPath, theirsPath string) ([]MergeConflict, error) {
	baseDB, err := db.InitDB(basePath)
	if err != nil {
		return nil, fmt.Errorf("open base: %w", err)
	}
	oursDB, err := db.InitDB(oursPath)
	if err != nil {
		baseDB.Close()
		return nil, fmt.Errorf("open ours: %w", err)
	}
	theirsDB, err := db.InitDB(theirsPath)
	if err != nil {
		baseDB.Close()
		oursDB.Close()
		return nil, fmt.Errorf("open theirs: %w", err)
	}

	baseNodes, err := readNodes(baseDB)
	if err != nil {
		return nil, fmt.Errorf("read base nodes: %w", err)
	}
	oursNodes, err := readNodes(oursDB)
	if err != nil {
		return nil, fmt.Errorf("read ours nodes: %w", err)
	}
	theirsNodes, err := readNodes(theirsDB)
	if err != nil {
		return nil, fmt.Errorf("read theirs nodes: %w", err)
	}
	baseEdges, err := readEdges(baseDB)
	if err != nil {
		return nil, fmt.Errorf("read base edges: %w", err)
	}
	oursEdges, err := readEdges(oursDB)
	if err != nil {
		return nil, fmt.Errorf("read ours edges: %w", err)
	}
	theirsEdges, err := readEdges(theirsDB)
	if err != nil {
		return nil, fmt.Errorf("read theirs edges: %w", err)
	}
	baseJusts, err := readJustifications(baseDB)
	if err != nil {
		return nil, fmt.Errorf("read base justifications: %w", err)
	}
	oursJusts, err := readJustifications(oursDB)
	if err != nil {
		return nil, fmt.Errorf("read ours justifications: %w", err)
	}
	theirsJusts, err := readJustifications(theirsDB)
	if err != nil {
		return nil, fmt.Errorf("read theirs justifications: %w", err)
	}
	baseEvents, err := readEvents(baseDB)
	if err != nil {
		return nil, fmt.Errorf("read base events: %w", err)
	}
	oursEvents, err := readEvents(oursDB)
	if err != nil {
		return nil, fmt.Errorf("read ours events: %w", err)
	}
	theirsEvents, err := readEvents(theirsDB)
	if err != nil {
		return nil, fmt.Errorf("read theirs events: %w", err)
	}

	baseDB.Close()
	oursDB.Close()
	theirsDB.Close()

	// --- Merge nodes ---
	mergedNodes := make(map[string]nodeRec)
	var conflicts []MergeConflict

	for _, label := range allNodeKeys(baseNodes, oursNodes, theirsNodes) {
		base, inBase := baseNodes[label]
		ours, inOurs := oursNodes[label]
		theirs, inTheirs := theirsNodes[label]

		switch {
		case !inBase && inOurs && !inTheirs:
			mergedNodes[label] = ours

		case !inBase && !inOurs && inTheirs:
			mergedNodes[label] = theirs

		case !inBase && inOurs && inTheirs:
			// New in both branches
			if !nodeChanged(ours, theirs) {
				mergedNodes[label] = ours
			} else {
				n := ours
				n.content = conflictMarkers(ours.content, theirs.content)
				n.needsReview = 1
				mergedNodes[label] = n
				conflicts = append(conflicts, MergeConflict{label, ours.content, theirs.content})
			}

		case inBase && !inOurs && !inTheirs:
			// Deleted in both — omit

		case inBase && inOurs && !inTheirs:
			// Deleted in theirs
			if nodeChanged(base, ours) {
				// Ours modified it; theirs deleted it — conflict, keep ours
				n := ours
				n.content = conflictMarkers(ours.content, "[deleted in theirs]")
				n.needsReview = 1
				mergedNodes[label] = n
				conflicts = append(conflicts, MergeConflict{label, ours.content, "[deleted in theirs]"})
			}
			// else: ours unchanged, theirs deleted → respect deletion

		case inBase && !inOurs && inTheirs:
			// Deleted in ours
			if nodeChanged(base, theirs) {
				// Theirs modified it; ours deleted it — conflict, keep theirs
				n := theirs
				n.content = conflictMarkers("[deleted in ours]", theirs.content)
				n.needsReview = 1
				mergedNodes[label] = n
				conflicts = append(conflicts, MergeConflict{label, "[deleted in ours]", theirs.content})
			}
			// else: theirs unchanged, ours deleted → respect deletion

		case inBase && inOurs && inTheirs:
			oursChanged := nodeChanged(base, ours)
			theirsChanged := nodeChanged(base, theirs)
			switch {
			case !oursChanged && !theirsChanged:
				mergedNodes[label] = ours
			case oursChanged && !theirsChanged:
				mergedNodes[label] = ours
			case !oursChanged && theirsChanged:
				mergedNodes[label] = theirs
			default:
				if !nodeChanged(ours, theirs) {
					mergedNodes[label] = ours // same change on both sides
				} else {
					n := ours
					n.content = conflictMarkers(ours.content, theirs.content)
					n.needsReview = 1
					mergedNodes[label] = n
					conflicts = append(conflicts, MergeConflict{label, ours.content, theirs.content})
				}
			}
		}
	}

	// --- Merge edges ---
	allEdgeKeys := make(map[edgeKey]bool)
	for k := range baseEdges {
		allEdgeKeys[k] = true
	}
	for k := range oursEdges {
		allEdgeKeys[k] = true
	}
	for k := range theirsEdges {
		allEdgeKeys[k] = true
	}

	var mergedEdges []edgeKey
	// Sort for determinism
	sortedEdges := make([]edgeKey, 0, len(allEdgeKeys))
	for k := range allEdgeKeys {
		sortedEdges = append(sortedEdges, k)
	}
	sort.Slice(sortedEdges, func(i, j int) bool {
		a, b := sortedEdges[i], sortedEdges[j]
		if a.from != b.from {
			return a.from < b.from
		}
		if a.relation != b.relation {
			return a.relation < b.relation
		}
		return a.to < b.to
	})

	for _, k := range sortedEdges {
		inBase := baseEdges[k]
		inOurs := oursEdges[k]
		inTheirs := theirsEdges[k]

		keep := false
		if !inBase {
			keep = inOurs || inTheirs // new in either branch
		} else {
			keep = inOurs && inTheirs // only if neither side deleted it
		}
		if !keep {
			continue
		}
		if _, ok := mergedNodes[k.from]; !ok {
			continue
		}
		if _, ok := mergedNodes[k.to]; !ok {
			continue
		}
		mergedEdges = append(mergedEdges, k)
	}

	// --- Merge justifications ---
	allJusts := make(map[justGroupKey]justGroupEntry)
	for k, v := range baseJusts {
		allJusts[k] = v
	}
	for k, v := range oursJusts {
		allJusts[k] = v
	}
	for k, v := range theirsJusts {
		allJusts[k] = v
	}

	// Sort for determinism
	justKeys := make([]justGroupKey, 0, len(allJusts))
	for k := range allJusts {
		justKeys = append(justKeys, k)
	}
	sort.Slice(justKeys, func(i, j int) bool {
		a, b := justKeys[i], justKeys[j]
		if a.nodeLabel != b.nodeLabel {
			return a.nodeLabel < b.nodeLabel
		}
		return a.justifiers < b.justifiers
	})

	var mergedJusts []justGroupEntry
	for _, k := range justKeys {
		entry := allJusts[k]
		_, inBase := baseJusts[k]
		_, inOurs := oursJusts[k]
		_, inTheirs := theirsJusts[k]

		keep := false
		if !inBase {
			keep = inOurs || inTheirs
		} else {
			keep = inOurs && inTheirs
		}
		if !keep {
			continue
		}
		// Skip if any referenced node was dropped from the merge
		if _, ok := mergedNodes[entry.nodeLabel]; !ok {
			continue
		}
		allPresent := true
		for _, j := range entry.justifiers {
			if _, ok := mergedNodes[j]; !ok {
				allPresent = false
				break
			}
		}
		if allPresent {
			mergedJusts = append(mergedJusts, entry)
		}
	}

	// --- Merge events ---
	seen := make(map[string]bool)
	var mergedEvents []eventRec
	addEvent := func(e eventRec) {
		key := e.occurredAt + "|" + e.eventType + "|" + e.nodeLabel + "|" + e.detail
		if !seen[key] {
			seen[key] = true
			mergedEvents = append(mergedEvents, e)
		}
	}
	for _, e := range baseEvents {
		addEvent(e)
	}
	for _, e := range oursEvents {
		addEvent(e)
	}
	for _, e := range theirsEvents {
		addEvent(e)
	}

	// --- Write merged result ---
	tmpPath := oursPath + ".brain-merge-tmp"
	if err := writeMerged(tmpPath, mergedNodes, mergedEdges, mergedJusts, mergedEvents, len(conflicts)); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("write merged db: %w", err)
	}
	if err := os.Rename(tmpPath, oursPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("replace ours with merged result: %w", err)
	}

	return conflicts, nil
}
