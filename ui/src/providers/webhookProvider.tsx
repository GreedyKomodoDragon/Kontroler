import {
  createContext,
  useContext,
  createSignal,
  onCleanup,
  Accessor,
} from "solid-js";
import { getWebsocketUrl } from "../api/utils";

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
  const [currentPodUID, setCurrentPodUID] = createSignal<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = createSignal<number>(0);

  const MAX_RECONNECT_ATTEMPTS = 3;

  // Establish WebSocket connection
  const connectWebSocket = (podUUID: string) => {
    // Close existing connection if switching pods
    if (ws() && currentPodUID() !== podUUID) {
      ws()?.close();
      setWs(null);
      setLogs([]); // Clear logs when switching pods
    }

    if (ws()) return; // Avoid duplicate connections for same pod

    const socket = new WebSocket(`${getWebsocketUrl()}/ws/logs?pod=${podUUID}`);
    setCurrentPodUID(podUUID);

    socket.onopen = () => {
      console.log(`WebSocket Connected for pod: ${podUUID}`);
      setReconnectAttempts(0); // Reset reconnect attempts on successful connection
    };

    socket.onmessage = (event) => {
      if (isStreaming() && currentPodUID() === podUUID) {
        const timestamp = new Date().toISOString();
        const logEntry = `[${timestamp}] ${event.data}`;
        setLogs((prev) => [...prev, logEntry]);
      }
    };

    socket.onclose = () => {
      console.log(`WebSocket Disconnected for pod ${podUUID}`);
      setIsStreaming(false);
      setWs(null);

      // Attempt reconnection if streaming was active
      if (isStreaming() && reconnectAttempts() < MAX_RECONNECT_ATTEMPTS) {
        setReconnectAttempts((prev) => prev + 1);
        setTimeout(() => connectWebSocket(podUUID), 1000 * reconnectAttempts());
      }
    };

    setWs(socket);
  };

  // Start log streaming
  const startLogs = (podUID: string) => {
    if (!podUID) {
      console.error("Invalid pod UID provided");
      return;
    }

    if (currentPodUID() !== podUID) {
      setLogs([]); // Clear logs when switching pods
    }

    connectWebSocket(podUID);
    setIsStreaming(true);
  };

  // Stop log streaming
  const stopLogs = () => {
    setIsStreaming(false);
    ws()?.close();
    setLogs([]);
    setCurrentPodUID(null);
    setReconnectAttempts(0);
  };

  // Close WebSocket when provider unmounts
  onCleanup(() => {
    ws()?.close();
    setCurrentPodUID(null);
    setReconnectAttempts(0);
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
