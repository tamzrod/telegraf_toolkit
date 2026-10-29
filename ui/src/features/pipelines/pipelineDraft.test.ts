import { describe, expect, it } from "vitest";
import { findPlugin, type PluginKind } from "./catalog";
import {
  addInstance,
  deleteInstance,
  emptyDraft,
  issues,
  moveLayout,
  moveProcessor,
  rejectDrawnConnection,
  semanticEdges,
  setOption,
  setSkip,
  type PipelineDraft,
} from "./pipelineDraft";

function add(draft: PipelineDraft, kind: PluginKind, plugin: string) {
  const result = addInstance(draft, kind, plugin);
  if ("error" in result) {
    throw new Error(result.error.message);
  }
  return result;
}

describe("pipeline draft graph", () => {
  it("rejects a plugin outside the verified catalog", () => {
    const result = addInstance(emptyDraft(), "input", "kafka");
    expect(result).toEqual({
      error: expect.objectContaining({ code: "unsupported-plugin" }),
    });
    expect(findPlugin("processor", "rename")?.options.some((option) => option.key === "order")).toBe(
      false,
    );
  });

  it("keeps canvas coordinates out of processor order", () => {
    let draft = emptyDraft();
    draft = add(draft, "processor", "rename").draft;
    draft = add(draft, "processor", "rename").draft;
    const before = [...draft.processorOrder];
    draft = moveLayout(draft, "rename-1", 900, 10);
    expect(draft.processorOrder).toEqual(before);
    expect(semanticEdges(draft).map((edge) => `${edge.source}->${edge.target}`)).toEqual([
      "rename-1->rename-2",
    ]);
  });

  it("reverses the execution edge when processor order changes", () => {
    let draft = emptyDraft();
    draft = add(draft, "processor", "rename").draft;
    draft = add(draft, "processor", "rename").draft;
    const moved = moveProcessor(draft, "rename-2", -1);
    expect("draft" in moved).toBe(true);
    if (!("draft" in moved)) {
      return;
    }
    expect(moved.draft.processorOrder).toEqual(["rename-2", "rename-1"]);
    expect(semanticEdges(moved.draft)[0]).toMatchObject({
      source: "rename-2",
      target: "rename-1",
      relation: "processor-order",
      label: "then",
    });
    expect(moveProcessor(moved.draft, "rename-2", -1)).toEqual({
      error: expect.objectContaining({ code: "invalid-order" }),
    });
  });

  it("does not invent an edge between independent inputs", () => {
    let draft = emptyDraft();
    draft = add(draft, "input", "cpu").draft;
    draft = add(draft, "input", "mem").draft;
    expect(semanticEdges(draft)).toEqual([]);
  });

  it("fans every aggregator out from the same source and does not chain them", () => {
    let draft = emptyDraft();
    draft = add(draft, "input", "cpu").draft;
    draft = add(draft, "aggregator", "basicstats").draft;
    draft = add(draft, "aggregator", "basicstats").draft;
    const edges = semanticEdges(draft);
    expect(edges.map((edge) => `${edge.label}: ${edge.source} to ${edge.target}`)).toEqual([
      "fan-out, keeps original: cpu-1 to basicstats-1",
      "fan-out, keeps original: cpu-1 to basicstats-2",
    ]);
    expect(edges.some((edge) => edge.source.startsWith("basicstats") && edge.target.startsWith("basicstats"))).toBe(
      false,
    );
  });

  it("drops the original only when every selected aggregator drops it", () => {
    let draft = emptyDraft();
    draft = add(draft, "processor", "rename").draft;
    draft = add(draft, "aggregator", "basicstats").draft;
    draft = add(draft, "aggregator", "basicstats").draft;
    draft = add(draft, "output", "file").draft;
    const dropping = setOption(draft, "basicstats-1", "drop_original", {
      kind: "bool",
      bool: true,
    });
    expect("draft" in dropping).toBe(true);
    if (!("draft" in dropping)) {
      return;
    }
    const labels = semanticEdges(dropping.draft).map(
      (edge) => `${edge.label}: ${edge.source} to ${edge.target}`,
    );
    expect(labels).toContain("fan-out, drops original: rename-1 to basicstats-1");
    expect(labels).toContain("fan-out, keeps original: rename-1 to basicstats-2");
    expect(labels).toContain("original continues: rename-1 to file-1");
    expect(labels).toContain("aggregate delivery: basicstats-1 to file-1");
    expect(labels).toContain("aggregate delivery: basicstats-2 to file-1");

    const both = setOption(dropping.draft, "basicstats-2", "drop_original", {
      kind: "bool",
      bool: true,
    });
    if (!("draft" in both)) {
      throw new Error("expected draft");
    }
    const dropped = semanticEdges(both.draft).map((edge) => edge.label);
    expect(dropped).not.toContain("original continues");
    expect(dropped).toContain("aggregate delivery");
  });

  it("rejects skipping both processor passes", () => {
    const once = setSkip(emptyDraft(), "skipProcessorsBeforeAggregators", true);
    expect("draft" in once).toBe(true);
    if (!("draft" in once)) {
      return;
    }
    expect(setSkip(once.draft, "skipProcessorsAfterAggregators", true)).toEqual({
      error: expect.objectContaining({
        code: "invalid-document",
        message: "Both processor passes cannot be skipped.",
      }),
    });
  });

  it("rejects embedded credentials without keeping the value", () => {
    const added = add(emptyDraft(), "output", "influxdb_v2");
    const literal = "user:placeholder-literal@127.0.0.1";
    const rejected = setOption(added.draft, added.id, "urls", {
      kind: "string_list",
      strings: [`http://${literal}:8086`],
    });
    expect(rejected).toEqual({
      error: expect.objectContaining({ code: "secret-material-rejected" }),
    });
    expect(JSON.stringify(rejected)).not.toContain("placeholder-literal");
    expect(added.id).toBe("influxdb-v2-1");
    expect(added.draft.instances[0]?.options.urls).toEqual({
      kind: "string_list",
      strings: ["http://127.0.0.1:8086"],
    });
  });

  it("rejects a non-id credential reference and accepts an id", () => {
    const added = add(emptyDraft(), "output", "influxdb_v2");
    const rejected = setOption(added.draft, added.id, "token", {
      kind: "secret_ref",
      secretRef: "not a token",
    });
    expect(rejected).toEqual({
      error: expect.objectContaining({ code: "secret-material-rejected" }),
    });
    expect(JSON.stringify(rejected)).not.toContain("not a token");
    const accepted = setOption(added.draft, added.id, "token", {
      kind: "secret_ref",
      secretRef: "cred-influx-writer",
    });
    expect(accepted).toEqual({
      draft: expect.objectContaining({
        instances: [
          expect.objectContaining({
            options: expect.objectContaining({
              token: { kind: "secret_ref", secretRef: "cred-influx-writer" },
            }),
          }),
        ],
      }),
    });
  });

  it("rejects the basicstats name variance and accepts s2", () => {
    const added = add(emptyDraft(), "aggregator", "basicstats");
    const variance = setOption(added.draft, added.id, "stats", {
      kind: "string_list",
      strings: ["variance"],
    });
    if (!("draft" in variance)) {
      throw new Error("expected stored list");
    }
    expect(issues(variance.draft).map((issue) => issue.message)).toContain(
      "variance is not an accepted value.",
    );
    const s2 = setOption(added.draft, added.id, "stats", {
      kind: "string_list",
      strings: ["s2"],
    });
    if (!("draft" in s2)) {
      throw new Error("expected stored list");
    }
    expect(issues(s2.draft).some((issue) => issue.message.includes("s2"))).toBe(false);
  });

  it("removes an instance from order and edges", () => {
    let draft = emptyDraft();
    draft = add(draft, "processor", "rename").draft;
    draft = add(draft, "processor", "rename").draft;
    draft = deleteInstance(draft, "rename-1");
    expect(draft.processorOrder).toEqual(["rename-2"]);
    expect(semanticEdges(draft)).toEqual([]);
  });

  it("names the drawn-connection rejection", () => {
    expect(rejectDrawnConnection()).toEqual({
      code: "invalid-relationship",
      message:
        "Drawn connections are rejected. Edges follow processor order and aggregator fan-out.",
    });
  });
});
