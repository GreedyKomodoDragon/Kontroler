import { Show, createEffect, createSignal, onCleanup } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { useParams } from "@solidjs/router";
import { getLogs } from "../api/logs";
import LogHighlighter from "../components/code/logViewer";
import { useWebSocket } from "../providers/webhookProvider";
import Loadable from "../components/loadable";

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
          <svg width={50} height={50} viewBox="0 0 60 215" class="animate-spin-slow">
            <path d="m123.997 95.268-1.562 4.763c-33.47 4.051-51.817-12.044-49.758-54.016.237-4.829 3.382-3.964 4.834-2.174 1.103 1.359 2.908 3.179 4.47 4.659 1.885 1.786 4.941 3.908 7.701 3.06 3.148-.966 15.543-.171 19.862.562-7.228 13.527-4.029 28.479 8 44.499z" transform="rotate(45 139.166 -57.646)scale(1.45086)" />
          </svg>
        </Show>
      </h2>
      <div class="mt-4"></div>

      <Loadable loading={logs.isLoading} error={logs.isError ? (logs.error as any)?.message : undefined} onRetry={() => logs.refetch()}>
        <div>
          <Show when={logs.isSuccess && logs.data?.trim()}>
            <LogHighlighter logs={logs.data!.trimEnd()} />
          </Show>
          <Show when={liveLogs().length > 0}>
            <LogHighlighter logs={liveLogs().join("\n")} />
          </Show>
        </div>
      </Loadable>
    </div>
  );
}
