import axios from "axios";
import { Dag, DagRun, DagRunMeta } from "../types/dag";

export async function getDags(): Promise<Dag[]> {
  const result = await axios.get("http://localhost:8080/api/v1/dag/meta/1", {});

  return result.data.dags;
}

export async function getDagRuns(page: number): Promise<DagRunMeta[]> {
  const result = await axios.get(`http://localhost:8080/api/v1/dag/runs/${page}`, {});

  return result.data;
}


export async function getDagRun(runId: number): Promise<DagRun> {
  const result = await axios.get(`http://localhost:8080/api/v1/dag/run/${runId}`, {});

  return result.data;
}
