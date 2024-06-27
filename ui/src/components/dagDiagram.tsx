import { createSignal, onCleanup, onMount } from "solid-js";

type task = {
  status: string;
};

type dagDiagramProps = {
  connections: Record<string, string[]>;
  taskInfo: Record<string, task>;
};

export default function DagDiagram(props: dagDiagramProps) {
  const { connections, taskInfo } = props;
  let canvas: HTMLCanvasElement | undefined;

  const [pipelineContainer, setPipelineContainer] = createSignal<
    HTMLDivElement | undefined
  >(undefined);

  const [taskPositions, setTaskPositions] = createSignal<
    Record<string, { x: number; y: number }>
  >({});

  const calculateTaskPositions = () => {
    let container = pipelineContainer();
    if (container === undefined) return;

    const containerWidth = container.offsetWidth / 8;

    const positions: Record<string, { x: number; y: number }> = {};
    const levels: Record<number, number> = {};

    const calculatePosition = (taskId: string, level: number) => {
      if (!positions[taskId]) {
        const y = levels[level] || 0;
        positions[taskId] = { x: level * containerWidth + 20, y: y * 100 + 20 };
        levels[level] = (levels[level] || 0) + 1;
      }
      connections[taskId].forEach((child) =>
        calculatePosition(child, level + 1)
      );
    };

    calculatePosition(Object.keys(connections)[0], 0);
    setTaskPositions(positions);
  };

  const drawLines = (ctx: CanvasRenderingContext2D) => {
    ctx.clearRect(0, 0, canvas!.width, canvas!.height); // Clear the canvas before drawing
    ctx.strokeStyle = "white";
    ctx.lineWidth = 2;

    const positions = taskPositions();
    for (const [from, toList] of Object.entries(connections)) {
      toList.forEach((to) => {
        ctx.beginPath();
        ctx.moveTo(positions[from].x + 50, positions[from].y + 25); // Adjusted to center of the task
        ctx.lineTo(positions[to].x + 50, positions[to].y + 25); // Adjusted to center of the task
        ctx.stroke();
      });
    }
  };

  onMount(() => {
    if (!canvas || !pipelineContainer) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const resizeCanvas = () => {
      let container = pipelineContainer();
      if (container === undefined || !canvas) return;

      canvas.width = container.offsetWidth;
      canvas.height = container.offsetHeight;
      calculateTaskPositions();
      drawLines(ctx);
    };

    resizeCanvas(); // Initial drawing
    window.addEventListener("resize", resizeCanvas);

    onCleanup(() => {
      window.removeEventListener("resize", resizeCanvas);
    });
  });

  return (
    <div
      class="pipeline-container relative flex gap-5 items-start"
      ref={(el) => setPipelineContainer(el)}
      style={{ height: "36vh", width: "80vw" }}
    >
      {Object.entries(taskPositions()).map(([taskId, pos]) => (
        <div
          class={`pipeline-task ${
            taskInfo[taskId].status === "failed" ? "bg-red-500" : "bg-green-500"
          } text-white w-24 h-12 flex justify-center items-center rounded absolute z-10`}
          id={taskId}
          style={{ left: `${pos.x}px`, top: `${pos.y}px` }}
        >
          {taskId.replace("task", "Task ")}
        </div>
      ))}
      <canvas
        id="pipeline-canvas"
        class="absolute top-0 left-0 z-1"
        ref={(el) => (canvas = el!)}
      ></canvas>
    </div>
  );
}
