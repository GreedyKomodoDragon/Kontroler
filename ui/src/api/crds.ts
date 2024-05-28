import axios from "axios";
import type { Schedule } from "../types/schedules";

export async function getScheduleCrds(): Promise<Schedule[]> {
  const result = await axios.get(
    "http://localhost:8080/api/v1/crd/cronjob",
    {},
  );

  return result.data.crds;
}
