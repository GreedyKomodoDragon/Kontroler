import { Component, createSignal } from "solid-js";
import { getDagRunPageCount, getDagRuns } from "../api/dags";
import DagRunComponent from "../components/dagRunComponent";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import PaginationComponent from "../components/pagination";
import Loadable from "../components/loadable";
import SkeletonCard from "../components/skeletonCard";
import { A } from "@solidjs/router";

const DagRuns: Component = () => {
  const [page, setPage] = createSignal(1);
  const queryClient = useQueryClient();

  const pageCountQuery = createQuery(() => ({
    queryKey: ["dag-run-page-count"],
    queryFn: getDagRunPageCount,
    staleTime: 5 * 60 * 1000,
  }));

  const runs = createQuery(() => ({
    queryKey: ["dags-runs", page().toString()],
    queryFn: getDagRuns,
    staleTime: 5 * 60 * 1000,
  }));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">DAG Runs</h2>
      <div class="my-4 flex">
        <A href="/dags/runs/create" class="ml-auto bg-blue-500 p-2 rounded-md">Create New DagRun</A>
      </div>

      <Loadable
        loading={runs.isLoading}
        error={runs.isError && (runs.error as any)?.message}
        onRetry={() => runs.refetch()}
        skeleton={
          <div class="space-y-4">
            {Array.from({ length: 6 }).map(() => (
              <SkeletonCard titleLines={1} bodyLines={2} />
            ))}
          </div>
        }
      >
        <div>
          {runs.data && runs.data.length !== 0 ? (
            runs.data.map((run) => (
              <DagRunComponent
                dagRun={run}
                onDelete={() => {
                  queryClient.invalidateQueries({ queryKey: ["dag-run-page-count"] });
                  queryClient.invalidateQueries({ queryKey: ["dags-runs"] });
                }}
              />
            ))
          ) : (
            <p>No DAG Runs found!</p>
          )}
        </div>
      </Loadable>

      {pageCountQuery.data && pageCountQuery.data > 1 && (
        <PaginationComponent setPage={setPage} maxPage={() => pageCountQuery.data!} />
      )}
    </div>
  );
};

export default DagRuns;
