import { A } from "@solidjs/router";
import { TaskRunDetails } from "../../types/dag";

function formatTime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainingSeconds = seconds % 60;

  const parts = [];
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  if (remainingSeconds > 0) parts.push(`${remainingSeconds}s`);

  return parts.length > 0 ? parts.join(' ') : '0s';
}

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
            <th class="px-4 py-2 border-b text-left">Start Time</th>
            <th class="px-4 py-2 border-b text-left">Duration</th>
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
                {pod.duration !== null ? formatTime(pod.duration) : "N/A"}
              </td>
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
