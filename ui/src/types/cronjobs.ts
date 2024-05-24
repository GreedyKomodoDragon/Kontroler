export interface CronJob {
  name: string;
  schedule: string;
  status: string;
  retries: number;
}
