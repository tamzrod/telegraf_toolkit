export type Health = {
  status: string;
  service: string;
  version: string;
  scaffold: boolean;
  connected: boolean;
  description: string;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * Runtime guard for the health payload. It rejects a malformed or partially
 * shaped response instead of coercing it, so the UI cannot render a status it
 * did not actually receive.
 */
export function isHealth(value: unknown): value is Health {
  if (!isRecord(value)) {
    return false;
  }
  return (
    typeof value.status === "string" &&
    typeof value.service === "string" &&
    typeof value.version === "string" &&
    typeof value.scaffold === "boolean" &&
    typeof value.connected === "boolean" &&
    typeof value.description === "string"
  );
}
