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

class AccountError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AccountError';
  }
}

export async function createAccount(
  username: string,
  password: string,
  role: string,
): Promise<void> {
  try {
    await axios.post(
      `${getApiUrl()}/api/v1/auth/create`,
      {
        username,
        password,
        role,
      },
      {
        withCredentials: true,
      }
    );
  } catch (error) {
    if (axios.isAxiosError(error)) {
      switch (error.response?.status) {
        case 400:
          throw new AccountError('Invalid username or password format.');
        case 401:
          throw new AccountError('Authentication required. Please log in.');
        case 403:
          throw new AccountError('You do not have permission to create accounts, must be an Admin');
        case 409:
          throw new AccountError(`Username '${username}' already exists.`);
        case 500:
          throw new AccountError('Server error occurred while creating account.');
        default:
          throw new AccountError(error.message || 'Failed to create account.');
      }
    }
    throw new AccountError('Network error occurred while creating account.');
  }
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
