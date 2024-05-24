import { Component, createSignal, onMount } from "solid-js";
import { CronJobRun } from "../types/runs";
import RunsComponent from "../components/runsComponent";
import { getCronJobRuns } from "../api/runs";

const Runs: Component = () => {
  // Define cronJobRuns as a signal
  const [cronJobRuns, setCronJobRuns] = createSignal<CronJobRun[]>([]);

  onMount(async () => {
    // Fetch cron job runs from your API
    const runs = await getCronJobRuns();
    setCronJobRuns(runs);
  });

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">CronJob Runs</h2>
      <div>
        {cronJobRuns().map((cronJobRun, index) => (
          <RunsComponent key={index} cronJobRun={cronJobRun} />
        ))}
      </div>
    </div>
  );
};

export default Runs;
