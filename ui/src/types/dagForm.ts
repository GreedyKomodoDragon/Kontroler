export type DagParameterSpec = {
  id: string;
  name: string;
  isSecret: boolean;
  value: string;
};

export type TaskSpec = {
  name: string;
  command?: string[]; // will need to convert this into an array
  args?: string[]; // will need to convert this into an array
  image: string;
  runAfter?: string[];
  backoffLimit: number;
  retryCodes?: number[];
  parameters?: string[]; // Parameters assigned to the task
  podTemplate?: string;
};

export type DagFormObj = {
  name: string;
  schedule?: string;
  tasks: TaskSpec[];
  parameters?: DagParameterSpec[]; // Global parameters
};
