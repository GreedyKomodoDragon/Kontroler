import axios from "axios";
import {
  Dag,
  DagRunAll,
  DagRunGraph,
  DagRunMeta,
  DashboardStats,
  TaskDetails,
  TaskRunDetails,
} from "../types/dag";
import { DagFormObj } from "../types/dagForm";

export async function getDags(): Promise<Dag[]> {
  const result = await axios.get("http://localhost:8080/api/v1/dag/meta/1", {
      withCredentials: true
    });

  return result.data.dags;
}

export async function getDagRuns(page: number): Promise<DagRunMeta[]> {
  const result = await axios.get(
    `http://localhost:8080/api/v1/dag/runs/${page}`,
    {
      withCredentials: true
    }
  );

  return result.data;
}

export async function getDagRunGraph(runId: number): Promise<DagRunGraph> {
  const result = await axios.get(
    `http://localhost:8080/api/v1/dag/run/${runId}`,
    {
      withCredentials: true
    }
  );

  return result.data;
}

export async function getDagRunAll(runId: number): Promise<DagRunAll> {
  const result = await axios.get(
    `http://localhost:8080/api/v1/dag/run/all/${runId}`,
    {
      withCredentials: true
    }
  );
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
    `http://localhost:8080/api/v1/dag/run/task/${runId}/${taskId}`,
    {
      withCredentials: true
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

  const result = await axios.get(
    `http://localhost:8080/api/v1/dag/task/${taskId}`,
    {
      withCredentials: true
    }
  );
  return result.data;
}

export async function createDag(dagForm: DagFormObj): Promise<any> {
  const result = await axios.post(
    `http://localhost:8080/api/v1/dag/create`,
    dagForm,
    {
      withCredentials: true
    }
  );
  return result.data;
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const result = await axios.get(
    'http://localhost:8080/api/v1/stats/dashboard ',
    {
      withCredentials: true
    }
  );
  return result.data;
}


