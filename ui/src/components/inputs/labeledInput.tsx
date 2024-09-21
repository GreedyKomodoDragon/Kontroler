type LabeledInputProps = {
  label: string;
};

export default function LabeledInput(props: LabeledInputProps) {
  return (
    <div class="flex items-center border rounded-md">
      <div class="w-36 px-3 py-2.5 rounded-l-md text-black bg-gray-50 border-r flex-shrink-0 truncate">
        {props.label}
      </div>
      <input
        type="text"
        placeholder="input parameter value"
        id="website-url"
        class="w-full p-2.5 ml-2 bg-transparent outline-none"
      />
    </div>
  );
}
