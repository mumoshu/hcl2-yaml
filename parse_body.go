package hcl2yaml

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
	"reflect"
	"strconv"
	"strings"
)

type YamlBody struct {
	fileName string

	bytes []byte

	yamlNode *yaml.Node
}

type yamlBody struct {
	fileName     string
	bytes        []byte
	yamlNode     *yaml.Node
	attrSchemas  map[string]hcl.AttributeSchema
	blockSchemas map[string]hcl.BlockHeaderSchema
}

func (f *YamlBody) Content(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	attrSchemas := map[string]hcl.AttributeSchema{}

	for _, a := range schema.Attributes {
		attrSchemas[a.Name] = a
	}

	blockSchemas := map[string]hcl.BlockHeaderSchema{}

	for _, b := range schema.Blocks {
		blockSchemas[b.Type] = b
	}

	ff := &yamlBody{
		bytes:        f.bytes,
		yamlNode:     f.yamlNode,
		fileName:     f.fileName,
		attrSchemas:  attrSchemas,
		blockSchemas: blockSchemas,
	}

	return ff.content()
}

func (f *yamlBody) content() (*hcl.BodyContent, hcl.Diagnostics) {
	value := f.yamlNode

	// 4
	if value.Kind == yaml.MappingNode {
		return f.parseMapping(value)
	}

	// 1
	if value.Kind == yaml.DocumentNode {
		return f.parseMapping((value.Content[0]))
	}

	err := fmt.Errorf("unexpected yaml node kind: expected DocumentNode(1) or MappingNode(4), got %v", value.Kind)

	return nil, hcl.Diagnostics{
		&hcl.Diagnostic{
			Severity:    hcl.DiagError,
			Summary:     err.Error(),
			Detail:      "",
			Subject:     nil,
			Context:     nil,
			Expression:  nil,
			EvalContext: nil,
		},
	}
}

func (f *YamlBody) PartialContent(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Body, hcl.Diagnostics) {
	panic("implement me")
}

func (f *YamlBody) JustAttributes() (hcl.Attributes, hcl.Diagnostics) {
	panic("implement me")
}

func (f *YamlBody) MissingItemRange() hcl.Range {
	panic("implement me")
}

var _ hcl.Body = &YamlBody{}

func (f *yamlBody) parseMapping(node *yaml.Node) (*hcl.BodyContent, hcl.Diagnostics) {
	var bodyContent hcl.BodyContent

	keyToValue := map[string]*yaml.Node{}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode {
			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Unexpected key kind. Expected ScalarNode(8), got %v", keyNode.Kind),
					Detail:   "",
					Subject: hcl.Range{
						Filename: f.fileName,
						Start: hcl.Pos{
							Line:   keyNode.Line,
							Column: keyNode.Column,
						},
						End: hcl.Pos{
							Line:   keyNode.Line,
							Column: keyNode.Column + len(keyNode.Value),
						},
					}.Ptr(),
					Context: hcl.Range{
						Filename: f.fileName,
						Start: hcl.Pos{
							Line:   keyNode.Line - 1,
							Column: 0,
						},
						End: hcl.Pos{
							Line:   keyNode.Line + 1,
							Column: 0,
						},
					}.Ptr(),
					Expression:  nil,
					EvalContext: nil,
				},
			}
		}

		k := keyNode.Value

		vs := valueNode

		keyToValue[k] = vs
	}

	attrs := map[string]*hcl.Attribute{}

	for k, attrSchema := range f.attrSchemas {
		c, exists := keyToValue[k]
		if attrSchema.Required && !exists {
			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("no yaml mapping found for required attribute %q", k),
				},
			}
		}

		attr, diags := f.parseAttrsFromYaml(k, c)
		if diags.HasErrors() {
			return nil, diags
		}

		attrs[k] = attr
	}

	var blocks []*hcl.Block

	for k, blockSchema := range f.blockSchemas {
		c, exists := keyToValue[k]
		if !exists {
			// TODO Pluralization support
			c, exists = keyToValue[k]
			if !exists {
				return nil, hcl.Diagnostics{
					&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("no yaml mapping found for expected block %q", k),
					},
				}
			}
		}

		switch c.Kind {
		case yaml.SequenceNode:
			bls, diags := f.parseBlocksFromYamlSequence(k, blockSchema, c)
			if diags.HasErrors() {
				return nil, diags
			}

			blocks = append(blocks, bls...)
		case yaml.MappingNode:
			bl, diags := f.parseBlockFromYamlMapping(k, blockSchema, c)
			if diags.HasErrors() {
				return nil, diags
			}

			blocks = append(blocks, bl)
		default:
			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity:    hcl.DiagError,
					Summary:     fmt.Sprintf("unsupported type of yaml node: %v", c.Kind),
					Detail:      "",
					Subject:     nil,
					Context:     nil,
					Expression:  nil,
					EvalContext: nil,
				},
			}
		}
	}

	bodyContent.Attributes = attrs
	bodyContent.Blocks = blocks

	return &bodyContent, nil
}

