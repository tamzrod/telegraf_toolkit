import type { CapabilityOutcome } from "../../api/registry";

/**
 * Local, in-memory mapping draft model for the Canonical Mapper (P3-004).
 *
 * A draft is a *plan* over one source measurement and one destination
 * measurement. It holds names and references only: no endpoint, no credential,
 * no query text and no write. Every string here is data. Nothing in this module
 * builds, concatenates or executes a query; the plan preview renders names as
 * quoted literal text.
 *
 * Validation never coerces silently. A type change needs an explicit unit
 * transform; an unknown or unobserved schema blocks validation with a
 * diagnostic instead of being treated as supported.
 */

export const FIELD_TYPES = ["float", "integer", "unsigned", "string", "boolean"] as const;
export type FieldType = (typeof FIELD_TYPES)[number];

const NUMERIC_TYPES: ReadonlySet<string> = new Set(["float", "integer", "unsigned"]);

/** Destination names are operator-chosen, so they use a strict identifier form. */
export const DESTINATION_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/;
export const MAX_RULES = 100;
export const MAX_FILTER_VALUE_LENGTH = 256;
export const MAX_UNIT_LENGTH = 32;

/** Schema state of one side for the selected measurement, as seen by the browser. */
export type SideLoadState = CapabilityOutcome | "unloaded";

export type SideView = {
  /** Profile identity (connection ID and canonical scope) the view was read for. */
  identity: string;
  measurementsState: SideLoadState;
  measurements: string[];
  /** Measurement the field and tag states below belong to ("" if none). */
  measurement: string;
  fieldsState: SideLoadState;
  fields: { name: string; type: string }[];
  tagsState: SideLoadState;
  tags: string[];
};

export type Transform =
  | { kind: "none" }
  | { kind: "unit"; factor: string; fromUnit: string; toUnit: string };

export type FieldRule = {
  id: string;
  sourceField: string;
  destinationField: string;
  destinationType: string;
  transform: Transform;
};

export type TagRule = {
  id: string;
  sourceTag: string;
  destinationTag: string;
};

export type TagFilter = {
  id: string;
  tagKey: string;
  value: string;
};

export type MappingDraft = {
  sourceIdentity: string;
  destinationIdentity: string;
  sourceMeasurement: string;
  destinationMeasurement: string;
  fieldRules: FieldRule[];
  tagRules: TagRule[];
  filters: TagFilter[];
};

export type Diagnostic = {
  severity: "error" | "warning";
  code: string;
  subject: string;
  message: string;
};

export function createEmptyDraft(
  sourceIdentity: string,
  destinationIdentity: string,
): MappingDraft {
  return {
    sourceIdentity,
    destinationIdentity,
    sourceMeasurement: "",
    destinationMeasurement: "",
    fieldRules: [],
    tagRules: [],
    filters: [],
  };
}

function nextId(prefix: string, existing: readonly { id: string }[]): string {
  const used = new Set(existing.map((row) => row.id));
  let counter = existing.length + 1;
  let candidate = `${prefix}${counter}`;
  while (used.has(candidate)) {
    counter += 1;
    candidate = `${prefix}${counter}`;
  }
  return candidate;
}

/** A field's observed type is a valid starting point only if it is a known type. */
export function defaultDestinationType(sourceType: string): string {
  return (FIELD_TYPES as readonly string[]).includes(sourceType) ? sourceType : "string";
}

export function addFieldRule(
  draft: MappingDraft,
  sourceField: string,
  sourceType: string,
): MappingDraft {
  if (draft.fieldRules.length >= MAX_RULES) {
    return draft;
  }
  const rule: FieldRule = {
    id: nextId("field-", draft.fieldRules),
    sourceField,
    destinationField: sourceField,
    destinationType: defaultDestinationType(sourceType),
    transform: { kind: "none" },
  };
  return { ...draft, fieldRules: [...draft.fieldRules, rule] };
}

