import { createEffect, createSignal } from "solid-js";
import { Dag, TaskDetails } from "../types/dag";
import { getTaskDetails, deleteDag } from "../api/dags";
import ShellScriptViewer from "./code/shellScriptViewer";
import JsonToYamlViewer from "./code/JsonToYamlViewer";
import DagViz from "./dagViz";
import { DeleteTaskButton } from "./deleteTaskButton";
import { useError } from "../providers/ErrorProvider";

interface Props {
  dag: Dag;
  onDelete: () => void;
}

type deleteArgs = {
  namespace: string;
  name: string;
}

const DagComponent = ({ dag, onDelete }: Props) => {
  const [open, setOpen] = createSignal<boolean>(false);
  const [selectedTask, setSelectedTask] = createSignal<number>(-1);
  const [taskDetails, setTaskDetails] = createSignal<TaskDetails | undefined>();
  const { setGlobalErrorMessage } = useError();

  const handleDelete = async (arg: deleteArgs) => {
    try {
      await deleteDag(arg.namespace, arg.name);
      onDelete();
    } catch (err) {
      setGlobalErrorMessage(err instanceof Error ? err.message : "An unknown error occurred");
    }
  };

  createEffect(() => {
    if (selectedTask() === -1) return;

    getTaskDetails(selectedTask()).then((data) => setTaskDetails(data));
  });

  return (
    <div class="bg-gray-800 shadow-2xl rounded-lg p-6 mb-6 text-white border border-gray-700 relative">
      <div class="flex justify-between items-center border-b border-gray-600 pb-4">
        <h3 class="text-3xl font-bold tracking-tight text-gray-100">
          {dag.name}
        </h3>
        <div class="flex gap-4 items-center">
          <button
            class="rounded-md bg-blue-600 hover:bg-blue-500 transition-colors duration-300 px-4 py-2 text-sm font-semibold relative z-10"
            onClick={() => setOpen(!open())}
          >
            {open() ? "Hide Diagram" : "Show Diagram"}
          </button>
          <DeleteTaskButton delete={handleDelete} taskIndex={{
            namespace: dag.namespace,
            name: dag.name,
          }} size="s" />
        </div>
      </div>
      <div class="mt-4 space-y-2">
        {dag.schedule && (
          <p class="text-sm text-gray-400">
            <strong class="font-medium text-gray-300">Schedule:</strong>{" "}
            {dag.schedule}
          </p>
        )}
        <p class="text-sm text-gray-400">
          <strong class="font-medium text-gray-300">ID:</strong> {dag.dagId}
        </p>
        {open() && (
          <p class="text-sm text-gray-400">
            Click on node to see information on task
          </p>
        )}
      </div>

      {open() && (
        <div class="mt-6">
          <DagViz
            connections={dag.connections}
            setSelectedTask={setSelectedTask}
          />
        </div>
      )}

      {open() && selectedTask() !== -1 && taskDetails() && (
        <div class="mt-8 p-6 bg-gray-700 rounded-lg shadow-inner">
          <h4 class="text-xl font-semibold text-gray-200 mb-4">Task Details</h4>
          <div class="space-y-3">
            <p>
              <strong class="font-medium text-gray-300">Name:</strong>{" "}
              {taskDetails()?.name}
            </p>
            {taskDetails()?.command && (
              <p>
                <strong class="font-medium text-gray-300">Command:</strong>{" "}
                <span class="text-gray-400">
                  {taskDetails()?.command!.join(" ")}
                </span>
              </p>
            )}
            {taskDetails()?.args && (
              <p>
                <strong class="font-medium text-gray-300">Args:</strong>{" "}
                <span class="text-gray-400">
                  {taskDetails()?.args!.join(" ")}
                </span>
              </p>
            )}
            {taskDetails()?.script && (
              <div>
                <p class="mb-2">
                  <strong class="font-medium text-gray-300">Script:</strong>
                </p>
                <ShellScriptViewer script={taskDetails()?.script!} />
              </div>
            )}
            <p>
              <strong class="font-medium text-gray-300">Image:</strong>{" "}
              {taskDetails()?.image}
            </p>
            <p>
              <strong class="font-medium text-gray-300">Parameters:</strong>
            </p>
            <ul class="ml-4 list-disc text-gray-400">
              {taskDetails()?.parameters &&
                taskDetails()!.parameters.map((param) => (
                  <li>
                    {param.name} - Default
                    {param.isSecret && " Secret"}:{" "}
                    {param.defaultValue ? param.defaultValue : "N/A"}
                  </li>
                ))}
            </ul>
            <p>
              <strong class="font-medium text-gray-300">BackOff Limit:</strong>{" "}
              {taskDetails()?.backOffLimit}
            </p>
            <p>
              <strong class="font-medium text-gray-300">Conditional:</strong>{" "}
              {taskDetails()?.isConditional ? "Yes" : "No"}
            </p>
            <p>
              <strong class="font-medium text-gray-300">Retry Codes:</strong>{" "}
              {taskDetails()?.retryCodes}
            </p>
            {taskDetails()?.podTemplate && (
              <div>
                <p class="mb-2">
                  <strong class="font-medium text-gray-300">
                    Pod Template:
                  </strong>
                </p>
                <JsonToYamlViewer json={taskDetails()?.podTemplate!} />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default DagComponent;
