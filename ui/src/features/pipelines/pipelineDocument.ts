import type { Instance, OptionValue, PipelineDraft } from "./pipelineDraft";

type ContractOption = Record<string, unknown>;

/** Serializes a canvas draft into the pipeline contract document. */
export function toContractDocument(draft: PipelineDraft): Record<string, unknown> {
  const document: Record<string, unknown> = {
    schema_version: draft.schemaVersion,
    telegraf_version: draft.telegrafVersion,
    mode: "graph",
    graph: {
      instances: draft.instances.map(toContractInstance),
      processor_order: draft.processorOrder,
      skip_processors_before_aggregators: draft.skipProcessorsBeforeAggregators,
      skip_processors_after_aggregators: draft.skipProcessorsAfterAggregators,
    },
  };
  if (draft.layout.length > 0) {
    document.layout = {
      nodes: draft.layout.map((node) => ({
        instance_id: node.instanceId,
        x: node.x,
        y: node.y,
      })),
    };
  }
  return document;
}

/** Serializes manual TOML into the pipeline contract document. */
export function toManualDocument(draft: PipelineDraft, toml: string): Record<string, unknown> {
  return {
    schema_version: draft.schemaVersion,
    telegraf_version: draft.telegrafVersion,
    mode: "manual",
    manual: { toml },
  };
}

/**
 * Returns a stable comparison key for the authoritative part of a document.
 * Canvas layout is presentation and is excluded, so moving a node does not
 * count as an unsaved configuration change. Object key order is normalized.
 */
export function documentEquivalenceKey(document: Record<string, unknown>): string {
  const authoritative = { ...document };
  delete authoritative.layout;
  return stableStringify(authoritative);
}

function stableStringify(value: unknown): string {
  if (value === null || typeof value !== "object") {
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(",")}]`;
  }
  const entries = Object.entries(value as Record<string, unknown>)
    .filter(([, entry]) => entry !== undefined)
    .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0));
  return `{${entries
    .map(([key, entry]) => `${JSON.stringify(key)}:${stableStringify(entry)}`)
    .join(",")}}`;
}

function toContractInstance(instance: Instance): Record<string, unknown> {
  const options = toContractOptions(instance.options);
  const encoded: Record<string, unknown> = {
    id: instance.id,
    kind: instance.kind,
    plugin: instance.plugin,
  };
  if (Object.keys(options).length > 0) {
    encoded.options = options;
  }
  return encoded;
}

function toContractOptions(options: Record<string, OptionValue>): Record<string, ContractOption> {
  const encoded: Record<string, ContractOption> = {};
  for (const [key, value] of Object.entries(options)) {
    encoded[key] = toContractOption(value);
  }
  return encoded;
}

function toContractOption(value: OptionValue): ContractOption {
  switch (value.kind) {
    case "bool":
      return { kind: "bool", bool: value.bool };
    case "string":
      return { kind: "string", string: value.string };
    case "integer":
      return { kind: "integer", integer: value.integer };
    case "duration":
      return { kind: "duration", duration: value.duration };
    case "string_list":
      return { kind: "string_list", strings: value.strings };
    case "string_map":
      return { kind: "string_map", string_map: value.stringMap };
    case "secret_ref":
      return { kind: "secret_ref", secret_ref: value.secretRef };
    case "rename_replace":
      return {
        kind: "rename_replace",
        replaces: value.replaces.map((rule) => ({
          [rule.sourceKind]: rule.source,
          dest: rule.dest,
        })),
      };
  }
}
