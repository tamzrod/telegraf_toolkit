import { findPlugin, type OptionSpec } from "./catalog";
import {
  issues,
  type DraftIssue,
  type Instance,
  type OptionValue,
  type PipelineDraft,
  type RenameRule,
} from "./pipelineDraft";

export type CheckClass = "parse" | "config" | "support";

export type PreviewIssue = {
  class: CheckClass;
  message: string;
};

const secretStoreName = /^[A-Za-z0-9_]+$/;

/**
 * Read-only graph preview. The bytes match the Go generator for the same
 * graph. Manual TOML is not produced here, and secret values are not written.
 */
export function graphPreview(
  draft: PipelineDraft,
): { toml: string } | { issues: PreviewIssue[] } {
  const found = issues(draft);
  if (found.length > 0) {
    return { issues: found.map(classifyIssue) };
  }
  const rendered = render(draft);
  if ("issues" in rendered) {
    return rendered;
  }
  return { toml: rendered.toml };
}

export function checkClass(code: string): CheckClass {
  if (
    code === "unsupported-plugin" ||
    code === "unsupported-option" ||
    code === "unsupported-pattern" ||
    code === "unsupported-version"
  ) {
    return "support";
  }
  if (code === "parse") {
    return "parse";
  }
  return "config";
}

function classifyIssue(issue: DraftIssue): PreviewIssue {
  return { class: checkClass(issue.code), message: issue.message };
}

function render(draft: PipelineDraft): { toml: string } | { issues: PreviewIssue[] } {
  const lines = [
    "# telegraf-toolkit graph preview",
    "# schema_version = 1",
    "# telegraf_version = \"1.40.1\"",
    "# layout coordinates are omitted",
    "# secret values are omitted; credential options are secret-store references",
    "# this preview is not deployed",
    "",
    "[agent]",
    `  skip_processors_before_aggregators = ${draft.skipProcessorsBeforeAggregators}`,
    `  skip_processors_after_aggregators = ${draft.skipProcessorsAfterAggregators}`,
  ];
  for (const instance of orderedInstances(draft)) {
    const encoded = encodeInstance(instance);
    if ("issues" in encoded) {
      return encoded;
    }
    lines.push(...encoded.lines);
  }
  return { toml: `${lines.join("\n")}\n` };
}

function orderedInstances(draft: PipelineDraft): Instance[] {
  const processors = new Map(draft.instances.map((instance) => [instance.id, instance]));
  const ordered: Instance[] = [];
  ordered.push(...draft.instances.filter((instance) => instance.kind === "input"));
  for (const id of draft.processorOrder) {
    const processor = processors.get(id);
    if (processor) {
      ordered.push(processor);
    }
  }
  ordered.push(...draft.instances.filter((instance) => instance.kind === "aggregator"));
  ordered.push(...draft.instances.filter((instance) => instance.kind === "output"));
  return ordered;
}

function encodeInstance(instance: Instance): { lines: string[] } | { issues: PreviewIssue[] } {
  const spec = findPlugin(instance.kind, instance.plugin);
  if (!spec) {
    return {
      issues: [
        {
          class: "support",
          message: "That plugin is outside the verified Telegraf 1.40.1 catalog.",
        },
      ],
    };
  }
  const header = tablePrefix(instance.kind);
  const lines = ["", `# toolkit.instance = ${quoteTOML(instance.id)}`, `[[${header}.${instance.plugin}]]`];
  for (const option of spec.options) {
    const value = instance.options[option.key];
    if (!value) {
      continue;
    }
    const encoded = encodeOption(header, instance.plugin, option, value);
    if ("issues" in encoded) {
      return encoded;
    }
    lines.push(...encoded.lines);
  }
  return { lines };
}

function encodeOption(
  header: string,
  plugin: string,
  option: OptionSpec,
  value: OptionValue,
): { lines: string[] } | { issues: PreviewIssue[] } {
  switch (value.kind) {
    case "bool":
      return { lines: [`  ${option.key} = ${value.bool}`] };
    case "string":
      return { lines: [`  ${option.key} = ${quoteTOML(value.string)}`] };
    case "integer":
      return { lines: [`  ${option.key} = ${value.integer}`] };
    case "duration":
      return { lines: [`  ${option.key} = ${quoteTOML(value.duration)}`] };
    case "string_list":
      return { lines: [`  ${option.key} = ${quoteList(value.strings)}`] };
    case "string_map":
      return { lines: [`  ${option.key} = ${quoteStringMap(value.stringMap)}`] };
    case "secret_ref":
      if (!secretStoreName.test(value.secretRef)) {
        return {
          issues: [
            {
              class: "config",
              message:
                "credential reference cannot be encoded as a Telegraf 1.40.1 secret-store reference",
            },
          ],
        };
      }
      return { lines: [`  ${option.key} = ${quoteTOML(`@{toolkit:${value.secretRef}}`)}`] };
    case "rename_replace":
      return { lines: value.replaces.flatMap((rule) => encodeRule(header, plugin, rule)) };
    default:
      return { issues: [{ class: "config", message: "option kind is not supported" }] };
  }
}

function encodeRule(header: string, plugin: string, rule: RenameRule): string[] {
  const sourceKey = rule.sourceKind;
  return [
    `  [[${header}.${plugin}.replace]]`,
    `    ${sourceKey} = ${quoteTOML(rule.source)}`,
    `    dest = ${quoteTOML(rule.dest)}`,
  ];
}

function tablePrefix(kind: Instance["kind"]): string {
  switch (kind) {
    case "input":
      return "inputs";
    case "processor":
      return "processors";
    case "aggregator":
      return "aggregators";
    case "output":
      return "outputs";
  }
}

function quoteList(values: string[]): string {
  return `[${values.map((value) => quoteTOML(value)).join(", ")}]`;
}

function quoteStringMap(values: Record<string, string>): string {
  const body = Object.keys(values)
    .sort()
    .map((key) => `${key} = ${quoteTOML(values[key] ?? "")}`)
    .join(", ");
  return `{${body}}`;
}

function quoteTOML(value: string): string {
  let out = '"';
  for (const char of value) {
    switch (char) {
      case "\\":
      case '"':
        out += `\\${char}`;
        break;
      case "\n":
        out += "\\n";
        break;
      case "\r":
        out += "\\r";
        break;
      case "\t":
        out += "\\t";
        break;
      default:
        out += char;
        break;
    }
  }
  return `${out}"`;
}
