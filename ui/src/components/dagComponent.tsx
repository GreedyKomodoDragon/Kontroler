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
            connections={{
              task2: ["task1", "task13"],
              task3: ["task1", "task13"],
              task4: ["task2"],
              task9: ["task6"],
              task10: ["task6", "task7", "task8"],
              task11: ["task9", "task10"],
              task12: ["task10"],
              task1: [],
              task5: ["task2"],
              task6: ["task3"],
              task7: ["task4", "task5"],
              task8: ["task5"],
              task13: [],
            }}
            taskInfo={{
              task1: {
                status: "finished",
              },
              task2: {
                status: "finished",
              },
              task3: {
                status: "finished",
              },
              task4: {
                status: "finished",
              },
              task5: {
                status: "finished",
              },
              task6: {
                status: "finished",
              },
              task7: {
                status: "finished",
              },
              task8: {
                status: "finished",
              },
              task9: {
                status: "running",
              },
              task10: {
                status: "finished",
              },
              task11: {
                status: "failed",
              },
              task12: {
                status: "pending",
              },
              task13: {
                status: "finished",
              },
            }}
          />
        </>
      )}
    </div>
  );
};

export default DagComponent;
