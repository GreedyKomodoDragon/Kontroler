export interface Metadata {
  name: string;
}

export interface Conditional {
  enabled: boolean;
  retryCodes: number[];
}

export interface Spec {
  cronSchedule: string;
  imageName: string;
  command: string[];
  args: string[];
  backoffLimit: number;
  conditional?: Conditional;
}

export interface Schedule {
  apiVersion: string;
  kind: string;
  metadata: Metadata;
  spec: Spec;
}
