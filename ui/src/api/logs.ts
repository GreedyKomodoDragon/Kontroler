import { getApiUrl } from "./utils";
import { request } from "./http";

export async function getLogs({
  queryKey,
}: {
  queryKey: string[];
}): Promise<string> {
  if (queryKey[1] === "") {
    return "";
  }

  const data = await request(`${getApiUrl()}/api/v1/logs/run/${queryKey[1]}/pod/${queryKey[2]}`);

  // request() will return parsed JSON or text. Logs endpoint returns a string body.
  if (typeof data === "string") return data;
  return data;
}
