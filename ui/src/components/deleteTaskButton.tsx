type DeleteButtonProps = {
  taskIndex: any;
  delete: (index: any) => void;
  size?: 'sm' | 's' | 'md' | 'lg';
};

export function DeleteTaskButton(props: DeleteButtonProps) {
  const sizes = {
    sm: { container: 'p-1', svg: '24' },
    s: { container: 'p-1', svg: '28' },
    md: { container: 'p-2', svg: '32' },
    lg: { container: 'p-3', svg: '40' },
  };

  const { container, svg } = sizes[props.size || 'md'];

  return (
    <div
      class={`bg-red-400 rounded-lg ${container} cursor-pointer`}
      onClick={() => {
        props.delete(props.taskIndex);
      }}
    >
      <svg
        width={svg}
        height={svg}
        viewBox="0 0 24 24"
        fill="white"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M7 4a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v2h4a1 1 0 1 1 0 2h-1.069l-.867 12.142A2 2 0 0 1 17.069 22H6.93a2 2 0 0 1-1.995-1.858L4.07 8H3a1 1 0 0 1 0-2h4zm2 2h6V4H9zM6.074 8l.857 12H17.07l.857-12zM10 10a1 1 0 0 1 1 1v6a1 1 0 1 1-2 0v-6a1 1 0 0 1 1-1m4 0a1 1 0 0 1 1 1v6a1 1 0 1 1-2 0v-6a1 1 0 0 1 1-1"
          fill="#0D0D0D"
        />
      </svg>
    </div>
  );
}
