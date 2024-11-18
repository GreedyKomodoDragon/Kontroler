import { createSignal, For, onMount, Show } from "solid-js";

function ErrorToast(props: { messages: string[]; clear: () => void }) {
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
        class="fixed bottom-16 left-1/2 transform -translate-x-1/2 bg-red-600 text-white p-4 rounded-lg shadow-lg z-50 transition-opacity duration-300"
        role="alert"
        aria-live="assertive"
      >
        <h4 class="font-semibold mb-2">Errors:</h4>
        <ul class="list-disc list-inside">
          <For each={props.messages}>{(msg) => <li>{msg}</li>}</For>
        </ul>
      </div>
    </Show>
  );
}

export default ErrorToast;
