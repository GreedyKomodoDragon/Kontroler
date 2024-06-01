import { onCleanup, onMount } from "solid-js";
import ApexCharts, { ApexOptions } from "apexcharts";

type ChartComponentProps = {
  options: ApexOptions;
};

const Chart = (props: ChartComponentProps) => {
  let chartRef: HTMLDivElement | undefined;

  onMount(() => {
    if (chartRef) {
      const chart = new ApexCharts(chartRef, props.options);
      chart.render();

      onCleanup(() => {
        chart.destroy();
      });
    }
  });

  return <div ref={(el) => (chartRef = el!)}></div>;
};

export default Chart;
