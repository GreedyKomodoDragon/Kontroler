export type Dag = {
  dagId: string;
  name: string;
  schedule: string;
  version: number;
  active: boolean;
  nexttime: string;
};

export type DagRunMeta = {
  id: number;
  dagId: number;
  status: string;
  successfulCount: number;
  failedCount: number;
};

export type DagRun = {
  connections: Record<string, string[]>;
  taskInfo: Record<string, Task>;
};

export type Task = {
  status: string;
};