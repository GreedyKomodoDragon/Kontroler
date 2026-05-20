package dsl_test

import (
	"strings"
	"testing"

	v1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/dagdsl"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SimpleGraph(t *testing.T) {
	src := `
graph {
  a -> b
  a -> c
}

task a { image "alpine:latest" script "echo a" }
task b { image "alpine:latest" script "echo b" }
task c { image "alpine:latest" script "echo c" }
`

	dag, err := dagdsl.ParseDSL(src)
	require.NoError(t, err)
	require.NotNil(t, dag)

	// Validate using the package validator to ensure graph references are correct
	result := dagdsl.ValidateDAGSpec(dag)
	assert.True(t, result.Valid)
}

func TestParse_UndefinedTaskReference(t *testing.T) {
	src := `
graph {
  a -> b
}

task a { image "alpine" script "x" }
`
	spec, err := dagdsl.ParseDSL(src)
	require.NoError(t, err)

	result := dagdsl.ValidateDAGSpec(spec)
	assert.False(t, result.Valid)
	require.Greater(t, len(result.Errors), 0)
	// Accept either a "no graph" message or a missing task message depending on implementation details
	msg := result.Errors[0].Message
	// Ensure the specific missing task is mentioned
	require.Contains(t, msg, "b")
	// Accept a range of possible phrasings from the validator
	keywords := []string{"undefined", "missing", "not found", "not defined"}
	found := false
	for _, k := range keywords {
		if strings.Contains(msg, k) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error message to mention that task 'b' is undefined/missing; got: %s", msg)
}

func TestParse_CircularDependency(t *testing.T) {
	src := `graph { a -> b b -> c c -> a }

task a { image "alpine" script "a" }
 task b { image "alpine" script "b" }
 task c { image "alpine" script "c" }`

	spec, err := dagdsl.ParseDSL(src)
	require.NoError(t, err)

	// Cycle detection lives on the DAG type validation (API-level checks)
	dag := &v1alpha1.DAG{Spec: *spec}
	err = dag.ValidateDAG(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cyclic")
}
