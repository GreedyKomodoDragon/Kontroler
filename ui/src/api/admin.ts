import { request } from "./http";
import { User } from "../types/admin";
import { getApiUrl } from "./utils";

export async function getUsers({
  queryKey,
}: {
  queryKey: string[];
}): Promise<User[]> {
  const page = Number(queryKey[1]);

  const data = await request(`${getApiUrl()}/api/v1/auth/users/page/${page - 1}`);

  return data.users;
}

export async function getUserPageCount(): Promise<number> {
  const data = await request(`${getApiUrl()}/api/v1/auth/users/pages/count`);

  return data.count;
}

export async function createAccount(
  username: string,
  password: string,
  role: string
): Promise<void> {
  await request(`${getApiUrl()}/api/v1/auth/create`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password, role }),
  });
}

export async function deleteAccount(username: string): Promise<void> {
  await request(`${getApiUrl()}/api/v1/auth/users/${username}`, {
    method: "DELETE",
  });
}

export async function updatePassword(
  oldPassword: string,
  password: string
): Promise<void> {
  await request(`${getApiUrl()}/api/v1/auth/password/change`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ oldPassword, password }),
  });
}
