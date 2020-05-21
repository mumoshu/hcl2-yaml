package hcl2yaml

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mitchellh/mapstructure"
)

func DecodeBodyIntoMap(ctx *hcl.EvalContext, body hcl.Body, schema MapSchema, result interface{}) hcl.Diagnostics {
	bodySchema := schema.BodySchema()

	bodyContent, diags := body.Content(bodySchema)

	if diags.HasErrors() {
		return diags
	}

	switch dest := result.(type) {
	case map[string]interface{}:
		return parseMap(ctx, bodyContent, schema, dest)
	default:
		m := map[string]interface{}{}

		if diags := parseMap(ctx, bodyContent, schema, m); diags.HasErrors() {
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

func parseMap(ctx *hcl.EvalContext, bodyContent *hcl.BodyContent, schema MapSchema, dest map[string]interface{}) hcl.Diagnostics {
	if diags := parseBlocksIntoMap(ctx, bodyContent, schema.Blocks, dest); diags.HasErrors() {
		return diags
	}

	if diags := parseAttributesIntoMap(ctx, bodyContent, schema.Attributes, dest); diags.HasErrors() {
		return diags
	}

	return nil
}

