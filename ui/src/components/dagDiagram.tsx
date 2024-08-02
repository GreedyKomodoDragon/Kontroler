import { createSignal, onCleanup, onMount } from "solid-js";

type Task = {
  status: string;
};

type DagDiagramProps = {
  connections: Record<string, string[]>;
  taskInfo?: Record<string, Task>;
};

export default function DagDiagram(props: DagDiagramProps) {
  const { connections, taskInfo } = props;
  let canvas: HTMLCanvasElement | undefined;

  const [pipelineContainer, setPipelineContainer] = createSignal<
    HTMLDivElement | undefined
  >(undefined);
  const [taskPositions, setTaskPositions] = createSignal<
    Record<string, { x: number; y: number }>
  >({});

  const calculateTaskPositions = () => {
    const container = pipelineContainer();
    if (!container) return;

    const containerWidth = container.offsetWidth / 6; // Increase horizontal spacing
    const containerHeight = container.offsetHeight / 3; // Increase vertical spacing

    const positions: Record<string, { x: number; y: number }> = {};
    const levelWidth: Record<number, number> = {};
    const taskLevels: Record<string, number> = {};

    const calculateLevels = (taskId: string, level: number) => {
      if (taskLevels[taskId] !== undefined) {
        taskLevels[taskId] = Math.max(taskLevels[taskId], level);
      } else {
        taskLevels[taskId] = level;
      }
      connections[taskId].forEach((child) => calculateLevels(child, level + 1));
    };

    Object.keys(connections).forEach((taskId) => {
      if (taskLevels[taskId] === undefined) {
        calculateLevels(taskId, 0);
      }
    });

    // Determine the maximum level to calculate flip positions
    const maxLevel = Math.max(...Object.values(taskLevels));

    Object.entries(taskLevels).forEach(([taskId, level]) => {
      if (!levelWidth[level]) {
        levelWidth[level] = 0;
      }
      const y = levelWidth[level];
      // Flip the x position by subtracting from maxLevel
      positions[taskId] = {
        x: (maxLevel - level) * containerWidth + 40,
        y: y * containerHeight + 40,
      }; // Increased margin
      levelWidth[level] += 1;
    });

    setTaskPositions(positions);
  };

  const drawLines = (ctx: CanvasRenderingContext2D) => {
    ctx.clearRect(0, 0, canvas!.width, canvas!.height);
    const positions = taskPositions();

    const hoverEventHandler = (event: MouseEvent) => {
      ctx.clearRect(0, 0, canvas!.width, canvas!.height);

      const rect = canvas!.getBoundingClientRect();
      const x = event.clientX - rect.left;
      const y = event.clientY - rect.top;

      for (const [from, toList] of Object.entries(connections)) {
        toList.forEach((to) => {
          if (positions[from] && positions[to]) {
            const fromX = positions[from].x + 50;
            const fromY = positions[from].y + 25;
            const toX = positions[to].x + 50;
            const toY = positions[to].y + 25;
            const controlX1 = fromX + (toX - fromX) / 2;
            const controlY1 = fromY;
            const controlX2 = fromX + (toX - fromX) / 2;
            const controlY2 = toY;

            ctx.beginPath();
            ctx.moveTo(fromX, fromY);
            ctx.bezierCurveTo(
              controlX1,
              controlY1,
              controlX2,
              controlY2,
              toX,
              toY
            );

            // Check if the mouse is near this path
            const pathHovered = ctx.isPointInStroke(x, y);

            if (pathHovered) {
              ctx.strokeStyle = "cyan";
              ctx.lineWidth = 6;
            } else {
              ctx.strokeStyle = "white";
              ctx.lineWidth = 4;
            }

            ctx.stroke();
          }
        });
      }
    };

    // Draw all paths initially
    for (const [from, toList] of Object.entries(connections)) {
      toList.forEach((to) => {
        if (positions[from] && positions[to]) {
          ctx.beginPath();
          const fromX = positions[from].x + 50;
          const fromY = positions[from].y + 25;
          const toX = positions[to].x + 50;
          const toY = positions[to].y + 25;
          const controlX1 = fromX + (toX - fromX) / 2;
          const controlY1 = fromY;
          const controlX2 = fromX + (toX - fromX) / 2;
          const controlY2 = toY;
          ctx.moveTo(fromX, fromY);
          ctx.bezierCurveTo(
            controlX1,
            controlY1,
            controlX2,
            controlY2,
            toX,
            toY
          );
          ctx.strokeStyle = "white";
          ctx.lineWidth = 4;
          ctx.stroke();
        }
      });
    }

    // Add hover event listener
    canvas!.addEventListener("mousemove", hoverEventHandler);

    onCleanup(() => {
      canvas!.removeEventListener("mousemove", hoverEventHandler);
    });
  };

  onMount(() => {
    if (!canvas || !pipelineContainer) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const resizeCanvas = () => {
      const container = pipelineContainer();
      if (!container || !canvas) return;

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

  const getTaskColour = (taskId: string) => {
    if (taskInfo == undefined || taskInfo == null) {
      return "bg-neutral-500"; 
    }

    switch (taskInfo[taskId].status) {
      case "success":
        return "bg-green-500";
      case "running":
        return "bg-blue-500";
      case "pending":
        return "bg-neutral-500";
      default:
        return "bg-red-500";
    }
  };

  return (
    <div
      class="pipeline-container relative flex gap-5 items-start"
      ref={(el) => setPipelineContainer(el)}
      style={{ height: "36vh", width: "75vw" }}
    >
      {Object.entries(taskPositions()).map(([taskId, pos]) => (
        <div
          class={`pipeline-task ${getTaskColour(taskId)} text-white w-24 h-12 flex justify-center items-center rounded absolute z-10`}
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
