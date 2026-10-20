import { describe, expect, it } from "vitest";
import { isVerified, stateLabel, type SyncState } from "./syncState";

describe("COUNT_MATCH versus VERIFIED", () => {
  it("never treats a count match as verified", () => {
    expect(isVerified("COUNT_MATCH", "count-only")).toBe(false);
    expect(isVerified("COUNT_MATCH", "point-level")).toBe(false);
    expect(isVerified("COUNT_MATCH", "canonical-digest")).toBe(false);
  });

  it("requires a stronger policy before claiming verification", () => {
    expect(isVerified("VERIFIED", "count-only")).toBe(false);
    expect(isVerified("VERIFIED", "point-level")).toBe(true);
    expect(isVerified("VERIFIED", "canonical-digest")).toBe(true);
  });

  it("keeps count-match and verified labels distinct", () => {
    expect(stateLabel("COUNT_MATCH")).not.toBe(stateLabel("VERIFIED"));
    expect(stateLabel("COUNT_MATCH")).toMatch(/not verified/i);
  });

  it("does not claim verification for non-terminal states", () => {
    const nonVerified: SyncState[] = [
      "PENDING",
      "SCANNING",
      "DIRTY",
      "MISSING",
      "ERROR",
      "QUEUED",
      "COPYING",
      "VERIFYING",
      "NEEDS_REVIEW",
    ];
    for (const state of nonVerified) {
      expect(isVerified(state, "canonical-digest")).toBe(false);
    }
  });
});
