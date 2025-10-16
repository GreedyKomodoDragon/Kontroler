package dagdsl_test

import (
	"testing"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/dagdsl"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDSL_ComplexGraph(t *testing.T) {
	dslInput := `graph {
  a -> b
  a -> c
  b -> d
  c -> d
}

task a {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Task A'"]
}

task b {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Task B'"]
}

task c {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Task C'"]
}

task d {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Task D'"]
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check that we have 4 tasks
	assert.Len(t, dagSpec.Task, 4)

	// Create a map of tasks by name for easier testing
	taskMap := make(map[string]*struct {
		name     string
		image    string
		command  []string
		args     []string
		runAfter []string
	})

	for i := range dagSpec.Task {
		task := &dagSpec.Task[i]
		taskMap[task.Name] = &struct {
			name     string
			image    string
			command  []string
			args     []string
			runAfter []string
		}{
			name:     task.Name,
			image:    task.Image,
			command:  task.Command,
			args:     task.Args,
			runAfter: task.RunAfter,
		}
	}

	// Test task A
	taskA := taskMap["a"]
	require.NotNil(t, taskA)
	assert.Equal(t, "alpine:latest", taskA.image)
	assert.Equal(t, []string{"sh", "-c"}, taskA.command)
	assert.Equal(t, []string{"echo 'Task A'"}, taskA.args)
	assert.Empty(t, taskA.runAfter)

	// Test task B - should depend on A
	taskB := taskMap["b"]
	require.NotNil(t, taskB)
	assert.Equal(t, "alpine:latest", taskB.image)
	assert.Contains(t, taskB.runAfter, "a")

	// Test task C - should depend on A
	taskC := taskMap["c"]
	require.NotNil(t, taskC)
	assert.Equal(t, "alpine:latest", taskC.image)
	assert.Contains(t, taskC.runAfter, "a")

	// Test task D - should depend on B and C
	taskD := taskMap["d"]
	require.NotNil(t, taskD)
	assert.Equal(t, "alpine:latest", taskD.image)
	assert.Contains(t, taskD.runAfter, "b")
	assert.Contains(t, taskD.runAfter, "c")
}

func TestParseDSL_SimpleLinearGraph(t *testing.T) {
	dslInput := `graph {
  a -> b
  b -> c
}

task a {
  image "alpine:latest"
  script "echo 'Task A'"
}

task b {
  image "alpine:latest"
  script "echo 'Task B'"
}

task c {
  image "alpine:latest"
  script "echo 'Task C'"
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check that we have 3 tasks
	assert.Len(t, dagSpec.Task, 3)

	// Find task dependencies
	taskMap := make(map[string][]string)
	for _, task := range dagSpec.Task {
		taskMap[task.Name] = task.RunAfter
	}

	// Test dependencies
	assert.Empty(t, taskMap["a"])         // Task A has no dependencies
	assert.Contains(t, taskMap["b"], "a") // Task B depends on A
	assert.Contains(t, taskMap["c"], "b") // Task C depends on B
}

func TestParseDSL_NoGraph(t *testing.T) {
	dslInput := `task a {
  image "alpine:latest"
  command ["echo", "hello"]
}

task b {
  image "ubuntu:latest"
  script "echo 'world'"
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check that we have 2 tasks
	assert.Len(t, dagSpec.Task, 2)

	// Both tasks should have no dependencies
	for _, task := range dagSpec.Task {
		assert.Empty(t, task.RunAfter)
	}
}

func TestParseDSL_TaskWithScript(t *testing.T) {
	dslInput := `task test {
  image "python:3.9"
  script "print('Hello from Python')"
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	assert.Len(t, dagSpec.Task, 1)
	task := dagSpec.Task[0]
	assert.Equal(t, "test", task.Name)
	assert.Equal(t, "python:3.9", task.Image)
	assert.Equal(t, "print('Hello from Python')", task.Script)
	assert.Empty(t, task.Command)
	assert.Empty(t, task.Args)
}

func TestParseDSL_MixedTaskTypes(t *testing.T) {
	dslInput := `graph {
  script_task -> command_task
}

task script_task {
  image "python:3.9"
  script "import sys; print(sys.version)"
}

task command_task {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["ls", "-la"]
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	assert.Len(t, dagSpec.Task, 2)

	// Find tasks by name
	var scriptTask, commandTask *struct {
		name     string
		image    string
		script   string
		command  []string
		args     []string
		runAfter []string
	}

	for _, task := range dagSpec.Task {
		taskInfo := &struct {
			name     string
			image    string
			script   string
			command  []string
			args     []string
			runAfter []string
		}{
			name:     task.Name,
			image:    task.Image,
			script:   task.Script,
			command:  task.Command,
			args:     task.Args,
			runAfter: task.RunAfter,
		}

		if task.Name == "script_task" {
			scriptTask = taskInfo
		} else if task.Name == "command_task" {
			commandTask = taskInfo
		}
	}

	// Test script task
	require.NotNil(t, scriptTask)
	assert.Equal(t, "python:3.9", scriptTask.image)
	assert.Equal(t, "import sys; print(sys.version)", scriptTask.script)
	assert.Empty(t, scriptTask.command)
	assert.Empty(t, scriptTask.args)
	assert.Empty(t, scriptTask.runAfter)

	// Test command task
	require.NotNil(t, commandTask)
	assert.Equal(t, "alpine:latest", commandTask.image)
	assert.Equal(t, []string{"sh", "-c"}, commandTask.command)
	assert.Equal(t, []string{"ls", "-la"}, commandTask.args)
	assert.Contains(t, commandTask.runAfter, "script_task")
}

func TestParseDSL_InvalidSyntax(t *testing.T) {
	testCases := []struct {
		name     string
		dslInput string
	}{
		{
			name:     "missing closing brace",
			dslInput: `dag "test" { task a { image "alpine" }`,
		},
		{
			name:     "invalid task field",
			dslInput: `dag "test" { task a { invalid_field "value" } }`,
		},
		{
			name:     "missing dag name",
			dslInput: `dag { task a { image "alpine" } }`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dagdsl.ParseDSL(tc.dslInput)
			assert.Error(t, err, "Expected error for invalid syntax")
		})
	}
}

func TestParseDSL_EmptyDAG(t *testing.T) {
	dslInput := ``

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	assert.Empty(t, dagSpec.Task)
}

func TestParseDSL_TaskFieldVariations(t *testing.T) {
	dslInput := `task minimal {
  image "alpine"
}

task with_command {
  image "alpine"
  command ["echo"]
}

task with_args {
  image "alpine"
  args ["hello", "world"]
}

task full_command {
  image "alpine"
  command ["sh", "-c"]
  args ["echo hello && echo world"]
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	assert.Len(t, dagSpec.Task, 4)

	taskMap := make(map[string]*struct {
		image   string
		command []string
		args    []string
	})

	for _, task := range dagSpec.Task {
		taskMap[task.Name] = &struct {
			image   string
			command []string
			args    []string
		}{
			image:   task.Image,
			command: task.Command,
			args:    task.Args,
		}
	}

	// Test minimal task
	minimal := taskMap["minimal"]
	assert.Equal(t, "alpine", minimal.image)
	assert.Empty(t, minimal.command)
	assert.Empty(t, minimal.args)

	// Test with_command task
	withCommand := taskMap["with_command"]
	assert.Equal(t, "alpine", withCommand.image)
	assert.Equal(t, []string{"echo"}, withCommand.command)
	assert.Empty(t, withCommand.args)

	// Test with_args task
	withArgs := taskMap["with_args"]
	assert.Equal(t, "alpine", withArgs.image)
	assert.Empty(t, withArgs.command)
	assert.Equal(t, []string{"hello", "world"}, withArgs.args)

	// Test full_command task
	fullCommand := taskMap["full_command"]
	assert.Equal(t, "alpine", fullCommand.image)
	assert.Equal(t, []string{"sh", "-c"}, fullCommand.command)
	assert.Equal(t, []string{"echo hello && echo world"}, fullCommand.args)
}

func TestParseDSL_WithSchedule(t *testing.T) {
	dslInput := `schedule "0 0 * * *"

task a {
  image "alpine:latest"
  script "echo 'Scheduled task'"
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check schedule is set
	assert.Equal(t, "0 0 * * *", dagSpec.Schedule)

	// Check task is present
	assert.Len(t, dagSpec.Task, 1)
	task := dagSpec.Task[0]
	assert.Equal(t, "a", task.Name)
	assert.Equal(t, "alpine:latest", task.Image)
	assert.Equal(t, "echo 'Scheduled task'", task.Script)
}

func TestParseDSL_WithScheduleAndGraph(t *testing.T) {
	dslInput := `schedule "*/5 * * * *"

graph {
  a -> b
}

task a {
  image "alpine:latest"
  script "echo 'First task'"
}

task b {
  image "alpine:latest"  
  script "echo 'Second task'"
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check schedule is set
	assert.Equal(t, "*/5 * * * *", dagSpec.Schedule)

	// Check tasks and dependencies
	assert.Len(t, dagSpec.Task, 2)

	taskMap := make(map[string][]string)
	for _, task := range dagSpec.Task {
		taskMap[task.Name] = task.RunAfter
	}

	assert.Empty(t, taskMap["a"])
	assert.Contains(t, taskMap["b"], "a")
}

func TestParseDSL_MultilineScript(t *testing.T) {
	dslInput := `task multiline_task {
  image "python:3.9"
  script """
import os
import sys

print("Hello from multiline script!")
print(f"Python version: {sys.version}")
print(f"Current directory: {os.getcwd()}")

for i in range(3):
    print(f"Count: {i}")
"""
}

task singleline_task {
  image "alpine:latest" 
  script "echo 'Hello from single line'"
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	assert.Len(t, dagSpec.Task, 2)

	// Find tasks by name
	var multilineTask, singlelineTask *v1alpha1.TaskSpec
	for i := range dagSpec.Task {
		if dagSpec.Task[i].Name == "multiline_task" {
			multilineTask = &dagSpec.Task[i]
		} else if dagSpec.Task[i].Name == "singleline_task" {
			singlelineTask = &dagSpec.Task[i]
		}
	}

	// Test multiline task
	require.NotNil(t, multilineTask)
	assert.Equal(t, "python:3.9", multilineTask.Image)
	expectedMultilineScript := `
import os
import sys

print("Hello from multiline script!")
print(f"Python version: {sys.version}")
print(f"Current directory: {os.getcwd()}")

for i in range(3):
    print(f"Count: {i}")
`
	assert.Equal(t, expectedMultilineScript, multilineTask.Script)

	// Test single line task
	require.NotNil(t, singlelineTask)
	assert.Equal(t, "alpine:latest", singlelineTask.Image)
	assert.Equal(t, "echo 'Hello from single line'", singlelineTask.Script)
}

func TestParseDSL_WithParameters(t *testing.T) {
	dslInput := `schedule "0 */6 * * *"

parameters {
  environment {
    default "dev"
  }
  replicas {
    default "3"
  }
  secretKey {
    defaultFromSecret "my-secret"
  }
}

graph {
  setup -> run
}

task setup {
  image "alpine:latest"
  command ["sh", "-c"]
  args ["echo 'Setting up environment'"]
  parameters ["environment", "replicas"]
}

task run {
  image "alpine:latest"
  script "echo 'Running with env: $ENVIRONMENT'"
  parameters ["environment", "secretKey"]
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check schedule
	assert.Equal(t, "0 */6 * * *", dagSpec.Schedule)

	// Check parameters
	require.Len(t, dagSpec.Parameters, 3)

	// Check environment parameter
	envParam := findParameterByName(dagSpec.Parameters, "environment")
	require.NotNil(t, envParam)
	assert.Equal(t, "environment", envParam.Name)
	assert.Equal(t, "dev", envParam.DefaultValue)
	assert.Equal(t, "", envParam.DefaultFromSecret)

	// Check replicas parameter
	replicasParam := findParameterByName(dagSpec.Parameters, "replicas")
	require.NotNil(t, replicasParam)
	assert.Equal(t, "replicas", replicasParam.Name)
	assert.Equal(t, "3", replicasParam.DefaultValue)
	assert.Equal(t, "", replicasParam.DefaultFromSecret)

	// Check secretKey parameter
	secretParam := findParameterByName(dagSpec.Parameters, "secretKey")
	require.NotNil(t, secretParam)
	assert.Equal(t, "secretKey", secretParam.Name)
	assert.Equal(t, "", secretParam.DefaultValue)
	assert.Equal(t, "my-secret", secretParam.DefaultFromSecret)

	// Check tasks
	require.Len(t, dagSpec.Task, 2)

	// Check setup task parameters
	setupTask := findTaskByName(dagSpec.Task, "setup")
	require.NotNil(t, setupTask)
	assert.Equal(t, []string{"environment", "replicas"}, setupTask.Parameters)
	assert.Equal(t, []string{"sh", "-c"}, setupTask.Command)
	assert.Equal(t, []string{"echo 'Setting up environment'"}, setupTask.Args)

	// Check run task parameters
	runTask := findTaskByName(dagSpec.Task, "run")
	require.NotNil(t, runTask)
	assert.Equal(t, []string{"environment", "secretKey"}, runTask.Parameters)
	assert.Equal(t, "echo 'Running with env: $ENVIRONMENT'", runTask.Script)
	assert.Equal(t, []string{"setup"}, runTask.RunAfter)
}

func TestParseDSL_ParametersOnly(t *testing.T) {
	dslInput := `parameters {
  database_url {
    defaultFromSecret "db-config"
  }
  timeout {
    default "30s"
  }
}`

	dagSpec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)
	require.NotNil(t, dagSpec)

	// Check parameters
	require.Len(t, dagSpec.Parameters, 2)

	// Check database_url parameter
	dbParam := findParameterByName(dagSpec.Parameters, "database_url")
	require.NotNil(t, dbParam)
	assert.Equal(t, "database_url", dbParam.Name)
	assert.Equal(t, "", dbParam.DefaultValue)
	assert.Equal(t, "db-config", dbParam.DefaultFromSecret)

	// Check timeout parameter
	timeoutParam := findParameterByName(dagSpec.Parameters, "timeout")
	require.NotNil(t, timeoutParam)
	assert.Equal(t, "timeout", timeoutParam.Name)
	assert.Equal(t, "30s", timeoutParam.DefaultValue)
	assert.Equal(t, "", timeoutParam.DefaultFromSecret)

	// Check no tasks or schedule
	assert.Len(t, dagSpec.Task, 0)
	assert.Equal(t, "", dagSpec.Schedule)
}

// Helper function to find a parameter by name
func findParameterByName(params []v1alpha1.DagParameterSpec, name string) *v1alpha1.DagParameterSpec {
	for _, param := range params {
		if param.Name == name {
			return &param
		}
	}
	return nil
}

// Helper function to find a task by name
func findTaskByName(tasks []v1alpha1.TaskSpec, name string) *v1alpha1.TaskSpec {
	for _, task := range tasks {
		if task.Name == name {
			return &task
		}
	}
	return nil
}
