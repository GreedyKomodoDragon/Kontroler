import { Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import Spinner from "../components/spinner";
import { useParams } from "@solidjs/router";
import { getLogs } from "../api/logs";
import LogHighlighter from "../components/code/logViewer";

export default function Logs() {
  const params = useParams();

  const logs = createQuery(() => ({
    queryKey: ["logs", params.run, params.pod],
    queryFn: getLogs,
    staleTime: 5 * 60 * 1000, // 5 minutes
  }));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">Pod: {params.pod} </h2>
      <div class="mt-4"></div>
      <Show when={logs.isError}>
        <div>Error: {logs.error && logs.error.message}</div>
      </Show>
      <Show when={logs.isLoading}> 
        <Spinner />
      </Show>
      <Show when={logs.isSuccess}>
        <LogHighlighter logs={logs.data!} />
      </Show>
    </div>
  );
}
