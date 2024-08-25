import { onCleanup, onMount, createEffect } from "solid-js";
import ApexCharts, { ApexOptions } from "apexcharts";

type ChartComponentProps = {
  options: ApexOptions;
};

const Chart = (props: ChartComponentProps) => {
  let chart: ApexCharts | undefined;
  let chartRef: HTMLDivElement | undefined;

  onMount(() => {
    if (chartRef) {
      chart = new ApexCharts(chartRef, props.options);
      chart.render();

      onCleanup(() => {
        chart?.destroy();
      });
    }
  });

  // Reactively update the chart options when props.options changes
  createEffect(() => {
    if (chart) {
      chart.updateOptions(props.options, true, false);
    }
  });

  return <div ref={(el) => (chartRef = el!)}></div>;
};

export default Chart;
