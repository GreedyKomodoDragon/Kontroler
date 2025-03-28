// src/contexts/AuthContext.tsx
import {
  createContext,
  createSignal,
  useContext,
  onMount,
  JSX,
} from "solid-js";
import { getApiUrl } from "../api/utils";

interface AuthContextType {
  isAuthenticated: () => boolean;
  isLoading: () => boolean;
  login: (username: string, password: string) => Promise<boolean>;
  logout: () => Promise<boolean>;
  username: () => string;
  role: () => string;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider(props: { children: JSX.Element }) {
  const [isAuthenticated, setIsAuthenticated] = createSignal<boolean>(false);
  const [isLoading, setIsLoading] = createSignal<boolean>(true);
  const [username, setUsername] = createSignal<string>("");
  const [role, setRole] = createSignal<string>("");

  const checkAuthentication = async () => {
    try {
      const response = await fetch(`${getApiUrl()}/api/v1/auth/check`, {
        method: "GET",
        credentials: "include",
      });

      if (response.ok) {
        setIsAuthenticated(true);
        response.json().then((data) => {
          setUsername(data.username);
          setRole(data.role);
        });

      } else {
        setIsAuthenticated(false);
      }
    } catch (error) {
      console.error("Authentication check failed:", error);
      setIsAuthenticated(false);
    } finally {
      setIsLoading(false);
    }
  };

  onMount(() => {
    checkAuthentication(); // Check auth on initial load
  });

  const login = async (
    username: string,
    password: string
  ): Promise<boolean> => {
    setIsLoading(true);
    try {
      const response = await fetch(`${getApiUrl()}/api/v1/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ username, password }),
        credentials: "include",
      });

      if (response.ok) {
        setUsername(username);
        setIsAuthenticated(true);
        setIsLoading(false);
        
        const body = await response.json();
        if (body.role) {
          setRole(body.role);
        }

        return true;
      }

      throw new Error("Login failed");
    } catch (error) {
      setIsAuthenticated(false);
      setIsLoading(false);
      return false;
    }
  };

  const logout = async (): Promise<boolean> => {
    setIsLoading(true);
    let worked = true;
    try {
      await fetch(`${getApiUrl()}/api/v1/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
      setIsAuthenticated(false);
    } catch (error) {
      console.error("Logout error:", error);
      worked = false;
    } finally {
      setIsLoading(false);
    }

    return worked;
  };

  const value: AuthContextType = {
    isAuthenticated,
    isLoading,
    login,
    logout,
    username,
    role,
  };

  return (
    <AuthContext.Provider value={value}>{props.children}</AuthContext.Provider>
  );
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
