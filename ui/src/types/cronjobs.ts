import { Conditional } from "./schedules";

export interface CronJob {
  id: string;
  schedule: string;
  imageName: string;
  command: string[];
  args: string[];
  backoffLimit: number;
  conditionalRetry: Conditional;
}
