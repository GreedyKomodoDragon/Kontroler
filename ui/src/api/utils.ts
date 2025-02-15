export const getApiUrl = () => {
  return window.__ENV__?.API_URL || "http://localhost:8082";
};

export const getWebsocketUrl = () => {
  return window.__ENV__?.WS_URL || "ws://localhost:8082";
};
