# Agent Connect Forced Login Amendment

The reauthentication implementation must use a server-side timestamp barrier,
not a frontend-only logout marker. The first reauthentication request records
`reauthentication_after` once and logs out sessions created on or before it.
The options and authorize endpoints must accept only a live dashboard session
whose `UserSession.CreatedAt` is strictly after this timestamp. Repeated page
loads retain the timestamp and do not log out a fresh session.
