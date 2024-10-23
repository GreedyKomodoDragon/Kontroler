import axios from "axios";
import { getApiUrl } from "./utils";

export async function getLogs({
    queryKey,
  }: {
    queryKey: string[];
  }): Promise<string> {
    if (queryKey[1] === "") {
      return "";
    }
  
    const result = await axios.get(
      `${getApiUrl()}/api/v1/logs/pod/${queryKey[1]}`,
      {
        withCredentials: true,
      }
    );
  
    return result.data;
  }