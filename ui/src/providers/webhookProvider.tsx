import {
  createContext,
  useContext,
  createSignal,
  onCleanup,
  Accessor,
} from "solid-js";

interface WebSocketContextType {
  logs: Accessor<string[]>;
  startLogs: (pod: string) => void;
  stopLogs: () => void;
  isStreaming: Accessor<boolean>;
}

const WebSocketContext = createContext<WebSocketContextType>();

export function WebSocketProvider(props: { children: any }) {
  const [ws, setWs] = createSignal<WebSocket | null>(null);
  const [logs, setLogs] = createSignal<string[]>([]);
  const [isStreaming, setIsStreaming] = createSignal<boolean>(false);

  // Establish WebSocket connection
  const connectWebSocket = (podUUID: string) => {
    if (ws()) return; // Avoid duplicate connections

    const socket = new WebSocket(`ws://localhost:8082/ws/logs?pod=${podUUID}`);

    socket.onopen = () => console.log("WebSocket Connected");
    socket.onmessage = (event) => {
      if (isStreaming()) {
        console.log("WebSocket Message:", event.data);
        setLogs((prev) => [...prev, event.data]); // Append new logs
      }
    };
    socket.onerror = (err) => console.error("WebSocket Error:", err);
    socket.onclose = () => {
      console.log("WebSocket Disconnected");
      setIsStreaming(false);
      setWs(null); // Reset WebSocket on disconnect
    };

    setWs(socket);
  };

  // Start log streaming
  const startLogs = (podUID: string) => {
    if (!ws()) connectWebSocket(podUID); // Ensure connection is active
    setIsStreaming(true);
  };

  // Stop log streaming
  const stopLogs = () => {
    if (ws()) {
      setIsStreaming(false);
      ws()?.close();
      setLogs([]); // Clear logs
    }
  };

  // Close WebSocket when provider unmounts
  onCleanup(() => {
    ws()?.close();
  });

  return (
    <WebSocketContext.Provider
      value={{ logs, startLogs, stopLogs, isStreaming }}
    >
      {props.children}
    </WebSocketContext.Provider>
  );
}

export function useWebSocket() {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error("useWebSocket must be used within a WebSocketProvider");
  }
  return context;
}
