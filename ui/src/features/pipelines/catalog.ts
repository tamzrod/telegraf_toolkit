/**
 * Focused Telegraf 1.40.1 plugin catalog for the pipeline designer.
 *
 * The names and option keys mirror backend/internal/pipelinecontract, which was
 * read from tag v1.40.1 commit 26b8f4478b676f4f5e5d8ce1622cdf4f6c273bda.
 * This module does not contact Telegraf or invent options outside that set.
 */

export const TELEGRAF_VERSION = "1.40.1";
export const SCHEMA_VERSION = 1;
export const UPSTREAM_COMMIT = "26b8f4478b676f4f5e5d8ce1622cdf4f6c273bda";

export type PluginKind = "input" | "processor" | "aggregator" | "output";

export type ValueKind =
  | "bool"
  | "string"
  | "integer"
  | "duration"
  | "string_list"
  | "string_map"
  | "secret_ref"
  | "rename_replace";

export type OptionSpec = {
  key: string;
  label: string;
  kind: ValueKind;
  required?: boolean;
  allowed?: readonly string[];
  minInteger?: number;
  allowZeroDuration?: boolean;
  url?: boolean;
  nonEmpty?: boolean;
};

export type PluginSpec = {
  kind: PluginKind;
  name: string;
  options: readonly OptionSpec[];
};

export const BASIC_STATS = [
  "count",
  "diff",
  "first",
  "interval",
  "last",
  "max",
  "mean",
  "min",
  "non_negative_diff",
  "non_negative_rate",
  "percent_change",
  "rate",
  "s2",
  "stdev",
  "sum",
] as const;

const inputCommon: OptionSpec[] = [
  { key: "alias", label: "Alias", kind: "string", nonEmpty: true },
  { key: "interval", label: "Interval", kind: "duration" },
  { key: "name_override", label: "Name override", kind: "string", nonEmpty: true },
  { key: "name_prefix", label: "Name prefix", kind: "string", nonEmpty: true },
  { key: "name_suffix", label: "Name suffix", kind: "string", nonEmpty: true },
  { key: "tags", label: "Tags", kind: "string_map" },
];

const processorCommon: OptionSpec[] = [
  { key: "alias", label: "Alias", kind: "string", nonEmpty: true },
];

const aggregatorCommon: OptionSpec[] = [
  { key: "alias", label: "Alias", kind: "string", nonEmpty: true },
  { key: "name_override", label: "Name override", kind: "string", nonEmpty: true },
  { key: "name_prefix", label: "Name prefix", kind: "string", nonEmpty: true },
  { key: "name_suffix", label: "Name suffix", kind: "string", nonEmpty: true },
  { key: "period", label: "Period", kind: "duration", required: true },
  {
    key: "delay",
    label: "Delay",
    kind: "duration",
    required: true,
    allowZeroDuration: true,
  },
  {
    key: "grace",
    label: "Grace",
    kind: "duration",
    required: true,
    allowZeroDuration: true,
  },
  { key: "drop_original", label: "Drop original metrics", kind: "bool", required: true },
];

const outputCommon: OptionSpec[] = [
  { key: "alias", label: "Alias", kind: "string", nonEmpty: true },
  { key: "name_override", label: "Name override", kind: "string", nonEmpty: true },
  { key: "name_prefix", label: "Name prefix", kind: "string", nonEmpty: true },
  { key: "name_suffix", label: "Name suffix", kind: "string", nonEmpty: true },
];

function withCommon(common: OptionSpec[], specific: OptionSpec[]): OptionSpec[] {
  return [...common, ...specific];
}

export const PLUGINS: readonly PluginSpec[] = [
  {
    kind: "input",
    name: "cpu",
    options: withCommon(inputCommon, [
      { key: "percpu", label: "Per CPU", kind: "bool" },
      { key: "totalcpu", label: "Total CPU", kind: "bool" },
      { key: "collect_cpu_time", label: "Collect CPU time", kind: "bool" },
      { key: "report_active", label: "Report active", kind: "bool" },
      { key: "core_tags", label: "Core tags", kind: "bool" },
    ]),
  },
  {
    kind: "input",
    name: "mem",
    options: withCommon(inputCommon, [
      { key: "collect_extended", label: "Collect extended memory stats", kind: "bool" },
    ]),
  },
  {
    kind: "input",
    name: "disk",
    options: withCommon(inputCommon, [
      { key: "mount_points", label: "Mount points", kind: "string_list" },
      { key: "ignore_fs", label: "Ignore filesystem types", kind: "string_list" },
      { key: "ignore_mount_opts", label: "Ignore mount options", kind: "string_list" },
    ]),
  },
  {
    kind: "processor",
    name: "rename",
    options: withCommon(processorCommon, [
      { key: "replace", label: "Replace rules", kind: "rename_replace" },
    ]),
  },
  {
    kind: "aggregator",
    name: "basicstats",
    options: withCommon(aggregatorCommon, [
      { key: "stats", label: "Stats", kind: "string_list", allowed: BASIC_STATS },
    ]),
  },
  {
    kind: "output",
    name: "file",
    options: withCommon(outputCommon, [
      { key: "files", label: "Files", kind: "string_list", required: true },
      { key: "use_batch_format", label: "Use batch format", kind: "bool" },
      {
        key: "rotation_interval",
        label: "Rotation interval",
        kind: "duration",
        allowZeroDuration: true,
      },
      {
        key: "rotation_max_archives",
        label: "Rotation max archives",
        kind: "integer",
        minInteger: -1,
      },
      {
        key: "data_format",
        label: "Data format",
        kind: "string",
        required: true,
        nonEmpty: true,
        allowed: ["influx"],
      },
    ]),
  },
  {
    kind: "output",
    name: "influxdb_v2",
    options: withCommon(outputCommon, [
      { key: "urls", label: "URLs", kind: "string_list", required: true, url: true },
      { key: "token", label: "Credential reference", kind: "secret_ref", required: true },
      { key: "organization", label: "Organization", kind: "string", required: true, nonEmpty: true },
      { key: "bucket", label: "Bucket", kind: "string", required: true, nonEmpty: true },
      { key: "timeout", label: "Timeout", kind: "duration" },
      {
        key: "content_encoding",
        label: "Content encoding",
        kind: "string",
        allowed: ["gzip", "identity"],
      },
      { key: "influx_uint_support", label: "Influx unsigned integer support", kind: "bool" },
      { key: "concurrent_writes", label: "Concurrent writes", kind: "integer", minInteger: 1 },
    ]),
  },
];

export function pluginKey(kind: PluginKind, name: string): string {
  return `${kind}.${name}`;
}

export function findPlugin(kind: PluginKind, name: string): PluginSpec | undefined {
  return PLUGINS.find((plugin) => plugin.kind === kind && plugin.name === name);
}

export function searchPlugins(query: string): PluginSpec[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") {
    return [...PLUGINS];
  }
  return PLUGINS.filter((plugin) =>
    `${plugin.kind} ${plugin.name}`.toLowerCase().includes(needle),
  );
}
