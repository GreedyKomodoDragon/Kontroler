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

// Layout component to wrap content with Header and Sidebar
const Layout: Component<{ children: JSX.Element }> = (props) => {
  return (
    <div class="flex flex-col h-screen bg-gray-950 text-gray-50 overflow-hidden">
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

// Main App component
const App: Component = () => {
  return (
    <AuthProvider>
      <Router>
        {/* Route for login without Layout */}
        <Route path="/login" component={Login} />
        <Route
          path="/logout"
          component={(props) => (
            <ProtectedRoute>
              <Logout />
            </ProtectedRoute>
          )}
        />
        <Route
          path="/"
          component={(props) => (
            <ProtectedRoute>
              <Layout>
                <Main />
              </Layout>
            </ProtectedRoute>
          )}
        />
        <Route
          path="/create"
          component={(props) => (
            <ProtectedRoute>
              <Layout>
                <Create />
              </Layout>
            </ProtectedRoute>
          )}
        />
        <Route
          path="/dags"
          component={(props) => (
            <ProtectedRoute>
              <Layout>
                <Dags />
              </Layout>
            </ProtectedRoute>
          )}
        />
        <Route
          path="/dags/runs"
          component={(props) => (
            <ProtectedRoute>
              <Layout>
                <DagRuns />
              </Layout>
            </ProtectedRoute>
          )}
        />
        <Route
          path="/dags/run/:id"
          component={(props) => (
            <ProtectedRoute>
              <Layout>
                <DagRun />
              </Layout>
            </ProtectedRoute>
          )}
        />
      </Router>
    </AuthProvider>
  );
};

export default App;
