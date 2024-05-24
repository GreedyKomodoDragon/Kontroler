import { Component } from "solid-js";
import Header from "./components/header";
import Sidebar from "./components/sidebar";
import { Router, Route } from "@solidjs/router";
import Main from "./pages/main";
import CRDs from "./pages/crds";
import CronJobs from "./pages/cronjobs";
import Runs from "./pages/runs";

const App: Component = () => {
  return (
    <div class="flex flex-col h-screen bg-gray-950 text-gray-50 overflow-hidden">
      <Header />
      <div class="flex flex-1">
        <Sidebar />
        <div
          class="flex-1 p-6 overflow-y-auto"
          style="max-height: calc(100vh - 64px);"
        >
          <Router>
            <Route path="/" component={Main} />
            <Route path="/crds" component={CRDs} />
            <Route path="/cronjobs" component={CronJobs} />
            <Route path="/runs" component={Runs} />
          </Router>
        </div>
      </div>
    </div>
  );
};

export default App;
