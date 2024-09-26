// Declare the structure of the window.__ENV__ object
interface EnvConfig {
  API_URL: string;
}

// Extend the Window interface to include __ENV__
interface Window {
  __ENV__?: EnvConfig;
}
