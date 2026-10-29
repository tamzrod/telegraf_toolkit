import { useState } from "react";
import type { DragEvent } from "react";
import { EmptyState } from "../../components/common/StatePanels";
import { useSchemaFields, useSchemaMeasurements, useSchemaTags } from "../../hooks/useSchemaBrowser";
import type { ApiSchemaKind, ApiSchemaResult, RegistryProfile } from "../../api/registry";
import {
  addFieldRule,
  addFilter,
  addTagRule,
  createEmptyDraft,
  FIELD_TYPES,
  isPlanReady,
  isValidDestinationName,
  planLines,
  removeFieldRule,
  removeFilter,
  removeTagRule,
  updateFieldRule,
  updateFilter,
  updateTagRule,
  validateDraft,
} from "./mappingDraft";
import type { Diagnostic, MappingDraft, SideLoadState, SideView } from "./mappingDraft";
import { profileIdentity, schemaResultProblem } from "./canonicalWorkflow";
import { mockSchemaPreview } from "./mockSchemaPreview";

/**
 * Local mapping-draft editor (P3-004).
 *
 * Holds one draft in component memory only. It is keyed by the source and
 * destination identities by its parent, so a selection or scope change starts a
 * new, empty draft. It reads schema solely through the existing GET hooks and
 * performs no write, no sample query and no save or export.
 *
 * Drag and drop is a convenience: every drop has a keyboard-operable button
 * alternative, and a drop is accepted only for a name observed on the source.
 */

const DROP_PREFIX_FIELD = "field:";
const DROP_PREFIX_TAG = "tag:";

type Load = {
  state: SideLoadState;
  result: ApiSchemaResult | undefined;
  reason: string;
};

function resolveLoad({
  live,
  profile,
  kind,
  measurement,
  enabled,
  data,
  error,
  pending,
}: {
  live: boolean;
  profile: RegistryProfile;
  kind: ApiSchemaKind;
  measurement: string;
  enabled: boolean;
  data: ApiSchemaResult | undefined;
  error: Error | null;
  pending: boolean;
}): Load {
  if (!live) {
    const preview = mockSchemaPreview(profile);
    return { state: preview.outcome, result: preview, reason: "Synthetic mock preview." };
  }
  if (!enabled || pending) {
    return { state: "unloaded", result: undefined, reason: "Not read yet." };
  }
  if (error) {
    return { state: "unknown", result: undefined, reason: "The read failed; the answer is unknown." };
  }
  if (data) {
    const problem = schemaResultProblem(profile, kind, measurement, data);
    if (problem) {
      return { state: "unknown", result: undefined, reason: problem };
    }
    return { state: data.outcome, result: data, reason: "" };
  }
  return { state: "unloaded", result: undefined, reason: "Not read yet." };
}

function toView(
  profile: RegistryProfile,
  measurement: string,
  measurements: Load,
  fields: Load,
  tags: Load,
): SideView {
  return {
    identity: profileIdentity(profile),
    measurementsState: measurements.state,
    measurements: measurements.result?.measurements ?? [],
    measurement,
    fieldsState: fields.state,
    fields: fields.result?.fields ?? [],
    tagsState: tags.state,
    tags: tags.result?.tags ?? [],
  };
}

function stateLabel(state: SideLoadState): string {
  return state === "unloaded" ? "not loaded" : state;
}

function SideStateNotice({ title, load }: { title: string; load: Load }) {
  if (load.state === "supported" || load.reason === "" || load.reason === "Not read yet.") {
    return null;
  }
  return (
    <p className="mt-1 text-xs text-gray-700">
      {title}: {stateLabel(load.state)}. {load.reason}
    </p>
  );
}

