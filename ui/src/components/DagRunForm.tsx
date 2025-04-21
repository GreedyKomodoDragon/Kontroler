import { createSignal, For } from "solid-js";
import { createStore } from "solid-js/store";
import ErrorAlert from "./errorAlert";
import SuccessfulAlert from "./successfulAlert";
import SelectMenu from "./inputs/selectMenu";
import LabeledInput from "./inputs/labeledInput";
import { createQuery } from "@tanstack/solid-query";
import { createDagRun, getDagNames, getDagParameters } from "../api/dags";
import { useError } from "../providers/ErrorProvider";
import ErrorSingleAlert from "./alerts/errorSingleAlert";

function debounce<T extends (...args: any[]) => void>(fn: T, delay: number) {
  let timeoutId: ReturnType<typeof setTimeout>;
  return (...args: Parameters<T>) => {
    if (timeoutId) clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  };
}

export default function DagRunForm() {
  const { handleApiError } = useError();

  const [errorMsgs, setErrorMsgs] = createSignal<string[]>([]);
  const [successMsg, setSuccessMsg] = createSignal<string>("");
  const [selectedDag, setSelectedDag] = createSignal<string>("");
  const [namespace, setNamespace] = createSignal<string>("");
  const [runName, setRunName] = createSignal<string>("");

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
    setErrorMsgs([]);

    if (!parameters.data) {
      setErrorMsgs(["No parameters available to submit."]);
      return;
    }

    const errors: string[] = [];
    for (const element of parameters.data) {
      const param = element;
      const userValue = parameterStore[param.name];

      if ((!userValue || userValue.trim() === "") && (param.defaultValue === undefined || param.defaultValue === "")) {
        errors.push(`${param.name} is required but has no value.`);
      }
    }

    if (errors.length > 0) {
      setErrorMsgs(errors);
      return;
    }

    createDagRun(selectedDag(), parameterStore, namespace(), runName())
      .then(() => {
        setSuccessMsg("DAG run created successfully.");
      })
      .catch((error) => handleApiError(error));
  };

  return (
    <form
      onSubmit={handleSubmit}
      class="mx-auto text-gray-200 shadow-md rounded-lg"
    >
      <h2 class="text-2xl mt-4">DagRun Details</h2>
      <div class="my-2">
        <LabeledInput
          label={"Run Name"}
          placeholder={"name"}
          oninput={(e) => setRunName(e.currentTarget.value)}
        />
      </div>
      <div class="mt-2 mb-4">
        <LabeledInput
          label={"Namespace"}
          placeholder={"namespace"}
          oninput={(e) => setNamespace(e.currentTarget.value)}
        />
      </div>
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
        class="mt-2 px-6 py-3 disabled:bg-slate-600 bg-indigo-600 text-white rounded-md shadow hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        disabled={selectedDag() === ""}
      >
        Submit DagRun
      </button>
      {errorMsgs().length !== 0 && (
        <div class="mt-4">
          <ErrorAlert msgs={errorMsgs()} />
        </div>
      )}
      {successMsg() && (
        <div class="mt-4">
          <SuccessfulAlert msg={successMsg()} />
        </div>
      )}
    </form>
  );
}
