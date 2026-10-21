/**
 * In-memory session token holder for the authorized Registry API.
 *
 * The token is kept in a module-level variable only. It is deliberately never
 * written to `localStorage`, `sessionStorage`, cookies, the URL or IndexedDB,
 * so a browser refresh drops it and it cannot be read back by another tab or a
 * later session. This mirrors the ADR-0003 rule that secrets must not live in
 * browser storage; the token is an operator session credential, not a database
 * credential, but it is treated with the same care.
 */

let sessionToken = "";

/** Sets the token for the current in-memory session. An empty value clears it. */
export function setSessionToken(token: string): void {
  sessionToken = token.trim();
}

/** Clears the session token. */
export function clearSessionToken(): void {
  sessionToken = "";
}

/** Reports whether a session token is currently held. */
export function hasSessionToken(): boolean {
  return sessionToken !== "";
}

/**
 * Returns the current token for the typed client to attach as a bearer header.
 * It is not for display; the UI never re-renders a token value.
 */
export function getSessionToken(): string {
  return sessionToken;
}
