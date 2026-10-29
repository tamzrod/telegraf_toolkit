import {
  findPlugin,
  type OptionSpec,
  type PluginKind,
  type PluginSpec,
  SCHEMA_VERSION,
  TELEGRAF_VERSION,
} from "./catalog";

export type OptionValue =
  | { kind: "bool"; bool: boolean }
  | { kind: "string"; string: string }
  | { kind: "integer"; integer: number }
  | { kind: "duration"; duration: string }
  | { kind: "string_list"; strings: string[] }
  | { kind: "string_map"; stringMap: Record<string, string> }
  | { kind: "secret_ref"; secretRef: string }
  | { kind: "rename_replace"; replaces: RenameRule[] };

export type RenameRule = {
  sourceKind: "measurement" | "tag" | "field";
  source: string;
  dest: string;
};

export type Instance = {
  id: string;
  kind: PluginKind;
  plugin: string;
  options: Record<string, OptionValue>;
};

export type LayoutNode = {
  instanceId: string;
  x: number;
  y: number;
};

/** Graph document. Layout coordinates are not execution order. */
export type PipelineDraft = {
  schemaVersion: typeof SCHEMA_VERSION;
  telegrafVersion: typeof TELEGRAF_VERSION;
  mode: "graph";
  instances: Instance[];
  processorOrder: string[];
  skipProcessorsBeforeAggregators: boolean;
  skipProcessorsAfterAggregators: boolean;
  layout: LayoutNode[];
};

export type DraftIssue = {
  code: string;
  message: string;
  instanceId?: string;
};

export type SemanticRelation =
  | "processor-order"
  | "aggregator-fanout"
  | "output-delivery";

export type SemanticEdge = {
  id: string;
  source: string;
  target: string;
  relation: SemanticRelation;
  label: string;
};

const secretRefPattern = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
const credentialURLPattern = /:\/\/[^/\s:@]+:[^/\s@]+@/;
const durationPattern = /^(?:\d+(?:\.\d+)?(?:ns|us|µs|ms|s|m|h))+$/;

const laneY: Record<PluginKind, number> = {
  input: 24,
  processor: 150,
  aggregator: 276,
  output: 402,
};

export function emptyDraft(): PipelineDraft {
  return {
    schemaVersion: SCHEMA_VERSION,
    telegrafVersion: TELEGRAF_VERSION,
    mode: "graph",
    instances: [],
    processorOrder: [],
    skipProcessorsBeforeAggregators: false,
    skipProcessorsAfterAggregators: false,
    layout: [],
  };
}

export function rejectDrawnConnection(): DraftIssue {
  return {
    code: "invalid-relationship",
    message:
      "Drawn connections are rejected. Edges follow processor order and aggregator fan-out.",
  };
}

export function addInstance(
  draft: PipelineDraft,
  kind: PluginKind,
  plugin: string,
): { draft: PipelineDraft; id: string } | { error: DraftIssue } {
  const spec = findPlugin(kind, plugin);
  if (!spec) {
    return {
      error: {
        code: "unsupported-plugin",
        message: "That plugin is outside the verified Telegraf 1.40.1 catalog.",
      },
    };
  }
  const id = nextId(draft, plugin);
  const instance: Instance = {
    id,
    kind,
    plugin,
    options: initialOptions(spec),
  };
  const next: PipelineDraft = {
    ...draft,
    instances: [...draft.instances, instance],
    processorOrder:
      kind === "processor" ? [...draft.processorOrder, id] : draft.processorOrder,
    layout: [
      ...draft.layout,
      { instanceId: id, x: 24 + countKind(draft, kind) * 220, y: laneY[kind] },
    ],
  };
  return { draft: next, id };
}

export function deleteInstance(draft: PipelineDraft, id: string): PipelineDraft {
  return {
    ...draft,
    instances: draft.instances.filter((instance) => instance.id !== id),
    processorOrder: draft.processorOrder.filter((processorId) => processorId !== id),
    layout: draft.layout.filter((node) => node.instanceId !== id),
  };
}

