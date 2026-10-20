import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useHealth } from "./useHealth";
import { MOCK_HEALTH, healthResponse } from "../api/mock/health";

function Probe() {
  const { data, isPending, isError } = useHealth();

  if (isPending) {
    return <p>loading</p>;
  }
  if (isError) {
    return <p>error</p>;
  }
  return <p>connected: {String(data.connected)}</p>;
}

function renderProbe() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <Probe />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useHealth", () => {
  it("exposes loading then success for the health query", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    renderProbe();

    expect(screen.getByText("loading")).toBeInTheDocument();
    expect(await screen.findByText(/connected: false/i)).toBeInTheDocument();
  });

  it("exposes an error state when the request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network down"));

    renderProbe();

    expect(await screen.findByText("error")).toBeInTheDocument();
  });

  it("aborts the in-flight request when the consumer unmounts", async () => {
    let capturedSignal: AbortSignal | undefined;
    vi.spyOn(globalThis, "fetch").mockImplementation((_url, init) => {
      capturedSignal = init?.signal ?? undefined;
      return new Promise<Response>(() => {
        // Never resolves: the request stays in flight until it is aborted.
      });
    });

    const { unmount } = renderProbe();

    await waitFor(() => expect(capturedSignal).toBeDefined());
    expect(capturedSignal?.aborted).toBe(false);

    unmount();

    await waitFor(() => expect(capturedSignal?.aborted).toBe(true));
  });

  it("uses the shared health query key so the cache can be invalidated", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(healthResponse());
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <Probe />
      </QueryClientProvider>,
    );

    await screen.findByText(/connected: false/i);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryData(["health"])).toEqual(MOCK_HEALTH);
  });
});
