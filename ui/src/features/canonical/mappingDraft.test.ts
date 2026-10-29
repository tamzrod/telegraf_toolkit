import { describe, expect, it } from "vitest";
import {
  MAX_FILTER_VALUE_LENGTH,
  MAX_RULES,
  addFieldRule,
  addFilter,
  addTagRule,
  createEmptyDraft,
  isPlanReady,
  isValidDestinationName,
  parseFactor,
  planLines,
  removeFieldRule,
  updateFieldRule,
  updateFilter,
  validateDraft,
} from "./mappingDraft";
import type { Diagnostic, MappingDraft, SideView } from "./mappingDraft";

const SRC_ID = "src-id::db=metrics";
const DST_ID = "dst-id::db=archive";

function sourceView(overrides: Partial<SideView> = {}): SideView {
  return {
    identity: SRC_ID,
    measurementsState: "supported",
    measurements: ["sensor", "environment"],
    measurement: "sensor",
    fieldsState: "supported",
    fields: [
      { name: "temperature", type: "float" },
      { name: "value", type: "integer" },
      { name: "label", type: "string" },
      { name: "no_type", type: "" },
    ],
    tagsState: "supported",
    tags: ["host", "sensor_id"],
    ...overrides,
  };
}

function destinationView(overrides: Partial<SideView> = {}): SideView {
  return {
    identity: DST_ID,
    measurementsState: "supported",
    measurements: ["sensor_archive"],
    measurement: "sensor_archive",
    fieldsState: "supported",
    fields: [{ name: "temp_c", type: "float" }],
    tagsState: "supported",
    tags: ["hostname"],
    ...overrides,
  };
}

function baseDraft(overrides: Partial<MappingDraft> = {}): MappingDraft {
  return {
    ...createEmptyDraft(SRC_ID, DST_ID),
    sourceMeasurement: "sensor",
    destinationMeasurement: "sensor_archive",
    ...overrides,
  };
}

const CURRENT = { sourceIdentity: SRC_ID, destinationIdentity: DST_ID };

function codes(diagnostics: Diagnostic[], severity?: "error" | "warning"): string[] {
  return diagnostics
    .filter((d) => severity === undefined || d.severity === severity)
    .map((d) => d.code);
}

function validate(draft: MappingDraft, src = sourceView(), dst = destinationView()) {
  return validateDraft(draft, src, dst, CURRENT);
}

describe("destination names", () => {
  it("accepts strict identifiers", () => {
    expect(isValidDestinationName("temp_c")).toBe(true);
    expect(isValidDestinationName("_x1")).toBe(true);
  });

  it("rejects injection-shaped and malformed names", () => {
    for (const bad of [
      "",
      "1starts_with_digit",
      "a-b",
      "a.b",
      "a b",
      'x"; DROP MEASUREMENT m; --',
      "x' OR '1'='1",
      "SELECT * INTO other FROM m",
      "a\nb",
      "${inject}",
      "a".repeat(65),
    ]) {
      expect(isValidDestinationName(bad), bad).toBe(false);
    }
  });
});

