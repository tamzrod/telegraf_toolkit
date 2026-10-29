# Pipeline configuration contract

Status: **Implemented** for the focused Phase 4 domain model in
`backend/internal/pipelinecontract`.
Task: P4-002.
Scope: versioned graph and manual pipeline documents, a verified Telegraf
1.40.1 plugin subset, validation, filter evaluation and an execution plan.
This contract does not generate TOML, open an HTTP route, deploy Telegraf,
restart a runtime, write a historian, or resolve a secret value.

## Authority

- Product boundaries: [`IDENTITY.md`](../../IDENTITY.md).
- Version pin: [`docs/adr/0001-official-telegraf-and-version-pinning.md`](../adr/0001-official-telegraf-and-version-pinning.md).
  The only accepted Telegraf version in this contract is `1.40.1`.
- Phase 4 decisions: [`docs/PHASE_4_PLAN.md`](../PHASE_4_PLAN.md).

`schema_version` is `1`. `mode` is `graph` or `manual`. A document carries one
authoritative body. Canvas `layout` coordinates are presentation and are not
an input to the execution plan.

## Catalog provenance

The supported plugins and options were read from the official tag `v1.40.1`,
which peels to commit `26b8f4478b676f4f5e5d8ce1622cdf4f6c273bda`
(annotated tag object `c23023ec014359c536e710d3cf4aaf93dbb48e2d`,
tagged 2026-09-21). The files below were inspected on 2026-10-29 via that tag.
A plugin or option is supported only when it appears in this catalog.
Candidate names are rejected.

Shared behavior was taken from:

| Path | What it fixes |
| --- | --- |
| `docs/CONFIGURATION.md` | Common plugin keys, selector and modifier filters, processor passes, aggregator `period`, `delay`, `grace`, and `drop_original` |
| `models/filter.go` | Selector order and tagpass/tagdrop matching |
| `filter/filter.go` | Glob wildcard characters |
| `agent/agent.go` | Aggregator fan-out, original-metric drop, when the second processor pass starts |
| `config/config.go` | Each processor is built twice; both skip flags cannot be true; the before-pass list is cleared when that skip is set |

Plugin files:

| Kind | Name | Inspected paths |
| --- | --- | --- |
| input | `cpu` | `plugins/inputs/cpu/README.md` |
| input | `mem` | `plugins/inputs/mem/README.md` |
| input | `disk` | `plugins/inputs/disk/README.md` |
| processor | `rename` | `plugins/processors/rename/README.md` |
| aggregator | `basicstats` | `plugins/aggregators/basicstats/README.md`, `plugins/aggregators/basicstats/basicstats.go` |
| output | `file` | `plugins/outputs/file/README.md` |
| output | `influxdb_v2` | `plugins/outputs/influxdb_v2/README.md` |

### Supported options

Common input keys: `alias`, `interval`, `name_override`, `name_prefix`,
`name_suffix`, `tags`.

`cpu`: `percpu`, `totalcpu`, `collect_cpu_time`, `report_active`, `core_tags`.

`mem`: `collect_extended`.

`disk`: `mount_points`, `ignore_fs`, `ignore_mount_opts`.

Common processor key: `alias`. `rename` adds `replace`. Each replace entry has
`dest` and exactly one of `measurement`, `tag`, or `field`.

Common aggregator keys, all required on every aggregator instance: `period`,
`delay`, `grace`, `drop_original`. Also `alias`, `name_override`,
`name_prefix`, `name_suffix`.

`basicstats` adds `stats`. Accepted names are the `parseStats` cases in
`basicstats.go`: `count`, `diff`, `first`, `interval`, `last`, `max`, `mean`,
`min`, `non_negative_diff`, `non_negative_rate`, `percent_change`, `rate`,
`s2`, `stdev`, `sum`.

Common output keys: `alias`, `name_override`, `name_prefix`, `name_suffix`.

`file`: `files` and `data_format` are required. `data_format` accepts `influx`.
Also `use_batch_format`, `rotation_interval`, `rotation_max_archives`.

`influxdb_v2`: `urls`, `token`, `organization`, and `bucket` are required.
`token` is a credential reference ID. Also `timeout`, `content_encoding`
(`gzip` or `identity`), `influx_uint_support`, `concurrent_writes`.

