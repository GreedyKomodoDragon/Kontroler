export type DagParameterSpec = {
  id: string;
  name: string;
  isSecret: boolean;
  value: string;
};

export type TaskSpec = {
  name: string;
  command?: string[];
  args?: string[];
  image: string;
  runAfter?: string[];
  backoffLimit: number;
  retryCodes?: number[];
  parameters?: string[];
  podTemplate?: string;
};

export type DagFormObj = {
  name: string;
  schedule?: string;
  tasks: TaskSpec[];
  parameters?: DagParameterSpec[];
  namespace: string;
};
