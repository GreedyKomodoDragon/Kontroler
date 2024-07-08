import { createSignal } from "solid-js";
import { DagRun, DagRunMeta } from "../types/dag";
import DagDiagram from "./dagDiagram";
import { getDagRun } from "../api/dags";

interface Props {
  dagRun: DagRunMeta;
}

const DagRunComponent = ({ dagRun }: Props) => {
  const [open, setOpen] = createSignal<boolean>(false);
  const [runStatus, setRunStatus] = createSignal<DagRun | undefined>(undefined);

  return (
    <div
      class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white"
      onClick={async () => {
        if (runStatus() === undefined) {
          const status = await getDagRun(dagRun.id);
          setRunStatus(status);
        }

        setOpen(!open());
      }}
    >
      <div class="cursor-pointer">
        <div class="flex justify-between items-center">
          <h3 class="text-2xl font-semibold">DAG Run Id: {dagRun.id}</h3>
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
      {open() && runStatus() !== undefined && (
        <DagDiagram
          connections={runStatus()?.connections ?? {}}
          taskInfo={runStatus()?.taskInfo ?? {}}
        />
      )}
    </div>
  );
};

export default DagRunComponent;
