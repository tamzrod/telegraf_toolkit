# Architecture — Telegraf Toolkit

Status: **Proposal, not implemented**.

## Principles
- Management plane is separate from data plane. Telegraf continues collecting if the Toolkit UI/backend is unavailable.
- Synchronization workers are independent from live Telegraf pipelines.
- Non-destructive by default: no modification of source databases; no silent overwrites or deletes.
- Persist configuration revisions and block-job state; support restart/resume.
- Use least-privilege database credentials and protect secrets from logs, exports, and UI responses.

## Logical components

```text
Browser: Telegraf Manager | Pipeline Editor | Sync Designer | Canonical Registry
                             |
                       Management API
       +---------------------+-----------------------+
       |                     |                       |
 Instance/Config Service  Mapping Registry      Sync Controller
       |                     |                       |
 Telegraf process(es)   Profiles/schema          Job + Block Store
       |                                             |
 Inputs -> Processors -> Outputs                 Reader adapters
       |                                             |
 Live destination DBs                        Repair/backfill transport
                                                     |
                                           Destination DB + verifier
```

## Responsibilities

### Telegraf Manager
Discover/register instances, inspect state and logs, test plugins, validate and deploy versioned TOML configurations. Never assume remote process control is universally available; it requires an appropriate agent or host integration.

### Configuration Editor
Edit native TOML and provide a visual input -> processor -> output pipeline. Plugin metadata and parameter validation should be version-aware. Preview changes, test configuration, create revision, then explicitly apply/restart if required. Rollback to previous revision.

### Synchronizer
Own measurement/field selection, historical scans, time-block index, comparison, repair scheduling, retries, throttling and post-write verification. Use version-aware InfluxDB read adapters. Telegraf may serve as transformation/write transport where practical, but the controller must manage completion and verification itself.

### Canonical Registry and Mapper
Portable schema definitions; source-to-destination field/tag mappings; optional measurement renaming and safe unit conversions; versioned profiles. The same mapping logic is reused for live Telegraf configurations and historical backfill.

## Runtime and version decision
See [ADR-0001](adr/0001-official-telegraf-and-version-pinning.md). Use the upstream **official Telegraf 1.40.1** binary or image as the initial tested baseline; do not compile/fork Telegraf by default. Keep Telegraf Toolkit a separately deployed manager. **Never use floating runtime versions or the Docker `latest` tag**. Pin every runtime dependency to an explicit version, and production images to a verified digest. Upgrade pins only through deliberate testing and review.

## Containerized runtime isolation and orchestration

**Decision:** Use **Docker Compose on each host**, with **one independent Telegraf process/container per runtime**. The Toolkit management application is separately deployed. Multiple plugins or device connections may share one runtime where they belong to the same failure domain; one container per device or tag is **not** required.

```text
Telegraf Toolkit (management UI/API)
   |-- Runtime Manager -> Docker Engine / restricted host agent
   |       |-- telegraf-modbus       (independent container + TOML)
   |       |-- telegraf-opcua        (independent container + TOML)
   |       |-- telegraf-mqtt         (independent container + TOML)
   |       +-- telegraf-weather      (independent container + TOML)
   +-- Sync Controller -> independent backfill workers / transport
```

- Containers have independent restart policies, configuration, logs, lifecycle, and CPU/memory budgets. A fatal plugin/process failure should not stop unrelated collector runtimes.
- A plugin returning an ordinary error may not terminate Telegraf; isolation is a defense against process failures, not a guarantee that each plugin error causes a restart.
- Use separate Compose projects or clearly scoped services/runtime groups so applying a change to one collector does not unnecessarily recreate others.
- Monitor per-runtime status, restarts, CPU, RAM, input errors, output errors, network traffic, collection latency, disk buffers and logging volume.
- Enforce collection concurrency, polling intervals, retries and backpressure; resource exhaustion, shared infrastructure and device overload can still affect other containers.
- Telegraf Toolkit must not expose unrestricted Docker daemon/socket access to the public UI. Docker operations belong in a separately authorized management service or restricted host agent.

### Capacity and container count

**No artificial maximum container count.** Approximately 50 or more Telegraf containers on one adequately provisioned host is an acceptable **design target, not a performance guarantee**. Admission and capacity planning must be based on actual workloads, total memory, CPU scheduling, network and field-device polling loads, disk buffering and restart behavior, rather than a fixed container count.

Running many containers is not synonymous with using **Docker Swarm**. Docker Compose is sufficient for the initial **single-host** deployment, including tens of isolated runtimes. Do not introduce Kubernetes or Docker Swarm solely because the container count increases.

### When orchestration becomes necessary

Revisit the orchestration choice only when requirements justify it: multi-host scheduling, automated placement, host-level high availability/failover, coordinated distributed rollouts, or capacity management beyond one Docker host. A future lightweight host-agent model may provide central management across multiple Compose hosts without requiring a cluster orchestrator.

### Image version policy

All runtime images must use explicit, immutable version selections, consistent with [ADR-0001](adr/0001-official-telegraf-and-version-pinning.md): initial Telegraf baseline **1.40.1**, never `latest` or floating tags, and verified digest pins for production. Independent runtime upgrades are deliberate and tested.

## Technology selection
Go is a candidate backend language, not a committed dependency. UI framework, metadata store, deployment packaging, remote agent interface and exact Telegraf plugin integration are TBD. Prefer APIs and portable document formats over coupling to one frontend.

## Security and operations
- Encrypt secrets at rest where practical; redact them in logs and exported profiles.
- Role-separated view, configure, deploy and repair permissions.
- Dry-run for writes and historical repair; audit user-initiated actions.
- Resource budgets for queries/backfills; enforce concurrency and rate limits.
- Do not let historical repair disrupt ongoing telemetry.
