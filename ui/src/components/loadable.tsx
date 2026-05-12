import { JSX } from "solid-js";
import Spinner from "./spinner";
import ErrorSingleAlert from "./alerts/errorSingleAlert";

type LoadableProps = {
  loading?: boolean;
  error?: string | null | undefined;
  onRetry?: () => void;
  children: JSX.Element;
};

export default function Loadable(props: LoadableProps) {
  if (props.loading) {
    return (
      <div class="flex items-center justify-center w-full py-12">
        <Spinner />
      </div>
    );
  }

  if (props.error) {
    return (
      <div class="w-full py-4">
        <ErrorSingleAlert msg={props.error} />
        {props.onRetry ? (
          <div class="mt-4 flex justify-end">
            <button
              class="px-4 py-2 bg-blue-600 text-white rounded-md"
              onClick={props.onRetry}
            >
              Retry
            </button>
          </div>
        ) : null}
      </div>
    );
  }

  return <>{props.children}</>;
}
