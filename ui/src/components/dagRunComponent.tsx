import { DagRunMeta } from "../types/dag";
import { A } from "@solidjs/router";

interface Props {
  dagRun: DagRunMeta;
}

const DagRunComponent = ({ dagRun }: Props) => {
  return (
    <div class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white">
      <div class="flex justify-between items-center">
        <h3 class="text-2xl font-semibold">DAG Run Id: {dagRun.id}</h3>
        <A class="mr-2 bg-blue-500 p-2 rounded-md" href={`/dags/run/${dagRun.id}`}>
          Go to DagRun
        </A>
      </div>
      <div class="mt-2">
        <p>
          <strong>DAG ID:</strong> {dagRun.dagId}
        </p>
      </div>
      <div class="mt-2">
        <p>
          <strong>Successful Tasks:</strong> {dagRun.successfulCount}
        </p>
        <p>
          <strong>Failed Tasks:</strong> {dagRun.failedCount}
        </p>
      </div>
    </div>
  );
};

export default DagRunComponent;
