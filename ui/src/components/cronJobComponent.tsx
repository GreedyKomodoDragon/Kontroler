import { CronJob } from "../types/cronjobs";

interface Props {
  cronJob: CronJob;
}

const CronJobComponent = ({ cronJob }: Props) => {
  let statusColorClass = "bg-gray-400"; // Default color for unknown status

  // Determine the color class based on the status
  if (cronJob.status === "Running") {
    statusColorClass = "bg-green-500";
  } else if (cronJob.status === "Failed") {
    statusColorClass = "bg-red-500";
  } else if (cronJob.status === "Pending") {
    statusColorClass = "bg-yellow-500";
  }

  return (
    <div class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white">
      <div class="flex justify-between items-center">
        <h3 class="text-3xl font-semibold">{cronJob.name}</h3>
        <span class={`text-md px-3 py-2 rounded ${statusColorClass}`}>
          {cronJob.status}
        </span>
      </div>
      <div class="mt-2">
        <p>
          <strong>Schedule:</strong> {cronJob.schedule}
        </p>
        <p>
          <strong>Retries:</strong> {cronJob.retries}
        </p>
        {/* Add other details as needed */}
      </div>
    </div>
  );
};

export default CronJobComponent;
