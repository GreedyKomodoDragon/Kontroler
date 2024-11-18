import { createEffect, onCleanup, Setter } from "solid-js";
import { Network, Data, Options } from "vis-network/standalone/esm/vis-network";

type Task = {
  status: string;
};

interface Edge {
  from: number;
  to: number;
  arrows:
    | string
    | {
        to?: { enabled: boolean; scaleFactor: number };
        middle?: { enabled: boolean; scaleFactor: number };
        from?: { enabled: boolean };
      };
}

interface DagVizProps {
  connections: Record<string, string[]>;
  taskInfo?: Record<string, Task>;
  setSelectedTask?: Setter<number>;
}

export default function DagViz(props: DagVizProps) {
  const getTaskColour = (key: string) => {
    if (!props.taskInfo) {
      return "#6B7280"; // bg-neutral-500
    }

    const task = props.taskInfo[key];

    if (task == undefined || task == null) {
      return "#6B7280"; // bg-neutral-500
    }

    switch (task.status) {
      case "success":
        return "#10B981"; // bg-green-500
      case "running":
        return "#3B82F6"; // bg-blue-500
      case "pending":
        return "#6B7280"; // bg-neutral-500
      default:
        return "#EF4444"; // bg-red-500
    }
  };

  // Generate nodes from the keys of the connections
  const generateNodes = (): { id: number; label: string }[] => {
    return Object.keys(props.connections).map((key) => ({
      id: parseInt(key),
      label: `${key}`,
      color: {
        background: getTaskColour(key),
        border: "#34495E",
        highlight: {
          background: "#F0F8FF", // Light blue for highlight
          border: "#34495E", // Dark border for highlight
        },
        hover: {
          background: "#D5DBDB", // Light gray background on hover
          border: "#34495E", // Keep dark border on hover
        },
      },
      shape: "box",
      font: { size: 20, color: "white" },
    }));
  };

  // Generate edges based on the connections prop
  const generateEdges = (): Edge[] => {
    const edges: Edge[] = [];

    // Loop through the connections and generate edges
    Object.entries(props.connections).forEach(([from, toNodes]) => {
      toNodes.forEach((to) => {
        edges.push({
          // Flipped as connections are sent as dependencies
          from: parseInt(to),
          to: parseInt(from),
          arrows: {
            to: { enabled: true, scaleFactor: 1 },
            middle: { enabled: false, scaleFactor: 1.5 },
            from: { enabled: false },
          },
        });
      });
    });

    return edges;
  };

  const nodes = generateNodes();
  const edges = generateEdges();

  let networkContainer: HTMLDivElement | undefined;

  createEffect(() => {
    if (networkContainer) {
      const data: Data = {
        nodes: nodes,
        edges: edges,
      };

      const options: Options = {
        // Means DAG do not change each time they are reloaded - makes it hard to follow the flow
        layout: {
          randomSeed: 400,
        },
        interaction: {
          dragNodes: true,
          dragView: false,
          zoomView: false,
          selectable: true,
          hover: true,
        },
        edges: {
          arrows: {
            to: { enabled: true, scaleFactor: 1.5 },
            middle: { enabled: true, scaleFactor: 1.5 },
            from: { enabled: false },
          },
        },
      };

      const network = new Network(networkContainer, data, options);

      network.on("click", (event) => {
        if (event.nodes.length != 0 && props.setSelectedTask) {
          props.setSelectedTask(parseInt(event.nodes[0]));
        }
      });

      // Cleanup the network instance when the component is unmounted
      onCleanup(() => {
        network.destroy();
      });
    }
  });

  return (
    <div
      ref={networkContainer}
      class="h-[400px] overflow-auto max-w-full border border-gray-700 rounded-md bg-gray-900 p-4 m-2"
    ></div>
  );
}
