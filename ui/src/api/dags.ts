import { request } from "./http";
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
  const data = await request(`${getApiUrl()}/api/v1/dag/meta/${queryKey[1]}`);

  return data.dags;
}

export async function getDagRuns({
  queryKey,
}: {
  queryKey: string[];
}): Promise<DagRunMeta[]> {
  const data = await request(`${getApiUrl()}/api/v1/dag/runs/${queryKey[1]}`);

  return data;
}

export async function getDagRunGraph(runId: number): Promise<DagRunGraph> {
  const data = await request(`${getApiUrl()}/api/v1/dag/run/${runId}`);

  return data;
}

export async function getDagRunAll(runId: number): Promise<DagRunAll> {
  const data = await request(`${getApiUrl()}/api/v1/dag/run/all/${runId}`);
  return data;
}

export async function getTaskRunDetails(
  runId: number,
  taskID: number
): Promise<TaskRunDetails | undefined> {
  if (runId == -1) {
    return undefined;
  }

  const data = await request(`${getApiUrl()}/api/v1/dag/run/task/${runId}/${taskID}`);
  return data;
}

export async function getTaskDetails(
  taskID: number
): Promise<TaskDetails | undefined> {
  if (taskID == -1) {
    return undefined;
  }

  const data = await request(`${getApiUrl()}/api/v1/dag/task/${taskID}`);
  return data;
}

export async function createDag(dagForm: DagFormObj): Promise<any> {
  const data = await request(`${getApiUrl()}/api/v1/dag/create`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(dagForm),
  });
  return data;
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const data = await request(`${getApiUrl()}/api/v1/stats/dashboard`);
  return data;
}

export async function getDagRunPageCount(): Promise<number> {
  const data = await request(`${getApiUrl()}/api/v1/dag/run/pages/count`);

  return data.count;
}

export async function getDagPageCount(): Promise<number> {
  const data = await request(`${getApiUrl()}/api/v1/dag/pages/count`);

  return data.count;
}

export async function getDagNames({
  queryKey,
}: {
  queryKey: string[];
}): Promise<string[]> {
  if (queryKey[1] === "") {
    return [];
  }

  const data = await request(`${getApiUrl()}/api/v1/dag/names?term=${queryKey[1]}`);

  return data.names;
}

export async function getDagParameters({
  queryKey,
}: {
  queryKey: string[];
}): Promise<Parameter[]> {
  if (queryKey[1] === "") {
    return [];
  }

  const data = await request(`${getApiUrl()}/api/v1/dag/parameters?name=${queryKey[1]}`);

  return data.parameters;
}

class DagRunError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DagRunError";
  }
}

export async function createDagRun(
  name: string,
  parameters: { [x: string]: string },
  namespace: string,
  runName: string
): Promise<void> {
  try {
    await request(`${getApiUrl()}/api/v1/dag/run/create`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, parameters, namespace, runName }),
    });
  } catch (error) {
    const status = (error as any)?.status;
    if (typeof status === "number") {
      switch (status) {
        case 401:
          throw new DagRunError("Authentication required. Please log in.");
        case 403:
          throw new DagRunError(
            "You do not have permission to create DAG runs, must be at least an 'Editor'"
          );
        case 404:
          throw new DagRunError(
            `DAG '${name}' not found in namespace '${namespace}'.`
          );
        case 500:
          throw new DagRunError(
            "Server error occurred while creating DAG run."
          );
        default:
          throw new DagRunError((error as Error).message || "Failed to create DAG run.");
      }
    }
    throw new DagRunError("Network error occurred while creating DAG run.");
  }
}

export async function getDagTasks({
  queryKey,
}: {
  queryKey: string[];
}): Promise<DagTaskDetails[]> {
  try {
    const data = await request(`${getApiUrl()}/api/v1/dag/dagTask/pages/page/${queryKey[1]}`);
    return data;
  } catch (error) {
    console.error("Failed to fetch DAG tasks:", error);
    throw error;
  }
}

export async function getDagTaskPageCount(): Promise<number> {
  try {
    const data = await request(`${getApiUrl()}/api/v1/dag/dagTask/pages/count`);
    if (typeof data.count !== "number") {
      throw new Error("Invalid response format: count is not a number");
    }
    return data.count;
  } catch (error) {
    console.error("Failed to fetch DAG task page count:", error);
    throw error;
  }
}

export async function deleteDag(
  namespace: string,
  name: string
): Promise<void> {
  try {
    await request(`${getApiUrl()}/api/v1/dag/dag/${namespace}/${name}`, {
      method: "DELETE",
    });
  } catch (error) {
    const status = (error as any)?.status;
    if (typeof status === "number") {
      switch (status) {
        case 401:
          throw new Error("Authentication required. Please log in.");
        case 403:
          throw new Error("You don't have permission to delete this DAG");
        case 404:
          throw new Error(
            `DAG '${name}' not found in namespace '${namespace}'`
          );
        case 409:
          throw new Error("The DAG has been modified, please try again");
        default:
          throw new Error("Failed to delete DAG");
      }
    }
    throw new Error("Network error occurred while deleting DAG");
  }
}

export async function deleteDagRun(
  namespace: string,
  run: string
): Promise<void> {
  try {
    const params = new URLSearchParams({
      namespace: namespace,
      run: run,
    });
    await request(`${getApiUrl()}/api/v1/dag/run/remove?${params.toString()}`, {
      method: "DELETE",
    });
  } catch (error) {
    const status = (error as any)?.status;
    if (typeof status === "number") {
      switch (status) {
        case 401:
          throw new Error("Authentication required. Please log in.");
        case 403:
          throw new Error("You don't have permission to delete this DagRun");
        case 404:
          throw new Error(
            `DagRun '${run}' not found in namespace '${namespace}'`
          );
        default:
          throw new Error("Failed to delete DagRun");
      }
    }
    throw new Error("Network error occurred while deleting DagRun");
  }
}

export async function suspendDag(
  namespace: string,
  name: string,
  suspend: boolean
): Promise<void> {
  try {
    await request(`${getApiUrl()}/api/v1/dag/suspend`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ namespace: namespace, name: name, suspend: suspend }),
    });
  } catch (error) {
    const status = (error as any)?.status;
    if (typeof status === "number") {
      switch (status) {
        case 401:
          throw new Error("Authentication required. Please log in.");
        case 403:
          throw new Error("You don't have permission to suspend this DAG");
        case 404:
          throw new Error(`DAG '${name}' not found`);
        default:
          throw new Error("Failed to suspend DAG");
      }
    }
    throw new Error("Network error occurred while suspending DAG");
  }
}