Omitted optional options stay omitted. The contract does not copy upstream
defaults into the document. In particular it does not fill `basicstats.stats`
with `count`, `min`, `max`, `mean`, `stdev`, and `s2`, and it does not fill
`disk.ignore_fs` with the sample filesystem list.

## Execution model

Checked against `config.go` and `agent.go` at the commit above.

- Several instances may use the same plugin. Instance `id` is the identity.
- Inputs are independent sources in declaration order. They are not a chain.
- Processors run in `processor_order`. That list is the sequence. Telegraf's
  per-processor `order` integer is not a second field on the instance, so the
  two cannot disagree. Slice position and canvas position do not reorder
  processors.
- With one or more aggregators, the same processor sequence runs before
  aggregators and again after them. `skip_processors_before_aggregators` and
  `skip_processors_after_aggregators` must be set in the document. Both cannot
  be true. The 1.40.1 agent still fills a nil after-flag with false while
  logging that the default will change; this contract does not inherit that
  default.
- When the graph has no aggregators, Telegraf starts only the before-pass.
  The plan leaves the after-pass empty in that case.
- Every aggregator is offered each metric, in declaration order. A filter miss
  does not remove the metric from later aggregators. `drop_original` on any
  aggregator that selected the metric drops the original, including when field
  modifiers emptied it. Aggregated series are emitted on `period`. `delay` and
  `grace` bound the window. This contract records those three durations and
  does not simulate the window.
- Outputs are independent writers in declaration order.
- `global_tags` are metric tags from `CONFIGURATION.md`. They are not routes.

## Filters

Selector order is namepass, namedrop, tagpass, tagdrop.

- A non-empty namepass list keeps measurements that match any pattern.
- namedrop then removes matches.
- A present tagpass table keeps a metric when any listed tag key has a value
  matching any of its patterns. An empty tagpass table keeps nothing.
  `models/filter.go` `ShouldTagsPass` and the tagpass example in
  `CONFIGURATION.md` both use this OR across tag keys.
- tagdrop then removes a metric when any listed tag matches.
- fieldinclude / fieldexclude and taginclude / tagexclude run only after
  selectors accept the metric. An empty field set drops the metric.
- An input or output selector miss drops the metric for that plugin.
- A processor or aggregator selector miss bypasses that plugin and leaves the
  metric unchanged.

Patterns accept literals, `*`, and `?`. Patterns containing `{`, `}`, `[`,
`]`, `!`, or `\` are rejected. Telegraf compiles the wider gobwas/glob set;
this contract refuses the extra forms instead of evaluating them differently.

## Secrets

`influxdb_v2.token` accepts only `secret_ref`, a credential reference ID.
The value type has no secret field. The contract does not look up, invent, or
echo a credential. URLs with userinfo are rejected. Manual TOML that assigns a
non-empty `token`, `password`, `passwd`, or `secret` literal is rejected, and
so is a credential-bearing URL. Empty assignments are not treated as values.
Manual mode does not turn those literals into references.

## Limitations

- The catalog is a subset of Telegraf 1.40.1. Plugins such as `net`, `kafka`,
  and `influxdb` are real upstream plugins and are still rejected here.
- Options read upstream and left unsupported in this subset include processor
  `order` (represented by `processor_order`), `metricpass`, `namepass_separator`,
  `namedrop_separator`, deprecated `fieldpass` / `fielddrop`, `http_headers`,
  TLS settings, `http_proxy`, rate limits, `rotation_max_size`, and file
  compression. `basicstats` sample text lists `variance`; the parser accepts
  `s2` and ignores unknown names. This contract rejects `variance`.
- `basicstats` README states that the aggregator applies no tags, so the
  common aggregator `tags` key is not in its spec.
- `file.data_format` accepts `influx` only. Other serializers were not modeled.
- Filter patterns are the `*` / `?` subset described above.
- Rename rules are validated and are not applied to metrics.
- Aggregation time windows are recorded and are not simulated.
- Manual TOML is stored and is not parsed into instances, processor order, or
  layout. `ExecutionPlan` refuses that conversion.
- Phase 4 authors configuration artifacts only. Nothing in this package
  deploys a collector or writes a database.
