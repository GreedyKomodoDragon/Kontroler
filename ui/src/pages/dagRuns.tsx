import { Component, createSignal, Show } from "solid-js";
import { getDagRunPageCount, getDagRuns } from "../api/dags";
import DagRunComponent from "../components/dagRunComponent";
import { createQuery } from "@tanstack/solid-query";
import PaginationComponent from "../components/pagination";
import Spinner from "../components/spinner";

const DagRuns: Component = () => {
  const [maxPage, setMaxPage] = createSignal(-1);
  const [page, setPage] = createSignal(1);

  const runs = createQuery(() => ({
    queryKey: ["dag", page().toString()],
    queryFn: getDagRuns,
    staleTime: 5 * 60 * 1000, // 5 minutes
  }));

  getDagRunPageCount()
    .then((count) => {
      setMaxPage(count);
    })
    .catch((error) => console.error(error));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">DAG Runs</h2>
      <div class="mt-4"></div>
      <Show when={runs.isError}>
        <div>Error: {runs.error && runs.error.message}</div>
      </Show>
      <Show when={runs.isLoading}>
        <Spinner />
      </Show>
      <Show when={runs.isSuccess}>
        <div>
          {runs.data ?
            runs.data.map((run) => <DagRunComponent dagRun={run} />) : <p>No DAG Runs found!</p>}
        </div>
      </Show>
      <Show when={maxPage() > 1}>
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      </Show>
    </div>
  );
};

export default DagRuns;
