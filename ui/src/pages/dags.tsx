import { Component, createSignal } from "solid-js";
import { getDagPageCount, getDags } from "../api/dags";
import DagComponent from "../components/dagComponent";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import PaginationComponent from "../components/pagination";
import Loadable from "../components/loadable";

const Dags: Component = () => {
  const [maxPage, setMaxPage] = createSignal(-1);
  const [page, setPage] = createSignal(1);
  const queryClient = useQueryClient();

  const dags = createQuery(() => ({
    queryKey: ["dags", page().toString()],
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

      <Loadable
        loading={dags.isLoading}
        error={dags.isError && (dags.error as any)?.message}
        onRetry={() => dags.refetch()}
      >
        <div>
          {dags.data && dags.data.length > 0 ? (
            dags.data.map((dag) => (
              <DagComponent
                dag={dag}
                onDelete={() => {
                  getDagPageCount()
                    .then((count) => {
                      setMaxPage(count);
                      queryClient.invalidateQueries({
                        queryKey: ["dags"],
                      });
                    })
                    .catch((error) => console.error(error));
                }}
              />
            ))
          ) : (
            <p>No DAG found!</p>
          )}
        </div>
      </Loadable>

      {maxPage() > 1 && (
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      )}
    </div>
  );
};

export default Dags;