export function moveLayout(
  draft: PipelineDraft,
  id: string,
  x: number,
  y: number,
): PipelineDraft {
  return {
    ...draft,
    layout: draft.layout.map((node) =>
      node.instanceId === id ? { ...node, x, y } : node,
    ),
  };
}

export function moveProcessor(
  draft: PipelineDraft,
  id: string,
  direction: -1 | 1,
): { draft: PipelineDraft } | { error: DraftIssue } {
  const index = draft.processorOrder.indexOf(id);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= draft.processorOrder.length) {
    return {
      error: {
        code: "invalid-order",
        message: "Processor order cannot move past the ends of the sequence.",
      },
    };
  }
  const order = [...draft.processorOrder];
  const moved = order[index];
  if (!moved) {
    return {
      error: {
        code: "invalid-order",
        message: "Processor order cannot move past the ends of the sequence.",
      },
    };
  }
  order.splice(index, 1);
  order.splice(target, 0, moved);
  return { draft: { ...draft, processorOrder: order } };
}

export function setSkip(
  draft: PipelineDraft,
  flag: "skipProcessorsBeforeAggregators" | "skipProcessorsAfterAggregators",
  value: boolean,
): { draft: PipelineDraft } | { error: DraftIssue } {
  const next = { ...draft, [flag]: value };
  if (next.skipProcessorsBeforeAggregators && next.skipProcessorsAfterAggregators) {
    return {
      error: {
        code: "invalid-document",
        message: "Both processor passes cannot be skipped.",
      },
    };
  }
  return { draft: next };
}

export function setOption(
  draft: PipelineDraft,
  id: string,
  key: string,
  value: OptionValue,
): { draft: PipelineDraft } | { error: DraftIssue } {
  const instance = draft.instances.find((item) => item.id === id);
  const spec = instance ? findPlugin(instance.kind, instance.plugin) : undefined;
  const option = spec?.options.find((item) => item.key === key);
  if (!instance || !option) {
    return {
      error: {
        code: "unsupported-option",
        message: "That option is outside the verified catalog.",
      },
    };
  }
  const rejected = rejectSecretMaterial(option, value);
  if (rejected) {
    return { error: rejected };
  }
  return {
    draft: {
      ...draft,
      instances: draft.instances.map((item) =>
        item.id === id ? { ...item, options: { ...item.options, [key]: value } } : item,
      ),
    },
  };
}

export function clearOption(
  draft: PipelineDraft,
  id: string,
  key: string,
): PipelineDraft {
  return {
    ...draft,
    instances: draft.instances.map((instance) => {
      if (instance.id !== id) {
        return instance;
      }
      const options = { ...instance.options };
      delete options[key];
      return { ...instance, options };
    }),
  };
}

export function issues(draft: PipelineDraft): DraftIssue[] {
  const found: DraftIssue[] = [];
  if (
    draft.skipProcessorsBeforeAggregators &&
    draft.skipProcessorsAfterAggregators
  ) {
    found.push({
      code: "invalid-document",
      message: "Both processor passes cannot be skipped.",
    });
  }
  for (const instance of draft.instances) {
    const spec = findPlugin(instance.kind, instance.plugin);
    if (!spec) {
      found.push({
        code: "unsupported-plugin",
        message: "Plugin is outside the verified catalog.",
        instanceId: instance.id,
      });
      continue;
    }
    for (const option of spec.options) {
      const value = instance.options[option.key];
      if (!value) {
        if (option.required) {
          found.push({
            code: "invalid-option",
            message: `${option.label} is required.`,
            instanceId: instance.id,
          });
        }
        continue;
      }
      found.push(...optionIssues(option, value, instance.id));
    }
  }
  return found;
}

