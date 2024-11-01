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
    `${getApiUrl()}/api/v1/logs/run/${queryKey[1]}/pod/${queryKey[2]}`,
    {
      withCredentials: true,
    }
  );

  return result.data;
}
