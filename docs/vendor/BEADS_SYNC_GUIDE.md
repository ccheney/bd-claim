# Beads Synchronization Guide

This guide outlines the process for updating `bd-claim` to support a new version of the upstream `beads` repository.

## Prerequisites

1.  Ensure `vendor/beads` is initialized:
    ```bash
    git submodule update --init --recursive
    ```

## Update Workflow

1.  **Update the Submodule:**
    ```bash
    cd vendor/beads
    git fetch origin
    git checkout <target-tag-or-branch>
    cd ../..
    git add vendor/beads
    ```

2.  **Generate Context Dump:**
    Run the spelunker script to gather the relevant code files from the new version.
    ```bash
    chmod +x scripts/gather_beads_context.sh
    ./scripts/gather_beads_context.sh
    ```
    *Output will be at `docs/vendor/beads_context_dump.txt`*

3.  **Verify Beads CLI:**
    Ensure the updated submodule provides a working CLI.
    ```bash
    # Assuming bd is in your path or checking the vendor source
    bd --version
	bd quickstart
    bd --help
    ```

4.  **Generate Migration Plan (Using LLM):**
    Feed the `beads_context_dump.txt` and the current `docs/plans/SDD.md` into an LLM using the prompt below.

---

## LLM Prompt Template

**Role:** Senior Software Architect & maintainer of `bd-claim`.

**Task:**
Analyze the provided "Beads Context Dump" (Source Code) and compare it against the existing "Current Documentation" (if any). Generate a detailed "Specification & Migration Plan" for `bd-claim`.

**Input Data:**
1.  **Beads Context Dump:** (Paste content of `docs/vendor/beads_context_dump.txt`)
2.  **Current SDD:** (Paste content of `docs/plans/SDD.md`)

**Output Artifact:** `docs/vendor/beads@<VERSION>.md`

**Required Output Sections:**

1.  **Version Metadata:**
    *   Beads Commit/Tag: <HASH/TAG>
    *   Analysis Date: <YYYY-MM-DD>

2.  **Database Schema Specification:**
    *   List all Tables and Columns found in the `schema.go` and `migrations/`.
    *   *Crucial:* Identify any **changes** from the previous version (if comparing). For the Bootstrap run, just list the full schema.

3.  **The "Ready Predicate" Definition:**
    *   **Conceptual Definition:** What does "Ready" mean for a user? (Derived from comments/docs in code).
    *   **Technical Definition:** Exact logic (e.g., `status == 'done' AND assignees > 0`). Quote the Go code responsible.

4.  **CLI Impact Analysis:**
    *   Which `bd-claim` commands need updating?
    *   Are there new internal types we need to mirror?

5.  **Migration Todos:**
    *   A checklist of files to update in `bd-claim`.

---