export function semanticEdges(draft: PipelineDraft): SemanticEdge[] {
  const inputs = draft.instances.filter((instance) => instance.kind === "input");
  const processors = draft.processorOrder
    .map((id) => draft.instances.find((instance) => instance.id === id))
    .filter((instance): instance is Instance => instance !== undefined);
  const aggregators = draft.instances.filter(
    (instance) => instance.kind === "aggregator",
  );
  const outputs = draft.instances.filter((instance) => instance.kind === "output");
  const edges: SemanticEdge[] = [];
  const push = (
    source: string,
    target: string,
    relation: SemanticRelation,
    label: string,
  ) => {
    edges.push({
      id: `${relation}:${source}:${target}`,
      source,
      target,
      relation,
      label,
    });
  };

  for (let index = 0; index < processors.length - 1; index += 1) {
    push(processors[index].id, processors[index + 1].id, "processor-order", "then");
  }
  if (processors.length > 0) {
    for (const input of inputs) {
      push(input.id, processors[0].id, "processor-order", "into processors");
    }
  }

  const fanSources = processors.length > 0 ? [processors[processors.length - 1]] : inputs;
  for (const source of fanSources) {
    for (const aggregator of aggregators) {
      const drops = boolOption(aggregator, "drop_original");
      push(
        source.id,
        aggregator.id,
        "aggregator-fanout",
        drops ? "fan-out, drops original" : "fan-out, keeps original",
      );
    }
  }

  const anchor = processors.length > 0 ? [processors[processors.length - 1]] : inputs;
  const originalContinues =
    aggregators.length === 0 ||
    aggregators.some((aggregator) => !boolOption(aggregator, "drop_original"));
  if (originalContinues) {
    for (const source of anchor) {
      for (const output of outputs) {
        push(
          source.id,
          output.id,
          "output-delivery",
          aggregators.length === 0 ? "independent output" : "original continues",
        );
      }
    }
  }
  for (const aggregator of aggregators) {
    for (const output of outputs) {
      push(aggregator.id, output.id, "output-delivery", "aggregate delivery");
    }
  }
  return edges;
}

export function positionOf(draft: PipelineDraft, id: string): LayoutNode | undefined {
  return draft.layout.find((node) => node.instanceId === id);
}

function nextId(draft: PipelineDraft, plugin: string): string {
  const base = plugin.toLowerCase().replace(/_/g, "-").replace(/[^a-z0-9-]/g, "") || "plugin";
  let number = 1;
  while (draft.instances.some((instance) => instance.id === `${base}-${number}`)) {
    number += 1;
  }
  return `${base}-${number}`;
}

function countKind(draft: PipelineDraft, kind: PluginKind): number {
  return draft.instances.filter((instance) => instance.kind === kind).length;
}

function initialOptions(spec: PluginSpec): Record<string, OptionValue> {
  const options: Record<string, OptionValue> = {};
  for (const option of spec.options) {
    if (!option.required) {
      continue;
    }
    const value = initialValue(option);
    if (value) {
      options[option.key] = value;
    }
  }
  return options;
}

function initialValue(option: OptionSpec): OptionValue | undefined {
  switch (option.kind) {
    case "bool":
      return { kind: "bool", bool: false };
    case "string":
      if (option.allowed?.includes("influx")) {
        return { kind: "string", string: "influx" };
      }
      return { kind: "string", string: "" };
    case "string_list":
      if (option.key === "files") {
        return { kind: "string_list", strings: ["stdout"] };
      }
      if (option.key === "urls") {
        return { kind: "string_list", strings: ["http://127.0.0.1:8086"] };
      }
      return { kind: "string_list", strings: [] };
    case "duration":
      if (option.key === "period") {
        return { kind: "duration", duration: "30s" };
      }
      if (option.key === "delay") {
        return { kind: "duration", duration: "100ms" };
      }
      return { kind: "duration", duration: "0s" };
    case "secret_ref":
      return { kind: "secret_ref", secretRef: "" };
    case "integer":
      return { kind: "integer", integer: option.minInteger ?? 0 };
    default:
      return undefined;
  }
}

