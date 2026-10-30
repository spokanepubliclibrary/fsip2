# Eureka: roles, capabilities and sessions

> **Status:** final for **Trillium** (platform-lsp `R1-2026-GA`). Capability names, role behaviour and the POST sequence were verified on a Trillium test tenant (2026-10-27). The Okapi equivalent is not yet verified on an Okapi tenant.
>
> The lists on this page are checked against `internal/folio/capabilities.go` by a test (`TestCapabilities_DocsMatchTable`). Change the table first, then this page.

- [Concepts](#concepts)
- [Role SIP2 Service (required)](#role-sip2-service-required)
- [Role SIP2 Rolling Renewal (optional)](#role-sip2-rolling-renewal-optional)
- [Creating and assigning the roles](#creating-and-assigning-the-roles)
- [Verifying](#verifying)
- [Troubleshooting 403s](#troubleshooting-403s)
- [Patron verification: PIN only on Eureka](#patron-verification-pin-only-on-eureka)
- [Okapi equivalent](#okapi-equivalent)
- [Keycloak sessions](#keycloak-sessions)

## Concepts
- **Capabilities** are Eureka's replacement for Okapi permissions. Each Okapi permission maps to one capability (`circulation.loans.collection.get` → `circulation_loans_collection.view`). They're granted through **roles** (mod-roles-keycloak), not assigned to users directly.
- fsip2 needs **two roles**:
  - **`SIP2 Service`** (required): everything for circulation, patron, fee and item lookups.
  - **`SIP2 Rolling Renewal`** (optional): lets fsip2 extend patron expiration dates. Assign it only where `rollingRenewals` is enabled, because it grants write access to user records.
- **Roles are additive.** `SIP2 Rolling Renewal` goes **in addition to** `SIP2 Service`, on the same user. Rolling renewal reuses the kiosk connection's own token.
- **Roles go on each kiosk's FOLIO user**, the account the device logs in with (93 Login CN/CO). If 8 AMH units share one username, that's one user and one role assignment.
- **Module-to-module calls:** on Trillium, the calls FOLIO modules make to each other while serving fsip2 weren't found to be authorized against the kiosk user. `SIP2 Service` still includes the eight permissions those modules use, as a precaution (see the second table below).
- The `/authn/login-with-expiry`, `/authn/refresh` and `/authn/logout` endpoints need **no** capability.

## Role SIP2 Service (required)

### Called directly by fsip2
<!-- fsip2-capabilities:service-direct -->
| Endpoint | Used by (SIP message) | Okapi permission | Eureka capability |
|---|---|---|---|
| POST /circulation/check-out-by-barcode | 11 checkout | `circulation.check-out-by-barcode.post` | `circulation_check-out-by-barcode.execute` |
| POST /circulation/check-in-by-barcode | 09 checkin | `circulation.check-in-by-barcode.post` | `circulation_check-in-by-barcode.execute` |
| POST /circulation/renew-by-barcode | 29 renew | `circulation.renew-by-barcode.post` | `circulation_renew-by-barcode.execute` |
| POST /circulation/renew-by-id | 65 renew all | `circulation.renew-by-id.post` | `circulation_renew-by-id.execute` |
| GET /circulation/loans | 63, 17, 65 | `circulation.loans.collection.get` | `circulation_loans_collection.view` |
| GET /circulation/requests | 63, 17, 09 | `circulation.requests.collection.get` | `circulation_requests_collection.view` |
| GET /accounts | 37, 63, 23 | `accounts.collection.get` | `accounts_collection.view` |
| POST /accounts/{id}/pay | 37 single payment | `feesfines.accounts.pay.post` | `feesfines_accounts_pay.execute` |
| POST /accounts-bulk/pay | 37 bulk payment | `feesfines.accounts-bulk.pay.post` | `feesfines_accounts-bulk_pay.execute` |
| GET /manualblocks | 63, 23 | `manualblocks.collection.get` | `manualblocks_collection.view` |
| GET /automated-patron-blocks/{id} | 63, 23 | `patron-blocks.automated-patron-blocks.collection.get` | `patron-blocks_automated-patron-blocks_collection.view` |
| GET /inventory/items | most messages | `inventory.items.collection.get` | `inventory_items_collection.view` |
| GET /inventory/items/{id} | most messages (barcode lookup, then by id); 63, 29, 65 loans | `inventory.items.item.get` | `inventory_items_item.view` |
| GET /inventory/instances/{id} | most messages | `inventory.instances.item.get` | `inventory_instances_item.view` |
| GET /holdings-storage/holdings/{id} | most messages | `inventory-storage.holdings.item.get` | `inventory-storage_holdings_item.view` |
| GET /locations/{id} | 09 | `inventory-storage.locations.item.get` | `inventory-storage_locations_item.view` |
| GET /material-types/{id} | 09 | `inventory-storage.material-types.item.get` | `inventory-storage_material-types_item.view` |
| GET /service-points/{id} | 09, 17 | `inventory-storage.service-points.item.get` | `inventory-storage_service-points_item.view` |
| GET /users | most messages | `users.collection.get` | `users_collection.view` |
| GET /users/{id} | 09, 17, 63, 23 | `users.item.get` | `users_item.view` |
| GET /groups/{id} | 63 patron group | `usergroups.item.get` | `usergroups_item.view` |
| POST /patron-pin/verify | PIN verification | `patron-pin.validate` | `patron-pin_validate.execute` |
<!-- /fsip2-capabilities -->

Note `patron-pin.validate` → `patron-pin_validate.execute`, not `patron-pin.execute` as the general naming rule would suggest.

### Used internally by FOLIO modules
<!-- fsip2-capabilities:service-transitive -->
| Okapi permission | Eureka capability | Evidence (Trillium module descriptors) |
|---|---|---|
| `manualblocks.item.get` | `manualblocks_item.view` | mod-circulation check-out, renew |
| `usergroups.collection.get` | `usergroups_collection.view` | mod-circulation check-out, check-in, loans, requests |
| `feefines.collection.get` | `feefines_collection.view` | mod-circulation check-in, renew |
| `configuration.entries.collection.get` | `configuration_entries_collection.view` | mod-login-keycloak login-with-expiry |
| `configuration.entries.item.get` | `configuration_entries_item.view` | none found |
| `accounts.item.get` | `accounts_item.view` | none found |
| `users-bl.item.get` | `users-bl_item.view` | none found |
| `search.instances.collection.get` | `search_instances_collection.view` | none found |
<!-- /fsip2-capabilities -->

These eight are **kept as a precaution; they weren't observed to be required on Trillium in tests without blocks, holds or fees.** Removing any one of them changed no SIP response on the test tenant, but the test patron had no blocks, holds or fees, so those internal paths were not exercised, and failures without them have been seen before. They match edge-sip2's `edge-sip.all` set.

## Role SIP2 Rolling Renewal (optional)
<!-- fsip2-capabilities:rolling-renewal -->
| Endpoint | Used by | Okapi permission | Eureka capability |
|---|---|---|---|
| GET /users/{id} | rolling renewal (re-fetch before update) | `users.item.get` | `users_item.view` |
| PUT /users/{id} | rolling renewal | `users.item.put` | `users_item.edit` |
<!-- /fsip2-capabilities -->

`users_item.view` repeats a `SIP2 Service` grant on purpose, so the role describes everything rolling renewal needs.

Without this role, rolling renewal **fails gracefully**: the patron's 63/23 response is normal, the expiration date is not changed, and fsip2 logs `Rolling renewal failed - permission denied` (WARN).

### Deliberately not granted
- `circulation.override-*` (for check-out and renew): would let kiosks bypass blocks.
- `users.basic-read.execute` / `users.restricted-read.execute`: no enforcement was found in mod-users. Under `SIP2 Service` the kiosk sees the same user fields (name, email, barcode, expiration date) as an admin.
- **Future** features. fsip2 has placeholders for these, but they make no FOLIO calls today:
  - SIP 15 hold create: `circulation_requests_item.create`.
  - Request cancel: `circulation_requests_item.view` and `circulation_requests_item.edit`.
  - SIP 19 item status: `inventory_items_item_mark-<status>.execute`.

## Creating and assigning the roles
All calls go **through the gateway** (the same URL fsip2 uses as `okapiUrl`) with these headers:
```
X-Okapi-Tenant: <tenant>
X-Okapi-Token: <admin token>      # a user allowed to manage authorization roles
Content-Type: application/json
```

**1. Create `SIP2 Service`**
```http
POST /roles
{ "name": "SIP2 Service",
  "description": "fsip2 SIP2 kiosk/AMH account: circulation, patron, fee and item lookups" }
```
→ `201`; note the returned `"id"` as `<roleA>`.

**2. Attach its capabilities**
<!-- fsip2-capabilities:post-service -->
```http
POST /roles/capabilities
{ "roleId": "<roleA>",
  "capabilityNames": [
    "circulation_check-out-by-barcode.execute",
    "circulation_check-in-by-barcode.execute",
    "circulation_renew-by-barcode.execute",
    "circulation_renew-by-id.execute",
    "circulation_loans_collection.view",
    "circulation_requests_collection.view",
    "accounts_collection.view",
    "feesfines_accounts_pay.execute",
    "feesfines_accounts-bulk_pay.execute",
    "manualblocks_collection.view",
    "patron-blocks_automated-patron-blocks_collection.view",
    "inventory_items_collection.view",
    "inventory_items_item.view",
    "inventory_instances_item.view",
    "inventory-storage_holdings_item.view",
    "inventory-storage_locations_item.view",
    "inventory-storage_material-types_item.view",
    "inventory-storage_service-points_item.view",
    "users_collection.view",
    "users_item.view",
    "usergroups_item.view",
    "patron-pin_validate.execute",
    "manualblocks_item.view",
    "usergroups_collection.view",
    "feefines_collection.view",
    "configuration_entries_collection.view",
    "configuration_entries_item.view",
    "accounts_item.view",
    "users-bl_item.view",
    "search_instances_collection.view"
  ] }
```
<!-- /fsip2-capabilities -->
→ `201`, with all 30 capabilities attached.

**3. Create `SIP2 Rolling Renewal` and attach its capabilities** (skip if rolling renewals are off)
```http
POST /roles
{ "name": "SIP2 Rolling Renewal",
  "description": "Optional add-on to SIP2 Service: lets fsip2 extend patron expiration dates (rolling renewals)" }
```
→ `201`; note the `"id"` as `<roleB>`.
<!-- fsip2-capabilities:post-rolling-renewal -->
```http
POST /roles/capabilities
{ "roleId": "<roleB>", "capabilityNames": [ "users_item.view", "users_item.edit" ] }
```
<!-- /fsip2-capabilities -->

**4. Assign the roles to each kiosk user**
```http
POST /roles/users/<kiosk user UUID>
{ "userId": "<kiosk user UUID>", "roleIds": [ "<roleA>", "<roleB>" ] }
```
Omit `<roleB>` if rolling renewals are off. This call is **additive**: a user who already has roles keeps them, so `SIP2 Rolling Renewal` can be added later with `"roleIds": ["<roleB>"]` alone.

**If a capability name is rejected:** look it up by its Okapi permission with `GET /capabilities?query=permission=="<okapi permission>"`, then send `"capabilityIds": [...]` instead of `capabilityNames`.

**Re-running step 2 is safe.** `POST /roles/capabilities` is additive, so a missed capability can be added to an existing role with a second POST that names only it.

## Verifying
```http
GET /roles?query=name=="SIP2 Service"                    # → the role and its id
GET /roles/<roleA>/capabilities?limit=100                # → 30 capabilities
GET /roles/<roleB>/capabilities                          # → users_item.view, users_item.edit
GET /roles/users?query=userId==<kiosk user UUID>         # → one row per role the kiosk user holds
```
Then from the device: a 93 Login, a 63 Patron Information and a 17 Item Information should all succeed.

## Troubleshooting 403s
Okapi's 403 names the permission (`Access requires permission: X`). Eureka's gateway does not: it returns `403 {"code":"authorization_error","message":"Access Denied"}`. fsip2 names the missing capability itself: on every 403 it looks the endpoint up in its capability table and logs one WARN line.
```
WARN  FOLIO denied access: the kiosk user is missing a permission
      method=GET path=/circulation/loans tenant=<tenant> mapped=true
      okapi_permission=circulation.loans.collection.get
      eureka_capability=circulation_loans_collection.view role="SIP2 Service"
```
The handler's own error log for that SIP message carries the same names, for example `permission denied on GET /circulation/loans: requires Eureka capability circulation_loans_collection.view (Okapi permission circulation.loans.collection.get) from role "SIP2 Service"`.

**To fix it:** add the named capability to the named role with step 2 above. `mapped=false` means the endpoint isn't in fsip2's table. Report it as an fsip2 bug.

At the kiosk, a 403 usually shows as FOLIO's own message (`Access Denied`) or the handler's generic failure text in the screen message (AF). Every 403 produces the WARN line above, so search the fsip2 log for `FOLIO denied access`.

## Patron verification: PIN only on Eureka
> **⚠ On Eureka, patron verification must use the PIN.** Password verification logs the patron in with `/authn/login-with-expiry`, and that is not permitted for patrons on Eureka. A tenant configured for password mode will reject every patron password.
>
> Failing configuration:
> ```yaml
> patronPasswordVerificationRequired: true
> usePinForPatronVerification: false
> ```
> Set `usePinForPatronVerification: true`.
>
> **Gotcha:** `usePinForPatronVerification` has **no default**. If you omit it while `patronPasswordVerificationRequired` is `true`, fsip2 silently uses password mode. Always set it explicitly.

Password verification still works on Okapi. fsip2 doesn't warn at startup, because it can't tell Okapi from Eureka from the configuration.

## Okapi equivalent
On Okapi (Poppy or later), grant the same permissions as two permission sets. *Not yet verified on an Okapi tenant.*

**1. Create the sets** (mod-permissions, `POST /perms/permissions`):
```http
POST /perms/permissions
{ "permissionName": "sip2.service", "displayName": "SIP2 Service", "mutable": true,
  "subPermissions":
```
<!-- fsip2-capabilities:okapi-service -->
```json
    [ "circulation.check-out-by-barcode.post", "circulation.check-in-by-barcode.post",
      "circulation.renew-by-barcode.post", "circulation.renew-by-id.post",
      "circulation.loans.collection.get", "circulation.requests.collection.get",
      "accounts.collection.get", "feesfines.accounts.pay.post", "feesfines.accounts-bulk.pay.post",
      "manualblocks.collection.get", "patron-blocks.automated-patron-blocks.collection.get",
      "inventory.items.collection.get", "inventory.items.item.get", "inventory.instances.item.get",
      "inventory-storage.holdings.item.get", "inventory-storage.locations.item.get",
      "inventory-storage.material-types.item.get", "inventory-storage.service-points.item.get",
      "users.collection.get", "users.item.get", "usergroups.item.get", "patron-pin.validate",
      "manualblocks.item.get", "usergroups.collection.get", "feefines.collection.get",
      "configuration.entries.collection.get", "configuration.entries.item.get",
      "accounts.item.get", "users-bl.item.get", "search.instances.collection.get" ] }
```
<!-- /fsip2-capabilities -->
```http
POST /perms/permissions
{ "permissionName": "sip2.rolling-renewal", "displayName": "SIP2 Rolling Renewal", "mutable": true,
  "subPermissions":
```
<!-- fsip2-capabilities:okapi-rolling-renewal -->
```json
    [ "users.item.get", "users.item.put" ] }
```
<!-- /fsip2-capabilities -->

**2. Assign them to the kiosk user's permissions user:**
```http
GET  /perms/users?query=userId==<kiosk user UUID>        # → <permsUserId>
POST /perms/users/<permsUserId>/permissions
{ "permissionName": "sip2.service" }
```
Repeat with `sip2.rolling-renewal` if rolling renewals are on.

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
