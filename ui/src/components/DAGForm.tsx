import { createSignal, createUniqueId, For } from "solid-js";
import { createStore } from "solid-js/store";
import { DagParameterSpec, TaskSpec, DagFormObj } from "../types/dagForm";

type DeleteButtonProps = {
  taskIndex: number;
  delete: (index: number) => void;
};

export function DeleteButton(props: DeleteButtonProps) {
  return (
    <div
      class="bg-red-400 rounded-lg p-2 cursor-pointer"
      onClick={() => {
        props.delete(props.taskIndex);
      }}
    >
      <svg
        width="32"
        height="32"
        viewBox="0 0 24 24"
        fill="white"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M7 4a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v2h4a1 1 0 1 1 0 2h-1.069l-.867 12.142A2 2 0 0 1 17.069 22H6.93a2 2 0 0 1-1.995-1.858L4.07 8H3a1 1 0 0 1 0-2h4zm2 2h6V4H9zM6.074 8l.857 12H17.07l.857-12zM10 10a1 1 0 0 1 1 1v6a1 1 0 1 1-2 0v-6a1 1 0 0 1 1-1m4 0a1 1 0 0 1 1 1v6a1 1 0 1 1-2 0v-6a1 1 0 0 1 1-1"
          fill="#0D0D0D"
        />
      </svg>
    </div>
  );
}

export default function DAGForm() {
  const [parameters, setParameters] = createStore<DagParameterSpec[]>([]);
  const [tasks, setTasks] = createStore<TaskSpec[]>([]);

  const [selectedParameters, setSelectedParameters] = createSignal<
    Record<number, string>
  >({});

  const [name, setName] = createSignal("");
  const [schedule, setSchedule] = createSignal("");
  const [selectedTaskToAdd, setSelectedTaskToAdd] = createSignal("");

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
    if (selectedTaskToAdd()) {
      setTasks(taskIndex, "runAfter", [
        ...(tasks[taskIndex].runAfter || []),
        selectedTaskToAdd(),
      ]);
    }
  };

  const deleteTask = (index: number) => {
    const name = tasks[index].name;

    // Remove the task
    setTasks((tasks) => {
      const newTasks = [...tasks.slice(0, index), ...tasks.slice(index + 1)];

      // Now, update runAfter for the remaining tasks
      for (let i = 0; i < newTasks.length; i++) {
        const ind = newTasks[i].runAfter?.indexOf(name);
        const arr = newTasks[i].runAfter;

        if (ind !== undefined && ind > -1 && arr) {
          arr.splice(ind, 1); // Remove the task name from runAfter
          newTasks[i] = { ...newTasks[i], runAfter: arr }; // Update the task with the new runAfter array
        }
      }

      return newTasks;
    });
  };

  const setSelectedParameterForTask = (taskIndex: number, id: string) => {
    setSelectedParameters((prev) => ({
      ...prev,
      [taskIndex]: id,
    }));
  };

  const deleteParameter = (index: number) => {
    const paramId = parameters[index].id;

    // Remove the parameter
    setParameters((parameters) => {
      const newParameters = [
        ...parameters.slice(0, index),
        ...parameters.slice(index + 1),
      ];

      // Now, update each task's parameters to remove the deleted parameter
      setTasks((tasks) => {
        return tasks.map((task) => {
          const updatedParams = task.parameters?.filter(
            (param) => param !== paramId
          );

          // Return a new task object if parameters have been updated, otherwise return the original task
          return updatedParams?.length !== task.parameters?.length
            ? { ...task, parameters: updatedParams }
            : task;
        });
      });

      return newParameters;
    });
  };

  const addParameter = () => {
    setParameters([
      ...parameters,
      { id: createUniqueId(), name: "", value: "", isSecret: false },
    ]);
  };

  const addParameterToTask = (taskIndex: number) => {
    const selectedParamId = selectedParameters()[taskIndex];

    if (
      selectedParamId &&
      !tasks[taskIndex].parameters?.includes(selectedParamId)
    ) {
      setTasks(taskIndex, "parameters", [
        ...(tasks[taskIndex].parameters || []),
        selectedParamId,
      ]);
    }
  };

  const setParameterName = (paramIndex: number, newName: string) => {
    const oldParam = parameters[paramIndex];

    // Update the parameter name in the parameters store
    setParameters(paramIndex, "name", newName);

    // Now update the tasks that reference the old parameter
    setTasks(
      (task) => task.parameters?.includes(oldParam.id) ?? false, // Ensure it returns a boolean
      "parameters",
      (params) =>
        params?.map(
          (param) => (param === oldParam.id ? oldParam.id : param) // Ensure param ID remains the same
        )
    );
  };

  const handleParameterToggle = (index: number) => {
    setParameters(index, "isSecret", !parameters[index].isSecret);
  };

  const submitDAG = () => {
    const dagSpec: DagFormObj = {
      name: name(),
      schedule: schedule(),
      tasks: tasks,
      parameters: parameters,
    };
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
              <label class="text-lg font-medium flex items-center justify-between">
                Task Name
                <DeleteButton delete={deleteTask} taskIndex={i()} />
              </label>
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
                  <option value="">Select a </option>
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
                    setSelectedParameterForTask(i(), e.currentTarget.value);
                  }}
                  value={selectedParameters()[i()] || ""}
                  class="mt-1 block w-full px-3 py-2 border border-gray-600 bg-gray-800 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm text-gray-200"
                >
                  <option value="">Select a parameter</option>
                  <For each={parameters}>
                    {(param) => (
                      <option value={param.id}>
                        {param.name ||
                          `Parameter ${parameters.indexOf(param) + 1}`}
                      </option>
                    )}
                  </For>
                </select>
                <button
                  type="button"
                  onClick={() => addParameterToTask(i())}
                  class="px-3 py-1 bg-green-600 text-white rounded-md shadow hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
                >
                  Add
                </button>
              </div>

              <div class="mt-2">
                <For each={task.parameters}>
                  {(param) => (
                    <span class="inline-block bg-blue-600 text-gray-200 text-lg px-2 py-1 mt-2 rounded-full mr-2">
                      {
                        parameters.find((parameter) => (parameter.id = param))
                          ?.name
                      }
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
                <label class="text-lg font-medium flex items-center justify-between">
                  Parameter Name
                  <DeleteButton delete={deleteParameter} taskIndex={i()} />
                </label>
                <input
                  type="text"
                  value={param.name}
                  onInput={(e) => {
                    setParameterName(i(), e.currentTarget.value);
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
