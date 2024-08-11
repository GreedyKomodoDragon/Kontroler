import { Component } from "solid-js";
import Header from "./components/header";
import Sidebar from "./components/sidebar";
import { Router, Route } from "@solidjs/router";
import Main from "./pages/main";
import Dags from "./pages/dags";
import DagRuns from "./pages/dagRuns";
import DagRun from "./pages/dagRun";
import Create from "./pages/create";

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
            <Route path="/create" component={Create} />
            <Route path="/dags" component={Dags} />
            <Route path="/dags/runs" component={DagRuns} />
            <Route path="/dags/run/:id" component={DagRun} />
          </Router>
        </div>
      </div>
    </div>
  );
};

export default App;
