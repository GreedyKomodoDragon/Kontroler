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
  command?: string[];
  args?: string[];
  image: string;
  parameters: Parameter[];
  backOffLimit: number;
  isConditional: boolean;
  podTemplate: string;
  retryCodes: number[];
  script?: string;
};

export type DagTaskDetails = {
  id: number;
  name: string;
  command?: string[];
  args?: string[];
  image: string;
  parameters: string[];
  backOffLimit: number;
  isConditional: boolean;
  podTemplate: string;
  retryCodes: number[];
  script?: string;
};

export type DashboardStats = {
  dag_count: number;
  successful_dag_runs: number;
  failed_dag_runs: number;
  total_dag_runs: number;
  active_dag_runs: number;
  dag_type_counts: { [key: string]: number };
  task_outcomes: { [key: string]: number };
  daily_dag_run_counts: DailyDagRunCount[];
};

export type DailyDagRunCount = {
  day: string; 
  successful_count: number;
  failed_count: number;
};