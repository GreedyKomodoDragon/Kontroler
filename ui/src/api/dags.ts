import axios from "axios";
import {
  Dag,
  DagRunAll,
  DagRunGraph,
  DagRunMeta,
  DagTaskDetails,
  DashboardStats,
  Parameter,
  TaskDetails,
  TaskRunDetails,
} from "../types/dag";
import { DagFormObj } from "../types/dagForm";
import { getApiUrl } from "./utils";

export async function getDags({
  queryKey,
}: {
  queryKey: string[];
}): Promise<Dag[]> {
  const result = await axios.get(
    `${getApiUrl()}/api/v1/dag/meta/${queryKey[1]}`,
    {
      withCredentials: true,
    }
  );

  return result.data.dags;
}

export async function getDagRuns({
  queryKey,
}: {
  queryKey: string[];
}): Promise<DagRunMeta[]> {
  const result = await axios.get(
    `${getApiUrl()}/api/v1/dag/runs/${queryKey[1]}`,
    {
      withCredentials: true,
    }
  );

  return result.data;
}

export async function getDagRunGraph(runId: number): Promise<DagRunGraph> {
  const result = await axios.get(`${getApiUrl()}/api/v1/dag/run/${runId}`, {
    withCredentials: true,
  });

  return result.data;
}

export async function getDagRunAll(runId: number): Promise<DagRunAll> {
  const result = await axios.get(`${getApiUrl()}/api/v1/dag/run/all/${runId}`, {
    withCredentials: true,
  });
  return result.data;
}

export async function getTaskRunDetails(
  runId: number,
  taskId: number
): Promise<TaskRunDetails | undefined> {
  if (runId == -1) {
    return undefined;
  }

  const result = await axios.get(
    `${getApiUrl()}/api/v1/dag/run/task/${runId}/${taskId}`,
    {
      withCredentials: true,
    }
  );
  return result.data;
}

export async function getTaskDetails(
  taskId: number
): Promise<TaskDetails | undefined> {
  if (taskId == -1) {
    return undefined;
  }

  const result = await axios.get(`${getApiUrl()}/api/v1/dag/task/${taskId}`, {
    withCredentials: true,
  });
  return result.data;
}

export async function createDag(dagForm: DagFormObj): Promise<any> {
  const result = await axios.post(`${getApiUrl()}/api/v1/dag/create`, dagForm, {
    withCredentials: true,
  });
  return result.data;
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const result = await axios.get(`${getApiUrl()}/api/v1/stats/dashboard`, {
    withCredentials: true,
  });
  return result.data;
}

export async function getDagRunPageCount(): Promise<number> {
  const result = await axios.get(`${getApiUrl()}/api/v1/dag/run/pages/count`, {
    withCredentials: true,
  });

  return result.data.count;
}

export async function getDagPageCount(): Promise<number> {
  const result = await axios.get(`${getApiUrl()}/api/v1/dag/pages/count`, {
    withCredentials: true,
  });

  return result.data.count;
}

export async function getDagNames({
  queryKey,
}: {
  queryKey: string[];
}): Promise<string[]> {
  if (queryKey[1] === "") {
    return [];
  }

  const result = await axios.get(
    `${getApiUrl()}/api/v1/dag/names?term=${queryKey[1]}`,
    {
      withCredentials: true,
    }
  );

  return result.data.names;
}

export async function getDagParameters({
  queryKey,
}: {
  queryKey: string[];
}): Promise<Parameter[]> {
  if (queryKey[1] === "") {
    return [];
  }

  const result = await axios.get(
    `${getApiUrl()}/api/v1/dag/parameters?name=${queryKey[1]}`,
    {
      withCredentials: true,
    }
  );

  return result.data.parameters;
}

class DagRunError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'DagRunError';
  }
}

export async function createDagRun(
  name: string,
  parameters: { [x: string]: string },
  namespace: string,
  runName: string
): Promise<void> {
  try {
    await axios.post(
      `${getApiUrl()}/api/v1/dag/run/create`,
      {
        name,
        parameters,
        namespace,
        runName,
      },
      {
        withCredentials: true,
      }
    );
  } catch (error) {
    if (axios.isAxiosError(error)) {
      switch (error.response?.status) {
        case 401:
          throw new DagRunError('Authentication required. Please log in.');
        case 403:
          throw new DagRunError("You do not have permission to create DAG runs, must be at least an 'Editor'");
        case 404:
          throw new DagRunError(`DAG '${name}' not found in namespace '${namespace}'.`);
        case 500:
          throw new DagRunError('Server error occurred while creating DAG run.');
        default:
          throw new DagRunError(error.message || 'Failed to create DAG run.');
      }
    }
    throw new DagRunError('Network error occurred while creating DAG run.');
  }
}

export async function getDagTasks({
  queryKey,
}: {
  queryKey: string[];
}): Promise<DagTaskDetails[]> {
  try {
    const result = await axios.get(
      `${getApiUrl()}/api/v1/dag/dagTask/pages/page/${queryKey[1]}`,
      {
        withCredentials: true,
      }
    );
    return result.data;
  } catch (error) {
    console.error('Failed to fetch DAG tasks:', error);
    throw error;
  }
}

export async function getDagTaskPageCount(): Promise<number> {
  try {
    const result = await axios.get(
      `${getApiUrl()}/api/v1/dag/dagTask/pages/count`,
      {
        withCredentials: true,
      }
    );
    if (typeof result.data.count !== "number") {
      throw new Error("Invalid response format: count is not a number");
    }
    return result.data.count;
  } catch (error) {
    console.error("Failed to fetch DAG task page count:", error);
    throw error;
  }
}

export async function deleteDag(namespace: string, name: string): Promise<void> {
  try {
    await axios.delete(`${getApiUrl()}/api/v1/dag/dag/${namespace}/${name}`, {
      withCredentials: true,
    });
  } catch (error) {
    if (axios.isAxiosError(error)) {
      switch (error.response?.status) {
        case 401:
          throw new Error('Authentication required. Please log in.');
        case 403:
          throw new Error("You don't have permission to delete this DAG");
        case 404:
          throw new Error(`DAG '${name}' not found in namespace '${namespace}'`);
        case 409:
          throw new Error('The DAG has been modified, please try again');
        default:
          throw new Error('Failed to delete DAG');
      }
    }
    throw new Error('Network error occurred while deleting DAG');
  }
}
