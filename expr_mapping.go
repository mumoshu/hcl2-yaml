package hcl2yaml

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

type MappingExpression struct {
	f *yamlBody
	Node     *yaml.Node
}

func (e MappingExpression) Value(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	exprs, diags := e.parseExprs(e.Node)
	if diags.HasErrors() {
		panic(diags)
	}

	vals := map[string]cty.Value{}

	for k, expr := range exprs {
		val, diags := expr.Value(ctx)
		if diags.HasErrors() {
			return cty.DynamicVal, diags
		}

		vals[k] = val
	}

	return cty.MapVal(vals), nil
}

func (e MappingExpression) parseExprs(v *yaml.Node) (map[string]hcl.Expression, hcl.Diagnostics) {
	m := mappingKVs(v)

	exprs := map[string]hcl.Expression{}

	for k, v := range m {
		switch v.Kind {
		case yaml.MappingNode:
			exprs[k] = &MappingExpression{f: e.f, Node: v}
		case yaml.ScalarNode:
			expr, diags := e.f.ParseScalar(v)
			if diags.HasErrors() {
				return nil, diags
			}

			exprs[k] = expr
		case yaml.SequenceNode:
		default:
			panic(fmt.Errorf("unsupported yaml kind: %v: %v", v.Kind, v))
		}
	}

	return exprs, nil
}

func (e MappingExpression) Variables() []hcl.Traversal {
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

func (e MappingExpression) Range() hcl.Range {
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

func (e MappingExpression) StartRange() hcl.Range {
	return e.Range()
}

var _ hcl.Expression = &MappingExpression{}
