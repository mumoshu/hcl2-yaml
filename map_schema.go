package hclv2yaml

import (
	"github.com/hashicorp/hcl/v2"
	"reflect"
)

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

