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

## Technology selection
Go is a candidate backend language, not a committed dependency. UI framework, metadata store, deployment packaging, remote agent interface and exact Telegraf plugin integration are TBD. Prefer APIs and portable document formats over coupling to one frontend.

## Security and operations
- Encrypt secrets at rest where practical; redact them in logs and exported profiles.
- Role-separated view, configure, deploy and repair permissions.
- Dry-run for writes and historical repair; audit user-initiated actions.
- Resource budgets for queries/backfills; enforce concurrency and rate limits.
- Do not let historical repair disrupt ongoing telemetry.
