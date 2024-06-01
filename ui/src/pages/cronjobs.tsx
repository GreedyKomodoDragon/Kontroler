import { createSignal, type Component } from "solid-js";
import type { CronJob } from "../types/cronjobs";
import { getCronJob } from "../api/cronjobs";
import CronJobComponent from "../components/cronJobComponent";

const CronJobs: Component = () => {
  const [cronJobs, setCronJobs] = createSignal<CronJob[]>([]);

  getCronJob().then((data) => {
    setCronJobs(data);
  });

  return (
    <div class="p-4">
      <h2 class="text-2xl font-semibold mb-4">CronJobs</h2>
      <div>
        {cronJobs().map((cronJob) => (
          <CronJobComponent cronJob={cronJob} />
        ))}
      </div>
    </div>
  );
};

export default CronJobs;
