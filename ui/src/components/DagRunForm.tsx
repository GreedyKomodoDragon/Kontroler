import { createSignal, For } from "solid-js";
import ErrorAlert from "./errorAlert";
import SuccessfulAlert from "./successfulAlert";
import SelectMenu from "./inputs/selectMenu";
import LabeledInput from "./inputs/labeledInput";
import { createStore } from "solid-js/store";
import { createQuery } from "@tanstack/solid-query";
import { getDagNames } from "../api/dags";
import ErrorSingleAlert from "./alerts/errorSingleAlert";

type Parameter = {
  name: string;
  value: string;
};

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

  const [parameters, setParameters] = createStore<Parameter[]>([]);
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

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
      }}
      class="mx-auto text-gray-200 shadow-md rounded-lg"
    >
      <SelectMenu
        selectedValue={selectedDag()}
        search={debouncedSearch}
        setValue={setSelectedDag}
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
      <h2 class="mt-5 text-2xl font-semibold">Parameters</h2>
      <For each={parameters}>
        {(param, i) => <LabeledInput label={param.name} />}
      </For>
      <br/>
      <button
        type="submit"
        class="mt-8 px-6 py-3 bg-indigo-600 text-white rounded-md shadow hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
      >
        Submit DagRun
      </button>
      {errorMsgs().length !== 0 && <ErrorAlert msgs={errorMsgs()} />}
      {successMsg() && <SuccessfulAlert msg={successMsg()} />}
    </form>
  );
}
