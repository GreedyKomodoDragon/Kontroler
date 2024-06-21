import { Dag } from "../types/dag";

interface Props {
  dag: Dag;
}

const DagComponent = ({ dag }: Props) => {


  return (
    <div
      class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white cursor-pointer"
    >
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
  );
};

export default DagComponent;