function rejectSecretMaterial(option: OptionSpec, value: OptionValue): DraftIssue | undefined {
  const texts = textsOf(value);
  if (texts.some((text) => credentialURLPattern.test(text) || text.includes("@"))) {
    return {
      code: "secret-material-rejected",
      message: "Embedded credentials were rejected and were not stored.",
    };
  }
  if (value.kind === "secret_ref" && value.secretRef !== "" && !secretRefPattern.test(value.secretRef)) {
    return {
      code: "secret-material-rejected",
      message: "Credential reference must be an id. The value was not stored.",
    };
  }
  if (option.kind === "secret_ref" && value.kind !== "secret_ref") {
    return {
      code: "secret-material-rejected",
      message: "Credential reference must be an id. The value was not stored.",
    };
  }
  return undefined;
}

function optionIssues(option: OptionSpec, value: OptionValue, instanceId: string): DraftIssue[] {
  const issue = (message: string, code = "invalid-option"): DraftIssue => ({
    code,
    message,
    instanceId,
  });
  if (value.kind !== option.kind) {
    return [issue(`${option.label} has the wrong value kind.`)];
  }
  switch (value.kind) {
    case "string":
      if (option.nonEmpty && value.string.trim() === "") {
        return [issue(`${option.label} is required.`)];
      }
      if (option.allowed && value.string !== "" && !option.allowed.includes(value.string)) {
        return [issue(`${option.label} is not an accepted value.`)];
      }
      return [];
    case "duration":
      if (!durationPattern.test(value.duration)) {
        return [issue(`${option.label} is not a Telegraf interval.`)];
      }
      if (value.duration.startsWith("0") && !option.allowZeroDuration && /^0(?:\.0+)?(?:ns|us|µs|ms|s|m|h)$/.test(value.duration)) {
        return [issue(`${option.label} must be greater than zero.`)];
      }
      return [];
    case "string_list": {
      const found: DraftIssue[] = [];
      if (option.required && value.strings.length === 0) {
        found.push(issue(`${option.label} is required.`));
      }
      for (const item of value.strings) {
        if (option.url && credentialURLPattern.test(item)) {
          found.push(issue("Embedded credentials were rejected.", "secret-material-rejected"));
        }
        if (option.allowed && !option.allowed.includes(item)) {
          found.push(issue(`${item} is not an accepted value.`));
        }
      }
      return found;
    }
    case "secret_ref":
      if (value.secretRef === "") {
        return [issue("Credential reference is required.")];
      }
      if (!secretRefPattern.test(value.secretRef)) {
        return [issue("Credential reference must be an id.", "secret-material-rejected")];
      }
      return [];
    case "integer":
      if (option.minInteger !== undefined && value.integer < option.minInteger) {
        return [issue(`${option.label} is below the minimum.`)];
      }
      return [];
    case "rename_replace":
      if (value.replaces.some((rule) => rule.source.trim() === "" || rule.dest.trim() === "")) {
        return [issue("Each rename rule needs a source and dest.")];
      }
      return [];
    case "string_map":
      if (Object.keys(value.stringMap).some((key) => key.trim() === "")) {
        return [issue("Tag keys are required.")];
      }
      return [];
    default:
      return [];
  }
}

function textsOf(value: OptionValue): string[] {
  switch (value.kind) {
    case "string":
      return [value.string];
    case "duration":
      return [value.duration];
    case "string_list":
      return value.strings;
    case "string_map":
      return Object.entries(value.stringMap).flat();
    case "secret_ref":
      return [value.secretRef];
    case "rename_replace":
      return value.replaces.flatMap((rule) => [rule.source, rule.dest]);
    default:
      return [];
  }
}

function boolOption(instance: Instance, key: string): boolean {
  const value = instance.options[key];
  return value?.kind === "bool" ? value.bool : false;
}
