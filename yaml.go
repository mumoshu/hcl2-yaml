package hcl2yaml

import "gopkg.in/yaml.v3"

func mappingKVs(valNode *yaml.Node) map[string]*yaml.Node {
	c := valNode.Content

	m := map[string]*yaml.Node{}

	for i := 0; i < len(c); i += 2 {
		keyNode := c[i]
		valNode := c[i+1]

		m[keyNode.Value] = valNode
	}

	return m
}

