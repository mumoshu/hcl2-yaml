package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mitchellh/mapstructure"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

func getLinePos() hcl.Range {
	pc, _, line, _ := runtime.Caller(1)

	file := runtime.FuncForPC(pc).Name()

	return hcl.Range{
		Filename: file,
		Start:    hcl.Pos{Line: line, Column: 0},
		End:      hcl.Pos{Line: line + 1, Column: 0},
	}
}

type MapSchema struct {
	Attributes map[string]Attribute
	Blocks     map[string]Block
}

func (m *MapSchema) BodySchema() *hcl.BodySchema {
	return createBodySchema(m.Attributes, m.Blocks)
}

func createBodySchema(as map[string]Attribute, bs map[string]Block) *hcl.BodySchema {
	attrs := []hcl.AttributeSchema{}

	for k, v := range as {
		attrs = append(attrs, hcl.AttributeSchema{
			Name:     k,
			Required: !v.Optional,
		})
	}

	blocks := []hcl.BlockHeaderSchema{}

	for k, v := range bs {
		blocks = append(blocks, hcl.BlockHeaderSchema{
			Type:       k,
			LabelNames: v.LabelNames,
		})
	}

	bodySchema := &hcl.BodySchema{
		Attributes: attrs,
		Blocks:     blocks,
	}

	return bodySchema
}

type Block struct {
	Plural string

	LabelNames []string

	Singleton bool

	Blocks     map[string]Block
	Attributes map[string]Attribute
}

func (b *Block) BodySchema() *hcl.BodySchema {
	return createBodySchema(b.Attributes, b.Blocks)
}

type Attribute struct {
	Kind     reflect.Kind
	Optional bool
}

func main() {
	schema := MapSchema{
		Attributes: map[string]Attribute{
			"hello": {
				Kind:     reflect.String,
				Optional: false,
			},
			"intval": {
				Kind:     reflect.Int,
				Optional: true,
			},
		},
		Blocks: map[string]Block{
			"foo": {
				Plural:     "foos",
				LabelNames: []string{"fooFirstLabel"},
				Blocks:     nil,
				Attributes: map[string]Attribute{
					"baz": {
						Kind:     reflect.String,
						Optional: false,
					},
				},
			},
			"hoge": {
				LabelNames: []string{},
				Blocks:     nil,
				Attributes: map[string]Attribute{
					"fuga": {
						Kind:     reflect.String,
						Optional: false,
					},
				},
			},
		},
	}

	width, _, err := terminal.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		panic(err)
	}

	fileName := "example.yaml"

	yamlSource1 := []byte(`
foo:
  fooFirstLabel: bar
  baz: BAZ


hoge:
  fuga: FUGA

hello: "world"

intval: 1
`)
	yamlSource2 := []byte(`
foos:
- fooFirstLabel: bar
  baz: BAZ

hoge:
  fuga: FUGA

hello: "world"

intval: 1
`)
	//file, diags := hclsyntax.ParseConfig(source, fileName, hcl.InitialPos)
	yamlSources := [][]byte{yamlSource1, yamlSource2}

	for _, src := range yamlSources {
		result := map[string]interface{}{}

		file := ParseConfig(src, fileName, schema)

		files := map[string]*hcl.File{
			fileName: file,
		}

		diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), true)

		if diags := DecodeObject(file, schema, &result); diags.HasErrors() {
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stdout, "%v\n", result)
	}

	{
		type Result struct {
			Hello string
			Intval int
			Foos []struct{
				FooFirstLabel string
				Baz string
			}
		}

		var result Result

		file := ParseConfig(yamlSource2, fileName, schema)

		files := map[string]*hcl.File{
			fileName: file,
		}

		diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), true)

		if diags := DecodeObject(file, schema, &result); diags.HasErrors() {
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stdout, "%v\n", result)
	}
}

func ParseConfig(src []byte, fileName string, deprecatedSchema MapSchema) *hcl.File {
	yamlBody := &YamlBody{
		fileName:    fileName,
		bytes:       src,
		blockSchema: deprecatedSchema.Blocks,
		attrSchemas: deprecatedSchema.Attributes,
	}

	file := &hcl.File{
		Body:  yamlBody,
		Bytes: src,
		Nav:   nil,
	}

	return file
}

