import type { Component } from "solid-js";
import Chart from "../components/chart";
import { createSignal } from "solid-js";
import { ApexOptions } from "apexcharts";

const Main: Component = () => {
  const [runningJobs] = createSignal(100); // Example data: 5 running jobs
  const [recentEvents, setRecentEvents] = createSignal<number[]>([5]);

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
        data: [10, 15, 13, 20, 15, 25, 35, 45, 55],
      },
      {
        name: "Failed",
        data: [2, 3, 5, 25, 3, 4, 35, 12, 0],
      },
    ],
    colors: ["#00FF00", "#FF0000"],
    xaxis: {
      categories: [
        "Q1",
        "Q2",
        "Q3",
        "Q4",
        "Q1",
        "Q2",
        "Q3",
        "Q4",
        "Q1",
        "Q2",
        "Q3",
        "Q4",
      ],
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

  const [donutChartOptions] = createSignal({
    chart: {
      type: "donut",
    },
    series: [44, 15],
    labels: ["Successful", "Failed"],
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

  return (
    <div>
      <h2 class="text-center mb-5 text-2xl">Dashboard Overview</h2>
      <div>
        <Chart options={chartOptions2() as ApexOptions} />
      </div>
      <h2 class="text-center mb-10 mt-20 text-2xl">Dashboard Overview</h2>
      <div class="grid gap-10 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
        <div>
          <h2 class="text-center mb-2 text-xl">Recent Jobs Outcomes</h2>
          <ul class="flex items-center justify-center">
            {recentEvents().map((time) => (
              <li class="text-green-500">
                Successes - Event - {new Date(time).toLocaleTimeString()}
              </li>
            ))}
          </ul>
        </div>
        <div>
          <h2 class="text-center text-xl">Running Jobs Count</h2>
          <div class="flex items-center justify-center h-full text-9xl">
            {runningJobs()}
          </div>
        </div>
        <div>
          <h2 class="text-center text-xl">Job Outcome (Last 30 days)</h2>
          <Chart options={donutChartOptions() as ApexOptions} />
        </div>
      </div>
    </div>
  );
};

export default Main;
