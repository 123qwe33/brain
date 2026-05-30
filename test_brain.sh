#!/bin/bash
set -e

# Ensure brain is available
export PATH=$PATH:/home/agents/bin

# Clean up database for a fresh test
rm -f brain.db

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
