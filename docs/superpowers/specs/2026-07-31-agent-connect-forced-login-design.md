# Agent Connect Forced Login Design

## Goal

Every Agent Connect authorization link, including Hermes, MyAgents, and future
clients that use `agent_connect_requests`, must require a new The One login
before group/model options are displayed or a restricted token can be issued.

## Chosen approach

Entering `/agent-connect?request_id=...` invalidates the current browser login
session and clears its refresh cookie and client-side authentication state. The
user is redirected to `/sign-in` with the original, same-origin Agent Connect
URL as its redirect target. A password or 2FA login creates a fresh session;
only then may the authorization page load group/model options and confirm the
connection.

This is stricter than displaying the existing account or clearing only browser
state: an existing refresh cookie could otherwise silently recreate the prior
session. It intentionally signs the current browser out of The One, but does
not revoke other browser/device sessions for that user.

## Backend contract

Add a public, rate-limited Agent Connect reauthentication endpoint that accepts
only the opaque request ID. It validates that the request exists and has not
expired, then revokes the current authenticated browser session through the
existing session/logout machinery and clears the refresh cookie. It never
returns group, model, user, token, or request data.

The existing authenticated Agent Connect GET/authorize/cancel endpoints remain
the enforcement boundary. Therefore direct API calls cannot bypass the fresh
login page: the frontend cannot reach them after the reauthentication endpoint
has invalidated its session, and server-side authorization still checks the
current user and selected group/model.

## Frontend flow

When a page has a `request_id`, it invokes the reauthentication endpoint once
before loading connection options. It clears the local auth store and cached
user queries regardless of whether the browser had an active session, then
navigates to the normal sign-in route with the exact same-origin Agent Connect
path as `redirect`.

The reauthentication call is an intentional sign-out operation, not a normal
request retry: it must opt out of automatic refresh/auth retry so an old refresh
cookie cannot restore the session. The UI shows only a brief "Sign in to
continue" state; it must never render group/model choices before a new login.

## Failure handling

- An invalid or expired request shows the existing error state and does not
  sign out an unrelated browser session.
- A logout/session mismatch is handled by the existing logout recovery and
  still clears the browser cookie/client state.
- Repeated page loads are idempotent: after the first call no valid old session
  remains, but the sign-in redirect remains stable.
- Login/2FA remains entirely user-operated; no Skill accesses credentials.

## Tests and acceptance

- Controller/router tests verify the reauthentication route requires a valid
  unexpired request, clears the refresh cookie, and does not expose options or
  secrets.
- Frontend tests verify a connection URL calls reauthentication before the
  options query, clears local authentication, and redirects to sign-in with
  the exact connection URL.
- Existing Agent Connect authorization tests continue to prove that only an
  authenticated user with an allowed group/model can issue the 90-day token.
- Acceptance: opening a Hermes or MyAgents authorization URL while already
  logged in forces a new login; after login, only that newly authenticated
  account's groups and models are visible.
