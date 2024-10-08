type errorSingleAlertProps = {
  msg: string;
};

export default function errorSingleAlert(props: errorSingleAlertProps) {
  return (
    <div class="mx-4 px-4 rounded-md border-l-4 border-red-500 bg-red-50">
      <div class="flex justify-between py-3">
        <div class="flex">
          <div>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-6 w-6 text-red-500"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path
                fill-rule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
                clip-rule="evenodd"
              />
            </svg>
          </div>
          <div class="self-center ml-3">
            <span class="text-red-600 font-semibold">Error</span>
            <p class="text-red-600 mt-1"> {props.msg}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
