import { createSignal } from "solid-js";
import { getDagTaskPageCount, getDagTasks } from "../api/dags";
import { createQuery } from "@tanstack/solid-query";
import PaginationComponent from "../components/pagination";
import TaskBox from "../components/containers/taskBox";
import Loadable from "../components/loadable";
import SkeletonCard from "../components/skeletonCard";

export default function Tasks() {
  const [maxPage, setMaxPage] = createSignal(-1);
  const [page, setPage] = createSignal(1);

  const tasks = createQuery(() => ({
    queryKey: ["dagTasks", page().toString()],
    queryFn: getDagTasks,
    staleTime: 5 * 60 * 1000, // 5 minutes
  }));

  getDagTaskPageCount()
    .then((count) => {
      setMaxPage(count);
    })
    .catch((error) => console.error(error));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">Your Tasks</h2>
      <div class="mt-4"></div>

      <Loadable loading={tasks.isLoading} error={tasks.isError && (tasks.error as any)?.message} onRetry={() => tasks.refetch()} skeleton={<div class="space-y-4">{Array.from({ length: 8 }).map(() => <SkeletonCard titleLines={1} bodyLines={2} />)}</div>}>
        <div>
          {tasks.data && tasks.data.length > 0 ? (
            tasks.data.map((task) => <TaskBox taskDetails={task} />)
          ) : (
            <p>No Tasks found!</p>
          )}
        </div>
      </Loadable>

      {maxPage() > 1 && (
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      )}
    </div>
  );
}
