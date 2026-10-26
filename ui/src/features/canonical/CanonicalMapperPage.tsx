import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  DisabledActionGroup,
  PageHeader,
} from "../../components/common/ModulePageParts";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  MockDataNotice,
} from "../../components/common/StatePanels";
import { useLiveRegistryProfiles } from "../../hooks/useLiveRegistry";
import { useRegistryProfiles } from "../../hooks/useRegistryProfiles";
import {
  useSchemaFields,
  useSchemaMeasurements,
  useSchemaTags,
} from "../../hooks/useSchemaBrowser";
import { clearSessionToken, hasSessionToken, setSessionToken } from "../../api/session";
import { formatScope } from "../../api/registry";
import type {
  ApiSchemaKind,
  ApiSchemaResult,
  CapabilityOutcome,
  RegistryProfile,
} from "../../api/registry";
import { queryKeys } from "../../api/queryKeys";
import {
  describeSchemaFailure,
  isSchemaSupported,
  profileIdentity,
  schemaOutcomeLabel,
  schemaResultProblem,
  validateMapperSelection,
} from "./canonicalWorkflow";
import { mockSchemaPreview } from "./mockSchemaPreview";

const OUTCOME_CLASSES: Record<CapabilityOutcome, string> = {
  supported: "bg-green-100 text-green-900",
  unsupported: "bg-gray-200 text-gray-700",
  unknown: "bg-amber-100 text-amber-900",
};

function SchemaOutcomeBadge({ outcome }: { outcome: CapabilityOutcome }) {
  return (
    <span className={`rounded px-2 py-0.5 text-xs font-medium ${OUTCOME_CLASSES[outcome]}`}>
      {schemaOutcomeLabel(outcome)}
    </span>
  );
}

function SchemaEvidence({ result }: { result: ApiSchemaResult }) {
  if (result.observations.length === 0) {
    return null;
  }
  return (
    <details className="mt-2 text-xs text-gray-600">
      <summary>
        Read evidence ({result.observations.length} observation
        {result.observations.length === 1 ? "" : "s"})
      </summary>
      <ul className="mt-1 space-y-1">
        {result.observations.map((observation, index) => (
          <li key={`${observation.check}-${index}`}>
            <code>{observation.check}</code>
            {observation.method !== "none" ? ` — ${observation.method}` : ""}
            {observation.path !== "" ? ` ${observation.path}` : ""}
            {` — status ${observation.status}, ${schemaOutcomeLabel(observation.outcome)}. `}
            {observation.detail}
          </li>
        ))}
      </ul>
    </details>
  );
}

function OutcomeNotice({
  result,
  subject,
}: {
  result: ApiSchemaResult;
  subject: string;
}) {
  return (
    <div
      role="status"
      className="mt-2 rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900"
    >
      <p className="flex flex-wrap items-center gap-2 font-medium">
        <SchemaOutcomeBadge outcome={result.outcome} />
        <span>{subject}</span>
      </p>
      <p className="mt-1">
        {result.outcome === "unknown"
          ? "The Registry did not observe support for this metadata. Unknown is not a supported result and no data is shown."
          : "The deployment did not support this metadata read. No data is shown."}
      </p>
      {result.edition !== "" && (
        <p className="mt-1">Observed edition: {result.edition}.</p>
      )}
      <SchemaEvidence result={result} />
    </div>
  );
}

function StaleNotice({
  problem,
  onRetry,
}: {
  problem: string;
  onRetry?: () => void;
}) {
  return (
    <div role="alert" className="mt-2 rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800">
      <p className="font-medium">Stale schema answer</p>
      <p className="mt-1">
        {problem} The answer is not shown for the current selection.
      </p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-2 rounded border border-red-400 bg-white px-3 py-1 text-sm text-red-800 hover:bg-red-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-600"
        >
          Reload schema
        </button>
      )}
    </div>
  );
}

