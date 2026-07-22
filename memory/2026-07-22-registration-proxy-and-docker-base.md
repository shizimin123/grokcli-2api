# Debug Report: registration proxy and Docker base image

- Symptom: registration POST timed out while starting a TempMail-backed job; remote image rebuilds repeatedly downloaded browser assets.
- Root cause: TempMail and local Turnstile browser traffic did not receive the selected registration proxy. Stable OS, Python, Camoufox, and Chromium dependencies also shared the application image lifecycle.
- Fix: propagate the selected proxy through mailbox create/poll and local Turnstile task/browser context. Split the Dockerfile into a reusable `runtime-base` target and application stage, with a fingerprinted base build script.
- Evidence: remote base reuse check completed in 0.052s and application build in 3.326-4.108s. Registration POST returned HTTP 200 in 6.243s, solver logged `Proxy: configured`, and the session reached `imported`.
- Regression test: `tests/test_turnstile_proxy.py` verifies that the local Turnstile task carries the registration proxy.
- Status: DONE
