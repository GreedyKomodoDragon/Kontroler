export const getApiUrl = () => {
  return window.__ENV__?.API_URL || "http://localhost:8082";
};
