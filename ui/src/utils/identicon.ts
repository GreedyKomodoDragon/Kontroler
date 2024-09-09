export async function generateIdenticon(
  input: string,
  size: number = 100,
  gridSize: number = 7 // You can change this to any desired grid size
): Promise<HTMLCanvasElement> {
  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to get canvas context");

  canvas.width = size;
  canvas.height = size;

  const hash = await hashString(input);

  const color = generateColorFromHash(hash);

  // Background color (white)
  ctx.fillStyle = "#FFFFFF";
  ctx.fillRect(0, 0, size, size);

  ctx.fillStyle = color;

  const cellSize = Math.floor(size / gridSize);

  for (let y = 0; y < gridSize; y++) {
    for (let x = 0; x < Math.ceil(gridSize / 2); x++) {
      const index = y * gridSize + x;

      // Ensure we don't overflow the hash length
      const hashValue = parseInt(hash[index % hash.length], 16);

      if (hashValue % 2 === 0) {
        ctx.fillRect(x * cellSize, y * cellSize, cellSize, cellSize);
        ctx.fillRect(
          (gridSize - x - 1) * cellSize,
          y * cellSize,
          cellSize,
          cellSize
        );
      }
    }
  }

  return canvas;
}

// Generate a color from the hash
function generateColorFromHash(hash: string): string {
  const r = parseInt(hash.slice(0, 2), 16);
  const g = parseInt(hash.slice(2, 4), 16);
  const b = parseInt(hash.slice(4, 6), 16);

  return `rgb(${r}, ${g}, ${b})`;
}

async function hashString(input: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(input);
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);

  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const hashHex = hashArray
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  return hashHex;
}
