import { Show, createEffect, createSignal, onCleanup } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import Spinner from "../components/spinner";
import { useParams } from "@solidjs/router";
import { getLogs } from "../api/logs";
import LogHighlighter from "../components/code/logViewer";
import { useWebSocket } from "../providers/webhookProvider";

export default function Logs() {
  const params = useParams();
  const { logs: liveLogs, startLogs, stopLogs, isStreaming } = useWebSocket();
  const [lastStreamAttempt, setLastStreamAttempt] = createSignal(0);

  // Fetch logs via API
  const logs = createQuery(() => ({
    queryKey: ["logs", params.run, params.pod],
    queryFn: getLogs,
    staleTime: 5 * 60 * 1000, // 5 minutes
  }));

  const RETRY_COOLDOWN = 500;

  // Start WebSocket streaming if logs are empty, with cooldown
  createEffect(() => {
    if (logs.isSuccess && !logs.data?.trim() && liveLogs().length === 0) {
      const now = Date.now();
      if (now - lastStreamAttempt() >= RETRY_COOLDOWN) {
        setLastStreamAttempt(now);
        startLogs(params.pod);
      }
    }
  });

  // Stop streaming when leaving the page
  onCleanup(() => {
    setLastStreamAttempt(0);
    stopLogs();
  });

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4 flex items-center gap-2">
        Pod: {params.pod}
        <Show when={isStreaming()}>
          <Spinner height={50} width={50} />
        </Show>
      </h2>{" "}
      <div class="mt-4"></div>
      <Show when={logs.isError}>
        <div>Error: {logs.error?.message}</div>
      </Show>
      <Show when={logs.isLoading}>
        <Spinner />
      </Show>
      <Show when={logs.isSuccess && logs.data?.trim()}>
        <LogHighlighter logs={logs.data!.trimEnd()} />
      </Show>
      <Show when={liveLogs().length > 0}>
        <LogHighlighter logs={liveLogs().join("\n")} />
      </Show>
    </div>
  );
}
