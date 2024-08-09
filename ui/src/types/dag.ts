export type Dag = {
  dagId: string;
  name: string;
  schedule: string;
  version: number;
  active: boolean;
  nexttime: string;
  connections: Record<string, string[]>;
};

export type DagRunMeta = {
  id: number;
  dagId: number;
  status: string;
  successfulCount: number;
  failedCount: number;
};

export type DagRunGraph = {
  connections: Record<string, string[]>;
  taskInfo: Record<string, Task>;
};

export type Task = {
  status: string;
};

export type DagRunAll = {
  id: number;
  dagId: number;
  status: string;
  successfulCount: number;
  failedCount: number;
  connections: Record<string, string[]>;
  taskInfo: Record<string, Task>;
};

export type TaskPod = {
  podUID: string;
  status: string;
  name: string;
  exitCode: number;
};

export type TaskRunDetails = {
  id: number;
  status: string;
  attempts: number;
  pods: TaskPod[];
};

export type Parameter = {
  id: number;
  name: string;
  isSecret: boolean;
  defaultValue: string;
};

export type TaskDetails = {
  id: number;
  name: string;
  command: string[];
  args: string[];
  image: string;
  parameters: Parameter[];
  backOffLimit: number;
  isConditional: boolean;
  podTemplate: string;
  retryCodes: number[];
};
