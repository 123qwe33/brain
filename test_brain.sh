#!/bin/bash
set -e

# WARNING: This script is DESTRUCTIVE. It deletes the local brain.db to ensure a clean test state.
# DO NOT run this against a database containing real data.
# FUTURE IMPROVEMENT: Add a --db flag to the 'brain' binary to allow testing against a separate 'test_brain.db'.

# Ensure brain is available
export PATH=$PATH:/home/agents/bin

# Clean up database for a fresh test
rm -f brain.db
rm -f brain.db-wal brain.db-shm

echo "--- Asserting Node A and Node B ---"
brain assert A premise "Node A content"
brain assert B premise "Node B content"

echo "--- Justifying Node A with Node B ---"
brain justify A B

echo "--- Querying (Expected: A, B) ---"
brain query

echo "--- Invalidating Node B ---"
brain invalidate B

echo "--- Querying (Expected: Node B and Node A are OUT) ---"
# If A depends on B and B is OUT, A should also be OUT.
brain query
