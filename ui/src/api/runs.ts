// src/api/cronJobRuns.ts

import { CronJobRun } from "../types/runs";

export async function getCronJobRuns(): Promise<CronJobRun[]> {
  // Dummy data
  const dummyCronJobRuns: CronJobRun[] = [
    {
      id: "1",
      cronJobName: "example-cron-job-1",
      startTime: "2024-05-25T08:00:00Z",
      endTime: "2024-05-25T08:05:00Z",
      finalStatus: "Success",
      pods: [
        {
          id: "pod-1",
          exitCode: 0,
        },
        {
          id: "pod-2",
          exitCode: 0,
        },
      ],
    },
    {
      id: "2",
      cronJobName: "example-cron-job-2",
      startTime: "2024-05-25T09:00:00Z",
      endTime: "2024-05-25T09:05:00Z",
      finalStatus: "Failed",
      pods: [
        {
          id: "pod-3",
          exitCode: 1,
        },
        {
          id: "pod-4",
          exitCode: 0,
        },
      ],
    },
    // Add more dummy cron job runs as needed
  ];

  // Simulate API delay
  await new Promise((resolve) => setTimeout(resolve, 1000));

  return dummyCronJobRuns;
}
