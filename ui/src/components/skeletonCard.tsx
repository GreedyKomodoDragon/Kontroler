import { JSX } from "solid-js";

type SkeletonCardProps = {
  titleLines?: number;
  bodyLines?: number;
  height?: string; // e.g. 'h-24' or inline style
};

export default function SkeletonCard(props: SkeletonCardProps) {
  const titleCount = props.titleLines ?? 1;
  const bodyCount = props.bodyLines ?? 1;

  return (
    <div class="rounded-lg border bg-card text-card-foreground shadow-sm p-6">
      <div class="animate-pulse">
        {Array.from({ length: titleCount }).map(() => (
          <div class="h-4 bg-gray-600 rounded w-3/4 mb-3"></div>
        ))}

        <div class="grid gap-2">
          {Array.from({ length: bodyCount }).map(() => (
            <div class={`bg-gray-700 rounded w-full ${props.height ?? 'h-8'}`}></div>
          ))}
        </div>
      </div>
    </div>
  );
}
