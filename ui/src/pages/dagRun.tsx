import { Component, createEffect, createSignal } from "solid-js";
import { DagRunAll, TaskRunDetails } from "../types/dag";
import { getDagRunAll, getTaskRunDetails } from "../api/dags";
import { A, useParams } from "@solidjs/router";
import LoadingIcon from "../components/loadingIcon";
import DagViz from "../components/dagViz";
import { PodStatusTable } from "../components/tables/podStatusTable";
import LoadingButton from "../components/inputs/loadingbutton";

const DagRun: Component = () => {
  const params = useParams();

  const [dataRunMeta, setDataRunMeta] = createSignal<DagRunAll>();
  const [selectedTask, setSelectedTask] = createSignal<number>(-1);
  const [taskDetails, setTaskDetails] = createSignal<
    TaskRunDetails | undefined
  >();

  // Initial load
  const loadDagRun = async () => {
    try {
      const data = await getDagRunAll(parseInt(params.id));
      setDataRunMeta(data);
    } catch (error) {
      console.error("Error loading DAG run:", error);
    }
  };

  // Load initial data
  loadDagRun();

  getDagRunAll(parseInt(params.id)).then((data) => setDataRunMeta(data));

  createEffect(() => {
    if (selectedTask() === -1) return;

    getTaskRunDetails(dataRunMeta()?.id ?? -1, selectedTask()).then((data) =>
      setTaskDetails(data)
    );
  });

  return (
    <>
      {dataRunMeta() === undefined ? (
        <LoadingIcon />
      ) : (
        <div class="p-6 shadow-lg rounded-lg">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-3xl font-bold">Run ID: {dataRunMeta()?.id}</h2>
            <LoadingButton
              onClick={async () => {
                // wait faction of second to show loading icon
                await new Promise((r) => setTimeout(r, 100));
                await loadDagRun();
              }}
            />
          </div>
          <hr class="mb-4 border-gray-300" />
          <h4 class="text-xl font-semibold mb-4 ">
            DAG: {dataRunMeta()?.dagId}
          </h4>
          <h3 class="text-xl font-semibold ">
            Status: {dataRunMeta()?.status}
          </h3>
          <hr class="my-4 border-gray-300" />
          <h3 class="text-2xl font-semibold">Task Connections</h3>
          <DagViz
            connections={dataRunMeta()?.connections ?? {}}
            taskInfo={dataRunMeta()?.taskInfo}
            setSelectedTask={setSelectedTask}
          />

          {selectedTask() === -1 ? (
            <div class="text-lg  mt-4 italic">
              Click on a task to see its details.
            </div>
          ) : (
            taskDetails() && (
              <div class="mt-6  p-4 rounded-lg border border-gray-200">
                <h3 class="text-xl font-semibold  mb-2">Task Details</h3>
                <p class="mb-1">
                  <strong>TaskRun ID:</strong> {taskDetails()?.id}
                </p>
                <p class="mb-1">
                  <strong>Status:</strong> {taskDetails()?.status}
                </p>
                <p class="mb-1">
                  <strong>Attempts:</strong> {taskDetails()?.attempts}
                </p>

                <h4 class="text-lg font-semibold mt-4 mb-2">Pods:</h4>
                <PodStatusTable
                  details={taskDetails()!}
                  id={dataRunMeta()!.id}
                />
              </div>
            )
          )}
        </div>
      )}
    </>
  );
};

export default DagRun;
