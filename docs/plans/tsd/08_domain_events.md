## 8. Domain Events

The Issue Claiming context defines domain events that represent notable outcomes. In the current CLI implementation, these are primarily:

1. **IssueClaimed**

   * **When:** An issue has been successfully claimed.
   * **Payload:**

     * `issueId: IssueId`
     * `agent: AgentName`
     * `claimedAt: Timestamp`
   * **Uses:**

     * Structured logging,
     * Metrics (`claims_success_total`),
     * Future integration with orchestration/monitoring systems.

2. **NoIssueAvailable**

   * **When:** No eligible issue is found for the given filters.
   * **Payload:**

     * `agent: AgentName`
     * `filters: ClaimFilters`
     * `checkedAt: Timestamp`
   * **Uses:**

     * Logging to understand idling behavior,
     * Metrics for utilization.

3. **ClaimFailed**

   * **When:** Claim attempt fails due to technical reasons:

     * DB connection failure,
     * SQLite busy timeout exceeded,
     * Beads workspace not found,
     * Schema incompatibility.
   * **Payload:**

     * `agent: Option<AgentName>`
     * `errorCode: ClaimErrorCode`
     * `errorMessage: String`
     * `occurredAt: Timestamp`
   * **Uses:**

     * Error logging,
     * Metrics (`claims_error_total`),
     * Alerting in automation.
