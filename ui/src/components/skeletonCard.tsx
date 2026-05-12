import { JSX } from "solid-js";

type SkeletonCardProps = {
  titleLines?: number;
  bodyLines?: number;
  height?: string;
};

export default function SkeletonCard(props: SkeletonCardProps) {
  return (
    <div class="rounded-lg border bg-card text-card-foreground shadow-sm p-6">
      <div class="animate-pulse">
        <div class="h-6 bg-gray-600 rounded w-3/4 mb-4"></div>
        <div class={`grid gap-2 ${props.height ? '' : 'grid-cols-1'}`}>
          {Array.from({ length: props.bodyLines ?? 1 }).map(() => (
            <div class="h-8 bg-gray-700 rounded w-full"></div>
          ))}
        </div>
      </div>
    </div>
  );
}