export function updateFieldRule(
  draft: MappingDraft,
  id: string,
  patch: Partial<Omit<FieldRule, "id">>,
): MappingDraft {
  return {
    ...draft,
    fieldRules: draft.fieldRules.map((rule) =>
      rule.id === id ? { ...rule, ...patch } : rule,
    ),
  };
}

export function removeFieldRule(draft: MappingDraft, id: string): MappingDraft {
  return { ...draft, fieldRules: draft.fieldRules.filter((rule) => rule.id !== id) };
}

export function addTagRule(draft: MappingDraft, sourceTag: string): MappingDraft {
  if (draft.tagRules.length >= MAX_RULES) {
    return draft;
  }
  const rule: TagRule = {
    id: nextId("tag-", draft.tagRules),
    sourceTag,
    destinationTag: sourceTag,
  };
  return { ...draft, tagRules: [...draft.tagRules, rule] };
}

export function updateTagRule(
  draft: MappingDraft,
  id: string,
  patch: Partial<Omit<TagRule, "id">>,
): MappingDraft {
  return {
    ...draft,
    tagRules: draft.tagRules.map((rule) => (rule.id === id ? { ...rule, ...patch } : rule)),
  };
}

export function removeTagRule(draft: MappingDraft, id: string): MappingDraft {
  return { ...draft, tagRules: draft.tagRules.filter((rule) => rule.id !== id) };
}

export function addFilter(draft: MappingDraft, tagKey: string): MappingDraft {
  if (draft.filters.length >= MAX_RULES) {
    return draft;
  }
  const filter: TagFilter = { id: nextId("filter-", draft.filters), tagKey, value: "" };
  return { ...draft, filters: [...draft.filters, filter] };
}

export function updateFilter(
  draft: MappingDraft,
  id: string,
  patch: Partial<Omit<TagFilter, "id">>,
): MappingDraft {
  return {
    ...draft,
    filters: draft.filters.map((filter) => (filter.id === id ? { ...filter, ...patch } : filter)),
  };
}

export function removeFilter(draft: MappingDraft, id: string): MappingDraft {
  return { ...draft, filters: draft.filters.filter((filter) => filter.id !== id) };
}

export function isValidDestinationName(name: string): boolean {
  return DESTINATION_NAME_PATTERN.test(name);
}

/** Parses an operator-entered factor. Only a finite, non-zero number is accepted. */
export function parseFactor(factor: string): number | null {
  const trimmed = factor.trim();
  if (trimmed === "") {
    return null;
  }
  const value = Number(trimmed);
  if (!Number.isFinite(value) || value === 0) {
    return null;
  }
  return value;
}

function hasControlCharacter(value: string): boolean {
  // eslint-disable-next-line no-control-regex
  return /[\u0000-\u001f\u007f]/.test(value);
}

/**
 * Validates a draft against the observed source and destination schema.
 *
 * Deterministic: the same inputs always yield the same diagnostics in the same
 * order. An observed name that is absent is an error; a state that is unknown,
 * unloaded or unsupported is an error for the rule that depends on it, never a
 * pass. A warning never blocks the plan but is always shown.
 */
