package main

import (
	"fmt"
	"strings"
)

// recurse through type tree to get all layers of nested op
// returns rendered result, max depth.
func buildLayersRecurse(builder *strings.Builder, parentFieldPath []FieldPathDetail, depth int) int {

	if len(parentFieldPath) == 0 {
		return depth
	}

	if len(parentFieldPath) > 0 {
		builder.WriteString(strings.Repeat(" ", depth*4))
		builder.WriteString(parentFieldPath[0].Path)
		if parentFieldPath[0].IDInPath {
			builder.WriteString("(id: $")
			builder.WriteString(parentFieldPath[0].Path)
			builder.WriteString("ID)")
		}
		builder.WriteString(" {\n")
	}

	if len(parentFieldPath) > 1 {
		return buildLayersRecurse(builder, parentFieldPath[1:], depth+1)
	}

	return depth + 1
}

// BuildQuery - dynamically create GQL query, return (query document, query name)
func BuildQuery(method *GetMethod, variables *map[string]interface{}) (string, string) {

	// TODO - update this for nested operations
	builder := strings.Builder{}

	opName := strings.Title(method.OriginalField)

	nestDepth := 0
	// Build op name
	if (variables != nil) && (len(*variables) > 0) {
		builder.WriteString(fmt.Sprintf("query %s(", opName))

		inputs := make([]string, 0, len(method.QueryString))
		for k, v := range method.QueryString {
			inputs = append(inputs, fmt.Sprintf("$%s: %s", k, v.Type))
		}
		builder.WriteString(strings.Join(inputs, ", "))
		inputs = inputs[:0] // clear slice

		builder.WriteString(") {\n")
		nestDepth = buildLayersRecurse(&builder, method.FieldPath, 1)
		builder.WriteString(strings.Repeat("    ", nestDepth+1))
		builder.WriteString(method.OriginalField)
		builder.WriteString("(")

		for k := range method.QueryString {
			inputs = append(inputs, fmt.Sprintf("%s: $%s", k, k))
		}
		builder.WriteString(strings.Join(inputs, ", "))
		builder.WriteString(")")
	} else {
		builder.WriteString(fmt.Sprintf("query %s {\n", opName))
		nestDepth = buildLayersRecurse(&builder, method.FieldPath, 1)
		builder.WriteString(strings.Repeat("    ", nestDepth+1))
		builder.WriteString(method.OriginalField)

	}

	// render entire selection set
	// TODO - add include/exclude
	if len(method.ResultSelections) > 0 {

		builder.WriteString(" {\n")
		for _, sel := range method.ResultSelections {
			builder.WriteString(strings.Repeat("    ", nestDepth+2))
			builder.WriteString(sel)
			builder.WriteString("\n")
		}
		builder.WriteString(strings.Repeat("    ", nestDepth+1))
		builder.WriteString("}\n")
	} else {
		builder.WriteString("\n")
	}
	if nestDepth > 1 {
		builder.WriteString(strings.Repeat("    ", nestDepth))
		builder.WriteString(strings.Repeat("}", nestDepth-1))
	}

	// Query end
	builder.WriteString("\n}\n")

	return builder.String(), opName
}