describe("validateDraft", () => {
  it("reports an empty draft as incomplete with a warning and no errors", () => {
    const diagnostics = validateDraft(
      createEmptyDraft(SRC_ID, DST_ID),
      sourceView(),
      destinationView(),
      CURRENT,
    );
    expect(codes(diagnostics, "error")).toEqual([
      "source-measurement-missing",
      "destination-measurement-invalid",
    ]);
    expect(codes(diagnostics, "warning")).toContain("no-mappings");
  });

  it("refuses a field whose source type is not reported", () => {
    const draft = addFieldRule(baseDraft(), "no_type", "");
    expect(codes(validate(draft), "error")).toContain("source-type-unreported");
  });

  it("never coerces a type change silently", () => {
    let draft = addFieldRule(baseDraft(), "value", "integer");
    draft = updateFieldRule(draft, "field-1", { destinationType: "float" });
    expect(codes(validate(draft), "error")).toContain("type-mismatch");
  });

  it("accepts a matching type with no transform and no errors", () => {
    let draft = addFieldRule(baseDraft(), "temperature", "float");
    draft = updateFieldRule(draft, "field-1", { destinationField: "temp_c" });
    const diagnostics = validate(draft);
    expect(codes(diagnostics, "error")).toEqual([]);
    expect(isPlanReady(diagnostics)).toBe(true);
  });

  it("allows an explicit unit transform from a numeric source to a float destination", () => {
    let draft = addFieldRule(baseDraft(), "value", "integer");
    draft = updateFieldRule(draft, "field-1", {
      destinationField: "value_c",
      transform: { kind: "unit", factor: "0.5", fromUnit: "raw", toUnit: "celsius" },
    });
    draft = updateFieldRule(draft, "field-1", { destinationType: "float" });
    expect(codes(validate(draft), "error")).toEqual([]);
  });

  it("rejects a unit transform on a non-numeric source or non-float destination", () => {
    let draft = addFieldRule(baseDraft(), "label", "string");
    draft = updateFieldRule(draft, "field-1", {
      transform: { kind: "unit", factor: "2", fromUnit: "a", toUnit: "b" },
      destinationType: "float",
    });
    expect(codes(validate(draft), "error")).toContain("transform-type-unsupported");
  });

  it.each(["", "   ", "0", "NaN", "Infinity", "-Infinity", "1e999", "abc"])(
    "rejects factor %j",
    (factor) => {
      let draft = addFieldRule(baseDraft(), "temperature", "float");
      draft = updateFieldRule(draft, "field-1", {
        transform: { kind: "unit", factor, fromUnit: "c", toUnit: "f" },
      });
      expect(codes(validate(draft), "error")).toContain("transform-factor-invalid");
    },
  );

  it("requires both units to be declared and bounded", () => {
    let draft = addFieldRule(baseDraft(), "temperature", "float");
    draft = updateFieldRule(draft, "field-1", {
      transform: { kind: "unit", factor: "1", fromUnit: " ", toUnit: "x".repeat(33) },
    });
    expect(codes(validate(draft), "error")).toContain("transform-unit-invalid");
  });

  it("refuses a source field that is not observed", () => {
    const draft = addFieldRule(baseDraft(), "ghost", "float");
    expect(codes(validate(draft), "error")).toContain("source-field-not-observed");
  });

  it("refuses duplicate destination field names", () => {
    let draft = addFieldRule(baseDraft(), "temperature", "float");
    draft = addFieldRule(draft, "temperature", "float");
    draft = updateFieldRule(draft, "field-2", { destinationField: "temperature" });
    expect(codes(validate(draft), "error")).toContain("duplicate-destination-field");
  });

  it("refuses a destination field whose observed type conflicts", () => {
    let draft = addFieldRule(baseDraft(), "value", "integer");
    draft = updateFieldRule(draft, "field-1", { destinationField: "temp_c", destinationType: "integer" });
    expect(codes(validate(draft), "error")).toContain("destination-type-conflict");
  });

  it("refuses a destination name used as both a field and a tag", () => {
    let draft = addFieldRule(baseDraft(), "temperature", "float");
    draft = updateFieldRule(draft, "field-1", { destinationField: "shared" });
    draft = addTagRule(draft, "host");
    draft = { ...draft, tagRules: draft.tagRules.map((t) => ({ ...t, destinationTag: "shared" })) };
    expect(codes(validate(draft), "error")).toContain("destination-name-collision");
  });

  it("refuses duplicate destination tags and unobserved source tags", () => {
    let draft = addTagRule(baseDraft(), "host");
    draft = addTagRule(draft, "host");
    draft = { ...draft, tagRules: draft.tagRules.map((t) => ({ ...t, destinationTag: "same" })) };
    draft = addTagRule(draft, "ghost_tag");
    const found = codes(validate(draft), "error");
    expect(found).toContain("duplicate-destination-tag");
    expect(found).toContain("source-tag-not-observed");
  });

  it("refuses a measurement that is not observed on the source", () => {
    const draft = baseDraft({ sourceMeasurement: "missing" });
    expect(codes(validate(draft), "error")).toContain("source-measurement-not-observed");
  });

  it("blocks validation when a source schema is unknown rather than passing it", () => {
    const draft = addFieldRule(baseDraft(), "temperature", "float");
    const src = sourceView({ fieldsState: "unknown" });
    const diagnostics = validate(draft, src);
    expect(codes(diagnostics, "error")).toContain("source-schema-unknown");
    expect(isPlanReady(diagnostics)).toBe(false);
  });

  it("blocks validation while the source measurement is not yet loaded", () => {
    const draft = addFieldRule(baseDraft(), "temperature", "float");
    const src = sourceView({ measurement: "environment" });
    expect(codes(validate(draft, src), "error")).toContain("source-schema-not-loaded");
  });

  it("treats an unknown destination as a warning, not support", () => {
    const draft = addFieldRule(baseDraft(), "temperature", "float");
    const dst = destinationView({ measurementsState: "unknown", fieldsState: "unknown" });
    const diagnostics = validate(draft, sourceView(), dst);
    expect(codes(diagnostics, "warning")).toContain("destination-schema-unknown");
    expect(isPlanReady(diagnostics)).toBe(true);
  });

  it("flags a stale scope when either identity has changed", () => {
    const draft = baseDraft();
    const diagnostics = validateDraft(draft, sourceView(), destinationView(), {
      sourceIdentity: "src-id::db=other",
      destinationIdentity: DST_ID,
    });
    expect(codes(diagnostics, "error")).toContain("stale-scope");
  });

  it("refuses an invalid destination measurement and an injection-shaped one", () => {
    const draft = baseDraft({ destinationMeasurement: 'x"; DROP MEASUREMENT m; --' });
    expect(codes(validate(draft), "error")).toContain("destination-measurement-invalid");
  });

  it("warns about a new destination measurement without creating it", () => {
    const draft = baseDraft({ destinationMeasurement: "brand_new" });
    const diagnostics = validate(draft);
    expect(codes(diagnostics, "warning")).toContain("destination-measurement-new");
    expect(codes(diagnostics, "error")).not.toContain("destination-measurement-invalid");
  });

  it("refuses an invalid filter value and a value containing control characters", () => {
    let draft = addFilter(baseDraft(), "host");
    draft = updateFilter(draft, "filter-1", { value: "" });
    expect(codes(validate(draft), "error")).toContain("filter-value-invalid");

    draft = updateFilter(draft, "filter-1", { value: "a\nb" });
    expect(codes(validate(draft), "error")).toContain("filter-value-invalid");

    draft = updateFilter(draft, "filter-1", { value: "x".repeat(MAX_FILTER_VALUE_LENGTH + 1) });
    expect(codes(validate(draft), "error")).toContain("filter-value-invalid");
  });

  it("accepts injection-shaped filter text as data without error", () => {
    let draft = addFilter(baseDraft(), "host");
    draft = updateFilter(draft, "filter-1", { value: `"a' OR 1=1; DROP MEASUREMENT x; --" ` });
    const diagnostics = validate(draft);
    expect(codes(diagnostics, "error")).toEqual([]);
  });

  it("refuses a filter on an unobserved tag and conflicting values for one key", () => {
    let draft = addFilter(baseDraft(), "ghost_tag");
    draft = addFilter(draft, "host");
    draft = updateFilter(draft, "filter-2", { value: "alpha" });
    draft = addFilter(draft, "host");
    draft = updateFilter(draft, "filter-3", { value: "beta" });
    const found = codes(validate(draft), "error");
    expect(found).toContain("filter-tag-not-observed");
    expect(found).toContain("conflicting-filters");
  });

  it("is deterministic for identical inputs", () => {
    let draft = addFieldRule(baseDraft(), "value", "integer");
    draft = addTagRule(draft, "ghost");
    const a = validate(draft);
    const b = validate(draft);
    expect(a).toEqual(b);
  });

  it("caps rules per kind", () => {
    let draft = baseDraft();
    for (let i = 0; i < MAX_RULES + 1; i += 1) {
      draft = addFieldRule(draft, "temperature", "float");
    }
    expect(draft.fieldRules).toHaveLength(MAX_RULES);
  });
});

