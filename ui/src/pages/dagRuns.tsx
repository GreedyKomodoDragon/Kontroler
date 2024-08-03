import { Component, createSignal } from "solid-js";
import { DagRunMeta } from "../types/dag";
import { getDagRuns } from "../api/dags";
import DagRunComponent from "../components/dagRunComponent";

const DagRuns: Component = () => {
  const [runs, setRuns] = createSignal<DagRunMeta[]>([]);

  getDagRuns(1).then((data) => setRuns(data));

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">DAG Runs</h2>
      <div class="mt-4"></div>
      <div>
        {runs().map((run) => (
          <DagRunComponent dagRun={run} />
        ))}
      </div>
    </div>
  );
};

export default DagRuns;
