# Debug Report: token maintainer refresh window and recovery

- Symptom: deployed accounts could reach about 18 minutes remaining without being refreshed; revoked refresh tokens stayed expired even when the account retained an SSO cookie.
- Root cause 1: PostgreSQL stored `token_refresh_skew_sec=300`, while the Go maintainer only read `token_maintain_enabled`. The persisted interval and refresh window were never applied at startup or after admin settings updates.
- Root cause 2: the Go default window was 180 seconds and the settings validators capped it at 1800 seconds, so a one-hour refresh window could not be configured.
- Root cause 3: a permanent OAuth `invalid_grant` marked `refresh_invalid` and excluded the account from future cycles. Accounts with SSO were retained but no SSO reauthorization was attempted.
- Root cause 4: every transient refresh error immediately changed `pool_status` to `expired`, regardless of token expiry or prior failures.
- Fix: default and deploy the refresh window as 3600 seconds, hot-apply durable interval/window settings, and expose a 30-7200 second settings range.
- Fix: permanently rejected refresh tokens now use the existing SSO import sidecar to mint and persist a new OAuth grant. Normal refresh candidates remain prioritized; historical invalid accounts are recovered in a separate small batch with a 15-minute failure backoff.
- Fix: transient errors now expire an account only after two consecutive failures or when the access token is actually expired. Permanent OAuth/SSO failures still expire immediately, and accounts without SSO are deleted as before.
- Proxy: both direct refresh and SSO reauthorization use the configured `GROK2API_XAI_PROXY`; the deployed container has the registration sidecar URL and xAI proxy configured.
- Verification: maintainer/store/server/cmd tests pass under Go 1.24. A live PostgreSQL integration test confirmed first transient failure = `normal/1` and second consecutive failure = `expired/2`.
- Status: DONE
