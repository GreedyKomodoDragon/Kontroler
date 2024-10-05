import axios from "axios";
import { User } from "../types/admin";
import { getApiUrl } from "./utils";

export async function getUsers({
  queryKey,
}: {
  queryKey: string[];
}): Promise<User[]> {
  const page = Number(queryKey[1]);

  const result = await axios.get(
    `${getApiUrl()}/api/v1/auth/users/page/${page - 1}`,
    {
      withCredentials: true,
    }
  );

  return result.data.users;
}

export async function getUserPageCount(): Promise<number> {
  const result = await axios.get(
    `${getApiUrl()}/api/v1/auth/users/pages/count`,
    {
      withCredentials: true,
    }
  );

  return result.data.count;
}

export async function createAccount(
  username: string,
  password: string
): Promise<void> {
  await axios.post(
    `${getApiUrl()}/api/v1/auth/create`,
    {
      username,
      password,
    },
    {
      withCredentials: true,
    }
  );
}

export async function deleteAccount(username: string): Promise<void> {
  await axios.delete(`${getApiUrl()}/api/v1/auth/users/${username}`, {
    withCredentials: true,
  });
}

export async function updatePassword(
  oldPassword: string,
  password: string
): Promise<void> {
  await axios.post(
    `${getApiUrl()}/api/v1/auth/password/change`,
    {
      oldPassword,
      password,
    },
    {
      withCredentials: true,
    }
  );
}
