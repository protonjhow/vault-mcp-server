## Why

jinx-stack runs this server with an engineer's OpenBao OIDC token, which
has a 4-hour TTL. The server never renews it, so every 4 hours the tools
start answering `permission denied` and the engineer has to log in again,
re-store the token and reload the stack. The token is renewable
(`renewable: true`, `explicit_max_ttl: 0`, `creation_ttl: 14400`), so the
server can keep it alive itself, up to the ceiling OpenBao enforces.

## What Changes

- When the server creates a session's Vault client, it looks up its own
  token (`auth/token/lookup-self`). If the token is renewable, it starts
  the Vault API's `LifetimeWatcher`, which renews the token through
  `auth/token/renew-self` before it expires, each time asking for the
  token's creation TTL.
- A token that is not renewable, or a lookup the policy denies, changes
  nothing: one log line, then the server runs as before.
- When renewal stops (the ceiling is reached or a renewal fails), the
  server logs a warning naming the token's remaining lifetime. Tools keep
  working until the token expires, then answer Vault's
  `permission denied`, as today.
- The watcher stops when the session ends.
- No new configuration.

## Capabilities

### New Capabilities
- `token-renewal`: when and how the server renews its Vault token, what
  it does when renewal is impossible or stops, and when it stops renewing.

### Modified Capabilities

None. `openspec/specs/` has no specs yet.

## Impact

- `pkg/client/renew.go` (new) and `pkg/client/client.go` (start the
  watcher with the client, stop it on session end).
- `pkg/client/renew_test.go` (new).
- No new dependencies: `LifetimeWatcher` is in `github.com/hashicorp/vault/api`
  v1.21.0, already required.
- Every renewal is an `auth/token/renew-self` call, about every 2.7 hours
  for a 4-hour token; needs the `default` policy's renew-self capability.
- jinx-stack re-pins its `vault` server to the new commit afterwards; that
  is a separate change in jinx-stack.