func DecodeObject(file *hcl.File, schema MapSchema, result interface{}) hcl.Diagnostics {
	bodySchema := schema.BodySchema()

	bodyContent, diags := file.Body.Content(bodySchema)

	if diags.HasErrors() {
		return diags
	}

	switch dest := result.(type) {
	case map[string]interface{}:
		return parseMap(bodyContent, schema, dest)
	default:
		m := map[string]interface{}{}

		if diags := parseMap(bodyContent, schema, m); diags.HasErrors() {
			return diags
		}

		if err := mapstructure.Decode(m, result); err != nil {
			return hcl.Diagnostics{
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
	}

	return nil
}

func parseMap(bodyContent *hcl.BodyContent, schema MapSchema, dest map[string]interface{}) hcl.Diagnostics {
	if diags := parseBlocksIntoMap(bodyContent, schema.Blocks, dest); diags.HasErrors() {
		return diags
	}

	if diags := parseAttributesIntoMap(bodyContent, schema.Attributes, dest); diags.HasErrors() {
		return diags
	}

	return nil
}

type YamlBody struct {
	fileName string

	bytes []byte

	blockSchema map[string]Block

	attrSchemas map[string]Attribute
}

func (f *YamlBody) Content(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	var value yaml.Node

	yamlDecoder := yaml.NewDecoder(bytes.NewReader(f.bytes))

	if err := yamlDecoder.Decode(&value); err != nil {
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

	debugEncoder := json.NewEncoder(os.Stdout)
	debugEncoder.SetIndent("", "  ")
	if err := debugEncoder.Encode(value); err != nil {
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

	// 4
	if value.Kind == yaml.MappingNode {
		return f.parseMapping(&value)
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

func (f *YamlBody) parseMapping(node *yaml.Node) (*hcl.BodyContent, hcl.Diagnostics) {
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
		if !exists {
			return nil, hcl.Diagnostics{
				&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("no yaml mapping found for expected attribute %q", k),
				},
			}
		}

		attr, diags := f.parseAttrsFromYaml(k, attrSchema, c)
		if diags.HasErrors() {
			return nil, diags
		}

		attrs[k] = attr
	}

	var blocks []*hcl.Block

	for k, blockSchema := range f.blockSchema {
		c, exists := keyToValue[k]
		if !exists {
			c, exists = keyToValue[blockSchema.Plural]
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
					Summary:     fmt.Sprintf("unsupported type of yaml node: %s", c.Kind),
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

func (f *YamlBody) parseAttrsFromYaml(name string, attrSchema Attribute, valNode *yaml.Node) (*hcl.Attribute, hcl.Diagnostics) {
	switch attrSchema.Kind {
	case reflect.Slice:
		var vs []cty.Value

		for _, v := range valNode.Content {
			switch v.Kind {
			case yaml.ScalarNode:
				vs = append(vs, cty.StringVal(v.Value))
			}
		}

		lastElem := valNode.Content[len(valNode.Content)-1]

		rng := hcl.Range{
			Filename: f.fileName,
			Start: hcl.Pos{
				Column: valNode.Column,
				Line:   valNode.Line,
			},
			End: hcl.Pos{
				Column: lastElem.Column + len(lastElem.Value),
				Line:   lastElem.Line,
			},
		}

		attr := &hcl.Attribute{
			Name:      name,
			Expr:      hcl.StaticExpr(cty.ListVal(vs), rng),
			Range:     rng,
			NameRange: hcl.Range{},
		}

		return attr, nil
	case reflect.String:
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
		attr := &hcl.Attribute{
			Name:      name,
			Expr:      hcl.StaticExpr(cty.StringVal(v), rng),
			Range:     rng,
			NameRange: hcl.Range{},
		}

		return attr, nil
	case reflect.Int:
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

		attr := &hcl.Attribute{
			Name:      name,
			Expr:      hcl.StaticExpr(cty.NumberIntVal(int64(intval)), rng),
			Range:     rng,
			NameRange: hcl.Range{},
		}

		return attr, nil
	default:
		return nil, hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     fmt.Sprintf("unsupported type of attribute value: %s", attrSchema.Kind.String()),
				Detail:      "",
				Subject:     nil,
				Context:     nil,
				Expression:  nil,
				EvalContext: nil,
			},
		}
	}
}

func (f *YamlBody) parseBlocksFromYamlSequence(tpe string, blockSchema Block, valNode *yaml.Node) ([]*hcl.Block, hcl.Diagnostics) {
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

func (f *YamlBody) parseBlockFromYamlMapping(tpe string, blockSchema Block, valNode *yaml.Node) (*hcl.Block, hcl.Diagnostics) {
	c := valNode.Content

	var block hcl.Block

	block.Type = tpe

	m := map[string]*yaml.Node{}

	for i := 0; i < len(c); i += 2 {
		keyNode := c[i]
		valNode := c[i+1]

		m[keyNode.Value] = valNode
	}

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

	w := &bytes.Buffer{}
	e := yaml.NewEncoder(w)
	if err := e.Encode(valNode); err != nil {
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

	yamlSource := w.Bytes()

	ff := &YamlBody{
		fileName:    f.fileName,
		bytes:       yamlSource,
		blockSchema: blockSchema.Blocks,
		attrSchemas: blockSchema.Attributes,
	}

	block.Body = ff

	return &block, nil
}

func parseBlocksIntoMap(bodyContent *hcl.BodyContent, blockToMapSchema map[string]Block, dest map[string]interface{}) hcl.Diagnostics {
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

			if diags := parseAttributesIntoMap(blockBodyContent, blockSchema.Attributes, m); diags.HasErrors() {
				return diags
			}

			for i, name := range blockSchema.LabelNames {
				m[name] = b.Labels[i]
			}

			if diags := parseBlocksIntoMap(blockBodyContent, blockSchema.Blocks, m); diags.HasErrors() {
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

func parseAttributesIntoMap(bodyContent *hcl.BodyContent, attrSchemas map[string]Attribute, dest map[string]interface{}) hcl.Diagnostics {
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

		ctx := &hcl.EvalContext{
			Variables: map[string]cty.Value{},
			Functions: map[string]function.Function{},
		}

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
