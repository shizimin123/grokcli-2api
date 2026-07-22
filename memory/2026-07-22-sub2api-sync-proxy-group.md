# Debug Report: sub2api sync proxy and group

- Symptom: accounts synced to sub2api had no `proxy_id` and were assigned to an unrelated existing group instead of a dedicated group.
- Root cause: both Python automatic sync and Go manual sync sent a null proxy. The Go path also fell back to the first remote group and never auto-created the configured/default group. Registration imports did not retain their selected proxy.
- Fix: persist the registration proxy on the local account, preserve it across token refreshes, map its protocol/host/port to a sub2api proxy ID, and pass that ID through OAuth and SSO conversion. Resolve groups by requested ID, configured ID, or configured/default name, then auto-create when enabled; never fall back to the first group.
- Evidence: remote integration test passed. A live sync created group `grokcli-2api` with ID 12 and account ID 669 with `proxy_id=1` and `group_ids=[12]`.
- Regression tests: `tests/test_sub2api_sync.py` and `internal/integrations/sub2api_export_test.go` cover proxy matching and default group creation.
- Status: DONE
