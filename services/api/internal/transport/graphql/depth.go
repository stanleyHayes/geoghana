package graphql

import (
	"context"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// depthLimit rejects a valid but excessively nested operation before any
// resolver or repository work starts.
type depthLimit struct{ max int }

func (d depthLimit) ExtensionName() string               { return "DepthLimit" }
func (d depthLimit) Validate(gql.ExecutableSchema) error { return nil }
func (d depthLimit) MutateOperationContext(_ context.Context, op *gql.OperationContext) *gqlerror.Error {
	if selectionDepth(op.Operation.SelectionSet, map[string]bool{}) > d.max {
		return gqlerror.Errorf("operation depth exceeds the limit of %d", d.max)
	}
	return nil
}

func selectionDepth(set ast.SelectionSet, visiting map[string]bool) int {
	deepest := 0
	for _, selection := range set {
		depth := 0
		switch selected := selection.(type) {
		case *ast.Field:
			depth = 1 + selectionDepth(selected.SelectionSet, visiting)
		case *ast.InlineFragment:
			depth = selectionDepth(selected.SelectionSet, visiting)
		case *ast.FragmentSpread:
			if selected.Definition == nil || visiting[selected.Name] {
				continue
			}
			visiting[selected.Name] = true
			depth = selectionDepth(selected.Definition.SelectionSet, visiting)
			delete(visiting, selected.Name)
		}
		if depth > deepest {
			deepest = depth
		}
	}
	return deepest
}

var _ interface {
	gql.HandlerExtension
	gql.OperationContextMutator
} = depthLimit{}
