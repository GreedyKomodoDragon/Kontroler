import { A } from "@solidjs/router";
import { TaskRunDetails } from "../../types/dag";

type PodStatusTableProps = {
  details: TaskRunDetails;
  id: number;
};

export function PodStatusTable(props: PodStatusTableProps) {
  return (
    <div class="overflow-x-auto">
      <table class="min-w-full table-auto border-collapse">
        <thead>
          <tr class="bg-gray-800">
            <th class="px-4 py-2 border-b text-left">Execution Order</th>
            <th class="px-4 py-2 border-b text-left">Pod Name</th>
            <th class="px-4 py-2 border-b text-left">Status</th>
            <th class="px-4 py-2 border-b text-left">Exit Code</th>
            <th class="px-4 py-2 border-b text-left">Actions</th>
          </tr>
        </thead>
        <tbody>
          {props.details.pods.map((pod, i) => (
            <tr class="border-b hover:bg-gray-500">
              <td class="px-4 py-2">{i + 1}</td>
              <td class="px-4 py-2">{pod.name}</td>
              <td
                class={`px-4 py-2 ${
                  pod.status.toLowerCase() === "failed" ? "text-red-500" : ""
                }`}
              >
                {pod.status}
              </td>
              <td class="px-4 py-2">{pod.exitCode}</td>
              <td class="px-4 py-2">
                <A
                  href={`/logs/run/${props.id}/pod/${pod.podUID}`}
                  class="inline-block rounded-md bg-sky-700 p-2 text-white"
                >
                  See Logs
                </A>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
