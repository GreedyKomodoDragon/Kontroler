import { createSignal } from "solid-js";
import { CronJob } from "../types/cronjobs";

interface Props {
  cronJob: CronJob;
}

const CronJobComponent = ({ cronJob }: Props) => {
  const [showDetails, setShowDetails] = createSignal(false);

  const toggleDetails = () => setShowDetails(!showDetails());

  return (
    <div
      class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white cursor-pointer"
      onClick={toggleDetails}
    >
      <div class="flex justify-between items-center">
        <h3 class="text-2xl font-semibold">{cronJob.id}</h3>
      </div>
      <div class="mt-2">
        <p>
          <strong>Schedule:</strong> {cronJob.schedule}
        </p>
      </div>
      <div class="mt-2">
        <p>
          <strong>ID:</strong> {cronJob.id}
        </p>
      </div>
      {showDetails() && (
        <>
          <h4 class="text-lg font-semibold mt-2">Details:</h4>
          <div class="bg-gray-700 rounded-md p-3 mt-2">
            <p>
              <strong>Image:</strong> {cronJob.imageName}
            </p>
            <p>
              <strong>Command:</strong> {cronJob.command}
            </p>
            <p>
              <strong>Args:</strong> {cronJob.args}
            </p>
            <p>
              <strong>BackoffLimit:</strong> {cronJob.backoffLimit}
            </p>
            {cronJob.conditionalRetry.enabled && (
              <p>
                <strong>RetyCodes:</strong>{" "}
                {cronJob.conditionalRetry.retryCodes}
              </p>
            )}
          </div>
        </>
      )}
    </div>
  );
};

export default CronJobComponent;
