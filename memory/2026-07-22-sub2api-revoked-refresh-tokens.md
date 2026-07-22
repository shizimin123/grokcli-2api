# Debug Report: sub2api revoked Grok refresh tokens

- Symptom: 12 of the latest 50 Grok OAuth records entered `error` with `invalid_grant: Refresh token has been revoked` immediately after import.
- Root cause: 49 of the 50 emails already had older sub2api records. The failed new records reused the same stale refresh-token fingerprints as older records. Refresh-token rotation across duplicate records, previous authorizations, or another credential owner leaves copied credentials permanently invalid.
- Evidence: all 50 records have `proxy_id=1`, refresh tokens, and group 12. Direct `auth.x.ai` access timed out, while five proxy requests completed TLS and returned an OAuth response in 0.75-0.81 seconds. The same refresh cycle successfully refreshed 11 newly imported records but received explicit `invalid_grant` responses for 12 others.
- Assessment: the account identity is not necessarily unusable, but the rejected credential is. Retrying or reimporting the same refresh token cannot recover it; the account requires reauthorization and a newly issued credential.
- Fix: both Python automatic sync and Go manual sync now upsert by exact Grok account name. They prefer an active record with the freshest expiry, preserve newer remote credentials, update proxy/group metadata, recover error state only for newer credentials, and mark other visible duplicates inactive. Per-account locks prevent concurrent local imports from racing the query/create sequence.
- Evidence after fix: a live sync for an email with two visible records updated account 725, preserved its token fingerprint and expiry, marked account 670 inactive, created no new record, and returned `action=updated`, `credentials_updated=false`, `deduplicated=1`.
- Follow-up: use one token-refresh owner for each account. Existing revoked credentials still require reauthorization; deduplication cannot restore an already revoked token.
- Status: DONE
