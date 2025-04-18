import { Component, JSX } from "solid-js";
import { Router, Route } from "@solidjs/router";
import Header from "./components/header";
import Sidebar from "./components/sidebar";
import Main from "./pages/main";
import Dags from "./pages/dags";
import DagRuns from "./pages/dagRuns";
import DagRun from "./pages/dagRun";
import Create from "./pages/create";
import Login from "./pages/login";
import { AuthProvider } from "./providers/authProvider";
import { ProtectedRoute } from "./components/protectedRoute";
import Logout from "./pages/logout";
import Admin from "./pages/admin";
import CreateAccountPage from "./pages/createAccount";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import CreateDagRun from "./pages/createDagRun";
import UserProfile from "./pages/userProfile";
import Logs from "./pages/logs";
import Tasks from "./pages/tasks";
import { WebSocketProvider } from "./providers/webhookProvider";
import { ErrorProvider } from "./providers/ErrorProvider";

// Layout component to wrap content with Header and Sidebar
const Layout: Component<{ children: JSX.Element }> = (props) => {
  return (
    <div class="flex flex-col h-screen text-gray-50 overflow-hidden">
      <Header />
      <div class="flex flex-1">
        <Sidebar />
        <div
          class="flex-1 p-6 overflow-y-auto"
          style="max-height: calc(100vh - 64px);"
        >
          {props.children}
        </div>
      </div>
    </div>
  );
};

const queryClient = new QueryClient();

// Main App component
function App() {
  return (
    <ErrorProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <WebSocketProvider>
            <Router>
              {/* Route for login without Layout */}
              <Route path="/login" component={Login} />
              <Route
                path="/logout"
                component={() => (
                  <ProtectedRoute>
                    <Logout />
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <Main />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/create"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <Create />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/tasks"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <Tasks />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/dags"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <Dags />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/dags/runs"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <DagRuns />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/dags/runs/create"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <CreateDagRun />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/dags/run/:id"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <DagRun />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/admin"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <Admin />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/admin/account/create"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <CreateAccountPage />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/account/profile"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <UserProfile />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
              <Route
                path="/logs/run/:run/pod/:pod"
                component={() => (
                  <ProtectedRoute>
                    <Layout>
                      <Logs />
                    </Layout>
                  </ProtectedRoute>
                )}
              />
            </Router>
          </WebSocketProvider>
        </AuthProvider>
      </QueryClientProvider>
    </ErrorProvider>
  );
}

export default App;
