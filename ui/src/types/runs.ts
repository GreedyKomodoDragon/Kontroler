export interface Pod {
  name: string;
  exitCode: number;
}

export interface CronJobRun {
  id: string;
  cronJobName: string;
  startTime: string;
  endTime: string;
  finalStatus: string;
  attempts: number;
}
