
type successAlertProps = {
  msg: string;
};

export default function SuccessfulAlert(props: successAlertProps) {
  return (
    <div class="mx-auto px-4">
      <div class="flex justify-between p-4 rounded-md bg-green-50 border border-green-300">
        <div class="flex items-start gap-3 w-full">
          <div>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-6 w-6 text-green-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width={2}
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </div>
          <div class="flex-1 self-center">
            <span class="text-green-600 font-medium">Successful</span>
            <div class="text-green-600">
              <p class="mt-2 sm:text-sm">{props.msg}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