export function validateDraft(
  draft: MappingDraft,
  source: SideView,
  destination: SideView,
  current: { sourceIdentity: string; destinationIdentity: string },
): Diagnostic[] {
  const out: Diagnostic[] = [];
  const seen = new Set<string>();
  const add = (d: Diagnostic) => {
    const key = `${d.severity}|${d.code}|${d.subject}`;
    if (!seen.has(key)) {
      seen.add(key);
      out.push(d);
    }
  };

  if (
    draft.sourceIdentity !== current.sourceIdentity ||
    draft.destinationIdentity !== current.destinationIdentity
  ) {
    add({
      severity: "error",
      code: "stale-scope",
      subject: "draft",
      message:
        "The selected profile or canonical scope changed after this draft was created. Revalidate or discard the draft.",
    });
  }

  if (draft.fieldRules.length > MAX_RULES || draft.tagRules.length > MAX_RULES || draft.filters.length > MAX_RULES) {
    add({
      severity: "error",
      code: "too-many-rules",
      subject: "draft",
      message: `A draft may hold at most ${MAX_RULES} rules per kind.`,
    });
  }

  // Source measurement.
  if (draft.sourceMeasurement === "") {
    add({
      severity: "error",
      code: "source-measurement-missing",
      subject: "source",
      message: "Select a source measurement.",
    });
  } else if (source.measurementsState !== "supported") {
    add({
      severity: "error",
      code: "source-schema-unknown",
      subject: "source",
      message: `The source measurement list is ${source.measurementsState}; the draft cannot be validated against it.`,
    });
  } else if (!source.measurements.includes(draft.sourceMeasurement)) {
    add({
      severity: "error",
      code: "source-measurement-not-observed",
      subject: "source",
      message: `Measurement ${JSON.stringify(draft.sourceMeasurement)} is not in the observed source list.`,
    });
  }

  const sourceLoaded =
    draft.sourceMeasurement !== "" && source.measurement === draft.sourceMeasurement;
  const sourceFieldsKnown = sourceLoaded && source.fieldsState === "supported";
  const sourceTagsKnown = sourceLoaded && source.tagsState === "supported";
  if (draft.sourceMeasurement !== "" && source.measurementsState === "supported" && !sourceLoaded) {
    add({
      severity: "error",
      code: "source-schema-not-loaded",
      subject: "source",
      message: "Field and tag keys for the source measurement have not been read yet.",
    });
  }

  // Destination measurement.
  if (!isValidDestinationName(draft.destinationMeasurement)) {
    add({
      severity: "error",
      code: "destination-measurement-invalid",
      subject: "destination",
      message:
        "The destination measurement must start with a letter or underscore and use only letters, digits and underscores (max 64).",
    });
  } else if (destination.measurementsState === "supported") {
    if (!destination.measurements.includes(draft.destinationMeasurement)) {
      add({
        severity: "warning",
        code: "destination-measurement-new",
        subject: "destination",
        message: `Measurement ${JSON.stringify(draft.destinationMeasurement)} is not observed on the destination. This increment never creates it.`,
      });
    }
  } else {
    add({
      severity: "warning",
      code: "destination-schema-unknown",
      subject: "destination",
      message: `The destination measurement list is ${destination.measurementsState}; destination names cannot be checked.`,
    });
  }

  const destinationLoaded =
    destination.measurement === draft.destinationMeasurement &&
    isValidDestinationName(draft.destinationMeasurement);
  const destinationFieldsSupported =
    destinationLoaded && destination.fieldsState === "supported";
  const destinationFieldTypes = new Map(
    destinationFieldsSupported
      ? destination.fields.map((field) => [field.name, field.type] as const)
      : [],
  );

  if (draft.fieldRules.length === 0 && draft.tagRules.length === 0 && draft.filters.length === 0) {
    add({
      severity: "warning",
      code: "no-mappings",
      subject: "draft",
      message: "The draft has no field, tag or filter rules yet.",
    });
  }

  // Field rules.
  const destinationFieldNames = new Map<string, number>();
  for (const rule of draft.fieldRules) {
    const subject = `field:${rule.id}`;
    const sourceField = source.fields.find((field) => field.name === rule.sourceField);

    if (!sourceFieldsKnown) {
      add({
        severity: "error",
        code: "source-schema-unknown",
        subject: "source-fields",
        message: `The source field keys are ${sourceLoaded ? source.fieldsState : "not loaded"}; field rules cannot be validated.`,
      });
    } else if (!sourceField) {
      add({
        severity: "error",
        code: "source-field-not-observed",
        subject,
        message: `Field ${JSON.stringify(rule.sourceField)} is not observed on the source measurement.`,
      });
    }

    if (!isValidDestinationName(rule.destinationField)) {
      add({
        severity: "error",
        code: "destination-field-invalid",
        subject,
        message: `Destination field name ${JSON.stringify(rule.destinationField)} is not a valid identifier.`,
      });
    } else {
      destinationFieldNames.set(
        rule.destinationField,
        (destinationFieldNames.get(rule.destinationField) ?? 0) + 1,
      );
    }

    if (!(FIELD_TYPES as readonly string[]).includes(rule.destinationType)) {
      add({
        severity: "error",
        code: "destination-type-invalid",
        subject,
        message: `Destination type ${JSON.stringify(rule.destinationType)} is not a supported field type.`,
      });
    }

    if (sourceField) {
      if (sourceField.type === "") {
        add({
          severity: "error",
          code: "source-type-unreported",
          subject,
          message: `The source type of ${JSON.stringify(rule.sourceField)} was not reported, so compatibility cannot be proven.`,
        });
      } else if (rule.transform.kind === "none") {
        if (sourceField.type !== rule.destinationType) {
          add({
            severity: "error",
            code: "type-mismatch",
            subject,
            message: `Source type ${sourceField.type} does not match destination type ${rule.destinationType}. Declare an explicit unit transform or change the type; nothing is coerced silently.`,
          });
        }
      } else {
        if (!NUMERIC_TYPES.has(sourceField.type) || rule.destinationType !== "float") {
          add({
            severity: "error",
            code: "transform-type-unsupported",
            subject,
            message: "A unit transform needs a numeric source type and a float destination type.",
          });
        }
        if (parseFactor(rule.transform.factor) === null) {
          add({
            severity: "error",
            code: "transform-factor-invalid",
            subject,
            message: "The conversion factor must be a finite, non-zero number.",
          });
        }
        const fromUnit = rule.transform.fromUnit.trim();
        const toUnit = rule.transform.toUnit.trim();
        if (
          fromUnit === "" ||
          toUnit === "" ||
          fromUnit.length > MAX_UNIT_LENGTH ||
          toUnit.length > MAX_UNIT_LENGTH ||
          hasControlCharacter(fromUnit) ||
          hasControlCharacter(toUnit)
        ) {
          add({
            severity: "error",
            code: "transform-unit-invalid",
            subject,
            message: `Both units must be declared, non-empty and at most ${MAX_UNIT_LENGTH} characters.`,
          });
        }
      }
    }

    if (isValidDestinationName(rule.destinationField)) {
      if (destinationFieldsSupported) {
        const observedType = destinationFieldTypes.get(rule.destinationField);
        if (observedType !== undefined && observedType !== "" && observedType !== rule.destinationType) {
          add({
            severity: "error",
            code: "destination-type-conflict",
            subject,
            message: `Destination field ${JSON.stringify(rule.destinationField)} is observed as ${observedType}, not ${rule.destinationType}.`,
          });
        }
      } else if (destinationLoaded) {
        add({
          severity: "warning",
          code: "destination-schema-unknown",
          subject: "destination-fields",
          message: `Destination field keys are ${destination.fieldsState}; the type of ${JSON.stringify(rule.destinationField)} cannot be checked.`,
        });
      }
    }
  }
  for (const [name, count] of destinationFieldNames) {
    if (count > 1) {
      add({
        severity: "error",
        code: "duplicate-destination-field",
        subject: `destination-field:${name}`,
        message: `Destination field ${JSON.stringify(name)} is produced by ${count} rules.`,
      });
    }
  }

  // Tag rules.
  const destinationTagNames = new Map<string, number>();
  for (const rule of draft.tagRules) {
    const subject = `tag:${rule.id}`;
    if (!sourceTagsKnown) {
      add({
        severity: "error",
        code: "source-schema-unknown",
        subject: "source-tags",
        message: `The source tag keys are ${sourceLoaded ? source.tagsState : "not loaded"}; tag rules cannot be validated.`,
      });
    } else if (!source.tags.includes(rule.sourceTag)) {
      add({
        severity: "error",
        code: "source-tag-not-observed",
        subject,
        message: `Tag ${JSON.stringify(rule.sourceTag)} is not observed on the source measurement.`,
      });
    }
    if (!isValidDestinationName(rule.destinationTag)) {
      add({
        severity: "error",
        code: "destination-tag-invalid",
        subject,
        message: `Destination tag name ${JSON.stringify(rule.destinationTag)} is not a valid identifier.`,
      });
    } else {
      destinationTagNames.set(
        rule.destinationTag,
        (destinationTagNames.get(rule.destinationTag) ?? 0) + 1,
      );
    }
  }
  for (const [name, count] of destinationTagNames) {
    if (count > 1) {
      add({
        severity: "error",
        code: "duplicate-destination-tag",
        subject: `destination-tag:${name}`,
        message: `Destination tag ${JSON.stringify(name)} is produced by ${count} rules.`,
      });
    }
    if (destinationFieldNames.has(name)) {
      add({
        severity: "error",
        code: "destination-name-collision",
        subject: `destination-name:${name}`,
        message: `${JSON.stringify(name)} is both a destination field and a destination tag.`,
      });
    }
  }

  // Tag filters (values are data, never query fragments).
  const filterValues = new Map<string, string>();
  for (const filter of draft.filters) {
    const subject = `filter:${filter.id}`;
    if (!sourceTagsKnown) {
      add({
        severity: "error",
        code: "source-schema-unknown",
        subject: "source-tags",
        message: `The source tag keys are ${sourceLoaded ? source.tagsState : "not loaded"}; filters cannot be validated.`,
      });
    } else if (!source.tags.includes(filter.tagKey)) {
      add({
        severity: "error",
        code: "filter-tag-not-observed",
        subject,
        message: `Filter tag ${JSON.stringify(filter.tagKey)} is not observed on the source measurement.`,
      });
    }
    if (
      filter.value === "" ||
      filter.value.length > MAX_FILTER_VALUE_LENGTH ||
      hasControlCharacter(filter.value)
    ) {
      add({
        severity: "error",
        code: "filter-value-invalid",
        subject,
        message: `A filter value must be non-empty, at most ${MAX_FILTER_VALUE_LENGTH} characters and free of control characters.`,
      });
    } else {
      const previous = filterValues.get(filter.tagKey);
      if (previous !== undefined && previous !== filter.value) {
        add({
          severity: "error",
          code: "conflicting-filters",
          subject: `filter-key:${filter.tagKey}`,
          message: `Tag ${JSON.stringify(filter.tagKey)} has two different filter values.`,
        });
      }
      filterValues.set(filter.tagKey, filter.value);
    }
  }

  return out;
}

