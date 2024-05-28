export interface Pod {
  id: string;
  exitCode: number;
}

export interface CronJobRun {
  id: string;
  cronJobName: string;
  startTime: string;
  endTime: string;
  finalStatus: string;
  attempts: number;
  pods: Pod[];
}
