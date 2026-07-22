# Debug Report: sub2api selected account transfer

- Problem: selected Grok accounts were copied to sub2api but retained locally, so grokcli-2api and sub2api could independently rotate the same refresh token.
- Risk: after either system refreshes first, the other may receive `invalid_grant` because it still owns the previous refresh token.
- Fix: `POST /admin/api/accounts/push-sub2api` now treats an explicit selected-account request as a transfer. It intersects the requested IDs with per-account successful push results and deletes only those local accounts in one transaction.
- Safety: failed remote imports, IDs outside the request, and duplicate result rows are never deleted. Local cleanup failure is returned separately and marks the operation unsuccessful.
- Scope at implementation time: `{ "all": true }` remained a copy operation. Registration-time push was later upgraded to transfer mode as documented in `2026-07-23-sub2api-registration-transfer.md`.
- UI: the selected action is labeled as a transfer, displays a destructive confirmation, reports the local cleanup count, and refreshes the local account list.
- Status: DONE
