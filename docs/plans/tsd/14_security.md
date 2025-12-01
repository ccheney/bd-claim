## 14. Security & Compliance Considerations

### 14.1 Trust Model

* Intended for **local use** on a trusted developer machine or CI environment.
* No network-facing endpoints; attack surface is primarily:

  * Malicious arguments,
  * Malicious/unexpected DB content.

### 14.2 Auditability

* Every claim should be:

  * Visible in DB (`status` and `assignee` changes),
  * Optionally logged as structured log events.

* Fields to log for auditing:

  * `timestamp`,
  * `issueId`,
  * `agent`,
  * `old_status`, `new_status`,
  * `old_assignee`, `new_assignee`,
  * `filters`.
