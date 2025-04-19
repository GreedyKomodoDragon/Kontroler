import { createContext, useContext, JSX } from "solid-js";
import { createSignal } from "solid-js";
import ErrorAlert from "../components/errorAlert";

const ErrorContext = createContext<{
  globalError: () => string | null;
  setGlobalErrorMessage: (message: string) => void;
  clearGlobalError: () => void;
  handleApiError: (error: any) => void;
} | null>(null);

export function ErrorProvider(props: { children: JSX.Element }) {
  const [globalError, setGlobalError] = createSignal<string | null>(null);
  let timeoutId: ReturnType<typeof setTimeout> | null = null;

  const setGlobalErrorMessage = (message: string) => {
    setGlobalError(message);

    if (timeoutId !== null) {
      clearTimeout(timeoutId);
    }

    timeoutId = setTimeout(() => {
      setGlobalError(null);
      timeoutId = null;
    }, 5000);
  };

  const clearGlobalError = () => {
    if (timeoutId !== null) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    setGlobalError(null);
  };

  const handleApiError = (error: any) => {
    if (error.response) {
      setGlobalErrorMessage(error.response.data?.message || "An error occurred.");
    } else if (error.request) {
      setGlobalErrorMessage("Network error. Please check your connection.");
    } else {
      console.error("Error:", error.message);
      setGlobalErrorMessage(error.message || "An unexpected error occurred.");
    }
    throw error;
  };

  return (
    <ErrorContext.Provider value={{ globalError, setGlobalErrorMessage, clearGlobalError, handleApiError }}>
      {props.children}
      {globalError() && <ErrorAlert msgs={[globalError()!]} />}
    </ErrorContext.Provider>
  );
}

export function useError() {
  const context = useContext(ErrorContext);
  if (!context) {
    throw new Error("useError must be used within an ErrorProvider");
  }
  return context;
}
