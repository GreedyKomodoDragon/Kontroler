package dsl_test

import (
	"testing"

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
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(dag.Task) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(dag.Task))
	}
}

func TestParse_UndefinedTaskReference(t *testing.T) {
	src := `graph { a -> b }\ntask a { image "alpine" script "x" }`
	_, err := dagdsl.ParseDSL(src)
	if err == nil {
		t.Fatalf("expected error for undefined task 'b', got nil")
	}
}

func TestParse_CircularDependency(t *testing.T) {
	src := `graph { a -> b b -> c c -> a }

task a { image "alpine" script "a" }
 task b { image "alpine" script "b" }
 task c { image "alpine" script "c" }`

	_, err := dagdsl.ParseDSL(src)
	// The parser/validator may return either a parse error or a validation error; ensure we get an error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}
