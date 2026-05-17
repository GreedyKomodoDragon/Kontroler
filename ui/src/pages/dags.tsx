import { Component, createSignal } from "solid-js";
import { getDagPageCount, getDags } from "../api/dags";
import DagComponent from "../components/dagComponent";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import PaginationComponent from "../components/pagination";
import Loadable from "../components/loadable";
import SkeletonCard from "../components/skeletonCard";

const Dags: Component = () => {
  const [page, setPage] = createSignal(1);
  const queryClient = useQueryClient();

  const pageCountQuery = createQuery(() => ({
    queryKey: ["dag-page-count"],
    queryFn: getDagPageCount,
    staleTime: 5 * 60 * 1000,
  }));

  const dags = createQuery(() => ({
    queryKey: ["dags", page().toString()],
    queryFn: getDags,
    staleTime: 5 * 60 * 1000,
  }));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">Your DAGs</h2>
      <div class="mt-4"></div>

      <Loadable
        loading={dags.isLoading}
        error={dags.isError && (dags.error as any)?.message}
        onRetry={() => dags.refetch()}
        skeleton={
          <div class="space-y-4">
            {Array.from({ length: 6 }).map(() => (
              <SkeletonCard titleLines={1} bodyLines={1} />
            ))}
          </div>
        }
      >
        <div>
          {dags.data && dags.data.length > 0 ? (
            dags.data.map((dag) => (
              <DagComponent
                dag={dag}
                onDelete={() => {
                  queryClient.invalidateQueries({ queryKey: ["dag-page-count"] });
                  queryClient.invalidateQueries({ queryKey: ["dags"] });
                }}
              />
            ))
          ) : (
            <p>No DAG found!</p>
          )}
        </div>
      </Loadable>

      {pageCountQuery.data && pageCountQuery.data > 1 && (
        <PaginationComponent setPage={setPage} maxPage={() => pageCountQuery.data!} />
      )}
    </div>
  );
};

export default Dags;
