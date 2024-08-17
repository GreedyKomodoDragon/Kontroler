import { DagFormObj, TaskSpec } from "../types/dagForm";

function isValidCron(cron: string): boolean {
  const cronRegex = /^(\*|([0-5]?\d))( (\*|([0-5]?\d))){4}$/;
  return cronRegex.test(cron);
}

function detectCycles(tasks: TaskSpec[]): string[] {
  const taskMap = new Map<string, TaskSpec>();
  const visited = new Set<string>();
  const recursionStack = new Set<string>();

  // Build a map of task names to TaskSpec objects
  tasks.forEach((task) => taskMap.set(task.name, task));

  function hasCycle(taskName: string): boolean {
    if (recursionStack.has(taskName)) return true;
    if (visited.has(taskName)) return false;

    visited.add(taskName);
    recursionStack.add(taskName);

    const task = taskMap.get(taskName);
    if (task?.runAfter) {
      for (const dependency of task.runAfter) {
        if (hasCycle(dependency)) return true;
      }
    }

    recursionStack.delete(taskName);
    return false;
  }

  const errors: string[] = [];
  for (const taskName of taskMap.keys()) {
    if (hasCycle(taskName)) {
      errors.push(`Cyclic dependency detected involving task "${taskName}".`);
    }
  }

  return errors;
}

export function validateDagFormObj(dagFormObj: DagFormObj): string[] {
  const errors: string[] = [];

  // Validate the name field (all characters only)
  if (!/^[a-zA-Z]+$/.test(dagFormObj.name)) {
    errors.push("Invalid DAG name: should contain only alphabetic characters.");
  }

  // Validate the schedule field (empty or valid cron string)
  if (dagFormObj.schedule && !isValidCron(dagFormObj.schedule)) {
    errors.push("Invalid schedule: should be empty or a valid cron string.");
  }

  if (dagFormObj.tasks.length === 0) {
    errors.push("You must provide at least one task for a dag");
  }

  // Validate tasks
  const taskNames = dagFormObj.tasks.map((task) => task.name);

  dagFormObj.tasks.forEach((task) => {
    if (!task.command || task.command.length === 0) {
      errors.push(`Task "${task.name}" is missing a command. Must be an array of strings`);
    }

    if (task.args === undefined) {
      errors.push(`Task "${task.name}": args is invalid, must be a array of strings`);
    }

    if (task.retryCodes === undefined) {
      errors.push(`Task "${task.name}": retry codes is invalid, must be a array of numbers`);
    }

    if (!task.image) {
      errors.push(`Task "${task.name}" is missing an image.`);
    }

    if (task.runAfter) {
      task.runAfter.forEach((dependency) => {
        if (dependency === task.name) {
          errors.push(
            `Task "${task.name}" has itself listed in runAfter, which is not allowed.`
          );
        } else if (!taskNames.includes(dependency)) {
          errors.push(
            `Task "${task.name}" has a dependency "${dependency}" that does not exist in the task list.`
          );
        }
      });
    }
  });

  // Detect cyclic dependencies
  errors.push(...detectCycles(dagFormObj.tasks));

  return errors;
}
