import { Component, createSignal, Show } from "solid-js";
import { getDagPageCount, getDags } from "../api/dags";
import DagComponent from "../components/dagComponent";
import { createQuery } from "@tanstack/solid-query";
import Spinner from "../components/spinner";
import PaginationComponent from "../components/pagination";

const Dags: Component = () => {
  const [maxPage, setMaxPage] = createSignal(-1);
  const [page, setPage] = createSignal(1);

  const dags = createQuery(() => ({
    queryKey: ["dagRuns", page().toString()],
    queryFn: getDags,
    staleTime: 5 * 60 * 1000, // 5 minutes
  }));

  getDagPageCount()
    .then((count) => {
      setMaxPage(count);
    })
    .catch((error) => console.error(error));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">Your DAGs</h2>
      <div class="mt-4"></div>
      <Show when={dags.isError}>
        <div>Error: {dags.error && dags.error.message}</div>
      </Show>
      <Show when={dags.isLoading}>
        <Spinner />
      </Show>
      <Show when={dags.isSuccess}>
        <div>
          {dags.data ? (
            dags.data.map((dag) => <DagComponent dag={dag} />)
          ) : (
            <p>No DAG found!</p>
          )}
        </div>
      </Show>
      <Show when={maxPage() > 1}>
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      </Show>
    </div>
  );
};

export default Dags;
