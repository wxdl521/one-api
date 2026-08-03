# Agent Connect Fresh-Login Session Binding

Each forced Agent Connect login is bound to the session created by that login,
not merely to a timestamp. Starting reauthentication creates a short-lived,
HttpOnly, same-origin cookie containing the request ID and a random nonce. The
server stores only the nonce HMAC in the request record. The page then signs
out the current browser session before navigating to normal sign-in.

Every successful login path reaches `setupLogin`; when the signed cookie is
present, it atomically records the HMAC of the newly issued session ID on the
matching live request and clears the cookie. Options and authorize compare the
current server-validated session ID HMAC to that request field. A copied link,
an old session, a manually altered frontend state, or a normal login without
the reauthentication cookie cannot pass the comparison.

Reopening a link after a different session was bound begins a new nonce flow
and replaces the binding only after the new successful login. Cookie/nonce
values, session IDs, and API keys are never exposed in an API response or log.
