import { createSignal, createUniqueId, For } from "solid-js";
import { createStore } from "solid-js/store";
import { DagParameterSpec, TaskSpec, DagFormObj } from "../types/dagForm";
import { validateDagFormObj } from "../utils/dagform";
import { createDag } from "../api/dags";
import { DeleteTaskButton } from "./deleteTaskButton";
import LabeledInput from "./inputs/labeledInput";
import { ParameterAssignment } from "./createForm/parameterAssignment";
import ErrorToast from "./toasts/errorToast";
import SuccessToast from "./toasts/successToast";

export default function DAGForm() {
  const [errorMsgs, setErrorMsgs] = createSignal<string[]>([]);
  const [successMsg, setSuccessMsg] = createSignal<string>("");
  const [parameters, setParameters] = createStore<DagParameterSpec[]>([]);
  const [tasks, setTasks] = createStore<TaskSpec[]>([]);

  const [selectedParameters, setSelectedParameters] = createSignal<
    Record<number, string>
  >({});

  const [name, setName] = createSignal("");
  const [namespace, setNamespace] = createSignal("");
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
        script: "",
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
    setErrorMsgs([]);
    setSuccessMsg("");

    const dagSpec: DagFormObj = {
      name: name(),
      schedule: schedule(),
      tasks: tasks,
      parameters: parameters,
      namespace: namespace(),
    };

    const errors = validateDagFormObj(dagSpec);
    if (errors.length !== 0) {
      setErrorMsgs(errors);
      return;
    }

    createDag(dagSpec)
      .then((res) => {
        setSuccessMsg(res.message);
      })
      .catch((err) => {
        setErrorMsgs([err.error]);
      });
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        submitDAG();
      }}
      class="w-full border  border-gray-700 bg-gray-800 shadow-2xl rounded-lg p-8 text-gray-200 space-y-8 relative"
    >
      {/* Toast Notifications */}
      {errorMsgs().length !== 0 && (
        <ErrorToast messages={errorMsgs()} clear={() => setErrorMsgs([])} />
      )}
      {successMsg() && (
        <SuccessToast message={successMsg()} clear={() => setSuccessMsg("")} />
      )}

      {/* Form Header */}
      <div class="space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
          <LabeledInput
            label="Name"
            placeholder="DAG Name"
            oninput={(e) => setName(e.currentTarget.value)}
            class="col-span-1"
          />
          <LabeledInput
            label="Namespace"
            placeholder="Kubernetes Namespace"
            oninput={(e) => setNamespace(e.currentTarget.value)}
            class="col-span-1"
          />
          <LabeledInput
            label="Schedule"
            placeholder="*/5 * * * * (Optional)"
            oninput={(e) => setSchedule(e.currentTarget.value)}
            class="col-span-1"
          />
        </div>
      </div>

      {/* Tasks Section */}
      <div class="space-y-6">
        <div class="flex justify-between items-center">
          <h2 class="text-2xl font-semibold">Tasks</h2>
          <button
            type="button"
            onClick={addTask}
            class="flex items-center px-4 py-2 bg-blue-600 text-white rounded-md shadow hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
          >
            <svg
              class="w-5 h-5 mr-2"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 4v16m8-8H4"
              />
            </svg>
            Add Task
          </button>
        </div>
        <For each={tasks}>
          {(task, i) => (
            <div class="bg-gray-700 rounded-lg shadow-inner p-6 space-y-6">
              <div class="flex justify-between items-center">
                <h3 class="text-xl font-medium">Task {i() + 1}</h3>
                <DeleteTaskButton delete={deleteTask} taskIndex={i()} />
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                {/* Task Name */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Task Name
                  </label>
                  <input
                    type="text"
                    value={task.name}
                    onInput={(e) => {
                      setTasks(i(), "name", e.currentTarget.value);
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                    required
                  />
                </div>

                {/* Command */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Command (JSON Array)
                  </label>
                  <input
                    type="text"
                    placeholder='e.g., ["echo", "Hello"]'
                    onInput={(e) => {
                      try {
                        const array = JSON.parse(e.currentTarget.value);
                        if (Array.isArray(array)) {
                          setTasks(i(), "command", array);
                        } else {
                          setTasks(i(), "command", []);
                        }
                      } catch (error) {
                        setTasks(i(), "command", []);
                      }
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                  />
                </div>

                {/* Args */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Args (JSON Array)
                  </label>
                  <input
                    type="text"
                    placeholder='e.g., ["arg1", "arg2"]'
                    onInput={(e) => {
                      try {
                        const array = JSON.parse(e.currentTarget.value);
                        if (Array.isArray(array)) {
                          setTasks(i(), "args", array);
                        } else {
                          setTasks(i(), "args", []);
                        }
                      } catch (error) {
                        setTasks(i(), "args", []);
                      }
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                  />
                </div>

                {/* Image */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Image
                  </label>
                  <input
                    type="text"
                    value={task.image}
                    onInput={(e) => {
                      setTasks(i(), "image", e.currentTarget.value);
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                  />
                </div>

                {/* Backoff Limit */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Backoff Limit
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={task.backoffLimit}
                    onInput={(e) => {
                      setTasks(
                        i(),
                        "backoffLimit",
                        parseInt(e.currentTarget.value) || 0
                      );
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                  />
                </div>

                {/* Retry Codes */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Retry Codes (JSON Array)
                  </label>
                  <input
                    type="text"
                    placeholder="e.g., [404, 500]"
                    onInput={(e) => {
                      try {
                        const array = JSON.parse(e.currentTarget.value);
                        if (
                          Array.isArray(array) &&
                          array.every((item) => typeof item === "number")
                        ) {
                          setTasks(i(), "retryCodes", array);
                        } else {
                          setTasks(i(), "retryCodes", []);
                        }
                      } catch (error) {
                        setTasks(i(), "retryCodes", []);
                      }
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                  />
                </div>

                {/* Script */}
                <div class="md:col-span-2">
                  <label class="block text-sm font-medium text-gray-400">
                    Script (Optional - If provided, Command and Args are ignored)
                  </label>
                  <textarea
                    onInput={(e) => {
                      setTasks(i(), "script", e.currentTarget.value);
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                    rows={3}
                    placeholder="Enter your script here..."
                  ></textarea>
                </div>

                {/* Pod Template */}
                <div class="md:col-span-2">
                  <label class="block text-sm font-medium text-gray-400">
                    Pod Template (Optional)
                  </label>
                  <textarea
                    value={task.podTemplate}
                    onInput={(e) => {
                      setTasks(i(), "podTemplate", e.currentTarget.value);
                    }}
                    rows={5}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                    placeholder="Enter Pod Template in JSON/YAML..."
                  ></textarea>
                </div>
              </div>

              {/* Run After Section */}
              <div>
                <label class="block text-sm font-medium text-gray-400">
                  Run After
                </label>
                <div class="flex flex-wrap items-center mt-2 space-x-2">
                  <select
                    onChange={(e) => {
                      setSelectedTaskToAdd(e.currentTarget.value);
                    }}
                    class="block w-full md:w-auto px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                  >
                    <option value="">Select a Task</option>
                    <For each={tasks.filter((_, index) => index !== i())}>
                      {(t) => (
                        <option
                          value={t.name || `Task ${tasks.indexOf(t) + 1}`}
                        >
                          {t.name || `Task ${tasks.indexOf(t) + 1}`}
                        </option>
                      )}
                    </For>
                  </select>
                  <button
                    type="button"
                    onClick={() => addRunAfter(i())}
                    class="px-4 py-2 bg-blue-600 text-white rounded-md shadow hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition"
                  >
                    Add
                  </button>
                </div>
                <div class="mt-2 flex flex-wrap">
                  <For each={tasks[i()].runAfter}>
                    {(runAfterTask) => (
                      <span class="inline-flex items-center bg-blue-600 text-gray-200 text-sm px-3 py-1 rounded-full mr-2 mb-2">
                        {runAfterTask}
                      </span>
                    )}
                  </For>
                </div>
              </div>

              {/* Parameter Assignment */}
              <ParameterAssignment
                taskIndex={i()}
                tasks={tasks}
                parameters={parameters}
                selectedParameters={selectedParameters()}
                setSelectedParameterForTask={setSelectedParameterForTask}
                addParameterToTask={addParameterToTask}
              />
            </div>
          )}
        </For>
      </div>

      {/* Parameters Section */}
      <div class="space-y-6">
        <div class="flex justify-between items-center">
          <h2 class="text-2xl font-semibold">Parameters</h2>
          <button
            type="button"
            onClick={addParameter}
            class="flex items-center px-4 py-2 bg-green-600 text-white rounded-md shadow hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-500 transition"
          >
            <svg
              class="w-5 h-5 mr-2"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 4v16m8-8H4"
              />
            </svg>
            Add Parameter
          </button>
        </div>
        <For each={parameters}>
          {(param, i) => (
            <div class="bg-gray-700 rounded-lg shadow-inner p-6 space-y-4">
              <div class="flex justify-between items-center">
                <h3 class="text-lg font-medium">Parameter {i() + 1}</h3>
                <DeleteTaskButton delete={deleteParameter} taskIndex={i()} />
              </div>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                {/* Parameter Name */}
                <div>
                  <label class="block text-sm font-medium text-gray-400">
                    Parameter Name
                  </label>
                  <input
                    type="text"
                    value={param.name}
                    onInput={(e) => {
                      setParameterName(i(), e.currentTarget.value);
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                    required
                  />
                </div>

                {/* Secret Toggle */}
                <div class="flex items-center">
                  <label class="flex items-center text-sm font-medium text-gray-400">
                    <input
                      type="checkbox"
                      checked={param.isSecret}
                      onChange={() => handleParameterToggle(i())}
                      class="form-checkbox h-5 w-5 text-indigo-600"
                    />
                    <span class="ml-2">Secret Env</span>
                  </label>
                </div>

                {/* Parameter Value */}
                <div class="md:col-span-2">
                  <label class="block text-sm font-medium text-gray-400">
                    {param.isSecret ? "Secret Name" : "Value"}
                  </label>
                  <input
                    type="text"
                    value={param.value}
                    onInput={(e) => {
                      setParameters(i(), "value", e.currentTarget.value);
                    }}
                    class="mt-1 block w-full px-3 py-2 bg-gray-800 border border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 text-gray-200"
                    required
                  />
                </div>
              </div>
            </div>
          )}
        </For>
      </div>

      {/* Submit Button */}
      <div class="flex justify-center">
        <button
          type="submit"
          class="w-full md:w-auto px-6 py-3 bg-indigo-600 text-white rounded-md shadow hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 transition"
        >
          Submit DAG
        </button>
      </div>
    </form>
  );
}
