import axios from "axios";
import { User } from "../types/admin";

export async function getUsers(page: number): Promise<User[]> {
  const result = await axios.get(
    `http://localhost:8080/api/v1/auth/users/${page}`,
    {
      withCredentials: true,
    }
  );

  return result.data.users;
}

export async function createAccount(
  username: string,
  password: string
): Promise<void> {
  await axios.post(
    `http://localhost:8080/api/v1/auth/create`,
    {
      username,
      password,
    },
    {
      withCredentials: true,
    }
  );
}
