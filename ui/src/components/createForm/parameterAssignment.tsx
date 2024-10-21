import { For } from "solid-js";
import { DagParameterSpec, TaskSpec } from "../../types/dagForm";

type ParameterAssignmentProps = {
  taskIndex: number;
  tasks: TaskSpec[];
  parameters: DagParameterSpec[];
  selectedParameters: Record<number, string>;
  setSelectedParameterForTask: (taskIndex: number, id: string) => void;
  addParameterToTask: (taskIndex: number) => void;
};

export function ParameterAssignment(props: ParameterAssignmentProps) {
  const { taskIndex, tasks, parameters, selectedParameters, setSelectedParameterForTask, addParameterToTask } = props;

  return (
    <div>
      <label class="block text-lg font-medium">Assigned Parameters</label>
      <div class="flex space-x-2 items-center">
        <select
          onChange={(e) => setSelectedParameterForTask(taskIndex, e.currentTarget.value)}
          value={selectedParameters[taskIndex] || ""}
          class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md text-gray-200"
        >
          <option value="">Select a parameter</option>
          <For each={parameters}>
            {(param) => <option value={param.id}>{param.name || `Parameter ${parameters.indexOf(param) + 1}`}</option>}
          </For>
        </select>
        <button type="button" onClick={() => addParameterToTask(taskIndex)} class="px-3 py-1 bg-green-600 text-white rounded-md">
          Add
        </button>
      </div>
      <div class="mt-2">
        <For each={tasks[taskIndex].parameters}>
          {(param) => (
            <span class="inline-block bg-blue-600 text-gray-200 text-lg px-2 py-1 mt-2 rounded-full mr-2">
              {parameters.find((parameter) => parameter.id === param)?.name}
            </span>
          )}
        </For>
      </div>
    </div>
  );
}
