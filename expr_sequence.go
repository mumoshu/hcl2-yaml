package hclv2yaml

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

type SequenceExpression struct {
	f    *yamlBody
	Node *yaml.Node
}

func (e SequenceExpression) Value(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	exprs, diags := e.parseExprs(e.Node)
	if diags.HasErrors() {
		panic(diags)
	}

	vals := []cty.Value{}

	for _, expr := range exprs {
		val, diags := expr.Value(ctx)
		if diags.HasErrors() {
			return cty.DynamicVal, diags
		}

		vals = append(vals, val)
	}

	return cty.ListVal(vals), nil
}

func (e SequenceExpression) parseExprs(v *yaml.Node) ([]hcl.Expression, hcl.Diagnostics) {
	var exprs []hcl.Expression

	for _, v := range v.Content {
		switch v.Kind {
		case yaml.MappingNode:
			exprs = append(exprs, &MappingExpression{f: e.f, Node: v})
		case yaml.ScalarNode:
			expr, diags := e.f.ParseScalar(v)
			if diags.HasErrors() {
				return nil, diags
			}

			exprs = append(exprs, expr)
		case yaml.SequenceNode:
			exprs = append(exprs, &SequenceExpression{f: e.f, Node: v})
		default:
			panic(fmt.Errorf("unsupported yaml kind: %v: %v", v.Kind, v))
		}
	}

	return exprs, nil
}

func (e SequenceExpression) Variables() []hcl.Traversal {
	exprs, diags := e.parseExprs(e.Node)
	if diags.HasErrors() {
		panic(diags)
	}

	var vars []hcl.Traversal

	for _, expr := range exprs {
		vars = append(vars, expr.Variables()...)
	}

	return vars
}

func (e SequenceExpression) Range() hcl.Range {
	n := e.Node

	lastElem := n.Content[len(n.Content)-1]

	return hcl.Range{
		Filename: e.f.fileName,
		Start: hcl.Pos{
			Column: n.Column,
			Line:   n.Line,
		},
		End: hcl.Pos{
			Column: n.Column + len(lastElem.Value),
			Line:   n.Line,
		},
	}
}

func (e SequenceExpression) StartRange() hcl.Range {
	return e.Range()
}

var _ hcl.Expression = &SequenceExpression{}
