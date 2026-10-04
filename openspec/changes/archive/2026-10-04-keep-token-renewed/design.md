## Context

This fork (`protonjhow/vault-mcp-server`, branch `jinx` = upstream
`v0.2.0` plus a KV mount lookup) runs in jinx-stack's gridctl gateway over
stdio, with `VAULT_TOKEN` set to an engineer's OpenBao OIDC token. That
token, looked up on 2026-10-04: `renewable: true`, `creation_ttl: 14400`
(4 hours), `explicit_max_ttl: 0`, `period: null`, `policies: ["default"]`,
path `auth/oidc/oidc/callback`. Its effective maximum lifetime is set by
the OIDC role's `token_max_ttl`, else the auth mount's `max_lease_ttl`,
else OpenBao's system maximum (32 days by default); none of these is
readable with the token's policy.

The server creates one `*api.Client` per MCP session in
`pkg/client/client.go` (`CreateVaultClientForSession`, stored in a
`sync.Map`), deletes it in `EndSessionHandler`, and never renews the token.
The session hooks are registered for both transports
(`cmd/vault-mcp-server/main.go:255-263`), so under stdio there is one
session, created at `initialize`, for the life of the process.

`github.com/hashicorp/vault/api` v1.21.0 (already in `go.mod`) has
`LifetimeWatcher` (`api/lifetime_watcher.go`): given a `*api.Secret` with
an `Auth` block it renews through `Auth().Token().RenewTokenAsSelf`,
renews once on `Start`, then sleeps two thirds of the remaining lease plus
jitter. `DoneCh()` yields an error when a renewal fails (after its retry
backoff), and `nil` when the lease stops being extended and the remaining
time is inside its grace period, or when `Stop()` is called.

## Goals / Non-Goals

**Goals:**
- A renewable token stays valid for as long as the server runs, up to the
  ceiling OpenBao enforces.
- Nothing changes for tokens that cannot be renewed, or policies that deny
  the lookup.
- A log line says when renewal is on, and a warning says when it stopped.

**Non-Goals:**
- Getting past the ceiling: that needs a new login (OIDC needs a browser).
- Treating HTTP mode differently: renewal attaches to each session's
  client in both transports; jinx-stack uses stdio only.
- Writing a renewed token anywhere: renewal keeps the token string, so the
  copy in gridctl's store stays valid.
- Configuration knobs.

## Decisions

### Use the library's LifetimeWatcher
It already implements renew-before-expiry with jitter, retry backoff and
ceiling detection. Alternative: a hand-written ticker calling
`RenewSelf` (re-implements all of that, worse).

### Look up the token, then build the watcher's secret
`Auth().Token().LookupSelf()` returns the token's data, not an `Auth`
block, so the code builds
`&api.Secret{Auth: &api.SecretAuth{ClientToken, Renewable, LeaseDuration: ttl}}`
from `renewable`, `ttl` and `creation_ttl`, with `Increment` set to
`creation_ttl` (so each renewal asks for the original 4 hours, not more).
Alternative: skip the lookup and call `renew-self` blindly (an extra
failing call per session for non-renewable tokens, and no TTL to log).

### Start with the client, stop with the session
`pkg/client/renew.go` holds one function,
`startRenewal(client *api.Client, logger) (stop func())`, called from
`NewVaultClient` after `SetToken`. The stop function is stored next to the
client and called by `DeleteVaultClient`. A `stopped` flag set by the stop
function tells a deliberate stop apart from the watcher's own `nil` on
`DoneCh`, so only the latter logs "renewal stopped".

### Renewal never blocks the server
The lookup and the watcher run in a goroutine; a failed lookup or a
non-renewable token logs once at info level and returns. Tool calls never
wait on renewal.

### No configuration
A renewable token is renewed. Upstream may want an opt-out
(`VAULT_TOKEN_RENEW=false`); it can be added if the change is offered
there.

## Risks / Trade-offs

- [Each server start renews once immediately, so every gridctl reload
  resets the token's TTL to 4 hours from then] → intended; it only extends
  the token, within the ceiling.
- [The ceiling is unknown; it may be well below "forever"] → the warning
  at stop says so; the live check measures the first extension, not the
  ceiling.
- [Renewal keeps a token alive for as long as the laptop runs the stack,
  which widens the window if the laptop is compromised] → the same token
  could be renewed by hand today; revoking it in OpenBao still ends it, and
  the watcher then logs the failure.
- [`renew-self` denied by a future policy without `default`] → the first
  renewal fails, the warning logs it, behaviour falls back to today's.
- [Several sessions in HTTP mode each renew the same env token] → harmless
  extra renewals; out of scope for jinx-stack's stdio use.

## Migration Plan

Commit on `jinx-token-renew`, the operator pushes it; jinx-stack re-pins
its `vault` server's `ref` to the new commit (separate change). Rollback:
re-pin to `75b09a9`.

## Open Questions

None blocking. The effective ceiling stays unknown until a token reaches
it.
