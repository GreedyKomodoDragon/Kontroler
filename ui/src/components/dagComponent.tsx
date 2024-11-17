import { createEffect, createSignal } from "solid-js";
import { Dag, TaskDetails } from "../types/dag";
import { getTaskDetails } from "../api/dags";
import ShellScriptViewer from "./code/shellScriptViewer";
import JsonToYamlViewer from "./code/JsonToYamlViewer";
import DagViz from "./dagViz";

interface Props {
  dag: Dag;
}

const DagComponent = ({ dag }: Props) => {
  const [open, setOpen] = createSignal<boolean>(false);
  const [selectedTask, setSelectedTask] = createSignal<number>(-1);
  const [taskDetails, setTaskDetails] = createSignal<TaskDetails | undefined>();

  createEffect(() => {
    if (selectedTask() === -1) return;

    getTaskDetails(selectedTask()).then((data) => setTaskDetails(data));
  });

  return (
    <div class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white">
      <div class="flex justify-between items-center">
        <h3 class="text-2xl font-semibold">{dag.name}</h3>
        <button
          class="rounded-md bg-blue-700 p-2"
          onClick={() => setOpen(!open())}
        >
          Toggle Diagram
        </button>
      </div>
      {dag.schedule && (
        <div class="mt-2">
          <p>
            <strong>Schedule:</strong> {dag.schedule}
          </p>
        </div>
      )}
      <div class="mt-2">
        <p>
          <strong>ID:</strong> {dag.dagId}
        </p>
      </div>
      {open() && (
        <DagViz
          connections={dag.connections}
          setSelectedTask={setSelectedTask}
        />
      )}
      {open() && selectedTask() !== -1 && taskDetails() && (
        <div class="mt-4 p-4 bg-gray-700 rounded-md">
          <h4 class="text-xl font-semibold mb-2">Task Details</h4>
          <p class="my-2">
            <strong>Name:</strong> {taskDetails()?.name}
          </p>
          {taskDetails()?.command && (
            <p class="my-2">
              <strong>Command:</strong> {taskDetails()?.command!.join(" ")}
            </p>
          )}
          {taskDetails()?.args && (
            <p class="my-2">
              <strong>Args:</strong> {taskDetails()?.args!.join(" ")}
            </p>
          )}
          {taskDetails()?.script && (
            <>
              <p class="my-2">
                <strong>Script:</strong>
              </p>
              <ShellScriptViewer script={taskDetails()?.script!} />
            </>
          )}
          <p>
            <strong>Image:</strong> {taskDetails()?.image}
          </p>
          <p class="mt-2">
            <strong>Parameters:</strong>
          </p>
          <ul class="ml-4 list-disc">
            {taskDetails()?.parameters &&
              taskDetails()!.parameters.map((param, index) => (
                <li>
                  {param.name} - Default{param.isSecret && " Secret"}:{" "}
                  {param.defaultValue ? param.defaultValue : "N/A"}
                </li>
              ))}
          </ul>
          <p class="my-2">
            <strong>BackOff Limit:</strong> {taskDetails()?.backOffLimit}
          </p>
          <p class="my-2">
            <strong>Conditional:</strong>{" "}
            {taskDetails()?.isConditional ? "Yes" : "No"}
          </p>
          <p class="my-2">
            <strong>Retry Codes:</strong> {taskDetails()?.retryCodes}
          </p>
          {taskDetails()?.podTemplate && (
            <>
              <p class="my-2">
                <strong>Pod Template:</strong>
              </p>
              <JsonToYamlViewer json={taskDetails()?.podTemplate!} />
            </>
          )}
        </div>
      )}
    </div>
  );
};

export default DagComponent;
