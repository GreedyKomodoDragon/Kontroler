package dagdsl_test

import (
	"testing"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/dagdsl"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDAGSpec_ValidWithGraph(t *testing.T) {
	spec := &v1alpha1.DAGSpec{
		Task: []v1alpha1.TaskSpec{
			{
				Name:     "a",
				Image:    "alpine",
				RunAfter: []string{},
			},
			{
				Name:     "b",
				Image:    "alpine",
				RunAfter: []string{"a"},
			},
		},
	}

	result := dagdsl.ValidateDAGSpec(spec)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateDAGSpec_NoGraph(t *testing.T) {
	spec := &v1alpha1.DAGSpec{
		Task: []v1alpha1.TaskSpec{
			{
				Name:  "a",
				Image: "alpine",
			},
			{
				Name:  "b",
				Image: "alpine",
			},
		},
	}

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "no graph definition provided")
	assert.Equal(t, "graph", result.Errors[0].Field)
}

func TestValidateDAGSpec_MissingTaskInGraph(t *testing.T) {
	spec := &v1alpha1.DAGSpec{
		Task: []v1alpha1.TaskSpec{
			{
				Name:     "a",
				Image:    "alpine",
				RunAfter: []string{},
			},
			{
				Name:     "b",
				Image:    "alpine",
				RunAfter: []string{"a", "missing_task"},
			},
		},
	}

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "missing_task")
	assert.Contains(t, result.Errors[0].Message, "tasks referenced in graph but not defined")
	assert.Equal(t, "graph", result.Errors[0].Field)
}

func TestValidateDAGSpec_MultipleMissingTasks(t *testing.T) {
	spec := &v1alpha1.DAGSpec{
		Task: []v1alpha1.TaskSpec{
			{
				Name:     "a",
				Image:    "alpine",
				RunAfter: []string{"missing1", "missing2"},
			},
		},
	}

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "missing1")
	assert.Contains(t, result.Errors[0].Message, "missing2")
}

func TestValidateAndParseDetails(t *testing.T) {
	spec := &v1alpha1.DAGSpec{
		Schedule: "0 0 * * *",
		Task: []v1alpha1.TaskSpec{
			{
				Name:     "a",
				Image:    "alpine",
				RunAfter: []string{},
			},
			{
				Name:     "b",
				Image:    "alpine",
				RunAfter: []string{"a"},
			},
			{
				Name:     "c",
				Image:    "alpine",
				RunAfter: []string{"a"},
			},
			{
				Name:     "d",
				Image:    "alpine",
				RunAfter: []string{"b", "c"},
			},
		},
	}

	result, details := dagdsl.ValidateAndParseDetails(spec)

	// Validation should pass
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)

	// Check details
	assert.Equal(t, 4, details["taskCount"])
	assert.Equal(t, true, details["hasSchedule"])
	assert.Equal(t, "0 0 * * *", details["schedule"])

	taskNames := details["taskNames"].([]string)
	assert.Contains(t, taskNames, "a")
	assert.Contains(t, taskNames, "b")
	assert.Contains(t, taskNames, "c")
	assert.Contains(t, taskNames, "d")

	deps := details["dependencies"].(map[string][]string)
	assert.Equal(t, []string{"a"}, deps["b"])
	assert.Equal(t, []string{"a"}, deps["c"])
	assert.Equal(t, []string{"b", "c"}, deps["d"])
	assert.NotContains(t, deps, "a") // a has no dependencies

	rootTasks := details["rootTasks"].([]string)
	assert.Equal(t, []string{"a"}, rootTasks)

	leafTasks := details["leafTasks"].([]string)
	assert.Equal(t, []string{"d"}, leafTasks)
}

func TestValidateDAGSpec_EmptySpec(t *testing.T) {
	spec := &v1alpha1.DAGSpec{
		Task: []v1alpha1.TaskSpec{},
	}

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "no graph definition provided")
}

func TestValidateDAGSpec_ComplexValidGraph(t *testing.T) {
	dslInput := `graph {
  a -> b
  a -> c
  b -> d
  c -> d
}

task a {
  image "alpine:latest"
  command ["echo", "a"]
}

task b {
  image "alpine:latest"
  command ["echo", "b"]
}

task c {
  image "alpine:latest"
  command ["echo", "c"]
}

task d {
  image "alpine:latest"
  command ["echo", "d"]
}`

	spec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateDAGSpec_ComplexInvalidGraph(t *testing.T) {
	dslInput := `graph {
  a -> b
  a -> c
  missing_task -> d
  c -> d
}

task a {
  image "alpine:latest"
  command ["echo", "a"]
}

task b {
  image "alpine:latest"
  command ["echo", "b"]
}

task c {
  image "alpine:latest"
  command ["echo", "c"]
}

task d {
  image "alpine:latest"
  command ["echo", "d"]
}`

	spec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "missing_task")
}

func TestValidateDAGSpec_ValidWithParameters(t *testing.T) {
	dslInput := `parameters {
  environment {
    default "dev"
  }
  secretKey {
    defaultFromSecret "my-secret"
  }
}

graph {
  a -> b
}

task a {
  image "alpine"
  parameters ["environment", "secretKey"]
}

task b {
  image "alpine"
}`

	spec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateDAGSpec_InvalidParameterBothDefaultAndSecret(t *testing.T) {
	dslInput := `parameters {
  badParam {
    default "value"
    defaultFromSecret "secret"
  }
}

graph {
  a -> b
}

task a {
  image "alpine"
}

task b {
  image "alpine"
}`

	spec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "cannot have both default value and defaultFromSecret")
}

func TestValidateDAGSpec_InvalidParameterNeitherDefaultNorSecret(t *testing.T) {
	dslInput := `parameters {
  badParam {
  }
}

graph {
  a -> b
}

task a {
  image "alpine"
}

task b {
  image "alpine"
}`

	spec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "must have either default value or defaultFromSecret")
}

func TestValidateDAGSpec_UndefinedParameterReference(t *testing.T) {
	dslInput := `parameters {
  environment {
    default "dev"
  }
}

graph {
  a -> b
}

task a {
  image "alpine"
  parameters ["environment", "undefinedParam"]
}

task b {
  image "alpine"
}`

	spec, err := dagdsl.ParseDSL(dslInput)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "parameters referenced in tasks but not defined")
	assert.Contains(t, result.Errors[0].Message, "undefinedParam")
}