export function MappingDraftEditor({
  source,
  destination,
  live,
}: {
  source: RegistryProfile;
  destination: RegistryProfile;
  live: boolean;
}) {
  const sourceIdentity = profileIdentity(source);
  const destinationIdentity = profileIdentity(destination);
  const [draft, setDraft] = useState<MappingDraft>(() =>
    createEmptyDraft(sourceIdentity, destinationIdentity),
  );
  const [notice, setNotice] = useState("");

  // Source: measurement list, then detail reads for the selected measurement.
  const sourceMeasurementsQuery = useSchemaMeasurements(source, live);
  const sourceSelected = draft.sourceMeasurement !== "";
  const sourceFieldsQuery = useSchemaFields(source, draft.sourceMeasurement, live && sourceSelected);
  const sourceTagsQuery = useSchemaTags(source, draft.sourceMeasurement, live && sourceSelected);
  const sourceMeasurementsLoad = resolveLoad({
    live,
    profile: source,
    kind: "measurements",
    measurement: "",
    enabled: true,
    data: sourceMeasurementsQuery.data,
    error: sourceMeasurementsQuery.error,
    pending: sourceMeasurementsQuery.isPending,
  });
  const sourceFieldsLoad = resolveLoad({
    live,
    profile: source,
    kind: "fields",
    measurement: draft.sourceMeasurement,
    enabled: sourceSelected,
    data: sourceFieldsQuery.data,
    error: sourceFieldsQuery.error,
    pending: sourceFieldsQuery.isPending,
  });
  const sourceTagsLoad = resolveLoad({
    live,
    profile: source,
    kind: "tags",
    measurement: draft.sourceMeasurement,
    enabled: sourceSelected,
    data: sourceTagsQuery.data,
    error: sourceTagsQuery.error,
    pending: sourceTagsQuery.isPending,
  });

  // Destination: detail reads only after the typed name is an observed measurement.
  const destinationMeasurementsQuery = useSchemaMeasurements(destination, live);
  const destinationMeasurementsLoad = resolveLoad({
    live,
    profile: destination,
    kind: "measurements",
    measurement: "",
    enabled: true,
    data: destinationMeasurementsQuery.data,
    error: destinationMeasurementsQuery.error,
    pending: destinationMeasurementsQuery.isPending,
  });
  const destinationObserved =
    destinationMeasurementsLoad.state === "supported" &&
    (destinationMeasurementsLoad.result?.measurements ?? []).includes(draft.destinationMeasurement);
  const destinationFieldsQuery = useSchemaFields(
    destination,
    draft.destinationMeasurement,
    live && destinationObserved,
  );
  const destinationTagsQuery = useSchemaTags(
    destination,
    draft.destinationMeasurement,
    live && destinationObserved,
  );
  const destinationFieldsLoad = resolveLoad({
    live,
    profile: destination,
    kind: "fields",
    measurement: draft.destinationMeasurement,
    enabled: destinationObserved,
    data: destinationFieldsQuery.data,
    error: destinationFieldsQuery.error,
    pending: destinationFieldsQuery.isPending,
  });
  const destinationTagsLoad = resolveLoad({
    live,
    profile: destination,
    kind: "tags",
    measurement: draft.destinationMeasurement,
    enabled: destinationObserved,
    data: destinationTagsQuery.data,
    error: destinationTagsQuery.error,
    pending: destinationTagsQuery.isPending,
  });

  const sourceView = toView(
    source,
    draft.sourceMeasurement,
    sourceMeasurementsLoad,
    sourceFieldsLoad,
    sourceTagsLoad,
  );
  const destinationView = toView(
    destination,
    destinationObserved ? draft.destinationMeasurement : "",
    destinationMeasurementsLoad,
    destinationFieldsLoad,
    destinationTagsLoad,
  );

  const diagnostics: Diagnostic[] = validateDraft(draft, sourceView, destinationView, {
    sourceIdentity,
    destinationIdentity,
  });
  const ready = isPlanReady(diagnostics);
  const errorCount = diagnostics.filter((d) => d.severity === "error").length;

  const sourceFieldMap = new Map(sourceView.fields.map((field) => [field.name, field.type]));

  function addFieldFromSource(name: string) {
    const type = sourceFieldMap.get(name);
    if (type === undefined) {
      setNotice(`Field ${JSON.stringify(name)} is not observed on the source measurement.`);
      return;
    }
    setDraft((current) => addFieldRule(current, name, type));
    setNotice(`Added a field mapping for ${JSON.stringify(name)}.`);
  }

  function addTagFromSource(name: string) {
    if (!sourceView.tags.includes(name)) {
      setNotice(`Tag ${JSON.stringify(name)} is not observed on the source measurement.`);
      return;
    }
    setDraft((current) => addTagRule(current, name));
    setNotice(`Added a tag mapping for ${JSON.stringify(name)}.`);
  }

  function addFilterForSource(name: string) {
    if (!sourceView.tags.includes(name)) {
      setNotice(`Tag ${JSON.stringify(name)} is not observed on the source measurement.`);
      return;
    }
    setDraft((current) => addFilter(current, name));
    setNotice(`Added a filter on tag ${JSON.stringify(name)}. Enter its value.`);
  }

  function onDragStartChip(event: DragEvent<HTMLElement>, payload: string) {
    event.dataTransfer.setData("text/plain", payload);
    event.dataTransfer.effectAllowed = "copy";
  }

  function onDropZone(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    const payload = event.dataTransfer.getData("text/plain");
    if (payload.startsWith(DROP_PREFIX_FIELD)) {
      addFieldFromSource(payload.slice(DROP_PREFIX_FIELD.length));
    } else if (payload.startsWith(DROP_PREFIX_TAG)) {
      addTagFromSource(payload.slice(DROP_PREFIX_TAG.length));
    } else {
      setNotice("Only a source field or tag can be dropped here.");
    }
  }

  const sourceMeasurementOptions = sourceView.measurementsState === "supported" ? sourceView.measurements : [];

  return (
    <section className="mt-6" aria-labelledby="mapping-draft-heading">
      <h2 id="mapping-draft-heading" className="text-lg font-medium">
        Mapping draft
      </h2>
      <p className="mt-1 text-sm text-gray-600">
        A local draft over one source and one destination measurement. It lives in
        this page only: nothing is saved, exported, written or executed. Field and
        tag names are checked against the observed schema.
      </p>
      {!live && (
        <p role="status" className="mt-2 rounded border border-blue-300 bg-blue-50 p-2 text-xs text-blue-900">
          <strong>MOCK draft.</strong> Validated against the synthetic preview; no database is contacted.
        </p>
      )}

      <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label htmlFor="draft-source-measurement" className="block text-sm font-medium">
            Source measurement
          </label>
          <select
            id="draft-source-measurement"
            className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
            value={draft.sourceMeasurement}
            onChange={(event) => {
              const next = event.target.value;
              setDraft((current) => ({ ...current, sourceMeasurement: next }));
            }}
          >
            <option value="">Select a source measurement…</option>
            {sourceMeasurementOptions.map((name) => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
          <SideStateNotice title="Source measurements" load={sourceMeasurementsLoad} />
        </div>
        <div>
          <label htmlFor="draft-destination-measurement" className="block text-sm font-medium">
            Destination measurement
          </label>
          <input
            id="draft-destination-measurement"
            type="text"
            autoComplete="off"
            spellCheck={false}
            className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
            value={draft.destinationMeasurement}
            aria-describedby="draft-destination-hint"
            onChange={(event) => {
              const next = event.target.value;
              setDraft((current) => ({ ...current, destinationMeasurement: next }));
            }}
          />
          <p id="draft-destination-hint" className="mt-1 text-xs text-gray-600">
            Letters, digits and underscores; starts with a letter or underscore.
            {!isValidDestinationName(draft.destinationMeasurement) && draft.destinationMeasurement !== ""
              ? " The current name is not valid."
              : ""}
          </p>
          <SideStateNotice title="Destination measurements" load={destinationMeasurementsLoad} />
        </div>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section aria-labelledby="draft-source-keys">
          <h3 id="draft-source-keys" className="text-sm font-medium">
            Source keys
          </h3>
          {!sourceSelected && (
            <p className="mt-1 text-xs text-gray-600">Select a source measurement to list its keys.</p>
          )}
          {sourceSelected && sourceFieldsLoad.state !== "supported" && (
            <SideStateNotice title="Source fields" load={sourceFieldsLoad} />
          )}
          {sourceSelected && sourceFieldsLoad.state === "supported" && (
            <ul aria-label="Source field keys" className="mt-2 space-y-1">
              {sourceView.fields.map((field) => (
                <li key={field.name} className="flex flex-wrap items-center gap-2 text-sm">
                  <span
                    draggable
                    onDragStart={(event) => onDragStartChip(event, `${DROP_PREFIX_FIELD}${field.name}`)}
                    className="cursor-grab rounded border border-gray-400 bg-gray-50 px-2 py-0.5"
                  >
                    {field.name} <span className="text-gray-600">({field.type === "" ? "type not reported" : field.type})</span>
                  </span>
                  <button
                    type="button"
                    className="rounded border border-gray-400 px-2 py-0.5 text-xs"
                    onClick={() => addFieldFromSource(field.name)}
                  >
                    Map field {field.name}
                  </button>
                </li>
              ))}
            </ul>
          )}
          {sourceSelected && sourceTagsLoad.state !== "supported" && (
            <SideStateNotice title="Source tags" load={sourceTagsLoad} />
          )}
          {sourceSelected && sourceTagsLoad.state === "supported" && (
            <ul aria-label="Source tag keys" className="mt-2 space-y-1">
              {sourceView.tags.map((tag) => (
                <li key={tag} className="flex flex-wrap items-center gap-2 text-sm">
                  <span
                    draggable
                    onDragStart={(event) => onDragStartChip(event, `${DROP_PREFIX_TAG}${tag}`)}
                    className="cursor-grab rounded border border-gray-400 bg-gray-50 px-2 py-0.5"
                  >
                    {tag} <span className="text-gray-600">(tag)</span>
                  </span>
                  <button
                    type="button"
                    className="rounded border border-gray-400 px-2 py-0.5 text-xs"
                    onClick={() => addTagFromSource(tag)}
                  >
                    Map tag {tag}
                  </button>
                  <button
                    type="button"
                    className="rounded border border-gray-400 px-2 py-0.5 text-xs"
                    onClick={() => addFilterForSource(tag)}
                  >
                    Filter on tag {tag}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section aria-labelledby="draft-drop-zone-heading">
          <h3 id="draft-drop-zone-heading" className="text-sm font-medium">
            Drop zone
          </h3>
          <div
            role="group"
            aria-labelledby="draft-drop-zone-heading"
            aria-describedby="draft-drop-zone-hint"
            onDragOver={(event) => {
              event.preventDefault();
              event.dataTransfer.dropEffect = "copy";
            }}
            onDrop={onDropZone}
            className="mt-2 min-h-16 rounded border-2 border-dashed border-gray-400 p-3 text-sm text-gray-700"
          >
            Drop a source field or tag here to add a rule.
          </div>
          <p id="draft-drop-zone-hint" className="mt-1 text-xs text-gray-600">
            Keyboard alternative: use the Map buttons beside each source key.
          </p>
        </section>
      </div>

      {notice !== "" && (
        <p role="status" aria-live="polite" className="mt-3 text-sm text-gray-800">
          {notice}
        </p>
      )}

      <section className="mt-4" aria-labelledby="draft-field-rules">
        <h3 id="draft-field-rules" className="text-sm font-medium">
          Field rules
        </h3>
        {draft.fieldRules.length === 0 ? (
          <EmptyState title="No field rules" description="Map a source field to add one." />
        ) : (
          <ul className="mt-2 space-y-3">
            {draft.fieldRules.map((rule) => (
              <li key={rule.id} className="rounded border border-gray-300 p-3 text-sm">
                <p>
                  Source field <strong>{rule.sourceField}</strong>
                </p>
                <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-3">
                  <label className="block">
                    <span className="block text-xs font-medium">Destination field name</span>
                    <input
                      type="text"
                      spellCheck={false}
                      autoComplete="off"
                      aria-label={`Destination name for field ${rule.sourceField}`}
                      className="mt-1 w-full rounded border border-gray-300 p-1"
                      value={rule.destinationField}
                      onChange={(event) => {
                        const next = event.target.value;
                        setDraft((current) => updateFieldRule(current, rule.id, { destinationField: next }));
                      }}
                    />
                  </label>
                  <label className="block">
                    <span className="block text-xs font-medium">Destination type</span>
                    <select
                      aria-label={`Destination type for field ${rule.sourceField}`}
                      className="mt-1 w-full rounded border border-gray-300 p-1"
                      value={rule.destinationType}
                      onChange={(event) => {
                        const next = event.target.value;
                        setDraft((current) => updateFieldRule(current, rule.id, { destinationType: next }));
                      }}
                    >
                      {FIELD_TYPES.map((type) => (
                        <option key={type} value={type}>
                          {type}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block">
                    <span className="block text-xs font-medium">Transform</span>
                    <select
                      aria-label={`Transform for field ${rule.sourceField}`}
                      className="mt-1 w-full rounded border border-gray-300 p-1"
                      value={rule.transform.kind}
                      onChange={(event) => {
                        const kind = event.target.value;
                        setDraft((current) =>
                          updateFieldRule(current, rule.id, {
                            transform:
                              kind === "unit"
                                ? { kind: "unit", factor: "", fromUnit: "", toUnit: "" }
                                : { kind: "none" },
                          }),
                        );
                      }}
                    >
                      <option value="none">None (types must match)</option>
                      <option value="unit">Explicit unit transform</option>
                    </select>
                  </label>
                </div>
                {rule.transform.kind === "unit" && (
                  <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-3">
                    <label className="block">
                      <span className="block text-xs font-medium">Factor</span>
                      <input
                        type="text"
                        inputMode="decimal"
                        autoComplete="off"
                        aria-label={`Conversion factor for field ${rule.sourceField}`}
                        className="mt-1 w-full rounded border border-gray-300 p-1"
                        value={rule.transform.factor}
                        onChange={(event) => {
                          const factor = event.target.value;
                          setDraft((current) =>
                            updateFieldRule(current, rule.id, {
                              transform: { ...(rule.transform as Extract<typeof rule.transform, { kind: "unit" }>), factor },
                            }),
                          );
                        }}
                      />
                    </label>
                    <label className="block">
                      <span className="block text-xs font-medium">From unit</span>
                      <input
                        type="text"
                        autoComplete="off"
                        aria-label={`From unit for field ${rule.sourceField}`}
                        className="mt-1 w-full rounded border border-gray-300 p-1"
                        value={rule.transform.fromUnit}
                        onChange={(event) => {
                          const fromUnit = event.target.value;
                          setDraft((current) =>
                            updateFieldRule(current, rule.id, {
                              transform: { ...(rule.transform as Extract<typeof rule.transform, { kind: "unit" }>), fromUnit },
                            }),
                          );
                        }}
                      />
                    </label>
                    <label className="block">
                      <span className="block text-xs font-medium">To unit</span>
                      <input
                        type="text"
                        autoComplete="off"
                        aria-label={`To unit for field ${rule.sourceField}`}
                        className="mt-1 w-full rounded border border-gray-300 p-1"
                        value={rule.transform.toUnit}
                        onChange={(event) => {
                          const toUnit = event.target.value;
                          setDraft((current) =>
                            updateFieldRule(current, rule.id, {
                              transform: { ...(rule.transform as Extract<typeof rule.transform, { kind: "unit" }>), toUnit },
                            }),
                          );
                        }}
                      />
                    </label>
                  </div>
                )}
                <button
                  type="button"
                  className="mt-2 rounded border border-gray-400 px-2 py-0.5 text-xs"
                  onClick={() => setDraft((current) => removeFieldRule(current, rule.id))}
                >
                  Remove field rule {rule.sourceField}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="mt-4" aria-labelledby="draft-tag-rules">
        <h3 id="draft-tag-rules" className="text-sm font-medium">
          Tag rules and filters
        </h3>
        {draft.tagRules.length === 0 && draft.filters.length === 0 ? (
          <EmptyState title="No tag rules or filters" description="Map a source tag or filter on one to add one." />
        ) : (
          <>
            {draft.tagRules.length > 0 && (
              <ul className="mt-2 space-y-2">
                {draft.tagRules.map((rule) => (
                  <li key={rule.id} className="flex flex-wrap items-end gap-2 text-sm">
                    <span>
                      Tag <strong>{rule.sourceTag}</strong>
                    </span>
                    <label className="block">
                      <span className="block text-xs font-medium">Destination tag name</span>
                      <input
                        type="text"
                        spellCheck={false}
                        autoComplete="off"
                        aria-label={`Destination name for tag ${rule.sourceTag}`}
                        className="mt-1 rounded border border-gray-300 p-1"
                        value={rule.destinationTag}
                        onChange={(event) => {
                          const next = event.target.value;
                          setDraft((current) => updateTagRule(current, rule.id, { destinationTag: next }));
                        }}
                      />
                    </label>
                    <button
                      type="button"
                      className="rounded border border-gray-400 px-2 py-0.5 text-xs"
                      onClick={() => setDraft((current) => removeTagRule(current, rule.id))}
                    >
                      Remove tag rule {rule.sourceTag}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {draft.filters.length > 0 && (
              <ul className="mt-2 space-y-2">
                {draft.filters.map((filter) => (
                  <li key={filter.id} className="flex flex-wrap items-end gap-2 text-sm">
                    <span>
                      Filter tag <strong>{filter.tagKey}</strong> equals
                    </span>
                    <label className="block">
                      <span className="block text-xs font-medium">Filter value</span>
                      <input
                        type="text"
                        spellCheck={false}
                        autoComplete="off"
                        aria-label={`Filter value for tag ${filter.tagKey}`}
                        className="mt-1 rounded border border-gray-300 p-1"
                        value={filter.value}
                        onChange={(event) => {
                          const next = event.target.value;
                          setDraft((current) => updateFilter(current, filter.id, { value: next }));
                        }}
                      />
                    </label>
                    <button
                      type="button"
                      className="rounded border border-gray-400 px-2 py-0.5 text-xs"
                      onClick={() => setDraft((current) => removeFilter(current, filter.id))}
                    >
                      Remove filter on {filter.tagKey}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </section>

      <section className="mt-4" aria-labelledby="draft-diagnostics">
        <h3 id="draft-diagnostics" className="text-sm font-medium">
          Validation
        </h3>
        <p role="status" className="mt-1 text-sm">
          {ready
            ? "Plan ready for preview. Warnings below do not block it."
            : `Plan blocked: ${errorCount} error${errorCount === 1 ? "" : "s"}.`}
        </p>
        {diagnostics.length > 0 && (
          <ul aria-label="Draft diagnostics" className="mt-2 space-y-1 text-sm">
            {diagnostics.map((diagnostic) => (
              <li
                key={`${diagnostic.severity}|${diagnostic.code}|${diagnostic.subject}`}
                className={diagnostic.severity === "error" ? "text-red-800" : "text-amber-900"}
              >
                <strong>{diagnostic.severity === "error" ? "Error" : "Warning"}</strong>{" "}
                <code>{diagnostic.code}</code>: {diagnostic.message}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="mt-4" aria-labelledby="draft-plan-preview">
        <h3 id="draft-plan-preview" className="text-sm font-medium">
          Plan preview
        </h3>
        {ready ? (
          <>
            <p className="mt-1 text-xs text-gray-600">
              Text description only. Nothing is executed, written or deployed; names and values are shown as quoted data.
            </p>
            <ol aria-label="Mapping plan lines" className="mt-2 list-decimal space-y-1 pl-5 text-sm">
              {planLines(draft).map((line, index) => (
                <li key={`${index}-${line}`}>{line}</li>
              ))}
            </ol>
          </>
        ) : (
          <p className="mt-1 text-sm text-gray-700">No preview while the plan is blocked.</p>
        )}
      </section>
    </section>
  );
}
