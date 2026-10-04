Ground rules: the agent never runs `git commit` (it stages and hands the
operator a `git commit -S -F- <<'EOF'` heredoc after checking
`git status -s`), never prints or copies a token (lengths only), and calls
only list tools against the real OpenBao. Go commands run in
`golang:1.24.6-alpine@sha256:c8c5f95d64aa79b6547f3b626eb84b16a7ce18a139e3e9ca19a8c078b85ba80d`,
with the tree streamed in by `tar` (Colima shares only `$HOME`, and the
container needs no bind mount).

## 1. Renewal

- [x] 1.1 `pkg/client/renew_test.go` first, against an `httptest` fake OpenBao that serves `auth/token/lookup-self` and counts `auth/token/renew-self`: a renewable token with `ttl` and `creation_ttl` of 3 gets at least two renewals within 5 seconds; a non-renewable token gets none; a `lookup-self` 403 gets none and does not panic; calling the stop function ends renewals (no further calls in the next 3 seconds) and logs no "renewal stopped" warning. Run it: it fails to compile (no `startRenewal` yet)
- [x] 1.2 `pkg/client/renew.go`: `startRenewal(client *api.Client, logger *log.Logger) (stop func())`, as in design.md: lookup in a goroutine, info log and return when not renewable or the lookup fails, otherwise a `LifetimeWatcher` with `Increment` = `creation_ttl`, debug log on each renewal, warning on `DoneCh` unless stopped
- [x] 1.3 `pkg/client/client.go`: keep the stop function next to each session's client, start it in `NewVaultClient` after `SetToken`, call it in `DeleteVaultClient`
- [x] 1.4 `go vet ./...`, `go test ./pkg/...` pass (upstream's `e2e` package needs Docker and is not run); the 1.1 test passes
- [x] 1.5 `DOCKER_BUILDKIT=0 docker build -f Dockerfile.jinx .` succeeds; the image still lists 16 tools over stdio

## 2. Live check (jinx-stack sandbox, nothing in jinx-stack committed)

- [x] 2.1 Operator records the token's `expire_time` in their own terminal (`bao token lookup -format=json | jq -r .data.expire_time`), and sets `VAULT_TOKEN` in a sandbox store `/tmp/vault-renew/home` from `~/.vault-token`
- [x] 2.2 A scratch stack with the `vault` server as a gridctl `local` source on `~/git/forks/vault-mcp` (`dockerfile: Dockerfile.jinx`), applied on port 18180; the container log shows renewal on, then one successful renewal with the new TTL
- [x] 2.3 Operator re-runs the lookup: `expire_time` is later than the one recorded in 2.1; `vault__list_secrets` on `netops-kv-v2` still answers
- [x] 2.4 Tear down by file; trash `/tmp/vault-renew`

## 3. Hand over

- [x] 3.1 Stage `pkg/client/renew.go`, `pkg/client/renew_test.go`, `pkg/client/client.go` and `openspec/`, check staging, hand over the commit heredoc and `git push -u origin jinx-token-renew`
- [ ] 3.2 Report the pushed commit SHA for jinx-stack's re-pin (a separate jinx-stack change)
