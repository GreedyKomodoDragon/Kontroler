import { Component, createSignal } from "solid-js";
import { getDagRunPageCount, getDagRuns } from "../api/dags";
import DagRunComponent from "../components/dagRunComponent";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import PaginationComponent from "../components/pagination";
import Loadable from "../components/loadable";
import SkeletonCard from "../components/skeletonCard";
import { A } from "@solidjs/router";

const DagRuns: Component = () => {
  const [maxPage, setMaxPage] = createSignal(-1);
  const [page, setPage] = createSignal(1);
  const queryClient = useQueryClient();

  const runs = createQuery(() => ({
    queryKey: ["dags-runs", page().toString()],
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
                  getDagRunPageCount()
                    .then((count) => {
                      setMaxPage(count);
                      queryClient.invalidateQueries({
                        queryKey: ["dags-runs"],
                      });
                    })
                    .catch((error) => console.error(error));
                }}
              />
            ))
          ) : (
            <p>No DAG Runs found!</p>
          )}
        </div>
      </Loadable>

      {maxPage() > 1 && (
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      )}
    </div>
  );
};

export default DagRuns;
