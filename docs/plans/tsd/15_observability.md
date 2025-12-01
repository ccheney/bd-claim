## 15. Observability (Logging)

### 15.1 Logging

* **Format:** JSON logs to stderr by default (unless `--quiet`).
* **Log Levels:** `ERROR`, `WARN`, `INFO`, `DEBUG`.
* **Key log events:**

  * `issue_claim_success`:

    * Fields: `issueId`, `agent`, `filters`, `duration_ms`.
  * `issue_claim_no_issue`:

    * Fields: `agent`, `filters`, `duration_ms`.
  * `issue_claim_error`:

    * Fields: `agent?`, `error.code`, `error.message`, `duration_ms`.
  * `workspace_discovery`:

    * Fields: `cwd`, `workspace_root`, `db_path`.


