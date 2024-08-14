import { createSignal, For } from "solid-js";
import { createStore } from "solid-js/store";

type DagParameterSpec = {
  name: string;
  isSecret: boolean;
  value: string;
};

type TaskSpec = {
  name: string;
  command?: string[]; // will need to convert this into an array
  args?: string[]; // will need to convert this into an array
  image: string;
  runAfter?: string[];
  backoffLimit: number;
  retryCodes?: number[];
  parameters?: string[]; // Parameters assigned to the task
  podTemplate?: string;
};

type DAGSpec = {
  name: string;
  schedule?: string;
  task: TaskSpec[];
  parameters?: DagParameterSpec[]; // Global parameters
};

export default function DAGForm() {
  const [parameters, setParameters] = createStore<DagParameterSpec[]>([]);
  const [tasks, setTasks] = createStore<TaskSpec[]>([]);

  const [name, setName] = createSignal("");
  const [schedule, setSchedule] = createSignal("");
  const [selectedTaskToAdd, setSelectedTaskToAdd] = createSignal("");
  const [selectedParameterToAdd, setSelectedParameterToAdd] = createSignal("");

  const addTask = () => {
    setTasks([
      ...tasks,
      {
        name: "",
        command: [],
        args: [],
        image: "",
        runAfter: [],
        backoffLimit: 0,
        retryCodes: [],
        parameters: [],
        podTemplate: "",
      },
    ]);
  };

  const addRunAfter = (taskIndex: number) => {
    console.log(taskIndex, selectedTaskToAdd());
    if (selectedTaskToAdd()) {
      setTasks(taskIndex, "runAfter", [
        ...(tasks[taskIndex].runAfter || []),
        selectedTaskToAdd(),
      ]);
    }
  };

  const addParameter = () => {
    setParameters([...parameters, { name: "", value: "", isSecret: false }]);
  };

  const addParameterToTask = (taskIndex: number) => {
    if (selectedParameterToAdd()) {
      const paramToAdd = parameters.find(
        (param) => param.name === selectedParameterToAdd()
      );
      if (paramToAdd) {
        setTasks(taskIndex, "parameters", [
          ...(tasks[taskIndex].parameters || []),
          selectedParameterToAdd(),
        ]);
      }
    }
  };

  const handleParameterToggle = (index: number) => {
    setParameters(index, "isSecret", !parameters[index].isSecret);
  };

  const submitDAG = () => {
    const dagSpec: DAGSpec = {
      name: name(),
      schedule: schedule(),
      task: tasks,
      parameters: parameters,
    };
    console.log(dagSpec);
    // Submit logic here
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        submitDAG();
      }}
      class="space-y-8 mx-auto text-gray-200 shadow-md rounded-lg"
    >
      <div>
        <label class="block text-lg font-medium">Name</label>
        <input
          type="text"
          value={name()}
          onInput={(e) => setName(e.currentTarget.value)}
          class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-700 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
        />
      </div>
      <div>
        <label class="block text-lg font-medium">
          Cron Schedule (Optional)
        </label>
        <input
          type="text"
          value={schedule()}
          onInput={(e) => setSchedule(e.currentTarget.value)}
          class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-700 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
        />
      </div>

      <h2 class="text-2xl font-semibold">Tasks</h2>
      <For each={tasks}>
        {(task, i) => (
          <div class="p-4 border rounded-lg bg-gray-700 border-gray-600 space-y-4">
            <div>
              <label class="block text-lg font-medium">Task Name</label>
              <input
                type="text"
                value={task.name}
                onInput={(e) => {
                  setTasks(i(), "name", e.currentTarget.value);
                }}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">Command</label>
              <input
                type="text"
                onInput={(e) => {
                  try {
                    const array = JSON.parse(e.currentTarget.value);
                    if (Array.isArray(array)) {
                      setTasks(i(), "command", array);
                    } else {
                      setTasks(i(), "command", undefined);
                    }
                  } catch (error) {
                    setTasks(i(), "command", undefined);
                  }
                }}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">Args</label>
              <input
                type="text"
                onInput={(e) => {
                  try {
                    const array = JSON.parse(e.currentTarget.value);
                    if (Array.isArray(array)) {
                      setTasks(i(), "args", array);
                    } else {
                      setTasks(i(), "args", undefined);
                    }
                  } catch (error) {
                    setTasks(i(), "args", undefined);
                  }
                }}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">Image</label>
              <input
                type="text"
                value={task.image}
                onInput={(e) => {
                  setTasks(i(), "image", e.currentTarget.value);
                }}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">Retry Codes</label>
              <input
                type="text"
                onInput={(e) => {
                  try {
                    const array = JSON.parse(e.currentTarget.value);
                    if (
                      Array.isArray(array) &&
                      array.every((item) => typeof item === "number")
                    ) {
                      setTasks(i(), "retryCodes", array);
                    } else {
                      setTasks(i(), "retryCodes", undefined);
                    }
                  } catch (error) {
                    setTasks(i(), "retryCodes", undefined);
                  }
                }}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">BackoffLimit</label>
              <input
                type="text"
                onInput={(e) => {
                  setTasks(
                    i(),
                    "backoffLimit",
                    parseInt(e.currentTarget.value)
                  );
                }}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">
                Pod Template (Optional)
              </label>
              <textarea
                value={task.podTemplate}
                onInput={(e) => {
                  setTasks(i(), "podTemplate", e.currentTarget.value);
                }}
                rows={10}
                class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
              />
            </div>

            <div>
              <label class="block text-lg font-medium">Run After</label>
              <div class="flex space-x-2 items-center">
                <select
                  onChange={(e) => {
                    setSelectedTaskToAdd(e.currentTarget.value);
                  }}
                  class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
                >
                  <option value="">Select a parameter</option>
                  <For each={tasks.filter((_, index) => index !== i())}>
                    {(t) => (
                      <option value={t.name}>
                        {t.name || `Task ${tasks.indexOf(t) + 1}`}
                      </option>
                    )}
                  </For>
                </select>
                <button
                  type="button"
                  onClick={() => addRunAfter(i())}
                  class="px-3 py-1 bg-blue-600 text-white rounded-md shadow hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                >
                  Add
                </button>
              </div>
              <div class="mt-2">
                <For each={tasks[i()].runAfter}>
                  {(runAfterTask) => (
                    <span class="inline-block bg-blue-600 text-gray-200 text-lg px-2 py-1 mt-2 rounded-full mr-2">
                      {runAfterTask}
                    </span>
                  )}
                </For>
              </div>
            </div>

            <div>
              <label class="block text-lg font-medium">
                Assigned Parameters
              </label>
              <div class="flex space-x-2 items-center">
                <select
                  onChange={(e) => {
                    setSelectedParameterToAdd(e.currentTarget.value);
                  }}
                  class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
                >
                  <option value="">Select a parameter</option>
                  <For each={parameters}>
                    {(param) => (
                      <option value={param.name}>
                        {param.name ||
                          `Parameter ${parameters.indexOf(param) + 1}`}
                      </option>
                    )}
                  </For>
                </select>
                <button
                  type="button"
                  onClick={() => {
                    addParameterToTask(i());
                  }}
                  class="px-3 py-1 bg-green-600 text-white rounded-md shadow hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
                >
                  Add
                </button>
              </div>
              <div class="mt-2">
                <For each={task.parameters}>
                  {(param) => (
                    <span class="inline-block bg-blue-600 text-gray-200 text-lg px-2 py-1 mt-2 rounded-full mr-2">
                      {param}
                    </span>
                  )}
                </For>
              </div>
            </div>
          </div>
        )}
      </For>
      <button
        type="button"
        onClick={addTask}
        class="mt-4 px-6 py-2 bg-blue-600 text-white rounded-md shadow hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
      >
        Add Task
      </button>

      <h2 class="text-2xl font-semibold">Parameters</h2>
      <div class="rounded-lg space-y-4">
        <For each={parameters}>
          {(param, i) => (
            <div class="mt-4 p-4 border rounded-lg bg-gray-700 border-gray-600 space-y-2">
              <div>
                <label class="block text-lg font-medium">Parameter Name</label>
                <input
                  type="text"
                  value={param.name}
                  onInput={(e) => {
                    setParameters(i(), "name", e.currentTarget.value);
                  }}
                  class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
                />
              </div>

              <div class="flex items-center space-x-4 mt-2">
                <label class="block text-lg font-medium">Secret Env</label>
                <input
                  type="checkbox"
                  checked={param.isSecret}
                  onChange={() => handleParameterToggle(i())}
                  class="form-checkbox h-5 w-5 text-indigo-600"
                />
              </div>
              <div>
                <label class="block text-lg font-medium">
                  {param.isSecret ? "Secret Name" : "Value"}
                </label>
                <input
                  type="text"
                  value={param.value}
                  onInput={(e) => {
                    setParameters(i(), "value", e.currentTarget.value);
                  }}
                  class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
                />
              </div>
            </div>
          )}
        </For>
        <button
          type="button"
          onClick={addParameter}
          class="mt-4 px-4 py-2 bg-green-600 text-white rounded-md shadow hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
        >
          Add Parameter
        </button>
      </div>

      <button
        type="submit"
        class="mt-8 px-6 py-3 bg-indigo-600 text-white rounded-md shadow hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
      >
        Submit DAG
      </button>
    </form>
  );
}