function FieldTable({ result }: { result: ApiSchemaResult }) {
  if (result.fields.length === 0) {
    return (
      <EmptyState
        title="No field keys observed"
        description="The deployment returned an empty field set for this measurement."
      />
    );
  }
  return (
    <table className="mt-2 w-full text-left text-sm">
      <caption className="text-left text-xs text-gray-600">
        Observed field keys and the type reported for each
      </caption>
      <thead>
        <tr>
          <th scope="col" className="border-b border-gray-300 py-1 pr-3 font-medium">
            Field key
          </th>
          <th scope="col" className="border-b border-gray-300 py-1 font-medium">
            Type
          </th>
        </tr>
      </thead>
      <tbody>
        {result.fields.map((field) => (
          <tr key={field.name}>
            <td className="border-b border-gray-200 py-1 pr-3">{field.name}</td>
            <td className="border-b border-gray-200 py-1 text-gray-700">
              {field.type === "" ? "not reported" : field.type}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function TagList({ result }: { result: ApiSchemaResult }) {
  if (result.tags.length === 0) {
    return (
      <EmptyState
        title="No tag keys observed"
        description="The deployment returned an empty tag-key set for this measurement."
      />
    );
  }
  return (
    <ul aria-label="Observed tag keys" className="mt-2 flex flex-wrap gap-2">
      {result.tags.map((tag) => (
        <li
          key={tag}
          className="rounded border border-gray-300 bg-gray-50 px-2 py-0.5 text-sm"
        >
          {tag}
        </li>
      ))}
    </ul>
  );
}

type SchemaDetailKind = Extract<ApiSchemaKind, "fields" | "tags">;

function SchemaDetailSection({
  headingId,
  heading,
  kind,
  profile,
  measurement,
  live,
  isPending,
  error,
  result,
  onRetry,
  children,
}: {
  headingId: string;
  heading: string;
  kind: SchemaDetailKind;
  profile: RegistryProfile;
  measurement: string;
  live: boolean;
  isPending: boolean;
  error: Error | null;
  result: ApiSchemaResult | undefined;
  onRetry: () => void;
  children: (result: ApiSchemaResult) => ReactNode;
}) {
  const problem =
    result && live ? schemaResultProblem(profile, kind, measurement, result) : null;

  let content: ReactNode = null;
  if (live && isPending) {
    content = <LoadingState label={`Reading ${heading.toLowerCase()}…`} />;
  } else if (live && error) {
    content = (
      <ErrorState
        title={`${heading} unavailable`}
        description={describeSchemaFailure(error)}
        onRetry={onRetry}
      />
    );
  } else if (problem) {
    content = <StaleNotice problem={problem} onRetry={onRetry} />;
  } else if (result) {
    content =
      result.outcome === "supported" ? (
        children(result)
      ) : (
        <OutcomeNotice result={result} subject={heading.toLowerCase()} />
      );
  }

  const showEvidence =
    result !== undefined &&
    problem === null &&
    result.outcome === "supported" &&
    !(live && (isPending || error !== null));

  return (
    <section className="mt-4" aria-labelledby={headingId}>
      <h4 id={headingId} className="text-sm font-medium">
        {heading}
      </h4>
      {content}
      {showEvidence && result && <SchemaEvidence result={result} />}
    </section>
  );
}

function SchemaPanel({
  side,
  profile,
  live,
}: {
  side: "source" | "destination";
  profile: RegistryProfile;
  live: boolean;
}) {
  const panelId = `schema-${side}`;
  const measurementsHeadingId = `${panelId}-measurements`;
  const fieldsHeadingId = `${panelId}-fields`;
  const tagsHeadingId = `${panelId}-tags`;
  const sideLabel = side === "source" ? "Source" : "Destination";

  const [requestedMeasurement, setRequestedMeasurement] = useState("");
  const measurementsQuery = useSchemaMeasurements(profile, live);
  const preview = live ? null : mockSchemaPreview(profile);
  const measurementResult = live ? measurementsQuery.data : preview;

  const measurementProblem =
    live && measurementResult
      ? schemaResultProblem(profile, "measurements", "", measurementResult)
      : null;
  const trustedNames =
    measurementResult &&
    measurementProblem === null &&
    isSchemaSupported(measurementResult.outcome)
      ? measurementResult.measurements
      : [];
  const selectedMeasurement = trustedNames.includes(requestedMeasurement)
    ? requestedMeasurement
    : "";

  const fieldsQuery = useSchemaFields(
    profile,
    selectedMeasurement,
    live && selectedMeasurement !== "",
  );
  const tagsQuery = useSchemaTags(
    profile,
    selectedMeasurement,
    live && selectedMeasurement !== "",
  );

  let measurementContent: ReactNode = null;
  if (live && measurementsQuery.isPending) {
    measurementContent = <LoadingState label="Reading measurements…" />;
  } else if (live && measurementsQuery.isError) {
    measurementContent = (
      <ErrorState
        title="Schema measurements unavailable"
        description={describeSchemaFailure(measurementsQuery.error)}
        onRetry={() => void measurementsQuery.refetch()}
      />
    );
  } else if (measurementProblem) {
    measurementContent = (
      <StaleNotice
        problem={measurementProblem}
        onRetry={() => void measurementsQuery.refetch()}
      />
    );
  } else if (measurementResult && !isSchemaSupported(measurementResult.outcome)) {
    measurementContent = (
      <OutcomeNotice result={measurementResult} subject="measurements" />
    );
  } else if (measurementResult) {
    measurementContent =
      measurementResult.measurements.length === 0 ? (
        <EmptyState
          title="No measurements observed"
          description="The deployment returned an empty measurement set. This is an observed empty schema, not a failed read."
        />
      ) : (
        <>
          {measurementResult.truncated && (
            <p
              role="status"
              className="mt-2 rounded border border-amber-300 bg-amber-50 p-2 text-xs text-amber-900"
            >
              The server truncated the measurement list at its result cap.
            </p>
          )}
          <ul
            aria-labelledby={measurementsHeadingId}
            className="mt-2 flex flex-wrap gap-2"
          >
            {measurementResult.measurements.map((name) => (
              <li key={name}>
                <button
                  type="button"
                  aria-pressed={selectedMeasurement === name}
                  className={`rounded border px-2 py-1 text-sm ${
                    selectedMeasurement === name
                      ? "border-blue-600 bg-blue-100 text-blue-900"
                      : "border-gray-400 bg-white text-gray-800"
                  }`}
                  onClick={() => setRequestedMeasurement(name)}
                >
                  {name}
                </button>
              </li>
            ))}
          </ul>
        </>
      );
  }

  const showMeasurementEvidence =
    measurementResult !== null &&
    measurementResult !== undefined &&
    measurementProblem === null &&
    isSchemaSupported(measurementResult.outcome) &&
    !(live && (measurementsQuery.isPending || measurementsQuery.isError));

  return (
    <article className="rounded border border-gray-300 p-4" aria-labelledby={`${panelId}-heading`}>
      <div className="flex flex-wrap items-center gap-2">
        <h3 id={`${panelId}-heading`} className="text-base font-medium">
          {sideLabel} schema — {profile.displayName}
        </h3>
        <span
          className={`rounded px-2 py-0.5 text-xs font-medium ${
            live
              ? "bg-green-100 text-green-900"
              : "bg-blue-100 text-blue-900"
          }`}
        >
          {live ? "LIVE" : "MOCK"}
        </span>
      </div>
      <p className="mt-1 text-sm text-gray-600">
        Connection {profile.connectionId} · {profile.family} {profile.majorVersion}.x
        {" · "}scope {formatScope(profile.scope)}
      </p>
      <p className="mt-1 text-xs text-gray-600">
        Credentials are resolved server-side. This page receives no credential
        value or reference.
      </p>
      {!live && (
        <p role="status" className="mt-2 rounded border border-blue-300 bg-blue-50 p-2 text-xs text-blue-900">
          <strong>Mock preview.</strong> The metadata below is synthetic. No
          endpoint is contacted, and this preview is never shown in live mode.
        </p>
      )}

      <section className="mt-4" aria-labelledby={measurementsHeadingId}>
        <h4 id={measurementsHeadingId} className="text-sm font-medium">
          Measurements
        </h4>
        {measurementContent}
        {measurementResult && (
          <p className="mt-2 text-xs text-gray-600">
            Observed edition:{" "}
            <strong>{measurementResult.edition === "" ? "unknown" : measurementResult.edition}</strong>
            {" · "}
            {measurementResult.requests === 0
              ? "no request made"
              : `${measurementResult.requests} read request${
                  measurementResult.requests === 1 ? "" : "s"
                }`}
            .
          </p>
        )}
        {showMeasurementEvidence && measurementResult && (
          <SchemaEvidence result={measurementResult} />
        )}
      </section>

      {selectedMeasurement !== "" && (
        <>
          <SchemaDetailSection
            headingId={fieldsHeadingId}
            heading="Fields"
            kind="fields"
            profile={profile}
            measurement={selectedMeasurement}
            live={live}
            isPending={fieldsQuery.isPending}
            error={fieldsQuery.error}
            result={live ? fieldsQuery.data : preview ?? undefined}
            onRetry={() => void fieldsQuery.refetch()}
          >
            {(result) => <FieldTable result={result} />}
          </SchemaDetailSection>

          <SchemaDetailSection
            headingId={tagsHeadingId}
            heading="Tags"
            kind="tags"
            profile={profile}
            measurement={selectedMeasurement}
            live={live}
            isPending={tagsQuery.isPending}
            error={tagsQuery.error}
            result={live ? tagsQuery.data : preview ?? undefined}
            onRetry={() => void tagsQuery.refetch()}
          >
            {(result) => <TagList result={result} />}
          </SchemaDetailSection>
        </>
      )}

      {selectedMeasurement === "" && trustedNames.length > 0 && (
        <p className="mt-3 text-xs text-gray-600">
          Select a measurement to inspect its field keys and tag keys.
        </p>
      )}
    </article>
  );
}

/**
 * Canonical Mapper — live dual-profile selection and typed schema browser.
 *
 * The only data this page reads is the authorized P3-002 GET metadata path:
 * measurements, field keys and tag keys for a stored profile. The browser
 * sends a connection ID and (after an observed measurement is selected) that
 * measurement name; it never sends an endpoint, a credential or a query.
 *
 * Mock mode is explicitly labeled and uses a synthetic preview. A live failure
 * never falls back to it.
 */
export function CanonicalMapperPage() {
  const [useLive, setUseLive] = useState(false);
  const [tokenInput, setTokenInput] = useState("");
  const [tokenPresent, setTokenPresent] = useState(hasSessionToken());
  const [tokenNotice, setTokenNotice] = useState("");
  const [sourceId, setSourceId] = useState("");
  const [destinationId, setDestinationId] = useState("");

  const mockQuery = useRegistryProfiles();
  const liveQuery = useLiveRegistryProfiles(useLive);
  const active = useLive ? liveQuery : mockQuery;
  const profiles = useMemo(() => active.data ?? [], [active.data]);

  const byId = useMemo(() => {
    const map = new Map<string, RegistryProfile>();
    for (const profile of profiles) {
      map.set(profile.connectionId, profile);
    }
    return map;
  }, [profiles]);

  const source = sourceId === "" ? null : byId.get(sourceId) ?? null;
  const destination =
    destinationId === "" ? null : byId.get(destinationId) ?? null;
  const selection = validateMapperSelection(source, destination);
  const queryClient = useQueryClient();

  const sourceMissing = sourceId !== "" && source === null;
  const destinationMissing = destinationId !== "" && destination === null;

  function clearSelection() {
    setSourceId("");
    setDestinationId("");
  }

  function dropSchemaQueries() {
    // Cancel first so an in-flight schema read cannot resolve after a token,
    // mode or selection change and then write into a live cache entry.
    void queryClient
      .cancelQueries({ queryKey: queryKeys.schema() })
      .then(() => {
        queryClient.removeQueries({ queryKey: queryKeys.schema() });
      });
  }

  function switchMode(nextLive: boolean) {
    if (nextLive === useLive) {
      return;
    }
    setUseLive(nextLive);
    clearSelection();
    dropSchemaQueries();
  }

  function selectProfile(side: "source" | "destination", connectionId: string) {
    const previous = side === "source" ? source : destination;
    if (side === "source") {
      setSourceId(connectionId);
    } else {
      setDestinationId(connectionId);
    }
    if (previous && previous.connectionId !== connectionId) {
      // Cancel and drop the previous side's schema reads. Its identity no
      // longer matches the selection, so its cached answers must not be reused
      // and an in-flight answer must not resolve into the new view.
      const previousId = previous.connectionId;
      const matchesPrevious = (query: { queryKey: readonly unknown[] }) =>
        query.queryKey.includes(previousId);
      void queryClient
        .cancelQueries({ predicate: matchesPrevious })
        .then(() => {
          queryClient.removeQueries({ predicate: matchesPrevious });
        });
    }
  }

  function applyToken() {
    const candidate = tokenInput;
    setTokenInput("");
    if (candidate.trim() === "") {
      setTokenNotice("Enter a session token before applying it.");
      return;
    }
    setSessionToken(candidate);
    setTokenPresent(true);
    setTokenNotice(
      "A session token is held for this session only. It is not stored in the browser.",
    );
    clearSelection();
    dropSchemaQueries();
    void liveQuery.refetch();
  }

  function clearToken() {
    clearSessionToken();
    setTokenPresent(false);
    setTokenNotice("No session token is held. Live reads answer 401.");
    clearSelection();
    dropSchemaQueries();
    void liveQuery.refetch();
  }

  return (
    <section aria-labelledby="canonical-mapper-heading">
      <PageHeader
        id="canonical-mapper-heading"
        title="Canonical Mapper"
        scope="Read-only schema browsing for an explicit source and destination profile; mapping and writes remain unimplemented."
        status="partial"
      />

      <section className="mt-6 rounded border border-gray-300 p-4" aria-labelledby="canonical-mode">
        <h2 id="canonical-mode" className="text-lg font-medium">
          Data source
        </h2>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <button
            type="button"
            className="rounded border border-gray-400 px-3 py-1 text-sm"
            aria-pressed={!useLive}
            onClick={() => switchMode(false)}
          >
            Mock adapter
          </button>
          <button
            type="button"
            className="rounded border border-gray-400 px-3 py-1 text-sm"
            aria-pressed={useLive}
            onClick={() => switchMode(true)}
          >
            Live Registry API
          </button>
        </div>

        {useLive && (
          <section className="mt-4" aria-labelledby="canonical-token">
            <h3 id="canonical-token" className="text-base font-medium">
              Session token
            </h3>
            <p className="mt-1 text-sm text-gray-600">
              The token is held in memory only. It is never written to browser
              storage, the URL or a log, and it is not echoed back into this page.
              A live schema read also requires <code>schema.read</code>.
            </p>
            <div className="mt-2 flex flex-wrap items-end gap-2">
              <div>
                <label htmlFor="canonical-token-input" className="block text-sm font-medium">
                  Session token value
                </label>
                <input
                  id="canonical-token-input"
                  type="password"
                  autoComplete="off"
                  className="mt-1 rounded border border-gray-300 p-2 text-sm"
                  value={tokenInput}
                  onChange={(event) => setTokenInput(event.target.value)}
                  aria-describedby="canonical-token-status"
                />
              </div>
              <button
                type="button"
                className="rounded border border-gray-500 px-3 py-1 text-sm"
                onClick={applyToken}
              >
                Use token
              </button>
              <button
                type="button"
                className="rounded border border-gray-400 px-3 py-1 text-sm"
                onClick={clearToken}
                disabled={!tokenPresent}
              >
                Clear token
              </button>
            </div>
            <p
              id="canonical-token-status"
              role="status"
              aria-live="polite"
              className="mt-2 text-sm text-gray-700"
            >
              {tokenNotice !== ""
                ? tokenNotice
                : tokenPresent
                  ? "A session token is held for this session."
                  : "No session token is held. Live requests answer 401 until one is applied."}
            </p>
          </section>
        )}

        {useLive ? (
          <p role="status" className="mt-4 rounded border border-blue-300 bg-blue-50 p-3 text-sm">
            <strong>Live Registry API.</strong> Profiles and metadata come from
            the authorized backend. Every schema read is a GET that the server
            resolves from the stored profile; a failed or unknown read is shown
            as such and is never replaced with mock data.
          </p>
        ) : (
          <p role="status" className="mt-4 rounded border border-amber-300 bg-amber-50 p-3 text-sm">
            <strong>Mock workflow.</strong> Profiles and schema are synthetic and
            labeled MOCK. No database is contacted and no credential is resolved.
            This preview is never used as a live fallback.
          </p>
        )}
      </section>

      <section className="mt-6" aria-labelledby="canonical-profiles">
        <h2 id="canonical-profiles" className="text-lg font-medium">
          Connection profiles
        </h2>

        {active.isPending && <LoadingState label="Loading registry profiles…" />}

        {active.isError && (
          <ErrorState
            title="Registry profiles unavailable"
            description={
              useLive
                ? "The live Registry API did not accept the request. A valid session token with profile.read is required; this is not a mock fallback."
                : "The typed registry endpoint did not respond. This is not a live connection."
            }
            onRetry={() => void active.refetch()}
          />
        )}

        {active.data && profiles.length === 0 && (
          <EmptyState
            title="No connection profiles"
            description={
              useLive
                ? "No profile is persisted in the Registry yet."
                : "No mock profile is registered in this shell."
            }
          />
        )}

        {active.data && profiles.length > 0 && (
          <>
            {!useLive && <MockDataNotice subject="connection profile list" />}
            <p className="mt-2 text-sm text-gray-700">
              {profiles.length} connection profile
              {profiles.length === 1 ? "" : "s"} available. Selecting a profile
              sends only its stored connection ID.
            </p>
          </>
        )}
      </section>

      <section className="mt-6" aria-labelledby="canonical-selection">
        <h2 id="canonical-selection" className="text-lg font-medium">
          Source and destination
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Both sides must be chosen explicitly and must be distinct before any
          schema is browsed. The mapper never infers a side or a scope.
        </p>

        <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label htmlFor="canonical-source" className="block text-sm font-medium">
              Source profile
            </label>
            <select
              id="canonical-source"
              className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
              value={sourceId}
              onChange={(event) => selectProfile("source", event.target.value)}
              disabled={profiles.length === 0}
            >
              <option value="">Select a source…</option>
              {profiles.map((profile) => (
                <option key={profile.connectionId} value={profile.connectionId}>
                  {profile.displayName} — {formatScope(profile.scope)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="canonical-destination" className="block text-sm font-medium">
              Destination profile
            </label>
            <select
              id="canonical-destination"
              className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
              value={destinationId}
              onChange={(event) => selectProfile("destination", event.target.value)}
              disabled={profiles.length === 0}
            >
              <option value="">Select a destination…</option>
              {profiles.map((profile) => (
                <option key={profile.connectionId} value={profile.connectionId}>
                  {profile.displayName} — {formatScope(profile.scope)}
                </option>
              ))}
            </select>
          </div>
        </div>

        {(sourceMissing || destinationMissing) && (
          <p role="alert" className="mt-2 rounded border border-red-300 bg-red-50 p-2 text-sm text-red-800">
            The selected{" "}
            {sourceMissing && destinationMissing
              ? "source and destination profiles are"
              : sourceMissing
                ? "source profile is"
                : "destination profile is"}{" "}
            no longer present in the Registry. Choose another profile.
          </p>
        )}

        {!selection.ok && !(sourceMissing || destinationMissing) && (
          <p role="alert" className="mt-2 rounded border border-red-300 bg-red-50 p-2 text-sm text-red-800">
            {selection.reason}
          </p>
        )}

        {selection.ok && source && destination && (
          <p role="status" className="mt-2 rounded border border-green-300 bg-green-50 p-2 text-sm text-green-900">
            Source <strong>{source.displayName}</strong> and destination{" "}
            <strong>{destination.displayName}</strong> are distinct and selected.
          </p>
        )}
      </section>

      {selection.ok && source && destination && (
        <section className="mt-6" aria-labelledby="canonical-schema-browsers">
          <h2 id="canonical-schema-browsers" className="text-lg font-medium">
            Schema browsers
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            Measurements, field keys and tag keys are shown as separate typed
            groups. A field or tag read is issued only after a measurement has
            been observed and selected.
          </p>
          <div className="mt-3 grid grid-cols-1 gap-4 xl:grid-cols-2">
            <SchemaPanel
              key={`source:${profileIdentity(source)}`}
              side="source"
              profile={source}
              live={useLive}
            />
            <SchemaPanel
              key={`destination:${profileIdentity(destination)}`}
              side="destination"
              profile={destination}
              live={useLive}
            />
          </div>
        </section>
      )}

      <section className="mt-6" aria-labelledby="canonical-bounds">
        <h2 id="canonical-bounds" className="text-lg font-medium">
          Read-only bounds
        </h2>
        <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-gray-700">
          <li>
            Only fixed GET statements are executed: <code>SHOW MEASUREMENTS</code>,{" "}
            <code>SHOW FIELD KEYS FROM …</code> and <code>SHOW TAG KEYS FROM …</code>.
            There is no caller-supplied query, POST, DDL, <code>SELECT … INTO</code>{" "}
            or write.
          </li>
          <li>
            Reads are bounded server-side (15 s total, 5 s per request, 8 requests,
            1 000 rows and 1 MiB); truncation is reported explicitly.
          </li>
          <li>
            A 401, 403, 404, timeout, malformed answer or unrecognized body stays{" "}
            <strong>unknown</strong>; it is never rendered as supported or as an
            empty schema.
          </li>
          <li>
            InfluxDB 1.x metadata remains unverified. A 1.x profile is shown as
            unknown and is not contacted.
          </li>
          <li>
            The browser receives no credential value or reference from this path;
            the server resolves the approved binding for the stored profile.
          </li>
        </ul>
      </section>

      <DisabledActionGroup
        heading="canonical-mapper"
        reason="Mapping drafts, canonical-schema versioning, validation and export are not implemented in P3-003. Schema browsing is read-only."
        actions={["Save mapping", "Validate mapping", "Export profile"]}
      />
    </section>
  );
}
