import { Component } from "solid-js";
import { DagRunAll } from "../types/dag";
import { getDagRunAll, getTaskRunDetails } from "../api/dags";
import { A, useParams } from "@solidjs/router";
import DagViz from "../components/dagViz";
import { createSignal, createMemo } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import Loadable from "../components/loadable";
import { PodStatusTable } from "../components/tables/podStatusTable";
import LoadingButton from "../components/inputs/loadingbutton";

const DagRun: Component = () => {
  const params = useParams();
  const id = createMemo(() => Number.parseInt(params.id));

  const [selectedTask, setSelectedTask] = createSignal<number>(-1);

  const runQuery = createQuery(() => ({
    queryKey: ["dagRun", id()],
    queryFn: () => getDagRunAll(id()),
    staleTime: 60 * 1000,
    enabled: () => Number.isFinite(id()),
  }));

  const taskDetailsQuery = createQuery(() => ({
    queryKey: ["taskRunDetails", runQuery.data?.id ?? -1, selectedTask().toString()],
    queryFn: () => getTaskRunDetails(runQuery.data?.id ?? -1, selectedTask()),
    enabled: () => selectedTask() !== -1 && !!runQuery.data,
  }));

  return (
    <Loadable loading={runQuery.isLoading} error={runQuery.isError && (runQuery.error as any)?.message} onRetry={() => runQuery.refetch()}>
      <div class="p-6 shadow-lg rounded-lg">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-3xl font-bold">Run ID: {runQuery.data?.id}</h2>
          <LoadingButton
            onClick={async () => {
              // refresh
              await runQuery.refetch();
            }}
          />
        </div>
        <hr class="mb-4 border-gray-300" />
        <h4 class="text-xl font-semibold mb-4 ">DAG: {runQuery.data?.dagId}</h4>
        <h3 class="text-xl font-semibold ">Status: {runQuery.data?.status}</h3>
        <hr class="my-4 border-gray-300" />
        <h3 class="text-2xl font-semibold">Task Connections</h3>
        <DagViz
          connections={runQuery.data?.connections ?? {}}
          taskInfo={runQuery.data?.taskInfo}
          setSelectedTask={setSelectedTask}
        />

        {selectedTask() === -1 ? (
          <div class="text-lg  mt-4 italic">Click on a task to see its details.</div>
        ) : (
          taskDetailsQuery.data && (
            <div class="mt-6  p-4 rounded-lg border border-gray-200">
              <h3 class="text-xl font-semibold  mb-2">Task Details</h3>
              <p class="mb-1">
                <strong>TaskRun ID:</strong> {taskDetailsQuery.data?.id}
              </p>
              <p class="mb-1">
                <strong>Status:</strong> {taskDetailsQuery.data?.status}
              </p>
              <p class="mb-1">
                <strong>Attempts:</strong> {taskDetailsQuery.data?.attempts}
              </p>

              <h4 class="text-lg font-semibold mt-4 mb-2">Pods:</h4>
              <PodStatusTable details={taskDetailsQuery.data!} id={runQuery.data!.id} />
            </div>
          )
        )}
      </div>
    </Loadable>
  );
};

export default DagRun;
