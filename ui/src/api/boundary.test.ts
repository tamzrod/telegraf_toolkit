import { describe, expect, it } from "vitest";

/**
 * Source files are loaded through Vite's raw glob so the checks read exactly
 * what is bundled, without needing Node filesystem types in the app tsconfig.
 */
const sources = import.meta.glob("../**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/** Glob keys are relative to this file's directory (`src/api/`). */
function toSrcRelative(key: string): string {
  const segments = `api/${key}`.split("/");
  const resolved: string[] = [];
  for (const segment of segments) {
    if (segment === "" || segment === ".") {
      continue;
    }
    if (segment === "..") {
      resolved.pop();
    } else {
      resolved.push(segment);
    }
  }
  return resolved.join("/");
}

const entries = Object.entries(sources).map(([path, contents]) => ({
  path: toSrcRelative(path),
  contents,
}));

const isTestFile = (path: string) => /\.test\.tsx?$/.test(path);
const isMockModule = (path: string) =>
  path.includes("api/mock/") || path.includes("fixtures/");

describe("mock separation", () => {
  it("collects the application source tree", () => {
    expect(entries.length).toBeGreaterThan(10);
  });

  it("keeps test-only mock adapters out of application code", () => {
    const offenders = entries
      .filter(({ path }) => !isTestFile(path) && !isMockModule(path))
      .filter(({ contents }) =>
        /from\s+["'][^"']*(\/mock\/|\/fixtures\/)[^"']*["']/.test(contents),
      )
      .map(({ path }) => path);

    expect(offenders).toEqual([]);
  });

  it("never hardcodes credential-like material in source", () => {
    const suspicious =
      /(password|passwd|secret|api[_-]?key|token)\s*[:=]\s*["'][^"']{8,}["']/i;
    const offenders = entries
      .filter(({ contents }) => suspicious.test(contents))
      .map(({ path }) => path);

    expect(offenders).toEqual([]);
  });

  it("sends every request through the single typed API boundary", () => {
    const offenders = entries
      .filter(({ path }) => !isTestFile(path) && !isMockModule(path))
      .filter(
        ({ path, contents }) =>
          /\bfetch\(/.test(contents) && !path.endsWith("api/client.ts"),
      )
      .map(({ path }) => path);

    expect(offenders).toEqual([]);
  });
});
