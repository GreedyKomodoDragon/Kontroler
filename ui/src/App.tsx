import { Component, JSX, lazy } from "solid-js";
import { Router, Route } from "@solidjs/router";
import Header from "./components/header";
import Sidebar from "./components/sidebar";
import { AuthProvider } from "./providers/authProvider";
import { ProtectedRoute } from "./components/protectedRoute";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { WebSocketProvider } from "./providers/webhookProvider";
import { ErrorProvider } from "./providers/ErrorProvider";

const Main = lazy(() => import("./pages/main"));
const Dags = lazy(() => import("./pages/dags"));
const DagRuns = lazy(() => import("./pages/dagRuns"));
const DagRun = lazy(() => import("./pages/dagRun"));
const Create = lazy(() => import("./pages/create"));
const Login = lazy(() => import("./pages/login"));
const Logout = lazy(() => import("./pages/logout"));
const Admin = lazy(() => import("./pages/admin"));
const CreateAccountPage = lazy(() => import("./pages/createAccount"));
const CreateDagRun = lazy(() => import("./pages/createDagRun"));
const UserProfile = lazy(() => import("./pages/userProfile"));
const Logs = lazy(() => import("./pages/logs"));
const Tasks = lazy(() => import("./pages/tasks"));

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
