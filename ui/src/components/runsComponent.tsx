import { CronJobRun, Pod } from "../types/runs";
import { createSignal } from "solid-js";

interface Props {
  cronJobRun: CronJobRun;
}

const RunComponent = ({ cronJobRun }: Props) => {
  const [showPods, setShowPods] = createSignal(false);

  const togglePods = () => setShowPods(!showPods());

  return (
    <div class="bg-gray-800 shadow-md rounded-md p-4 mb-4 text-white">
      <div class="flex justify-between items-center">
        <h3 class="text-lg font-semibold">Run ID: {cronJobRun.id}</h3>
        <span
          class={`text-sm px-2 py-1 rounded ${cronJobRun.finalStatus === "Success" ? "bg-green-500" : "bg-red-500"}`}
        >
          {cronJobRun.finalStatus}
        </span>
      </div>
      <div class="mt-2">
        <p>
          <strong>Cron Job:</strong> {cronJobRun.cronJobName}
        </p>
        <p>
          <strong>Start Time:</strong> {cronJobRun.startTime}
        </p>
        <p>
          <strong>End Time:</strong> {cronJobRun.endTime}
        </p>
        <h4 class="text-lg font-semibold mt-2">
          <button
            class="text-blue-500 hover:underline focus:outline-none"
            onClick={togglePods}
          >
            {showPods() ? "Hide Pods" : "Show Pods"}
          </button>
        </h4>
        {showPods() && (
          <div>
            <h4 class="text-lg font-semibold mt-2">Pods:</h4>
            {cronJobRun.pods.map((pod: Pod, index: number) => (
              <div key={index} class="bg-gray-700 rounded-md p-3 mt-2">
                <p>
                  <strong>Pod ID:</strong> {pod.id}
                </p>
                <p>
                  <strong>Exit Code:</strong> {pod.exitCode}
                </p>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default RunComponent;
