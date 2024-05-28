import { Component, createSignal } from "solid-js";
import ScheduleComponent from "../components/scheduleComponent";
import { Schedule } from "../types/schedules";
import { getScheduleCrds } from "../api/crds";

const CRDs: Component = () => {
  const [schedules, setSchedules] = createSignal<Schedule[]>([]);

  getScheduleCrds().then((data) => {
    setSchedules(data);
  });

  return (
    <div class="p-4">
      <h2 class="text-3xl font-semibold mb-4">
        Your Custom Resource Definitions
      </h2>
      <div class="mt-4">
        {schedules().map((schedule, index) => (
          <ScheduleComponent key={index} schedule={schedule} />
        ))}
      </div>
    </div>
  );
};

export default CRDs;
