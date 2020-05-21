package hcl2yaml

import (
	"bytes"
	"encoding/json"
	"github.com/hashicorp/hcl/v2"
	"gopkg.in/yaml.v3"
	"os"
)

func Parse(src []byte, fileName string) (*hcl.File, hcl.Diagnostics) {
	var value yaml.Node

	yamlDecoder := yaml.NewDecoder(bytes.NewReader(src))

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

	yamlBody := &YamlBody{
		bytes:    src,
		fileName: fileName,
		yamlNode: &value,
	}

	file := &hcl.File{
		Body:  yamlBody,
		Bytes: src,
		Nav:   nil,
	}

	return file, nil
}

