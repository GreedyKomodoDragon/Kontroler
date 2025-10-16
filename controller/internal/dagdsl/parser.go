package dagdsl

import (
	"fmt"

	"kontroler-controller/api/v1alpha1"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// DSLRoot represents the root of the DSL
type DSLRoot struct {
	Items []*DAGItem `parser:"@@*" json:"items"`
}

// DAGItem represents an item within a DAG (graph, schedule, or task)
type DAGItem struct {
	Schedule *ScheduleField `parser:"@@" json:"schedule,omitempty"`
	Graph    *GraphBlock    `parser:"| @@" json:"graph,omitempty"`
	Task     *TaskDef       `parser:"| @@" json:"task,omitempty"`
}

// ScheduleField represents a schedule definition
type ScheduleField struct {
	Schedule string `parser:"'schedule' @String" json:"schedule"`
}

// GraphBlock represents the graph definition section
type GraphBlock struct {
	Edges []*EdgeDef `parser:"'graph' '{' @@* '}'" json:"edges"`
}

// EdgeDef represents a dependency relationship between tasks
type EdgeDef struct {
	From string     `parser:"@Ident" json:"from"`
	To   *TargetSet `parser:"'->' @@" json:"to"`
}

// TargetSet represents either a single target or multiple targets in braces
type TargetSet struct {
	Single   *string   `parser:"@Ident" json:"single,omitempty"`
	Multiple *[]string `parser:"| ( '{' @Ident ( ',' @Ident )* '}' )" json:"multiple,omitempty"`
}

// TaskDef represents a task definition
type TaskDef struct {
	Name   string       `parser:"'task' @Ident '{'" json:"name"`
	Fields []*TaskField `parser:"@@* '}'" json:"fields"`
}

// TaskField represents a field within a task definition
type TaskField struct {
	Image   *string      `parser:"'image' @String" json:"image,omitempty"`
	Command *StringArray `parser:"| 'command' @@" json:"command,omitempty"`
	Args    *StringArray `parser:"| 'args' @@" json:"args,omitempty"`
	Script  *string      `parser:"| 'script' ( @String | @MultilineString )" json:"script,omitempty"`
}

// StringArray represents an array of strings in the DSL
type StringArray struct {
	Values []string `parser:"'[' @String ( ',' @String )* ']'" json:"values"`
}

var (
	// Define the lexer with proper token definitions
	dslLexer = lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Comment", Pattern: `//[^\n]*`},
		{Name: "Whitespace", Pattern: `\s+`},
		{Name: "MultilineString", Pattern: `"""[\s\S]*?"""`},
		{Name: "String", Pattern: `"[^"]*"`},
		{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_-]*`},
		{Name: "Arrow", Pattern: `->`},
		{Name: "LBrace", Pattern: `\{`},
		{Name: "RBrace", Pattern: `\}`},
		{Name: "LBracket", Pattern: `\[`},
		{Name: "RBracket", Pattern: `\]`},
		{Name: "Comma", Pattern: `,`},
	})

	// Create the parser
	parser = participle.MustBuild[DSLRoot](
		participle.Lexer(dslLexer),
		participle.Elide("Comment", "Whitespace"),
		participle.UseLookahead(2),
	)
)

// ParseDSL parses the DSL input and returns a populated DAGSpec
func ParseDSL(input string) (*v1alpha1.DAGSpec, error) {
	// Parse the DSL
	root, err := parser.ParseString("", input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSL: %w", err)
	}

	// Convert to DAGSpec
	dagSpec, err := convertToDAGSpec(root)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to DAGSpec: %w", err)
	}

	return dagSpec, nil
}

// convertToDAGSpec converts the parsed DSL structure to a v1alpha1.DAGSpec
func convertToDAGSpec(root *DSLRoot) (*v1alpha1.DAGSpec, error) {
	spec := &v1alpha1.DAGSpec{
		Task: []v1alpha1.TaskSpec{},
	}

	// Extract schedule, graph and task definitions
	var schedule string
	var graphBlock *GraphBlock
	taskDefs := []*TaskDef{}

	for _, item := range root.Items {
		if item.Schedule != nil {
			schedule = cleanString(item.Schedule.Schedule)
		} else if item.Graph != nil {
			graphBlock = item.Graph
		} else if item.Task != nil {
			taskDefs = append(taskDefs, item.Task)
		}
	}

	// Set schedule if provided
	if schedule != "" {
		spec.Schedule = schedule
	}

	// Build dependency map from graph
	dependencies := buildDependencyMap(graphBlock)

	// Convert tasks
	for _, task := range taskDefs {
		taskSpec := createTaskSpec(task, dependencies)
		spec.Task = append(spec.Task, taskSpec)
	}

	return spec, nil
}

// buildDependencyMap creates a map of task dependencies from the graph
func buildDependencyMap(graph *GraphBlock) map[string][]string {
	dependencies := make(map[string][]string)
	if graph == nil {
		return dependencies
	}

	for _, edge := range graph.Edges {
		targets := getTargets(edge.To)
		for _, target := range targets {
			dependencies[target] = append(dependencies[target], edge.From)
		}
	}

	return dependencies
}

// getTargets extracts target task names from a TargetSet
func getTargets(targetSet *TargetSet) []string {
	if targetSet.Single != nil {
		return []string{*targetSet.Single}
	}
	if targetSet.Multiple != nil {
		return *targetSet.Multiple
	}
	return []string{}
}

// createTaskSpec creates a TaskSpec from a TaskDef
func createTaskSpec(task *TaskDef, dependencies map[string][]string) v1alpha1.TaskSpec {
	taskSpec := v1alpha1.TaskSpec{
		Name: task.Name,
	}

	// Process task fields
	for _, field := range task.Fields {
		if field.Image != nil {
			taskSpec.Image = cleanString(*field.Image)
		} else if field.Command != nil {
			taskSpec.Command = cleanStringArray(field.Command.Values)
		} else if field.Args != nil {
			taskSpec.Args = cleanStringArray(field.Args.Values)
		} else if field.Script != nil {
			taskSpec.Script = cleanScriptString(*field.Script)
		}
	}

	// Set dependencies from graph
	if deps, exists := dependencies[task.Name]; exists {
		taskSpec.RunAfter = deps
	}

	return taskSpec
}

// cleanString removes surrounding quotes from a string
func cleanString(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// cleanScriptString handles both single-line and multi-line script strings
func cleanScriptString(s string) string {
	// Check if it's a multi-line string (triple quotes)
	if len(s) >= 6 && s[:3] == `"""` && s[len(s)-3:] == `"""` {
		return s[3 : len(s)-3]
	}
	// Otherwise treat as single-line string (double quotes)
	return cleanString(s)
}

// cleanStringArray removes quotes from each string in the array
func cleanStringArray(arr []string) []string {
	result := make([]string, len(arr))
	for i, s := range arr {
		result[i] = cleanString(s)
	}
	return result
}
