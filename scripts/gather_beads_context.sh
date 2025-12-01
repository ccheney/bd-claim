#!/bin/bash

# scripts/gather_beads_context.sh
# Scans the vendor/beads submodule for Schema and Logic definitions.

OUTPUT_FILE="docs/maintenance/beads_context_dump.txt"
BEADS_DIR="vendor/beads"

echo "Gathering Beads Context..." > "$OUTPUT_FILE"
echo "Date: $(date)" >> "$OUTPUT_FILE"
echo "Commit: $(git -C $BEADS_DIR rev-parse HEAD)" >> "$OUTPUT_FILE"
echo "---------------------------------------------------" >> "$OUTPUT_FILE"

# Function to append file content
append_file() {
    local file="$1"
    if [ -f "$file" ]; then
        echo "FILE: $file" >> "$OUTPUT_FILE"
        echo "START_CONTENT" >> "$OUTPUT_FILE"
        cat "$file" >> "$OUTPUT_FILE"
        echo "END_CONTENT" >> "$OUTPUT_FILE"
        echo "---------------------------------------------------" >> "$OUTPUT_FILE"
    else
        echo "WARNING: Could not find $file"
    fi
}

# 1. Gather Schema Definitions
echo "Gathering Schema..."
append_file "$BEADS_DIR/internal/storage/sqlite/schema.go"

# Gather all migrations
for migration in "$BEADS_DIR/internal/storage/sqlite/migrations/"*.go; do
    append_file "$migration"
done

# 2. Gather Ready Predicate Logic
echo "Gathering Ready Predicate..."
append_file "$BEADS_DIR/internal/storage/sqlite/ready.go"
append_file "$BEADS_DIR/cmd/bd/ready.go"

# 3. Gather Core Types
echo "Gathering Types..."
append_file "$BEADS_DIR/internal/types/types.go"
append_file "$BEADS_DIR/internal/types/process.go"

echo "Done. Context dump saved to $OUTPUT_FILE"
