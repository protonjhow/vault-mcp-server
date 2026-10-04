## ADDED Requirements

### Requirement: A renewable token is renewed before it expires
When the server creates a Vault client for a session, it SHALL look up its
token with `auth/token/lookup-self`. If the token is renewable, the server
SHALL renew it with `auth/token/renew-self` before it expires, requesting
an increment equal to the token's `creation_ttl`, for as long as the
session lasts and OpenBao keeps extending it.

#### Scenario: Renewable token
- **WHEN** a session starts with a token whose lookup reports `renewable: true` and a TTL of 3 seconds
- **THEN** the server calls `auth/token/renew-self` within 3 seconds, and again before each new TTL runs out

#### Scenario: Token outlives its original TTL
- **WHEN** the server has run longer than the token's original TTL with renewal active
- **THEN** a `bao token lookup` of that token shows an `expire_time` later than the one it had at issue

### Requirement: A token that cannot be renewed is left alone
If the lookup reports `renewable: false`, or the lookup itself fails, the
server SHALL log one line saying renewal is off and why, SHALL NOT call
`auth/token/renew-self`, and SHALL create the client exactly as before.

#### Scenario: Non-renewable token
- **WHEN** a session starts with a token whose lookup reports `renewable: false`
- **THEN** no `renew-self` call is made and tool calls work as they did before this change

#### Scenario: Lookup denied
- **WHEN** a session starts and `auth/token/lookup-self` answers `403`
- **THEN** the client is still created, the server logs that renewal is off, and tool calls work as before

### Requirement: The server reports when renewal stops
When renewal stops for any reason other than the session ending (OpenBao
no longer extends the token, or a renewal fails), the server SHALL log a
warning that says renewal stopped, the reason, and that the token will
expire.

#### Scenario: Renewal fails
- **WHEN** a `renew-self` call fails and the watcher gives up
- **THEN** the server logs a warning containing the error, and keeps serving tool calls with the current token

#### Scenario: Ceiling reached
- **WHEN** renewals stop extending the token (OpenBao caps each grant at the token's maximum lifetime) and its remaining lifetime falls inside the watcher's grace period
- **THEN** the watcher stops, the server logs a warning that the token has reached its maximum lifetime, and no further `renew-self` calls are made

### Requirement: Renewal ends with the session
When a session ends, the server SHALL stop that session's renewal along
with its client, and SHALL NOT log the stop as a renewal failure.

#### Scenario: Session ends
- **WHEN** the session that started renewal ends
- **THEN** no further `renew-self` calls are made for its token and no renewal-stopped warning is logged
