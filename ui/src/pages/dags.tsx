import { Component, createSignal } from "solid-js";
import { Dag } from "../types/dag";
import { getDags } from "../api/dags";
import DagComponent from "../components/dagComponent";
import DagDiagram from "../components/dagDiagram";

const Dags: Component = () => {
  const [dags, setDags] = createSignal<Dag[]>([]);

  getDags().then((data) => {
    console.log(data);
    setDags(data);
  });

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">Your DAGs</h2>
      <div class="mt-4"></div>
      <div>
        <DagDiagram />

        {dags().map((dag) => (
          <DagComponent dag={dag} />
        ))}
      </div>
    </div>
  );
};

export default Dags;