func (f *yamlBody) parseAttrsFromYaml(name string, valNode *yaml.Node) (*hcl.Attribute, hcl.Diagnostics) {
	switch valNode.Kind {
	case yaml.MappingNode:
		var expr hcl.Expression

		expr = &MappingExpression{f: f, Node: valNode}

		attr := &hcl.Attribute{
			Name:      name,
			Expr:      expr,
			Range:     expr.Range(),
			NameRange: hcl.Range{},
		}

		return attr, nil
	case yaml.SequenceNode:
		var expr hcl.Expression

		expr = &SequenceExpression{f: f, Node: valNode}

		attr := &hcl.Attribute{
			Name:      name,
			Expr:      expr,
			Range:     expr.Range(),
			NameRange: hcl.Range{},
		}

		return attr, nil
		//
		//var vs []cty.Value
		//
		//for _, v := range valNode.Content {
		//	switch v.Kind {
		//	case yaml.ScalarNode:
		//		vs = append(vs, cty.StringVal(v.Value))
		//	}
		//}
		//
		//lastElem := valNode.Content[len(valNode.Content)-1]
		//
		//rng := hcl.Range{
		//	Filename: f.fileName,
		//	Start: hcl.Pos{
		//		Column: valNode.Column,
		//		Line:   valNode.Line,
		//	},
		//	End: hcl.Pos{
		//		Column: lastElem.Column + len(lastElem.Value),
		//		Line:   lastElem.Line,
		//	},
		//}
		//
		//var expr hcl.Expression
		//
		//if len(vs) > 0 {
		//	expr = hcl.StaticExpr(cty.ListVal(vs), rng)
		//} else {
		//	expr = hcl.StaticExpr(cty.DynamicVal, rng)
		//}
		//
		//attr := &hcl.Attribute{
		//	Name:      name,
		//	Expr:      expr,
		//	Range:     rng,
		//	NameRange: hcl.Range{},
		//}
		//
		//return attr, nil
	case yaml.ScalarNode:
		expr, diags := f.ParseScalar(valNode)
		if diags.HasErrors() {
			return nil, diags
		}

		attr := &hcl.Attribute{
			Name:      name,
			Expr:      expr,
			Range:     expr.Range(),
			NameRange: hcl.Range{},
		}

		return attr, nil
	}

	return nil, hcl.Diagnostics{
		&hcl.Diagnostic{
			Severity:    hcl.DiagError,
			Summary:     fmt.Sprintf("unable to parse attribute of unsupported kind/tag %q: %v %s", name, valNode.Kind, valNode.Tag),
			Detail:      "",
			Subject:     nil,
			Context:     nil,
			Expression:  nil,
			EvalContext: nil,
		},
	}
}

func (f *yamlBody) parseBlocksFromYamlSequence(tpe string, blockSchema hcl.BlockHeaderSchema, valNode *yaml.Node) ([]*hcl.Block, hcl.Diagnostics) {
	if valNode.Kind != yaml.SequenceNode {
		return nil, hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     fmt.Sprintf("unsupported type of node for blocks %q. It must be SequenceNode, but got ", valNode.Kind),
				Detail:      "",
				Subject:     nil,
				Context:     nil,
				Expression:  nil,
				EvalContext: nil,
			},
		}
	}

	var bls []*hcl.Block

	for _, n := range valNode.Content {
		switch n.Kind {
		case yaml.MappingNode:

			bl, diags := f.parseBlockFromYamlMapping(tpe, blockSchema, n)
			if diags.HasErrors() {
				return nil, diags
			}

			bls = append(bls, bl)

		default:
			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity:    hcl.DiagError,
					Summary:     fmt.Sprintf("unsupported type of value node for blocks %q. It must be MappingNode", n.Kind),
					Detail:      "",
					Subject:     nil,
					Context:     nil,
					Expression:  nil,
					EvalContext: nil,
				},
			}
		}
	}

	return bls, nil
}

