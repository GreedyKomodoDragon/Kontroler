import { createSignal, For } from "solid-js";
import { createStore } from "solid-js/store";
import ErrorAlert from "./errorAlert";
import SuccessfulAlert from "./successfulAlert";
import SelectMenu from "./inputs/selectMenu";
import LabeledInput from "./inputs/labeledInput";
import { createQuery } from "@tanstack/solid-query";
import { getDagNames, getDagParameters } from "../api/dags";
import ErrorSingleAlert from "./alerts/errorSingleAlert";

function debounce<T extends (...args: any[]) => void>(fn: T, delay: number) {
  let timeoutId: ReturnType<typeof setTimeout>;
  return (...args: Parameters<T>) => {
    if (timeoutId) clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  };
}

export default function DagRunForm() {
  const [errorMsgs, setErrorMsgs] = createSignal<string[]>([]);
  const [successMsg, setSuccessMsg] = createSignal<string>("");
  const [selectedDag, setSelectedDag] = createSignal<string>("");

  const [debouncedValue, setDebouncedValue] = createSignal<string>("");

  const debouncedSearch = debounce((value: string) => {
    setDebouncedValue(value);
  }, 500);

  const dags = createQuery(() => ({
    queryKey: ["dags", debouncedValue()],
    queryFn: getDagNames,
    staleTime: 5 * 60 * 1000,
  }));

  const parameters = createQuery(() => ({
    queryKey: ["dagsParameters", selectedDag()],
    queryFn: getDagParameters,
    staleTime: 5 * 60 * 1000,
  }));

  const [parameterStore, setParameterStore] = createStore<
    Record<string, string>
  >({});

  const handleDagChange = (dagName: string) => {
    setSelectedDag(dagName);

    // Clear the parameters when DAG changes
    setParameterStore({});
  };

  const handleInputChange = (paramName: string, value: string) => {
    setParameterStore(paramName, value);
  };

  const handleSubmit = (e: Event) => {
    e.preventDefault();

    if (!parameters.data) {
      setErrorMsgs(["No parameters available to submit."]);
      return;
    }

    const parameterValues = { ...parameterStore };

    const errors: string[] = [];
    for (let i = 0; i < parameters.data.length; i++) {
      const param = parameters.data[i];
      const userValue = parameterStore[param.name];

      // if it is empty and there is no default value then error
      if (
        (!userValue || userValue.trim() === "") &&
        (param.defaultValue === undefined || param.defaultValue === "")
      ) {
        errors.push(`${param.name} is required but has no value.`);
      }
    }

    if (errors.length > 0) {
      setErrorMsgs(errors);
      return;
    }

    console.log("Submitting with parameters:", parameterValues);
  };

  return (
    <form
      onSubmit={handleSubmit}
      class="mx-auto text-gray-200 shadow-md rounded-lg"
    >
      <SelectMenu
        selectedValue={selectedDag()}
        search={debouncedSearch}
        setValue={handleDagChange}
        items={dags.data || []}
      />
      {!dags.isLoading &&
        debouncedValue() !== "" &&
        dags.data &&
        dags.data.length == 0 && (
          <div class="mt-20">
            <ErrorSingleAlert msg="No results found" />
          </div>
        )}

      {parameters.isLoading && <div>Loading parameters...</div>}
      {parameters.isError && <div>Error loading parameters</div>}
      {selectedDag() !== "" &&
        parameters.data &&
        parameters.data.length === 0 && (
          <div class="mt-4">No parameters available for this DAG.</div>
        )}

      {parameters.data && parameters.data.length > 0 && (
        <>
          <h2 class="text-2xl mt-4">Parameters</h2>
          <For each={parameters.data}>
            {(param) => (
              <div class="my-2">
                <LabeledInput
                  label={param.name}
                  placeholder={param.defaultValue}
                  oninput={(e) =>
                    handleInputChange(param.name, e.currentTarget.value)
                  } // Update store on input change
                />
              </div>
            )}
          </For>
        </>
      )}
      <br />
      <button
        type="submit"
        class="mt-8 px-6 py-3 disabled:bg-slate-600 bg-indigo-600 text-white rounded-md shadow hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        disabled={selectedDag() === ""}
      >
        Submit DagRun
      </button>
      {errorMsgs().length !== 0 && <ErrorAlert msgs={errorMsgs()} />}
      {successMsg() && <SuccessfulAlert msg={successMsg()} />}
    </form>
  );
}
