import axios from "axios";
import { Dag } from "../types/dag";

export async function getDags(): Promise<Dag[]> {
    const result = await axios.get(
      "http://localhost:8080/api/v1/dag/meta/1",
      {},
    );
  
    return result.data.dags;
  }
  