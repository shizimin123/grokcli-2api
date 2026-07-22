# Debug Report: Grok inference proxy

- Symptom: local clients received HTTP 502 with a direct dial timeout while posting to `cli-chat-proxy.grok.com/v1/responses`.
- Root cause: the Go inference and model-health transports only used `http.ProxyFromEnvironment`, while production configured `GROK2API_XAI_PROXY`. Account selection also did not alter this transport.
- Fix: resolve `GROK2API_XAI_PROXY` and `GROK2API_PROXY` before standard proxy variables for Grok inference and model-health requests. Invalid custom proxy URLs now fail closed.
- Evidence: direct access from the production container timed out after 5 seconds; the configured proxy reached the upstream in 0.8 seconds. The proxy regression tests and all affected Go packages pass.
- Regression test: `internal/upstream/grok/client_test.go` verifies custom proxy precedence and invalid configuration handling.
- Related: commit `a7f9594` added standard `HTTP(S)_PROXY` support but did not cover the application's custom proxy variables.
- Status: DONE
