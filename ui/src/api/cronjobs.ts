import axios from "axios";
import type { CronJob } from "../types/cronjobs";

export async function getCronJob(): Promise<CronJob[]> {
  const result = await axios.get(
    "http://localhost:8080/api/v1/single/cronjob",
    {},
  );

  return result.data.cronJobs;
}
