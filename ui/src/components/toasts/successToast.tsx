import { createSignal, onMount, Show } from "solid-js";

function SuccessToast(props: { message: string; clear: () => void }) {
  const [visible, setVisible] = createSignal(true);

  onMount(() => {
    const fadeTimeout = setTimeout(() => {
      setVisible(false);
      setTimeout(() => props.clear(), 300); // Ensure smooth fade-out before clearing
    }, 5000);

    const cleanTimeout = setTimeout(() => {
      try {
        props.clear();
      } catch (error) {
        console.error("Failed to clear toast:", error);
      }
    }, 5300);

    return () => {
      clearTimeout(fadeTimeout);
      clearTimeout(cleanTimeout);
    };
  });

  return (
    <Show when={visible()}>
      <div
        class="fixed bottom-16 left-1/2 transform -translate-x-1/2 bg-green-600 text-white p-4 rounded-lg shadow-lg z-50 transition-opacity duration-300"
        role="alert"
        aria-live="polite"
        aria-atomic="true"
      >
        <p>{props.message}</p>
      </div>
    </Show>
  );
}

export default SuccessToast;
