type SuspendButtonProps = {
  action: () => void;
  size?: "sm" | "s" | "md" | "lg";
  isSuspended: boolean;
};

export function SuspendButton(props: SuspendButtonProps) {
  const sizes = {
    sm: { container: "p-1", svg: "24" },
    s: { container: "p-1", svg: "28" },
    md: { container: "p-2", svg: "32" },
    lg: { container: "p-3", svg: "40" },
  };

  const { container, svg } = sizes[props.size || "md"];

  return (
    <div
      class={`${props.isSuspended ? 'bg-green-400' : 'bg-yellow-400'} rounded-lg ${container} cursor-pointer`}
      onClick={props.action}
    >
      <svg
        width={svg}
        height={svg}
        viewBox="0 0 24 24"
        fill="black"
        xmlns="http://www.w3.org/2000/svg"
      >
        {props.isSuspended ? (
          <path
            d="M7 4.5v15L19 12 7 4.5z"
          />
        ) : (
          <path
            id="primary"
            d="M19,4V20a2,2,0,0,1-2,2H15a2,2,0,0,1-2-2V4a2,2,0,0,1,2-2h2A2,2,0,0,1,19,4ZM9,2H7A2,2,0,0,0,5,4V20a2,2,0,0,0,2,2H9a2,2,0,0,0,2-2V4A2,2,0,0,0,9,2Z"
          />
        )}
      </svg>
    </div>
  );
}
