/**
 * Synchronization block state vocabulary (test-only mirror of the backend
 * contract). COUNT_MATCH must never be presented as VERIFIED.
 */

export type SyncState =
  | "PENDING"
  | "SCANNING"
  | "COUNT_MATCH"
  | "DIRTY"
  | "MISSING"
  | "ERROR"
  | "QUEUED"
  | "COPYING"
  | "VERIFYING"
  | "VERIFIED"
  | "NEEDS_REVIEW";

export type VerificationPolicy =
  | "count-only"
  | "point-level"
  | "canonical-digest";

export function isVerified(state: SyncState, policy: VerificationPolicy): boolean {
  if (state !== "VERIFIED") {
    return false;
  }
  return policy === "point-level" || policy === "canonical-digest";
}

export function stateLabel(state: SyncState): string {
  switch (state) {
    case "COUNT_MATCH":
      return "Count matched (not verified)";
    case "VERIFIED":
      return "Verified";
    case "NEEDS_REVIEW":
      return "Needs review";
    case "MISSING":
      return "Missing";
    case "DIRTY":
      return "Differs";
    default:
      return state;
  }
}
