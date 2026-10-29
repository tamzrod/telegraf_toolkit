import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Background,
  Handle,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeChange,
  type NodeProps,
  type OnConnectEnd,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ApiError } from "../../api/client";
import {
  approvePipelineRevision,
  commitPipelineTransition,
  createPipelineRevision,
  previewPipelineTransition,
  savePipelineRevision,
  type PipelineRevisionPayload,
} from "../../api/client";
import { hasSessionToken } from "../../api/session";
import { PageHeader } from "../../components/common/ModulePageParts";
import { PLUGINS, findPlugin, searchPlugins, type OptionSpec } from "./catalog";
import {
  addInstance,
  clearOption,
  deleteInstance,
  emptyDraft,
  issues,
  moveLayout,
  moveProcessor,
  positionOf,
  rejectDrawnConnection,
  semanticEdges,
  setOption,
  setSkip,
  type Instance,
  type OptionValue,
  type PipelineDraft,
  type RenameRule,
} from "./pipelineDraft";
import { toContractDocument, toManualDocument } from "./pipelineDocument";
import { reviewGraph, reviewManual, reviewRestore, type ModeReview, type Snapshot } from "./pipelineMode";
import { PipelineModePanel } from "./PipelineModePanel";
import { graphPreview } from "./pipelineToml";

type FlowNodeData = { label: string };

const nodeTypes = { pipeline: PipelineNode };

/**
 * Local pipeline designer. The canvas shows instances and derived edges.
 * Coordinates are presentation. The page does not call a runtime or resolve
 * a secret.
 */
