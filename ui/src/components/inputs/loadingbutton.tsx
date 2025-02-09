import { createSignal } from "solid-js";

interface LoadingButtonProps {
  onClick: () => Promise<void>;
}

export default function LoadingButton({ onClick }: LoadingButtonProps) {
  const [isLoading, setIsLoading] = createSignal(false);

  const handleClick = async () => {
    try {
      setIsLoading(true);
      await onClick();
    } catch (error) {
      console.error("Error in LoadingButton:", error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handleClick}
      disabled={isLoading()}
      title="Refresh DAG state"
      class={`inline-flex items-center gap-2 px-4 py-2`}
    >
      <svg
        class={`inline-block h-12 w-12 text-primary ${
          isLoading()
            ? "animate-spin motion-reduce:animate-[spin_10s_linear_infinite]"
            : ""
        }`}
        style={{ "animation-duration": "1.5s" }}
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 32 32"
        role="status"
        aria-label="Loading"
      >
        <path
          fill="none"
          stroke="white"
          stroke-width="2"
          stroke-miterlimit="10"
          d="M25.7,10.9C23.9,7.4,20.2,5,16,5
	c-4.7,0-8.6,2.9-10.2,7"
        />
        <path
          fill="none"
          stroke="white"
          stroke-width="2"
          stroke-miterlimit="10"
          d="M6.2,21c1.8,3.5,5.5,6,9.8,6c4.7,0,8.6-2.9,10.2-7"
        />
        <polyline
          fill="none"
          stroke="white"
          stroke-width="2"
          stroke-miterlimit="10"
          points="26,5 26,11 20,11 "
        />
        <polyline
          fill="none"
          stroke="white"
          stroke-width="2"
          stroke-miterlimit="10"
          points="6,27 6,21 12,21 "
        />
      </svg>
    </button>
  );
}
