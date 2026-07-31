# Agent Connect Forced Login Plan Amendment

The original plan is superseded where it describes a frontend-only logout gate.

1. Add nullable `ReauthenticationAfter *time.Time` to `AgentConnectRequest`,
   plus a transactionally locked `BeginAgentConnectReauthentication` operation
   that sets it only once for a live request.
2. Add `ErrAgentConnectReauthenticationRequired`. Controller options and
   authorization verify `middleware.GetSessionAuthIdentity` through
   `service.ValidateLoginSession` and require `UserSession.CreatedAt` strictly
   after the stored barrier.
3. The public reauthentication endpoint returns only
   `reauthentication_required`. It starts the barrier once, logs out an absent
   or pre-barrier session through `AuthLogout`, and leaves a post-barrier
   session active.
4. The frontend calls that endpoint before options. It redirects to sign-in
   only when `reauthentication_required` is true; on return from the new login,
   the server returns false and enables the normal options request.
5. Test the first barrier, idempotent repeat, old-session rejection,
   fresh-session acceptance, no old-token issuance, API response secrecy, and
   frontend gating before implementation.