/** True only when no error blocks the plan. Warnings never block it. */
export function isPlanReady(diagnostics: readonly Diagnostic[]): boolean {
  return diagnostics.every((diagnostic) => diagnostic.severity !== "error");
}

function describeTransform(transform: Transform): string {
  if (transform.kind === "none") {
    return "no transform (types must match)";
  }
  return `unit transform ${JSON.stringify(transform.fromUnit.trim())} → ${JSON.stringify(
    transform.toUnit.trim(),
  )} × ${transform.factor.trim()}`;
}

/**
 * Renders the mapping plan as plain text lines. Every name and value is shown
 * quoted as data. The output is a description, never a query, and it is only
 * produced for a draft that passes validation.
 */
export function planLines(draft: MappingDraft): string[] {
  const lines: string[] = [
    `Measurement ${JSON.stringify(draft.sourceMeasurement)} → ${JSON.stringify(
      draft.destinationMeasurement,
    )}`,
  ];
  for (const rule of draft.fieldRules) {
    lines.push(
      `Field ${JSON.stringify(rule.sourceField)} → ${JSON.stringify(rule.destinationField)} (${rule.destinationType}); ${describeTransform(rule.transform)}`,
    );
  }
  for (const rule of draft.tagRules) {
    lines.push(`Tag ${JSON.stringify(rule.sourceTag)} → ${JSON.stringify(rule.destinationTag)}`);
  }
  for (const filter of draft.filters) {
    lines.push(
      `Filter: tag ${JSON.stringify(filter.tagKey)} equals ${JSON.stringify(filter.value)} (value is data)`,
    );
  }
  return lines;
}