func (f *yamlBody) parseBlockFromYamlMapping(tpe string, blockSchema hcl.BlockHeaderSchema, valNode *yaml.Node) (*hcl.Block, hcl.Diagnostics) {
	var block hcl.Block

	block.Type = tpe

	m := mappingKVs(valNode)

	for _, label := range blockSchema.LabelNames {
		labelVal, exists := m[label]
		if !exists {
			var ks []string
			for k := range m {
				ks = append(ks, k)
			}

			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Value for label %q not found in %v: %v", label, strings.Join(ks, ", "), valNode),
					Detail:   "",
					Subject: hcl.Range{
						Filename: f.fileName,
						Start: hcl.Pos{
							Line:   valNode.Line,
							Column: valNode.Column,
						},
						End: hcl.Pos{
							Line:   valNode.Line,
							Column: valNode.Column,
						},
					}.Ptr(),
					Context: hcl.Range{
						Filename: f.fileName,
						Start: hcl.Pos{
							Line:   valNode.Line - 1,
							Column: 0,
						},
						End: hcl.Pos{
							Line:   valNode.Line + 1,
							Column: 0,
						},
					}.Ptr(),
					Expression:  nil,
					EvalContext: nil,
				},
			}
		}

		block.Labels = append(block.Labels, labelVal.Value)
	}

	ff := &YamlBody{
		bytes: f.bytes,
		fileName: f.fileName,
		yamlNode: valNode,
	}

	block.Body = ff

	return &block, nil
}

func (f *yamlBody) ParseScalar(valNode *yaml.Node) (hcl.Expression, hcl.Diagnostics) {
	switch valNode.Tag {
	case "!!exp":
		return f.ParseExpression(valNode)
	case "!!str":
		return f.ParseTemplate(valNode)
	case "!!int":
		v := valNode.Value

		rng := hcl.Range{
			Filename: f.fileName,
			Start: hcl.Pos{
				Column: valNode.Column,
				Line:   valNode.Line,
			},
			End: hcl.Pos{
				Column: valNode.Column + len(v),
				Line:   valNode.Line,
			},
		}

		intval, err := strconv.Atoi(v)
		if err != nil {
			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity:    hcl.DiagError,
					Summary:     err.Error(),
					Detail:      "",
					Subject:     &rng,
					Context:     nil,
					Expression:  nil,
					EvalContext: nil,
				},
			}
		}

		return hcl.StaticExpr(cty.NumberIntVal(int64(intval)), rng), nil
	}

	rng := hcl.Range{
		Filename: f.fileName,
		Start: hcl.Pos{
			Column: valNode.Column,
			Line:   valNode.Line,
		},
		End: hcl.Pos{
			Column: valNode.Column + len(valNode.Value),
			Line:   valNode.Line,
		},
	}

	return nil, hcl.Diagnostics{
		&hcl.Diagnostic{
			Severity:    hcl.DiagError,
			Summary:     fmt.Sprintf("unable to parse yaml node of unsupported tag: %v", valNode.Tag),
			Detail:      "",
			Subject:     &rng,
			Context:     nil,
			Expression:  nil,
			EvalContext: nil,
		},
	}
}

func (f *yamlBody) ParseExpression(valNode *yaml.Node) (hcl.Expression, hcl.Diagnostics) {
	//start := hcl.Pos{
	//	Column: valNode.Column + len(valNode.Tag),
	//	Line:   valNode.Line,
	//}
	//
	//return hclsyntax.ParseExpression(f.bytes, f.fileName, start)

	return hclsyntax.ParseExpression([]byte(valNode.Value), "", hcl.Pos{})
}

