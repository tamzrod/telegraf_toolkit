import { useMemo, useState } from "react";
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
import { useRegistryProfiles } from "../../hooks/useRegistryProfiles";
import { useLiveRegistryProfiles } from "../../hooks/useLiveRegistry";
import { clearSessionToken, hasSessionToken, setSessionToken } from "../../api/session";
import { formatScope } from "../../api/registry";
import type { CapabilityOutcome, RegistryProfile } from "../../api/registry";
import {
  capabilityRows,
  credentialState,
  outcomeLabel,
  validateSelection,
  validateDraft,
} from "./registryWorkflow";

const OUTCOME_CLASSES: Record<CapabilityOutcome, string> = {
  supported: "bg-green-100 text-green-900",
  unsupported: "bg-gray-200 text-gray-700",
  unknown: "bg-amber-100 text-amber-900",
};

function CapabilityBadge({ outcome }: { outcome: CapabilityOutcome }) {
  return (
    <span className={`rounded px-2 py-0.5 text-xs font-medium ${OUTCOME_CLASSES[outcome]}`}>
      {outcomeLabel(outcome)}
    </span>
  );
}

/**
 * Database Registry workflow.
 *
 * Two clearly separated sources:
 *
 *  - **Live** profiles from the authorized API. When a session token is held the
 *    page lists real, persisted profiles and can perform a read-only capability
 *    probe. Without a token the backend responds 401 and the page shows that
 *    plainly; it never substitutes mock data for a live result.
 *  - **Mock** profiles from the typed test-only adapter, always labeled MOCK.
 *
 * The page never resolves or displays a credential value and never writes a
 * secret to browser storage.
 */
