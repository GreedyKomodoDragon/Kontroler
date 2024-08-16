// validateDagFormObj.test.ts
import { describe, it, expect } from 'vitest';
import { DagFormObj } from '../types/dagForm';
import { validateDagFormObj } from './dagform';

describe('validateDagFormObj', () => {

  it('should pass with a valid DAG form object', () => {
    const validDagForm: DagFormObj = {
      name: "exampleDAG",
      schedule: "* * * * *",
      tasks: [
        {
          name: "task1",
          command: ["echo", "hello"],
          image: "alpine",
          runAfter: ["task2"],
          backoffLimit: 3,
        },
        {
          name: "task2",
          command: ["echo", "world"],
          image: "alpine",
          backoffLimit: 3,
        },
      ],
    };

    const errors = validateDagFormObj(validDagForm);
    expect(errors).toHaveLength(0);
  });

  it('should fail when the name contains non-alphabetic characters', () => {
    const invalidDagForm: DagFormObj = {
      name: "example123",
      tasks: [],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain("Invalid DAG name: should contain only alphabetic characters.");
  });

  it('should fail when the schedule is an invalid cron string', () => {
    const invalidDagForm: DagFormObj = {
      name: "validName",
      schedule: "invalid cron",
      tasks: [],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain("Invalid schedule: should be empty or a valid cron string.");
  });

  it('should fail when a task is missing a command', () => {
    const invalidDagForm: DagFormObj = {
      name: "validName",
      tasks: [
        {
          name: "task1",
          command: [],
          image: "alpine",
          backoffLimit: 3,
        },
      ],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain('Task "task1" is missing a command.');
  });

  it('should fail when a task is missing an image', () => {
    const invalidDagForm: DagFormObj = {
      name: "validName",
      tasks: [
        {
          name: "task1",
          command: ["echo", "hello"],
          image: "",
          backoffLimit: 3,
        },
      ],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain('Task "task1" is missing an image.');
  });

  it('should fail when a task has itself listed in runAfter', () => {
    const invalidDagForm: DagFormObj = {
      name: "validName",
      tasks: [
        {
          name: "task1",
          command: ["echo", "hello"],
          image: "alpine",
          runAfter: ["task1"],
          backoffLimit: 3,
        },
      ],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain('Task "task1" has itself listed in runAfter, which is not allowed.');
  });

  it('should fail when a task has a non-existent task in runAfter', () => {
    const invalidDagForm: DagFormObj = {
      name: "validName",
      tasks: [
        {
          name: "task1",
          command: ["echo", "hello"],
          image: "alpine",
          runAfter: ["task2"],
          backoffLimit: 3,
        },
      ],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain('Task "task1" has a dependency "task2" that does not exist in the task list.');
  });

  it('should fail when there are cyclic dependencies', () => {
    const invalidDagForm: DagFormObj = {
      name: "validName",
      tasks: [
        {
          name: "task1",
          command: ["echo", "hello"],
          image: "alpine",
          runAfter: ["task2"],
          backoffLimit: 3,
        },
        {
          name: "task2",
          command: ["echo", "world"],
          image: "alpine",
          runAfter: ["task1"],
          backoffLimit: 3,
        },
      ],
    };

    const errors = validateDagFormObj(invalidDagForm);
    expect(errors).toContain('Cyclic dependency detected involving task "task1".');
    expect(errors).toContain('Cyclic dependency detected involving task "task2".');
  });

});
