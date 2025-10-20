package dagdsl

import (
	"fmt"
	"strings"

	"kontroler-controller/api/v1alpha1"
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Message string
	Field   string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// ValidationResult contains the results of validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ValidateDAGSpec validates a parsed DAG specification
func ValidateDAGSpec(spec *v1alpha1.DAGSpec) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Check if graph is provided
	if !hasGraph(spec) {
		result.addError("graph", "no graph definition provided - DAG must include a graph block with task dependencies")
	} else {
		// If graph exists, validate that all referenced tasks are defined
		if err := validateGraphTaskReferences(spec); err != nil {
			result.addError("graph", err.Error())
		}
	}

	// Validate parameters if they exist
	if err := validateParameters(spec); err != nil {
		result.addError("parameters", err.Error())
	}

	// Validate parameter references in tasks
	if err := validateTaskParameterReferences(spec); err != nil {
		result.addError("parameters", err.Error())
	}

	return result
}

// addError adds an error to the validation result
func (r *ValidationResult) addError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// hasGraph checks if the DAG specification includes task dependencies
func hasGraph(spec *v1alpha1.DAGSpec) bool {
	// A graph exists if any task has RunAfter dependencies
	for _, task := range spec.Task {
		if len(task.RunAfter) > 0 {
			return true
		}
	}
	return false
}

// validateGraphTaskReferences validates that all tasks referenced in the graph are actually defined
func validateGraphTaskReferences(spec *v1alpha1.DAGSpec) error {
	// Create a set of defined task names
	definedTasks := make(map[string]bool)
	for _, task := range spec.Task {
		definedTasks[task.Name] = true
	}

	// Collect all referenced task names from RunAfter dependencies
	referencedTasks := make(map[string]bool)
	var missingTasks []string

	for _, task := range spec.Task {
		for _, dependency := range task.RunAfter {
			referencedTasks[dependency] = true
			if !definedTasks[dependency] {
				missingTasks = append(missingTasks, dependency)
			}
		}
	}

	if len(missingTasks) > 0 {
		return fmt.Errorf("tasks referenced in graph but not defined: %s",
			strings.Join(missingTasks, ", "))
	}

	return nil
}

// ValidateAndParseDetails provides detailed validation information
func ValidateAndParseDetails(spec *v1alpha1.DAGSpec) (ValidationResult, map[string]interface{}) {
	result := ValidateDAGSpec(spec)

	details := map[string]interface{}{
		"taskCount":    len(spec.Task),
		"hasSchedule":  spec.Schedule != "",
		"schedule":     spec.Schedule,
		"taskNames":    getTaskNames(spec),
		"dependencies": getDependencyMap(spec),
		"rootTasks":    getRootTasks(spec),
		"leafTasks":    getLeafTasks(spec),
	}

	return result, details
}

// getTaskNames returns a list of all task names
func getTaskNames(spec *v1alpha1.DAGSpec) []string {
	names := make([]string, len(spec.Task))
	for i, task := range spec.Task {
		names[i] = task.Name
	}
	return names
}

// getDependencyMap returns a map of task dependencies
func getDependencyMap(spec *v1alpha1.DAGSpec) map[string][]string {
	deps := make(map[string][]string)
	for _, task := range spec.Task {
		if len(task.RunAfter) > 0 {
			deps[task.Name] = task.RunAfter
		}
	}
	return deps
}

// getRootTasks returns tasks that have no dependencies
func getRootTasks(spec *v1alpha1.DAGSpec) []string {
	var roots []string
	for _, task := range spec.Task {
		if len(task.RunAfter) == 0 {
			roots = append(roots, task.Name)
		}
	}
	return roots
}

// getLeafTasks returns tasks that are not dependencies of other tasks
func getLeafTasks(spec *v1alpha1.DAGSpec) []string {
	// Create a set of all tasks that are dependencies
	isDependency := make(map[string]bool)
	for _, task := range spec.Task {
		for _, dep := range task.RunAfter {
			isDependency[dep] = true
		}
	}

	// Find tasks that are not dependencies of others
	var leaves []string
	for _, task := range spec.Task {
		if !isDependency[task.Name] {
			leaves = append(leaves, task.Name)
		}
	}
	return leaves
}

// validateParameters validates parameter definitions
func validateParameters(spec *v1alpha1.DAGSpec) error {
	for _, param := range spec.Parameters {
		// Check that parameter has a name
		if param.Name == "" {
			return fmt.Errorf("parameter missing name")
		}

		// Check that parameter has either default value or defaultFromSecret, but not both
		hasDefault := param.DefaultValue != ""
		hasSecret := param.DefaultFromSecret != ""

		if hasDefault && hasSecret {
			return fmt.Errorf("parameter '%s' cannot have both default value and defaultFromSecret", param.Name)
		}

		if !hasDefault && !hasSecret {
			return fmt.Errorf("parameter '%s' must have either default value or defaultFromSecret", param.Name)
		}
	}
	return nil
}

// validateTaskParameterReferences validates that task parameter references are defined
func validateTaskParameterReferences(spec *v1alpha1.DAGSpec) error {
	// Create a set of defined parameter names
	definedParams := make(map[string]bool)
	for _, param := range spec.Parameters {
		definedParams[param.Name] = true
	}

	// Check all task parameter references
	var missingParams []string
	for _, task := range spec.Task {
		for _, paramRef := range task.Parameters {
			if !definedParams[paramRef] {
				missingParams = append(missingParams, fmt.Sprintf("'%s' in task '%s'", paramRef, task.Name))
			}
		}
	}

	if len(missingParams) > 0 {
		return fmt.Errorf("parameters referenced in tasks but not defined: %s", strings.Join(missingParams, ", "))
	}

	return nil
}