export function DatabasesPage() {
  const [useLive, setUseLive] = useState(false);
  const [tokenInput, setTokenInput] = useState("");
  const [tokenPresent, setTokenPresent] = useState(hasSessionToken());

  const mockQuery = useRegistryProfiles();
  // The live query is enabled only when the live source is selected, so mock
  // mode never issues an unauthorized request.
  const liveQuery = useLiveRegistryProfiles(useLive);

  const active = useLive ? liveQuery : mockQuery;
  const { data, isPending, isError, refetch } = active;

  const [sourceId, setSourceId] = useState<string>("");
  const [destinationId, setDestinationId] = useState<string>("");
  const [draftConnectionId, setDraftConnectionId] = useState("");
  const [draftDisplayName, setDraftDisplayName] = useState("");

  const profiles = useMemo(() => data ?? [], [data]);

  const byId = useMemo(() => {
    const map = new Map<string, RegistryProfile>();
    for (const profile of profiles) {
      map.set(profile.connectionId, profile);
    }
    return map;
  }, [profiles]);

  const source = sourceId ? byId.get(sourceId) ?? null : null;
  const destination = destinationId ? byId.get(destinationId) ?? null : null;
  const selection = validateSelection(source, destination);
  const draft = validateDraft({
    connectionId: draftConnectionId,
    displayName: draftDisplayName,
    scope: { kind: "database", database: "draft" },
  });

  function applyToken() {
    if (tokenInput.trim() === "") {
      clearSessionToken();
      setTokenPresent(false);
    } else {
      setSessionToken(tokenInput);
      setTokenPresent(true);
    }
    // Never keep the value in component state after it is applied.
    setTokenInput("");
    void liveQuery.refetch();
  }

  function signOut() {
    clearSessionToken();
    setTokenPresent(false);
    void liveQuery.refetch();
  }

  return (
    <section aria-labelledby="databases-heading">
      <PageHeader
        id="databases-heading"
        title="Databases"
        scope="InfluxDB connection profiles, probed capabilities, explicit source/destination scope and credential references."
        status="partial"
      />

      <section className="mt-6 rounded border border-gray-300 p-4" aria-labelledby="source-mode">
        <h2 id="source-mode" className="text-base font-medium">
          Data source
        </h2>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <button
            type="button"
            className="rounded border border-gray-400 px-3 py-1 text-sm"
            aria-pressed={!useLive}
            onClick={() => setUseLive(false)}
          >
            Mock adapter
          </button>
          <button
            type="button"
            className="rounded border border-gray-400 px-3 py-1 text-sm"
            aria-pressed={useLive}
            onClick={() => setUseLive(true)}
          >
            Live Registry API
          </button>
        </div>

        {useLive && (
          <div className="mt-3">
            <p className="text-sm text-gray-700">
              The live API requires a bearer token. The token is held in memory
              only for this session; it is never written to browser storage and
              is dropped on refresh.
            </p>
            <div className="mt-2 flex flex-wrap items-end gap-2">
              <div>
                <label htmlFor="session-token" className="block text-sm font-medium">
                  Session token
                </label>
                <input
                  id="session-token"
                  type="password"
                  autoComplete="off"
                  className="mt-1 w-64 rounded border border-gray-300 p-2 text-sm"
                  value={tokenInput}
                  onChange={(event) => setTokenInput(event.target.value)}
                  aria-describedby="session-token-status"
                />
              </div>
              <button
                type="button"
                className="rounded border border-gray-400 px-3 py-1 text-sm"
                onClick={applyToken}
              >
                Use token
              </button>
              {tokenPresent && (
                <button
                  type="button"
                  className="rounded border border-gray-400 px-3 py-1 text-sm"
                  onClick={signOut}
                >
                  Sign out
                </button>
              )}
            </div>
            <p id="session-token-status" role="status" className="mt-1 text-sm text-gray-700">
              {tokenPresent
                ? "A session token is held for this session."
                : "No session token is held; live requests will be rejected with 401."}
            </p>
          </div>
        )}
      </section>

      {useLive ? (
        <p role="status" className="mt-6 rounded border border-blue-300 bg-blue-50 p-4 text-sm">
          <strong>Live Registry API.</strong> Profiles come from the authorized,
          persistent backend. Capabilities are probed per connection; a
          capability is shown as supported only when the probe confirmed it.
        </p>
      ) : (
        <p role="status" className="mt-6 rounded border border-amber-300 bg-amber-50 p-4 text-sm">
          <strong>Mock workflow.</strong> Profiles shown here come from a typed
          test-only adapter. No connection is probed, no credential is stored or
          resolved, and no profile is persisted. Saving, testing and deploying
          remain disabled until a later, separately authorized phase.
        </p>
      )}

      <section className="mt-6" aria-labelledby="registry-profiles">
        <h2 id="registry-profiles" className="text-lg font-medium">
          Connection profiles
        </h2>

        {isPending && <LoadingState label="Loading registry profiles…" />}

        {isError && (
          <ErrorState
            title="Registry profiles unavailable"
            description={
              useLive
                ? "The live Registry API did not accept the request. A valid session token is required; this is not a mock fallback."
                : "The typed registry endpoint did not respond. This is not a live connection."
            }
            onRetry={() => void refetch()}
          />
        )}

        {data && profiles.length === 0 && (
          <EmptyState
            title="No connection profiles"
            description={
              useLive
                ? "No profile is persisted in the Registry yet."
                : "No mock profile is registered in this shell."
            }
          />
        )}

        {data && profiles.length > 0 && (
          <>
            {!useLive && <MockDataNotice subject="connection profile list" />}
            <ul className="mt-3 space-y-4">
              {profiles.map((profile) => {
                const credential = credentialState(profile);
                return (
                  <li
                    key={profile.connectionId}
                    className="rounded border border-gray-300 p-4"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-medium">
                        {profile.displayName}
                      </h3>
                      {profile.mock && (
                        <span className="rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-900">
                          MOCK
                        </span>
                      )}
                      {useLive && !profile.mock && (
                        <span className="rounded bg-green-100 px-2 py-0.5 text-xs font-medium text-green-900">
                          LIVE
                        </span>
                      )}
                    </div>
                    <dl className="mt-2 grid grid-cols-1 gap-1 text-sm sm:grid-cols-2">
                      <div>
                        <dt className="inline font-medium">Connection ID: </dt>
                        <dd className="inline">{profile.connectionId}</dd>
                      </div>
                      <div>
                        <dt className="inline font-medium">Family: </dt>
                        <dd className="inline">
                          {profile.family} {profile.majorVersion}.x
                        </dd>
                      </div>
                      <div>
                        <dt className="inline font-medium">Scope: </dt>
                        <dd className="inline">
                          {profile.scope.kind} — {formatScope(profile.scope)}
                        </dd>
                      </div>
                      <div>
                        <dt className="inline font-medium">Credential: </dt>
                        <dd className="inline">
                          {credential.kind === "reference" ? (
                            <>
                              {credential.alias} ({credential.role} reference)
                            </>
                          ) : profile.credentialRefId ? (
                            <>
                              {profile.credentialRefId} (reference)
                            </>
                          ) : (
                            <span className="text-amber-800">
                              No credential reference attached
                            </span>
                          )}
                        </dd>
                      </div>
                    </dl>

                    <div className="mt-3">
                      <h4 className="text-sm font-medium">
                        Probed capabilities (per connection)
                      </h4>
                      <ul className="mt-1 flex flex-wrap gap-2">
                        {capabilityRows(profile).map((row) => (
                          <li key={row.label} className="flex items-center gap-1 text-xs">
                            <span className="text-gray-700">{row.label}:</span>
                            <CapabilityBadge outcome={row.outcome} />
                          </li>
                        ))}
                      </ul>
                    </div>
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </section>

      <section className="mt-6" aria-labelledby="scope-selection">
        <h2 id="scope-selection" className="text-lg font-medium">
          Source and destination scope
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Both sides must be chosen explicitly and must be distinct. This is
          local selection only; nothing is browsed or written.
        </p>

        <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label htmlFor="source-select" className="block text-sm font-medium">
              Source database
            </label>
            <select
              id="source-select"
              className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
              value={sourceId}
              onChange={(event) => setSourceId(event.target.value)}
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
            <label htmlFor="destination-select" className="block text-sm font-medium">
              Destination database
            </label>
            <select
              id="destination-select"
              className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
              value={destinationId}
              onChange={(event) => setDestinationId(event.target.value)}
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

        {!selection.ok && (
          <p
            role="alert"
            className="mt-2 rounded border border-red-300 bg-red-50 p-2 text-sm text-red-800"
          >
            {selection.reason}
          </p>
        )}
        {selection.ok && source && destination && (
          <p
            role="status"
            className="mt-2 rounded border border-green-300 bg-green-50 p-2 text-sm text-green-900"
          >
            Source <strong>{source.displayName}</strong> and destination{" "}
            <strong>{destination.displayName}</strong> are distinct and selected.
            Schema browsing is not implemented.
          </p>
        )}
      </section>

      <section className="mt-6" aria-labelledby="draft-profile">
        <h2 id="draft-profile" className="text-lg font-medium">
          New profile (unsaved local draft)
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          This draft exists only in this browser session. It is not saved, not
          persisted and not sent anywhere.
        </p>

        <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label htmlFor="draft-connection-id" className="block text-sm font-medium">
              Connection ID
            </label>
            <input
              id="draft-connection-id"
              className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
              value={draftConnectionId}
              onChange={(event) => setDraftConnectionId(event.target.value)}
              aria-invalid={!draft.ok && draftConnectionId.trim() === ""}
              aria-describedby="draft-status"
            />
          </div>
          <div>
            <label htmlFor="draft-display-name" className="block text-sm font-medium">
              Display name
            </label>
            <input
              id="draft-display-name"
              className="mt-1 w-full rounded border border-gray-300 p-2 text-sm"
              value={draftDisplayName}
              onChange={(event) => setDraftDisplayName(event.target.value)}
              aria-invalid={!draft.ok && draftDisplayName.trim() === ""}
              aria-describedby="draft-status"
            />
          </div>
        </div>

        <p
          id="draft-status"
          role="status"
          className={`mt-2 rounded border p-2 text-sm ${
            draft.ok
              ? "border-gray-300 bg-gray-50 text-gray-700"
              : "border-red-300 bg-red-50 text-red-800"
          }`}
        >
          {draft.ok
            ? "Draft is locally complete. It is still unsaved and cannot be submitted."
            : draft.reason}
        </p>
      </section>

      <DisabledActionGroup
        heading="databases"
        reason="Saving a profile from the UI, credential entry and deletion require the write flows to be reviewed for this deployment; live listing and read-only capability probing are available above."
        actions={["Save profile", "Attach credential", "Delete profile"]}
      />
    </section>
  );
}
