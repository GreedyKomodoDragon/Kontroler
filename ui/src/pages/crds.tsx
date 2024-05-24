import { Component, createSignal } from "solid-js";
import ScheduleComponent from "../components/scheduleComponent";
import { Schedule } from "../types/schedules";

const CRDs: Component = () => {
  const [schedules, setSchedules] = createSignal<Schedule[]>([
    {
      apiVersion: "kubeconductor.greedykomodo/v1alpha1",
      kind: "Schedule",
      metadata: {
        name: "schedule-sample-1",
      },
      spec: {
        cronSchedule: "*/1 * * * *",
        imageName: "alpine:latest",
        command: ["sh", "-c"],
        args: [
          "if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi",
        ],
        backoffLimit: 3,
        conditional: {
          enabled: true,
          retryCodes: [8],
        },
      },
    },
    // Add more schedules as needed
  ]);

  const addSchedule = () => {
    const newSchedule: Schedule = {
      apiVersion: "kubeconductor.greedykomodo/v1alpha1",
      kind: "Schedule",
      metadata: {
        name: `schedule-${schedules().length + 1}`,
      },
      spec: {
        cronSchedule: "*/1 * * * *",
        imageName: "alpine:latest",
        command: ["sh", "-c"],
        args: [
          "if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi",
        ],
        backoffLimit: 3,
        conditional: {
          enabled: true,
          retryCodes: [8],
        },
      },
    };
    setSchedules([...schedules(), newSchedule]);
  };

  return (
    <div class="p-4">
      <h2 class="text-3xl font-semibold mb-4">
        Your Custom Resource Definitions
      </h2>
      <div>
        <button
          class="bg-blue-500 hover:bg-blue-600 text-white font-semibold py-2 px-4 rounded"
          onClick={addSchedule}
        >
          Add New Schedule
        </button>
      </div>
      <div class="mt-4">
        {schedules().map((schedule, index) => (
          <ScheduleComponent key={index} schedule={schedule} />
        ))}
      </div>
    </div>
  );
};

export default CRDs;
