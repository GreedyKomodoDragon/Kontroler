import { createSignal } from "solid-js";
import { Dag } from "../types/dag";
import DagDiagram from "./dagDiagram";

interface Props {
  dag: Dag;
}

const DagComponent = ({ dag }: Props) => {
  const [open, setOpen] = createSignal<boolean>(false);

  return (
    <div
      class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white"
      onClick={() => setOpen(!open())}
    >
      <div class="cursor-pointer">
        <div class="flex justify-between items-center">
          <h3 class="text-2xl font-semibold">{dag.name}</h3>
        </div>
        <div class="mt-2">
          <p>
            <strong>Schedule:</strong> {dag.schedule}
          </p>
        </div>
        <div class="mt-2">
          <p>
            <strong>ID:</strong> {dag.dagId}
          </p>
        </div>
      </div>
      {open() && (
        <>
          <DagDiagram
            connections={dag.connections}
          />
        </>
      )}
    </div>
  );
};

export default DagComponent;