describe("parseFactor", () => {
  it("accepts finite non-zero numbers and rejects everything else", () => {
    expect(parseFactor(" 2.5 ")).toBe(2.5);
    expect(parseFactor("-1")).toBe(-1);
    expect(parseFactor("0")).toBeNull();
    expect(parseFactor("Infinity")).toBeNull();
    expect(parseFactor("")).toBeNull();
  });
});

describe("planLines", () => {
  it("quotes every name and value as literal data", () => {
    let draft = addFieldRule(baseDraft(), "temperature", "float");
    draft = updateFieldRule(draft, "field-1", { destinationField: "temp_c" });
    draft = addFilter(draft, "host");
    draft = updateFilter(draft, "filter-1", { value: "a'b" });
    const lines = planLines(draft);
    expect(lines[0]).toBe('Measurement "sensor" → "sensor_archive"');
    expect(lines).toContain('Field "temperature" → "temp_c" (float); no transform (types must match)');
    expect(lines.some((l) => l.includes('equals "a\'b" (value is data)'))).toBe(true);
  });

  it("removes a rule without touching others", () => {
    let draft = addFieldRule(baseDraft(), "temperature", "float");
    draft = addFieldRule(draft, "value", "integer");
    const after = removeFieldRule(draft, "field-1");
    expect(after.fieldRules.map((r) => r.sourceField)).toEqual(["value"]);
    expect(draft.fieldRules).toHaveLength(2);
  });
});
