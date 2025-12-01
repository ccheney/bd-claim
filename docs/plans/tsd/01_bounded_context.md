## 1. Bounded Context Name & Core Domain Type

**Name:** Issue Claiming (bd-claim)
**Type:** Supporting Domain (core to Local Swarm operation, supporting Beads’ core Issue Management domain)

**Purpose:**
Provide a **Beads-native**, **locally atomic** “claim a ready issue” operation that multiple agents can safely use in parallel on a single `.beads` workspace without double-claiming the same issue.
