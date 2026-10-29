import { describe, expect, it } from "vitest";
import golden from "./graph-preview.toml?raw";
import { addInstance, emptyDraft, moveProcessor, setOption, type PipelineDraft } from "./pipelineDraft";
import { graphPreview } from "./pipelineToml";

function mustDraft(draft: PipelineDraft, kind: PipelineDraft["instances"][number]["kind"], plugin: string) {
  const added = addInstance(draft, kind, plugin);
  if ("error" in added) {
    throw new Error(added.error.message);
  }
  return added;
}

describe("graph TOML preview", () => {
  it("matches the Go fixture for the same graph", () => {
    let draft = emptyDraft();
    draft = mustDraft(draft, "input", "cpu").draft;
    draft = mustSet(draft, "cpu-1", "alias", { kind: "string", string: "edge-host" });
    const first = mustDraft(draft, "processor", "rename");
    draft = mustSet(first.draft, first.id, "replace", {
      kind: "rename_replace",
      replaces: [{ sourceKind: "tag", source: "hostname", dest: "host" }],
    });
    const second = mustDraft(draft, "processor", "rename");
    const moved = moveProcessor(second.draft, second.id, -1);
    if ("error" in moved) {
      throw new Error(moved.error.message);
    }
    draft = mustDraft(moved.draft, "output", "file").draft;

    const preview = graphPreview(draft);
    expect("toml" in preview).toBe(true);
    if (!("toml" in preview)) {
      return;
    }
    expect(preview.toml).toBe(golden);
    expect(graphPreview(draft)).toEqual(preview);
  });

  it("keeps layout and processor slice order out of the preview", () => {
    let draft = emptyDraft();
    draft = mustDraft(draft, "processor", "rename").draft;
    const second = mustDraft(draft, "processor", "rename");
    const moved = moveProcessor(second.draft, second.id, -1);
    if (!("draft" in moved)) {
      throw new Error("expected reorder");
    }
    const preview = graphPreview(moved.draft);
    if (!("toml" in preview)) {
      throw new Error("expected toml");
    }
    const renamed = preview.toml.indexOf('toolkit.instance = "rename-2"');
    const first = preview.toml.indexOf('toolkit.instance = "rename-1"');
    expect(renamed).toBeGreaterThan(-1);
    expect(renamed).toBeLessThan(first);
    expect(preview.toml).not.toContain("12345");
  });

  it("encodes a secret-store reference and refuses a hyphenated id", () => {
    let draft = emptyDraft();
    const added = mustDraft(draft, "output", "influxdb_v2");
    draft = mustSet(added.draft, added.id, "token", { kind: "secret_ref", secretRef: "cred_writer" });
    draft = mustSet(draft, added.id, "organization", { kind: "string", string: "toolkit" });
    draft = mustSet(draft, added.id, "bucket", { kind: "string", string: "metrics" });
    const preview = graphPreview(draft);
    if (!("toml" in preview)) {
      throw new Error("expected toml");
    }
    const referenceLine = preview.toml
      .split("\n")
      .find((item) => item.includes("@{toolkit:cred_writer}"));
    expect(referenceLine?.trim().startsWith("token")).toBe(true);
    expect(preview.toml).not.toContain("placeholder-literal");

    const hyphen = "cred-placeholder-literal";
    const rejected = graphPreview(
      mustSet(draft, added.id, "token", { kind: "secret_ref", secretRef: hyphen }),
    );
    expect(rejected).toEqual({
      issues: [
        {
          class: "config",
          message:
            "credential reference cannot be encoded as a Telegraf 1.40.1 secret-store reference",
        },
      ],
    });
    expect(JSON.stringify(rejected)).not.toContain(hyphen);
  });

  it("classifies a missing required value as config and an unknown plugin as support", () => {
    const added = mustDraft(emptyDraft(), "output", "influxdb_v2");
    const preview = graphPreview(added.draft);
    if (!("issues" in preview)) {
      throw new Error("expected issues");
    }
    expect(preview.issues.some((issue) => issue.class === "config" && issue.message.includes("Organization"))).toBe(
      true,
    );
    const unsupported = addInstance(emptyDraft(), "input", "kafka");
    expect(unsupported).toEqual({
      error: expect.objectContaining({ code: "unsupported-plugin" }),
    });
  });
});

function mustSet(
  draft: PipelineDraft,
  id: string,
  key: string,
  value: Parameters<typeof setOption>[3],
): PipelineDraft {
  const result = setOption(draft, id, key, value);
  if ("error" in result) {
    throw new Error(result.error.message);
  }
  return result.draft;
}
