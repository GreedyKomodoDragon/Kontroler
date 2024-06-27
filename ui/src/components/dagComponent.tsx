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
              task1: ["task2", "task3"],
              task2: ["task4", "task5"],
              task3: ["task6"],
              task4: ["task7"],
              task5: ["task7", "task8"],
              task6: ["task9", "task10"],
              task7: ["task10"],
              task8: ["task10"],
              task9: ["task11"],
              task10: ["task11", "task12"],
              task11: [],
              task12: [],
              task13: ["task2", "task3"],
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
                status: "finished",
              },
              task10: {
                status: "finished",
              },
              task11: {
                status: "finished",
              },
              task12: {
                status: "finished",
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
