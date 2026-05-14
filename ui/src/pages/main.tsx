import type { Component } from "solid-js";
import { createSignal, createEffect } from "solid-js";
import Chart from "../components/chart";
import { ApexOptions } from "apexcharts";
import { getDashboardStats } from "../api/dags";
import { DashboardStats } from "../types/dag";
import { createQuery } from "@tanstack/solid-query";
import Loadable from "../components/loadable";
import SkeletonCard from "../components/skeletonCard";

const Main: Component = () => {
  const [timeLine, setTimeLine] = createSignal<ApexOptions>({
    chart: {
      type: "bar",
      height: 400,
      toolbar: { show: false },
    },
    series: [{ name: "Successful", data: [] as number[] }, { name: "Failed", data: [] as number[] }],
    colors: ["#00FF00", "#FF0000"],
    xaxis: { categories: [] as string[], labels: { style: { colors: "#FFFFFF" } } },
    yaxis: { labels: { style: { colors: "#FFFFFF" } } },
    tooltip: { theme: "dark", style: { fontSize: "12px", background: "#333333" } },
    legend: { labels: { colors: "#FFFFFF" } },
  });

  const [dagTypeDonutChartOptions, setDagTypeDonutChartOptions] = createSignal<ApexOptions>({
    chart: { type: "donut" },
    series: [0, 0],
    labels: ["Event Driven Only", "Scheduled"],
    colors: ["#FFA500", "#1E90FF"],
    stroke: { colors: ["#000"] },
    plotOptions: { pie: { donut: { labels: { show: true } } } },
    legend: { show: false },
  });

  const [taskOutcomesDonutChartOptions, setTaskOutcomesDonutChartOptions] = createSignal<ApexOptions>({
    chart: { type: "donut" },
    series: [0, 0],
    labels: ["Completed", "Failed"],
    colors: ["#00FF00", "#FF0000"],
    stroke: { colors: ["#000"] },
    plotOptions: { pie: { donut: { labels: { show: true } } } },
    legend: { show: false },
  });

  const statsQuery = createQuery(() => ({
    queryKey: ["dashboard-stats"],
    queryFn: getDashboardStats,
    staleTime: 5 * 60 * 1000,
  }));

  createEffect(() => {
    if (!statsQuery.isSuccess || !statsQuery.data) return;
    const data = statsQuery.data as DashboardStats;

    const series = [data.dag_type_counts["Event Driven"] || 0, data.dag_type_counts["Scheduled"] || 0];
    const seriesTask = [data.task_outcomes["Completed"] || 0, data.task_outcomes["Failed"] || 0];

    const seriesTime: string[] = [];
    const seriesSuccessful: number[] = [];
    const seriesFailed: number[] = [];

    for (const element of data.daily_dag_run_counts) {
      seriesFailed.push(element.failed_count);
      seriesSuccessful.push(element.successful_count);
      seriesTime.push(element.day.substring(0, 10));
    }

    setDagTypeDonutChartOptions((prev) => ({ ...prev, series }));
    setTaskOutcomesDonutChartOptions((prev) => ({ ...prev, series: seriesTask }));
    setTimeLine((prev) => ({
      ...prev,
      series: [{ name: "Successful", data: seriesSuccessful }, { name: "Failed", data: seriesFailed }],
      xaxis: { categories: seriesTime, labels: { style: { colors: "#FFFFFF" } } },
    }));
  });

  const stats = statsQuery.data;

  const skeleton = (
    <div class="flex flex-col w-full max-w-5xl mx-auto p-6 gap-6">
      <div class="flex flex-col gap-2">
        <div class="h-8 bg-gray-700 rounded w-48 animate-pulse"></div>
      </div>

      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        <SkeletonCard titleLines={1} bodyLines={1} />
        <SkeletonCard titleLines={1} bodyLines={1} />
        <SkeletonCard titleLines={1} bodyLines={1} />
        <SkeletonCard titleLines={1} bodyLines={1} />
      </div>

      <SkeletonCard titleLines={1} bodyLines={1} height="h-64" />

      <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
        <SkeletonCard titleLines={1} bodyLines={1} height="h-16" />
        <SkeletonCard titleLines={1} bodyLines={1} height="h-24" />
        <SkeletonCard titleLines={1} bodyLines={1} height="h-24" />
      </div>
    </div>
  );

  return (
    <Loadable
      loading={statsQuery.isLoading}
      error={statsQuery.error && (statsQuery.error as any).message}
      onRetry={() => statsQuery.refetch()}
      skeleton={skeleton}
    >
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
                <div class="text-4xl font-bold">
                  {stats ? stats.dag_count : 0}
                </div>
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
                <div class="text-4xl font-bold text-green-500">
                  {stats ? stats.successful_dag_runs : 0}
                </div>
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
                <div class="text-4xl font-bold text-red-500">
                  {stats ? stats.failed_dag_runs : 0}
                </div>
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
                <div class="text-4xl font-bold">
                  {stats ? stats.total_dag_runs : 0}
                </div>
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
              <Chart options={timeLine() as ApexOptions} />
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
                <div class="text-8xl font-bold text-fit">
                  {stats ? stats.active_dag_runs : 0}
                </div>
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
    </Loadable>
  );
};

export default Main;
