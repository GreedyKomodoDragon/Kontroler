import { DagRunMeta } from "../types/dag";
import { A } from "@solidjs/router";

interface Props {
  dagRun: DagRunMeta;
}

const DagRunComponent = ({ dagRun }: Props) => {
  return (
    <div class="bg-gray-800 shadow-2xl rounded-lg p-6 mb-6 text-white border border-gray-700 relative">
      {/* Content */}
      <div class="flex justify-between items-center border-b border-gray-700 pb-4">
        <h3 class="text-3xl font-bold tracking-tight text-gray-100">
          DAG Run ID: {dagRun.id}
        </h3>
        <A
          class="bg-blue-600 hover:bg-blue-500 transition-colors duration-300 px-4 py-2 rounded-md text-sm font-semibold relative z-10"
          href={`/dags/run/${dagRun.id}`}
        >
          Go to DAG Run
        </A>
      </div>
      <div class="mt-4 space-y-2">
        <p class="text-sm text-gray-400">
          <strong class="font-medium text-gray-300">DAG ID:</strong>{" "}
          {dagRun.dagId}
        </p>
        <p class="text-sm text-gray-400">
          <strong class="font-medium text-gray-300">Successful Tasks:</strong>{" "}
          {dagRun.successfulCount}
        </p>
        <p class="text-sm text-gray-400">
          <strong class="font-medium text-gray-300">Failed Tasks:</strong>{" "}
          {dagRun.failedCount}
        </p>
      </div>
    </div>
  );
};

export default DagRunComponent;