export function PipelinesPage() {
  const [draft, setDraft] = useState<PipelineDraft>(emptyDraft);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [rejection, setRejection] = useState<string | null>(null);
  const [canUndo, setCanUndo] = useState(false);
  const [mode, setMode] = useState<"graph" | "manual">("graph");
  const [manualToml, setManualToml] = useState("");
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [review, setReview] = useState<ModeReview | null>(null);
  const [serverRev, setServerRev] = useState<PipelineRevisionPayload | null>(null);
  const [serverNote, setServerNote] = useState(
    "A session is required to save a draft. Nothing is deployed.",
  );
  const past = useRef<PipelineDraft[]>([]);
  const selected = draft.instances.find((instance) => instance.id === selectedId) ?? null;
  const catalog = searchPlugins(query);
  const edges = useMemo(() => semanticEdges(draft), [draft]);
  const flowNodes = useMemo<Node<FlowNodeData>[]>(
    () =>
      draft.instances.map((instance) => {
        const position = positionOf(draft, instance.id) ?? { x: 0, y: 0 };
        return {
          id: instance.id,
          type: "pipeline",
          position: { x: position.x, y: position.y },
          selected: instance.id === selectedId,
          data: { label: `${instance.id} ${instance.kind} ${instance.plugin}` },
        };
      }),
    [draft, selectedId],
  );
  const flowEdges = useMemo<Edge[]>(
    () =>
      edges.map((edge) => ({
        id: edge.id,
        source: edge.source,
        target: edge.target,
        label: edge.label,
      })),
    [edges],
  );

  function commit(next: PipelineDraft) {
    past.current = [...past.current, draft].slice(-50);
    setDraft(next);
    setCanUndo(true);
    setRejection(null);
  }

  const undo = useCallback(() => {
    const previous = past.current[past.current.length - 1];
    if (!previous) {
      return;
    }
    past.current = past.current.slice(0, -1);
    setDraft(previous);
    setCanUndo(past.current.length > 0);
    setRejection(null);
    if (selectedId && !previous.instances.some((instance) => instance.id === selectedId)) {
      setSelectedId(null);
    }
  }, [selectedId]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== "z") {
        return;
      }
      if (mode !== "graph") {
        return;
      }
      const target = event.target;
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement
      ) {
        return;
      }
      event.preventDefault();
      undo();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [mode, undo]);

  function confirmReview() {
    if (!review || review.kind !== "proposal") {
      return;
    }
    if (review.to === "manual") {
      setSnapshot({ mode: "graph", draft, preview: review.proposedText });
      setManualToml(review.proposedText);
      setMode("manual");
      past.current = [];
      setCanUndo(false);
    } else {
      setDraft(review.draft);
      setMode("graph");
      setManualToml("");
      setSnapshot(null);
    }
    setReview(null);
    setRejection(null);
  }

  async function saveDraft() {
    if (!hasSessionToken()) {
      return;
    }
    if (mode === "graph" && issues(draft).length > 0) {
      setServerNote("The graph has validation issues. Nothing was saved or deployed.");
      return;
    }
    try {
      let current = serverRev ?? (await createPipelineRevision());
      if (mode === "graph") {
        current = await savePipelineRevision(
          current.pipeline_id,
          current.revision,
          current.generation,
          toContractDocument(draft),
        );
      } else if (snapshot) {
        if (current.mode !== "manual") {
          current = await savePipelineRevision(
            current.pipeline_id,
            current.revision,
            current.generation,
            toContractDocument(snapshot.draft),
          );
          const proposed = await previewPipelineTransition(
            current.pipeline_id,
            current.revision,
            "manual",
          );
          current = await commitPipelineTransition(
            current.pipeline_id,
            current.revision,
            current.generation,
            "manual",
            proposed.digest,
          );
        }
        if (current.export_toml !== manualToml) {
          current = await savePipelineRevision(
            current.pipeline_id,
            current.revision,
            current.generation,
            toManualDocument(draft, manualToml),
          );
        }
      }
      setServerRev(current);
      setServerNote(`Draft revision ${current.revision} saved. Nothing was deployed.`);
    } catch (error) {
      const status = error instanceof ApiError ? error.status : 0;
      setServerNote(`Save failed (${status}). Nothing was deployed.`);
    }
  }

  async function approveDraft() {
    if (!serverRev || serverRev.state !== "draft") {
      return;
    }
    try {
      const approved = await approvePipelineRevision(
        serverRev.pipeline_id,
        serverRev.revision,
        serverRev.generation,
      );
      setServerRev(approved);
      setServerNote(`Revision ${approved.revision} approved. Nothing was deployed.`);
    } catch (error) {
      const status = error instanceof ApiError ? error.status : 0;
      setServerNote(`Approve failed (${status}). Nothing was deployed.`);
    }
  }

  function onNodesChange(changes: NodeChange<Node<FlowNodeData>>[]) {
    setDraft((current) => {
      let next = current;
      for (const change of changes) {
        if (change.type === "position" && change.position) {
          const existing = positionOf(next, change.id);
          if (
            existing &&
            existing.x === change.position.x &&
            existing.y === change.position.y
          ) {
            continue;
          }
          next = moveLayout(next, change.id, change.position.x, change.position.y);
        }
      }
      return next;
    });
  }

  const onConnectEnd: OnConnectEnd = (_event, state) => {
    if ("fromHandle" in state && state.fromHandle) {
      setRejection(rejectDrawnConnection().message);
    }
  };

  const selectedIssues = issues(draft).filter((issue) => issue.instanceId === selectedId);
  const preview = graphPreview(draft);

  return (
    <section aria-labelledby="pipelines-heading">
      <PageHeader
        id="pipelines-heading"
        title="Pipelines"
        scope="Local Telegraf 1.40.1 graph or manual TOML. Canvas position is presentation only. Save and Approve do not deploy."
        status="partial"
      />
      <p role="status" className="mt-4 rounded border border-amber-300 bg-amber-50 p-3 text-sm">
        This designer edits a local graph for Telegraf {draft.telegrafVersion}. It does
        not deploy a runtime, write a historian, or resolve credential values.
      </p>

      <PipelineModePanel
        mode={mode}
        manualToml={manualToml}
        review={review}
        serverNote={serverNote}
        saveDisabled={!hasSessionToken() || (mode === "graph" && issues(draft).length > 0)}
        approveDisabled={!hasSessionToken() || !serverRev || serverRev.state !== "draft"}
        canRestore={snapshot !== null && mode === "manual"}
        onReviewManual={() => setReview(reviewManual(draft))}
        onReviewGraph={() => setReview(reviewGraph(manualToml, snapshot))}
        onReviewRestore={() => setReview(reviewRestore(manualToml, snapshot))}
        onConfirm={confirmReview}
        onCancel={() => setReview(null)}
        onManualChange={setManualToml}
        onSave={() => void saveDraft()}
        onApprove={() => void approveDraft()}
      />

      {mode === "graph" ? (
      <div
        className="mt-4 flex w-full flex-col gap-4 lg:flex-row lg:items-start"
        data-layout="canvas-with-settings"
      >
        <div className="min-w-0 flex-1">
          <h2 className="text-lg font-medium">Catalog</h2>
          <label className="mt-2 block text-sm" htmlFor="plugin-search">
            Search supported plugins
          </label>
          <input
            id="plugin-search"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="mt-1 w-full rounded border border-gray-300 px-2 py-1 text-sm"
          />
          {catalog.length === 0 ? (
            <p className="mt-2 text-sm text-gray-700">No supported plugin matches.</p>
          ) : (
            <ul className="mt-2 flex flex-wrap gap-2">
              {catalog.map((plugin) => (
                <li key={`${plugin.kind}.${plugin.name}`}>
                  <button
                    type="button"
                    className="rounded border border-gray-300 px-2 py-1 text-sm"
                    onClick={() => {
                      const added = addInstance(draft, plugin.kind, plugin.name);
                      if ("error" in added) {
                        setRejection(added.error.message);
                        return;
                      }
                      commit(added.draft);
                      setSelectedId(added.id);
                    }}
                  >
                    Add {plugin.kind} {plugin.name}
                  </button>
                </li>
              ))}
            </ul>
          )}

          <h2 className="mt-4 text-lg font-medium">Canvas</h2>
          <p className="mt-1 text-sm text-gray-700">
            Drawn connections are rejected. Edges follow processor order and
            aggregator fan-out.
          </p>
          {rejection ? (
            <p role="alert" className="mt-2 text-sm text-red-800">
              {rejection}
            </p>
          ) : null}
          <div className="mt-2 h-[30rem] w-full border border-gray-200" style={{ height: "30rem" }}>
            <ReactFlow
              nodes={flowNodes}
              edges={flowEdges}
              nodeTypes={nodeTypes}
              onNodesChange={onNodesChange}
              onNodeClick={(_event, node) => setSelectedId(node.id)}
              onConnectEnd={onConnectEnd}
              isValidConnection={() => false}
              deleteKeyCode={null}
              nodesConnectable
              fitView={false}
              proOptions={{ hideAttribution: false }}
            >
              <Background />
            </ReactFlow>
          </div>

          <h2 className="mt-4 text-lg font-medium">Execution edges</h2>
          {edges.length === 0 ? (
            <p className="mt-1 text-sm text-gray-700">
              No execution edges yet. Instances are not wired together until
              Telegraf has a processor sequence, aggregator fan-out, or output
              delivery to show.
            </p>
          ) : (
            <ul className="mt-1 list-disc pl-5 text-sm" aria-label="Execution edges">
              {edges.map((edge) => (
                <li key={edge.id}>
                  {edge.label}: {edge.source} to {edge.target}
                </li>
              ))}
            </ul>
          )}
          {draft.instances.some((instance) => instance.kind === "aggregator") ? null : (
            <p className="mt-1 text-sm text-gray-700">
              No aggregators are configured, so the second processor pass does not run.
            </p>
          )}

          <h2 className="mt-4 text-lg font-medium">Processor passes</h2>
          <p className="mt-1 text-sm text-gray-700">
            The processor sequence runs before aggregators and again after them
            unless a pass is skipped. Both passes cannot be skipped.
          </p>
          <label className="mt-2 flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={draft.skipProcessorsBeforeAggregators}
              onChange={(event) => {
                const result = setSkip(
                  draft,
                  "skipProcessorsBeforeAggregators",
                  event.target.checked,
                );
                if ("error" in result) {
                  setRejection(result.error.message);
                  return;
                }
                commit(result.draft);
              }}
            />
            Skip processors before aggregators
          </label>
          <label className="mt-2 flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={draft.skipProcessorsAfterAggregators}
              onChange={(event) => {
                const result = setSkip(
                  draft,
                  "skipProcessorsAfterAggregators",
                  event.target.checked,
                );
                if ("error" in result) {
                  setRejection(result.error.message);
                  return;
                }
                commit(result.draft);
              }}
            />
            Skip processors after aggregators
          </label>

          <h2 className="mt-4 text-lg font-medium">TOML preview</h2>
          <p className="mt-1 text-sm text-gray-700">
            Read-only graph preview for Telegraf {draft.telegrafVersion}. Manual TOML is a
            separate mode and is not produced from this canvas. Secret values are not
            included. Nothing is deployed.
          </p>
          {"toml" in preview ? (
            <textarea
              aria-label="Graph TOML preview"
              readOnly
              rows={16}
              className="mt-2 w-full rounded border border-gray-300 px-2 py-1 font-mono text-xs"
              value={preview.toml}
            />
          ) : (
            <ul className="mt-2 list-disc pl-5 text-sm text-red-800" aria-label="TOML preview issues">
              {preview.issues.map((issue) => (
                <li key={`${issue.class}:${issue.message}`}>
                  {issue.class}: {issue.message}
                </li>
              ))}
            </ul>
          )}
        </div>

        <aside className="w-full shrink-0 border border-gray-200 p-3 lg:sticky lg:top-0 lg:w-80">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-lg font-medium">Instances</h2>
            <button
              type="button"
              className="rounded border border-gray-300 px-2 py-1 text-sm disabled:opacity-50"
              onClick={undo}
              disabled={!canUndo}
            >
              Undo
            </button>
          </div>
          {draft.instances.length === 0 ? (
            <p className="mt-2 text-sm text-gray-700">No instances yet.</p>
          ) : (
            <ul className="mt-2 space-y-2">
              {draft.instances.map((instance) => {
                const orderIndex = draft.processorOrder.indexOf(instance.id);
                return (
                  <li key={instance.id} className="rounded border border-gray-200 p-2">
                    <button
                      type="button"
                      className="text-sm font-medium text-blue-800"
                      aria-pressed={instance.id === selectedId}
                      onClick={() => setSelectedId(instance.id)}
                    >
                      Select {instance.id}
                    </button>
                    <p className="text-xs text-gray-600">
                      {instance.kind} {instance.plugin}
                    </p>
                    {instance.kind === "processor" ? (
                      <div className="mt-1 flex gap-2">
                        <button
                          type="button"
                          className="rounded border border-gray-300 px-2 py-1 text-xs disabled:opacity-50"
                          disabled={orderIndex <= 0}
                          onClick={() => {
                            const result = moveProcessor(draft, instance.id, -1);
                            if ("error" in result) {
                              setRejection(result.error.message);
                              return;
                            }
                            commit(result.draft);
                          }}
                        >
                          Move {instance.id} earlier
                        </button>
                        <button
                          type="button"
                          className="rounded border border-gray-300 px-2 py-1 text-xs disabled:opacity-50"
                          disabled={orderIndex === draft.processorOrder.length - 1}
                          onClick={() => {
                            const result = moveProcessor(draft, instance.id, 1);
                            if ("error" in result) {
                              setRejection(result.error.message);
                              return;
                            }
                            commit(result.draft);
                          }}
                        >
                          Move {instance.id} later
                        </button>
                      </div>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}

          <h2 id="node-settings-heading" className="mt-4 text-lg font-medium">
            Node settings
          </h2>
          {selected ? (
            <NodeSettings
              draft={draft}
              instance={selected}
              issues={selectedIssues.map((issue) => issue.message)}
              onCommit={commit}
              onReject={setRejection}
              onDelete={() => {
                commit(deleteInstance(draft, selected.id));
                setSelectedId(null);
              }}
            />
          ) : (
            <p className="mt-2 text-sm text-gray-700">Select an instance to edit its settings.</p>
          )}
        </aside>
      </div>
      ) : null}
      <p className="sr-only">Supported catalog size {PLUGINS.length}</p>
    </section>
  );
}

function PipelineNode({ data }: NodeProps<Node<FlowNodeData>>) {
  return (
    <div className="rounded border border-gray-400 bg-white px-3 py-2 text-xs shadow-sm">
      <Handle type="target" position={Position.Left} />
      {data.label}
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function NodeSettings({
  draft,
  instance,
  issues: instanceIssues,
  onCommit,
  onReject,
  onDelete,
}: {
  draft: PipelineDraft;
  instance: Instance;
  issues: string[];
  onCommit: (next: PipelineDraft) => void;
  onReject: (message: string) => void;
  onDelete: () => void;
}) {
  const spec = findPlugin(instance.kind, instance.plugin);
  const position = positionOf(draft, instance.id);
  return (
    <div className="mt-2 space-y-3 text-sm">
      <p>
        {instance.kind} {instance.plugin}. Position{" "}
        {position ? `${position.x}, ${position.y}` : "unset"} is presentation only.
      </p>
      <div className="flex flex-wrap gap-2">
        <NudgeButton
          label={`Nudge ${instance.id} left`}
          onClick={() => nudge(draft, instance.id, -40, 0, onCommit)}
        />
        <NudgeButton
          label={`Nudge ${instance.id} right`}
          onClick={() => nudge(draft, instance.id, 40, 0, onCommit)}
        />
        <NudgeButton
          label={`Nudge ${instance.id} up`}
          onClick={() => nudge(draft, instance.id, 0, -40, onCommit)}
        />
        <NudgeButton
          label={`Nudge ${instance.id} down`}
          onClick={() => nudge(draft, instance.id, 0, 40, onCommit)}
        />
      </div>
      {spec?.options.map((option) => (
        <OptionEditor
          key={option.key}
          instance={instance}
          option={option}
          onCommit={onCommit}
          onReject={onReject}
          draft={draft}
        />
      ))}
      {instanceIssues.length > 0 ? (
        <ul className="list-disc pl-5 text-red-800">
          {instanceIssues.map((message) => (
            <li key={message}>{message}</li>
          ))}
        </ul>
      ) : null}
      <button
        type="button"
        className="rounded border border-red-300 px-2 py-1 text-red-800"
        onClick={onDelete}
      >
        Delete {instance.id}
      </button>
    </div>
  );
}

function NudgeButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button type="button" className="rounded border border-gray-300 px-2 py-1 text-xs" onClick={onClick}>
      {label}
    </button>
  );
}

function nudge(
  draft: PipelineDraft,
  id: string,
  dx: number,
  dy: number,
  onCommit: (next: PipelineDraft) => void,
) {
  const position = positionOf(draft, id);
  if (!position) {
    return;
  }
  onCommit(moveLayout(draft, id, position.x + dx, position.y + dy));
}

function OptionEditor({
  draft,
  instance,
  option,
  onCommit,
  onReject,
}: {
  draft: PipelineDraft;
  instance: Instance;
  option: OptionSpec;
  onCommit: (next: PipelineDraft) => void;
  onReject: (message: string) => void;
}) {
  const value = instance.options[option.key];
  const label = `${instance.id} ${option.label}`;
  const apply = (next: OptionValue) => {
    const result = setOption(draft, instance.id, option.key, next);
    if ("error" in result) {
      onReject(result.error.message);
      return;
    }
    onCommit(result.draft);
  };

  if (option.kind === "bool") {
    const checked = value?.kind === "bool" ? value.bool : false;
    return (
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={checked}
          onChange={(event) => apply({ kind: "bool", bool: event.target.checked })}
        />
        {label}
      </label>
    );
  }

  if (option.kind === "string" || option.kind === "duration" || option.kind === "secret_ref") {
    const text =
      value?.kind === "string"
        ? value.string
        : value?.kind === "duration"
          ? value.duration
          : value?.kind === "secret_ref"
            ? value.secretRef
            : "";
    return (
      <label className="block">
        {label}
        <input
          aria-label={label}
          className="mt-1 w-full rounded border border-gray-300 px-2 py-1"
          value={text}
          autoComplete={option.kind === "secret_ref" ? "off" : undefined}
          onChange={(event) => {
            const raw = event.target.value;
            if (option.kind === "string") {
              if (!option.required && raw === "") {
                onCommit(clearOption(draft, instance.id, option.key));
                return;
              }
              apply({ kind: "string", string: raw });
              return;
            }
            if (option.kind === "duration") {
              if (!option.required && raw === "") {
                onCommit(clearOption(draft, instance.id, option.key));
                return;
              }
              apply({ kind: "duration", duration: raw });
              return;
            }
            apply({ kind: "secret_ref", secretRef: raw });
          }}
        />
      </label>
    );
  }

  if (option.kind === "integer") {
    const integer = value?.kind === "integer" ? value.integer : option.minInteger ?? 0;
    return (
      <label className="block">
        {label}
        <input
          aria-label={label}
          type="number"
          className="mt-1 w-full rounded border border-gray-300 px-2 py-1"
          value={integer}
          onChange={(event) => {
            if (event.target.value === "") {
              return;
            }
            apply({ kind: "integer", integer: Number(event.target.value) });
          }}
        />
      </label>
    );
  }

  if (option.kind === "string_list") {
    const strings = value?.kind === "string_list" ? value.strings : [];
    return (
      <label className="block">
        {label}
        <textarea
          aria-label={label}
          className="mt-1 w-full rounded border border-gray-300 px-2 py-1"
          rows={3}
          value={strings.join("\n")}
          onChange={(event) => {
            const stringsNext = event.target.value
              .split("\n")
              .map((item) => item.trim())
              .filter((item) => item !== "");
            apply({ kind: "string_list", strings: stringsNext });
          }}
        />
      </label>
    );
  }

  if (option.kind === "string_map") {
    const entries = value?.kind === "string_map" ? Object.entries(value.stringMap) : [];
    return (
      <fieldset className="space-y-2">
        <legend>{label}</legend>
        {entries.map(([key, entry], index) => (
          <div key={`${key}-${index}`} className="flex gap-2">
            <input
              aria-label={`${label} key ${index + 1}`}
              className="w-1/2 rounded border border-gray-300 px-2 py-1"
              value={key}
              onChange={(event) => {
                const next = entries.map((pair, pairIndex) =>
                  pairIndex === index ? [event.target.value, pair[1]] : pair,
                ) as [string, string][];
                apply({ kind: "string_map", stringMap: Object.fromEntries(next) });
              }}
            />
            <input
              aria-label={`${label} value ${index + 1}`}
              className="w-1/2 rounded border border-gray-300 px-2 py-1"
              value={entry}
              onChange={(event) => {
                const next = entries.map((pair, pairIndex) =>
                  pairIndex === index ? [pair[0], event.target.value] : pair,
                ) as [string, string][];
                apply({ kind: "string_map", stringMap: Object.fromEntries(next) });
              }}
            />
          </div>
        ))}
        <button
          type="button"
          className="rounded border border-gray-300 px-2 py-1"
          onClick={() => {
            const nextKey = `tag${entries.length + 1}`;
            apply({
              kind: "string_map",
              stringMap: { ...Object.fromEntries(entries), [nextKey]: "" },
            });
          }}
        >
          Add tag to {instance.id}
        </button>
      </fieldset>
    );
  }

  const rules = value?.kind === "rename_replace" ? value.replaces : [];
  return (
    <fieldset className="space-y-2">
      <legend>{label}</legend>
      {rules.map((rule, index) => (
        <div key={index} className="space-y-1 rounded border border-gray-200 p-2">
          <label className="block">
            Source kind
            <select
              aria-label={`${label} source kind ${index + 1}`}
              className="mt-1 w-full rounded border border-gray-300 px-2 py-1"
              value={rule.sourceKind}
              onChange={(event) => {
                const sourceKind = event.target.value as RenameRule["sourceKind"];
                const next = rules.map((item, itemIndex) =>
                  itemIndex === index ? { ...item, sourceKind } : item,
                );
                apply({ kind: "rename_replace", replaces: next });
              }}
            >
              <option value="measurement">measurement</option>
              <option value="tag">tag</option>
              <option value="field">field</option>
            </select>
          </label>
          <label className="block">
            Source
            <input
              aria-label={`${label} source ${index + 1}`}
              className="mt-1 w-full rounded border border-gray-300 px-2 py-1"
              value={rule.source}
              onChange={(event) => {
                const next = rules.map((item, itemIndex) =>
                  itemIndex === index ? { ...item, source: event.target.value } : item,
                );
                apply({ kind: "rename_replace", replaces: next });
              }}
            />
          </label>
          <label className="block">
            Destination
            <input
              aria-label={`${label} destination ${index + 1}`}
              className="mt-1 w-full rounded border border-gray-300 px-2 py-1"
              value={rule.dest}
              onChange={(event) => {
                const next = rules.map((item, itemIndex) =>
                  itemIndex === index ? { ...item, dest: event.target.value } : item,
                );
                apply({ kind: "rename_replace", replaces: next });
              }}
            />
          </label>
        </div>
      ))}
      <button
        type="button"
        className="rounded border border-gray-300 px-2 py-1"
        onClick={() =>
          apply({
            kind: "rename_replace",
            replaces: [...rules, { sourceKind: "tag", source: "", dest: "" }],
          })
        }
      >
        Add rename rule to {instance.id}
      </button>
    </fieldset>
  );
}
