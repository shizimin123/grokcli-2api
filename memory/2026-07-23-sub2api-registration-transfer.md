# Debug Report: sub2api registration transfer

- Problem: registration-time sub2api push left the same refresh token in grokcli-2api and sub2api, allowing both maintainers to rotate one credential.
- Fix: automatic registration push now transfers ownership. Only per-account successful sub2api results are eligible for local deletion; failed pushes remain local.
- Ordering: deletion runs after CLIProxyAPI hooks, local model probing, and pool normalization so those post-registration steps can still read the new account.
- Concurrency: PostgreSQL cleanup deletes selected account IDs directly in one transaction instead of rewriting the full account map, preventing concurrent registrations from being removed accidentally.
- Result: registration session details include `cleaned`, `cleaned_ids`, `cleanup_failed`, per-row `local_deleted`, and `transferred_account_ids`.
- Scope: manual `{ "all": true }` sub2api import remains a copy operation.
- Status: DONE
