import { createSignal } from "solid-js";
import { Schedule } from "../types/schedules";

interface Props {
  schedule: Schedule;
}

const ScheduleComponent = ({ schedule }: Props) => {
  const [showDetails, setShowDetails] = createSignal(false);

  const toggleDetails = () => setShowDetails(!showDetails());

  return (
    <div class="bg-gray-800 shadow-lg rounded-lg p-4 mb-4 text-white">
      <div
        class="flex justify-between items-center cursor-pointer"
        onClick={toggleDetails}
      >
        <h3 class="text-lg font-semibold">{schedule.name}</h3>
        <span class="text-sm">{schedule.spec.cronSchedule}</span>
      </div>
      {showDetails() && (
        <div class="mt-4">
          <p class="mb-2">
            <strong>Image:</strong> {schedule.spec.imageName}
          </p>
          <p class="mb-2">
            <strong>Command:</strong> {schedule.spec.command.join(" ")}
          </p>
          <p class="mb-2">
            <strong>Args:</strong> {schedule.spec.args.join(" ")}
          </p>
          <p class="mb-2">
            <strong>Backoff Limit:</strong> {schedule.spec.backoffLimit}
          </p>
          {schedule.spec.conditional && (
            <div class="pl-4 border-l-2 border-gray-600">
              <p class="mb-2">
                <strong>Conditional Enabled:</strong>{" "}
                {schedule.spec.conditional.enabled ? "Yes" : "No"}
              </p>
              <p class="mb-2">
                <strong>Retry Codes:</strong>{" "}
                {schedule.spec.conditional.retryCodes.join(", ")}
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ScheduleComponent;
