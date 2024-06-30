export type Dag = {
  dagId: string;
  name: string;
  schedule: string;
  version: number;
  active: boolean;
  nexttime: string;
};

export type DagRun = {
  connections: Record<string, string[]>;
  taskInfo: Record<string, Task>;
};

export type Task = {
  status: string;
};