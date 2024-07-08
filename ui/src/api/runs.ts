import axios from "axios";
import { CronJobRun, Pod } from "../types/runs";

export async function getCronJobRuns(): Promise<CronJobRun[]> {
  const result = await axios.get(
    "http://localhost:8080/api/v1/single/run/1",
    {},
  );
  const list: CronJobRun[] = [];

  const runs = result.data.runs;

  for (let i = 0; i < runs.length; i++) {
    list.push({
      id: runs[i].runId,
      cronJobName: runs[i].jobUid,
      attempts: runs[i].numberOfAttempts,
      startTime: "2024-05-25T08:00:00Z",
      endTime: "2024-05-25T08:05:00Z",
      finalStatus: runs[i].status,
    });
  }

  return list;
}

export async function getPods(runId: string): Promise<Pod[]> {
  const result = await axios.get(
    `http://localhost:8080/api/v1/single/run/${runId}/pods`,
    {},
  );

  return result.data.pods;
}
