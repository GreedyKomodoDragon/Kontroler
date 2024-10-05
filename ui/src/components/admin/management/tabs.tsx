import { Accessor } from "solid-js";

type Tab = {
  id: string;
  label: string;
};

interface TabsProps {
  tabs: Tab[];
  activeTab: Accessor<string>;
  setActiveTab: (id: string) => void;
}

export default function Tabs({ tabs, activeTab, setActiveTab }: TabsProps) {
  return (
    <div class="grid w-full grid-cols-4 gap-4" role="tablist" aria-orientation="horizontal">
      {tabs.map((tab) => (
        <button
          role="tab"
          aria-selected={activeTab() === tab.id}
          aria-controls={`${tab.id}-tab`}
          id={`${tab.id}-tab-trigger`}
          class={`inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium ring-offset-background transition-all 
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 
              ${
                activeTab() === tab.id
                  ? "bg-blue-500 text-white shadow-lg ring-blue-400"
                  : "bg-gray-100 text-gray-700 hover:bg-blue-100"
              }`}
          onClick={() => setActiveTab(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
