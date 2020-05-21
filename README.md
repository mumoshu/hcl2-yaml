# hcl2yaml

> YAML syntax for HashiCorp Configuration Language Version 2

`hcl2yaml` is a go library that provides a YAML syntax for [HCL2](https://github.com/hashicorp/hcl/tree/hcl2). It's basically an equivalent to [HCL2's own JSON syntax](https://github.com/hashicorp/hcl/tree/hcl2/json), but for YAML.

The upstream HCL2 has both a native syntax for human and a JSON-based variant for machines. `hcl2yaml` adds one more variant for human.

The goal of `hcl2yaml` is to provide a solid foundation for building rich YAML-based DSLs with custom functions and variables.

## Usage

Here's an example to show you the basic usage of the library.

Please also see `integration/gohcl_integration_test.go` for more concrete and working examples.

```go
type Example struct {
	// Use the HCl2's native field tag `hcl:"..."` for mapping between HCL <-> Go
	Ary1 []map[string]string `hcl:"ary1,attr"`
	Ary2 []map[string]string `hcl:"ary2,attr"`
	Map1 map[string]string    `hcl:"map1,attr"`
	Map2 map[string]string    `hcl:"map2,attr"`
	Str1 string               `hcl:"str1,attr"`
	Int1 int                  `hcl:"int1,attr"`
}

yamlSource := `
# HCL2 and hcl2yaml is context-dependent.
# That is, the value for a array field can be any "expression" that evaluates to a YAML array.

# In the first array example, we only use a HCL expression in the value of the YAML hash in the array.
ary1:
# A YAML string is considered a HCL2's native "template" in which you can use the interpolation syntax.
# In the below example, "x${var.one}y" evaluates to "xONEy" when the variable `var.one` is set to `ONE` in the HCL2 eval context.
- a: "x${var.one}y"

# Unlike the JSON syntax, you can use HCL2 expression to build the whole array.
# Just prepend a YAML tag `!!exp` before starting a string of the expression
# In the below example, you call Terraform's "list" and "map" functions to build the array.
#
# See also:
# list: https://www.terraform.io/docs/configuration/functions/list.html
# map: https://www.terraform.io/docs/configuration/functions/map.html
# HCL2 Expression: https://github.com/hashicorp/hcl2/blob/master/hcl/hclsyntax/spec.md#expressions
ary2: !!exp list(map("a", "x${var.one}y"))

map1:
  foo: "x${var.one}y"

map2: !!exp map("foo", "x${var.one}y")

str1: "x${var.one}y"

int1: !!exp 1 + 2
`

file, diags := hcl2yaml.Parse(yamlSource, fileName)
if diags.HasErrors() {
	diagWriter.WriteDiagnostics(diags)
	os.Exit(1)
}

files := map[string]*hcl.File{
    fileName: file,
}

var example Example

if diags := gohcl.DecodeBody(file.Body, hclEavlCtx, &example); diags.HasErrors() {)
	diagWriter.WriteDiagnostics(diags))
	os.Exit(1))
}

fmt.Printf("%v\n", example)
// {[map[a:xONEy]] [map[a:xONEy]] map[foo:xONEy] map[foo:xONEy] xONEy 3}
```