func (f *yamlBody) ParseTemplate(valNode *yaml.Node) (hcl.Expression, hcl.Diagnostics) {
	//start := hcl.Pos{
	//	Column: valNode.Column,
	//	Line:   valNode.Line,
	//}

	//return hclsyntax.ParseTemplate(f.bytes, f.fileName, start)

	return hclsyntax.ParseTemplate([]byte(valNode.Value), "", hcl.Pos{})
}

func parseBlocksIntoMap(ctx *hcl.EvalContext, bodyContent *hcl.BodyContent, blockToMapSchema map[string]Block, dest map[string]interface{}) hcl.Diagnostics {
	blocksByType := bodyContent.Blocks.ByType()

	for tpe, blockSchema := range blockToMapSchema {
		blocks, ok := blocksByType[tpe]
		if !ok {
			blocks, ok = blocksByType[blockSchema.Plural]
			if !ok {
				continue
			}
		}

		delete(blocksByType, tpe)

		var r []interface{}

		bodySchema := blockSchema.BodySchema()

		for _, b := range blocks {
			blockBodyContent, diags := b.Body.Content(bodySchema)

			if diags.HasErrors() {
				return diags
			}

			m := map[string]interface{}{}

			if diags := parseAttributesIntoMap(ctx, blockBodyContent, blockSchema.Attributes, m); diags.HasErrors() {
				return diags
			}

			for i, name := range blockSchema.LabelNames {
				m[name] = b.Labels[i]
			}

			if diags := parseBlocksIntoMap(ctx, blockBodyContent, blockSchema.Blocks, m); diags.HasErrors() {
				return diags
			}

			r = append(r, m)
		}

		if blockSchema.Singleton && len(r) > 0 {
			return hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity:    hcl.DiagError,
					Summary:     fmt.Sprintf("Too many %s blocks found. Only one of them is allowed as per `singleton` set to true", tpe),
					Detail:      "",
					Subject:     nil,
					Context:     nil,
					Expression:  nil,
					EvalContext: nil,
				},
			}
		}

		if blockSchema.Plural != "" {
			dest[blockSchema.Plural] = r
		} else {
			dest[tpe] = r
		}
	}

	return hcl.Diagnostics{}
}

func parseAttributesIntoMap(ctx *hcl.EvalContext, bodyContent *hcl.BodyContent, attrSchemas map[string]Attribute, dest map[string]interface{}) hcl.Diagnostics {
	remainingAttrs := map[string]*hcl.Attribute{}

	for _, a := range bodyContent.Attributes {
		remainingAttrs[a.Name] = a
	}

	var diags hcl.Diagnostics

	for k, attrSchema := range attrSchemas {
		v, ok := remainingAttrs[k]

		if ok {
			delete(remainingAttrs, k)
		} else if attrSchema.Optional {
			continue
		}

		delete(remainingAttrs, k)

		switch attrSchema.Kind {
		case reflect.String:
			var s string

			if diags := gohcl.DecodeExpression(v.Expr, ctx, &s); diags.HasErrors() {
				return diags
			}

			dest[k] = s
		case reflect.Int:
			var i int

			if diags := gohcl.DecodeExpression(v.Expr, ctx, &i); diags.HasErrors() {
				return diags
			}

			dest[k] = i
		default:
			return hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity:    hcl.DiagError,
					Summary:     fmt.Sprintf("unable to parse Go %s from this value", attrSchema.Kind.String()),
					Detail:      "details",
					Subject:     v.Range.Ptr(),
					Context:     nil,
					Expression:  v.Expr,
					EvalContext: nil,
				},
			}
		}

		summary := fmt.Sprintf("attr %q = %v, successfully converted to %v", k, v, attrSchema.Kind.String())

		diags = diags.Append(&hcl.Diagnostic{
			Severity:    hcl.DiagWarning,
			Summary:     summary,
			Detail:      "details",
			Subject:     v.Range.Ptr(),
			Context:     nil,
			Expression:  v.Expr,
			EvalContext: nil,
		})
	}

	for k, v := range remainingAttrs {
		summary := fmt.Sprintf("attr %q (of %v) is redundant", k, v)

		return hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     summary,
				Detail:      "details",
				Subject:     v.Range.Ptr(),
				Context:     nil,
				Expression:  v.Expr,
				EvalContext: nil,
			},
		}
	}

	return hcl.Diagnostics{}
}
