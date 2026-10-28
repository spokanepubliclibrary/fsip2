# Eureka: capabilities and sessions

> **Status:** this file is being built up across the Eureka work. The **Keycloak sessions**
> section below is final (plan 01b). The roles and capabilities sections are added by plan 03.

## Keycloak sessions

### One session per SIP connection
- Each SIP connection logs in with its own 93 Login (`POST /authn/login-with-expiry`), and each login creates its **own** Keycloak session. Eight AMH units sharing one kiosk username hold eight sessions.
- Renewal uses `POST /authn/refresh`, which stays in the same session (the refresh token rotates). A full re-login is the fallback, and it creates a new session.
- Tokens are never shared between connections.

### When fsip2 ends a session
fsip2 calls `POST /authn/logout` for a connection's session:

| Event | Logged `reason` |
|---|---|
| The device closes the connection | `client_closed` |
| The idle read deadline (5 min) expires | `read_timeout` |
| Any other read or write error closes the connection | `error` |
| The server context is cancelled while the connection is between messages | `context_cancelled` |
| A 93 Login on an open connection replaces the session (expired token, or different credentials) | `relogin_93` |
| A 93 moves the connection to another tenant and its login fails | `tenant_changed` |
| A renewal falls back to re-login after a **transient** refresh failure (5xx, timeout, network) | `renewal_fallback` |

**Not ended:**
- **On fsip2 restart, deploy or pod rollout.** Sessions open at that moment expire on their own after the refresh-token lifetime (configured in each tenant).
- SIP 35 End Patron Session: it is patron-level; the kiosk session stays.
- After a refresh that returned 4xx: that session is already gone.

Each logout logs one line: `Keycloak logout accepted` (INFO), or `Keycloak logout skipped` / `rejected` (DEBUG), or `Keycloak logout failed` / `refused by guard` (WARN), with `reason`, `session_id`, `username` and `refreshed_first`. A 204 means the gateway accepted the logout; it is also returned for a session that had already ended, so it is not proof that a live session was just ended.

### How the logout is sent
- **Exactly this shape:** `folioRefreshToken` **and** `folioAccessToken` as cookies, **no** `X-Okapi-Token` header, to the gateway and tenant that **issued** the tokens.
- The access token must be current: the gateway answers **401** to an access token expired by more than ~30 s, and nothing is ended. Because SC Status (99) heartbeats never renew the token, it has usually expired by the time a connection closes, so fsip2 refreshes first and logs out with the fresh pair. A rejected refresh means the session is already gone, and no logout is sent.
- Logout is best-effort, bounded at 3 s, and never delays a SIP response (a 93 or renewal logout runs in the background; a close logs out after the socket is closed).
- `/authn/logout` needs no capability (`permissionsRequired: []`).

### ⚠ Never log a kiosk user out without its refresh token
On Eureka (mod-login-keycloak), a `/authn/logout` that carries an access token (header **or** cookie) with a **missing, empty or whitespace** refresh token ends **every** session of that user. For a kiosk username, that is every SIP device sharing it, and those devices then fail FOLIO calls with 401 until their tokens are renewed. `/authn/logout-all` does the same.

fsip2 refuses to send a logout unless the refresh token is non-blank and JWT-shaped (3 or 5 non-empty `.`-separated segments). **Admins:** never end a kiosk user's sessions by hand with `/authn/logout-all` or a token-only logout unless you intend to disconnect every device at the branch.

Behaviour measured on Trillium (test tenant, 2026-10-27/28):

| Sent to `/authn/logout` | Result |
|---|---|
| refresh cookie + access cookie (what fsip2 sends) | 204, only that session ends |
| refresh cookie only | 422, nothing ends |
| access token (header or cookie), no / blank / whitespace refresh | **204, every session of the user ends** |
| a rotated (stale) refresh + access | 204, that session's current tokens end; no other session |
| the refresh of an already-ended session + access | 204, nothing else ends |
| garbage (JWT-shaped) refresh + access | 422, nothing ends |
| access token expired ~5 min | 401, nothing ends |

### Okapi
Okapi Poppy+ (mod-login v7.10.0+) has `/authn/logout` too, and fsip2 sends the same request. Okapi has no Keycloak sessions; the logout revokes the refresh token. *Not yet verified on an Okapi tenant.*
