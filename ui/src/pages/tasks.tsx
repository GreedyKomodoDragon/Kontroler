import { createSignal, Show } from "solid-js";
import { getDagTaskPageCount, getDagTasks } from "../api/dags";
import { createQuery } from "@tanstack/solid-query";
import Spinner from "../components/spinner";
import PaginationComponent from "../components/pagination";
import TaskBox from "../components/containers/taskBox";

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
      <Show when={tasks.isError}>
        <div>Error: {tasks.error && tasks.error.message}</div>
      </Show>
      <Show when={tasks.isLoading}>
        <Spinner />
      </Show>
      <Show when={tasks.isSuccess}>
        <div>
          {tasks.data ? (
            tasks.data.map((task) => <TaskBox taskDetails={task} />)
          ) : (
            <p>No Tasks found!</p>
          )}
        </div>
      </Show>
      <Show when={maxPage() > 1}>
        <PaginationComponent setPage={setPage} maxPage={maxPage} />
      </Show>
    </div>
  );
}
