## 4. High-Level Architecture (Hexagonal)

Architecture style: **Hexagonal (Ports & Adapters)**

### 4.1 Layers

1. **Domain Layer**

   * Pure business logic.
   * Contains:

     * Aggregates: `Issue`.
     * Value Objects: `IssueId`, `AgentName`, `IssueStatus`, `LabelSet`, `Priority`, `ClaimFilters`, `ReadySpecification`.
     * Domain Services: `ClaimIssueService`, `ReadyIssueSelector`.
     * Domain Events: `IssueClaimed`, `NoIssueAvailable`, `ClaimFailed`.

2. **Application Layer**

   * Orchestrates use cases.
   * Contains:

     * Use Case Handlers: `ClaimIssueUseCase`.
     * DTOs for request/response: `ClaimIssueRequest`, `ClaimIssueResult`.
     * Port interfaces:

       * `IssueRepositoryPort`
       * `WorkspaceDiscoveryPort`
       * `ClockPort`
       * `LoggerPort`
       * `MetricsPort`

3. **Infrastructure Layer**

   * Adapters for DB, filesystem, CLI IO, logging, metrics.
   * Components:

     * `SQLiteIssueRepositoryAdapter`
     * `SQLiteUnitOfWork`
     * `WorkspaceDiscoveryAdapter` (locates `.beads/beads.db`)
     * `CLIAdapter` (argument parsing, stdout/stderr, exit codes)
     * `LoggingAdapter`
     * `MetricsAdapter`
     * `ConfigAdapter` (env vars, flags)

4. **Interface Layer (CLI Frontend)**

   * Thin wrapper around Application layer.
   * Responsible for:

     * CLI flag parsing,
     * JSON serialization/deserialization,
     * Mapping CLI exit codes to `status` fields.

### 4.2 Key Architectural Decisions

* **Single Responsibility per Invocation**

  * Each `bd-claim` process handles exactly one claim attempt then exits.
  * No daemonization; keeps failure modes simple and predictable.

* **Transaction Per Request**

  * 1 claim attempt = 1 DB transaction.
  * Aligns naturally with CLI lifetime.

* **Optimistic + Pessimistic Concurrency**

  * Primary: use SQL `UPDATE … WHERE predicate LIMIT 1` inside a transaction for atomicity.
  * SQLite locking ensures only one updater at a time.
  * Retry/backoff logic for busy DB situations.
