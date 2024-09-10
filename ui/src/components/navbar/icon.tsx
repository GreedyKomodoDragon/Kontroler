import { createSignal, onMount } from "solid-js";
import { generateIdenticon } from "../../utils/identicon";

export default function Identicon(props: {
  value: string;
  size?: number;
  gridSize?: number;
}) {
  let canvasRef: HTMLCanvasElement | null = null;
  const [size, setSize] = createSignal(props.size || 200);
  const [gridSize, setGridSize] = createSignal(props.gridSize || 10);

  // Use async function to generate identicon
  onMount(async () => {
    if (canvasRef) {
      const identiconCanvas = await generateIdenticon(
        props.value,
        size(),
        gridSize()
      );
      const ctx = canvasRef.getContext("2d");
      if (ctx) {
        ctx.clearRect(0, 0, size(), size());

        // Create a circular clipping path
        const radius = size() / 2;
        const centerX = size() / 2;
        const centerY = size() / 2;

        // Ensure the arc is perfectly centered
        ctx.beginPath();
        ctx.arc(centerX, centerY, radius, 0, Math.PI * 2, true);
        ctx.clip();

        ctx.drawImage(identiconCanvas, 0, 0, size(), size());
      }
    }
  });

  return (
    <canvas
      ref={(el) => (canvasRef = el)}
      width={size()}
      height={size()}
      class="rounded-full shadow-lg"
    ></canvas>
  );
}
