import type { Component } from "solid-js";
import Chart from "../components/chart";
import { createSignal } from "solid-js";
import { ApexOptions } from "apexcharts";
import { getDashboardStats } from "../api/dags";
import { DashboardStats } from "../types/dag";

const Main: Component = () => {
  // Updated bar chart data for DagRun Outcomes (30 Days)
  const [chartOptions2] = createSignal({
    chart: {
      type: "bar",
      height: 400,
      toolbar: {
        show: false, // Disable the toolbar
      },
    },
    series: [
      {
        name: "Successful",
        data: [12, 18, 15, 22, 19, 28, 32, 48, 60],
      },
      {
        name: "Failed",
        data: [3, 5, 7, 8, 4, 6, 10, 20, 1],
      },
    ],
    colors: ["#00FF00", "#FF0000"],
    xaxis: {
      categories: ["Q1", "Q2", "Q3", "Q4", "Q1", "Q2", "Q3", "Q4", "Q1"],
      labels: {
        style: {
          colors: "#FFFFFF",
        },
      },
    },
    yaxis: {
      labels: {
        style: {
          colors: "#FFFFFF",
        },
      },
    },
    tooltip: {
      theme: "dark",
      style: {
        fontSize: "12px",
        background: "#333333",
      },
    },
    legend: {
      labels: {
        colors: "#FFFFFF",
      },
    },
  });

  // New donut chart data for DAG Type
  const [dagTypeDonutChartOptions] = createSignal({
    chart: {
      type: "donut",
    },
    series: [50, 70], // New data specific to DAG Type
    labels: ["Type A", "Type B"],
    colors: ["#FFA500", "#1E90FF"],
    stroke: {
      colors: ["#000"],
    },
    plotOptions: {
      pie: {
        donut: {
          labels: {
            show: true,
          },
        },
      },
    },
    legend: {
      show: false,
    },
  });

  // New donut chart data for Task Outcomes (30 Days)
  const [taskOutcomesDonutChartOptions] = createSignal({
    chart: {
      type: "donut",
    },
    series: [70, 20], // New data specific to Task Outcomes
    labels: ["Completed", "Failed"],
    colors: ["#00FF00", "#FF0000"],
    stroke: {
      colors: ["#000"],
    },
    plotOptions: {
      pie: {
        donut: {
          labels: {
            show: true,
          },
        },
      },
    },
    legend: {
      show: false,
    },
  });

  const [stats, setStats] = createSignal<DashboardStats | undefined>();

  getDashboardStats()
    .then((data) => {
      setStats(data);
    })
    .catch((err) => {
      console.log(err);
    });

  return (
    <div>
      <div class="flex flex-col w-full max-w-5xl mx-auto p-6 gap-6">
        <div class="flex flex-col gap-2">
          <h1 class="text-3xl font-bold">Kontroler Dashboard</h1>
        </div>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-2xl font-semibold leading-none tracking-tight">
                DAG Count
              </h3>
            </div>
            <div class="p-6">
              <div class="text-4xl font-bold">{stats() ? stats()?.dag_count : 0}</div>
            </div>
          </div>
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-2xl font-semibold leading-none tracking-tight">
                Successful
              </h3>
            </div>
            <div class="p-6">
              <div class="text-4xl font-bold text-green-500">{stats() ? stats()?.successful_dag_runs : 0}</div>
            </div>
          </div>
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-2xl font-semibold leading-none tracking-tight">
                Failed
              </h3>
            </div>
            <div class="p-6">
              <div class="text-4xl font-bold text-red-500">{stats() ? stats()?.failed_dag_runs : 0}</div>
            </div>
          </div>
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-2xl font-semibold leading-none tracking-tight">
                Total DagRuns
              </h3>
            </div>
            <div class="p-6">
              <div class="text-4xl font-bold">{stats() ? stats()?.total_dag_runs : 0}</div>
            </div>
          </div>
        </div>

        <div
          class="rounded-lg border bg-card text-card-foreground shadow-sm"
          data-v0-t="card"
        >
          <div class="flex flex-col space-y-1.5 p-6">
            <h3 class="whitespace-nowrap text-2xl font-semibold leading-none tracking-tight">
              DagRun Outcomes (30 Days)
            </h3>
          </div>
          <div class="p-6">
            <Chart options={chartOptions2() as ApexOptions} />
          </div>
        </div>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-xl font-semibold leading-none tracking-tight">
                Current Active DagRuns
              </h3>
            </div>
            <div class="p-6">
              {/* Apply responsive font size */}
              <div class="text-8xl font-bold text-fit">{stats() ? stats()?.active_dag_runs : 0}</div>
            </div>
          </div>
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-xl font-semibold leading-none tracking-tight">
                DAG Type
              </h3>
            </div>
            <div class="p-6">
              <div class="text-4xl font-bold text-red-500">
                <Chart options={dagTypeDonutChartOptions() as ApexOptions} />
              </div>
            </div>
          </div>
          <div
            class="rounded-lg border bg-card text-card-foreground shadow-sm"
            data-v0-t="card"
          >
            <div class="flex flex-col space-y-1.5 p-6">
              <h3 class="whitespace-nowrap text-xl font-semibold leading-none tracking-tight">
                Task Outcomes (30 Days)
              </h3>
            </div>
            <div class="p-6">
              <div class="text-4xl font-bold text-yellow-500">
                <Chart
                  options={taskOutcomesDonutChartOptions() as ApexOptions}
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Main;
