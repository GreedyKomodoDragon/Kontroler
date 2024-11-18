import { createSignal, onMount, Show } from "solid-js";

function SuccessToast(props: { message: string; clear: () => void }) {
  const [visible, setVisible] = createSignal(true);

  onMount(() => {
    const timeout = setTimeout(() => {
      setVisible(false);
      setTimeout(() => props.clear(), 300); // Ensure smooth fade-out before clearing
    }, 5000);

    return () => clearTimeout(timeout); // Cleanup on component unmount
  });

  return (
    <Show when={visible()}>
      <div
        class="fixed bottom-16 left-1/2 transform -translate-x-1/2 bg-green-600 text-white p-4 rounded-lg shadow-lg z-50 transition-opacity duration-300"
      >
        <p>{props.message}</p>
      </div>
    </Show>
  );
}

export default SuccessToast;
