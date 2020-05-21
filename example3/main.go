package main

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mumoshu/hclv2yaml"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"reflect"
)

func main() {
	schema := hclv2yaml.MapSchema{
		Attributes: map[string]hclv2yaml.Attribute{
			"hello": {
				Kind:     reflect.String,
				Optional: false,
			},
			"intval": {
				Kind:     reflect.Int,
				Optional: true,
			},
		},
		Blocks: map[string]hclv2yaml.Block{
			"foo": {
				Plural:     "foos",
				LabelNames: []string{"fooFirstLabel"},
				Blocks:     nil,
				Attributes: map[string]hclv2yaml.Attribute{
					"baz": {
						Kind:     reflect.String,
						Optional: false,
					},
				},
			},
			"hoge": {
				LabelNames: []string{},
				Blocks:     nil,
				Attributes: map[string]hclv2yaml.Attribute{
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
foo:
- fooFirstLabel: bar
  baz: BAZ

hoge:
  fuga: FUGA

hello: "x${var.one}y"

intval: 1
`)
	//file, diags := hclsyntax.Parse(source, fileName, hcl.InitialPos)
	yamlSources := [][]byte{yamlSource1, yamlSource2}

	vs := map[string]cty.Value{
		"one": cty.StringVal("ONE"),
	}

	vars := map[string]cty.Value{
		"var": cty.MapVal(vs),
	}

	ctx := &hcl.EvalContext{
		Variables: vars,
		Functions: nil,
	}

	for _, src := range yamlSources {
		result := map[string]interface{}{}

		file, diags := hclv2yaml.Parse(src, fileName)
		if diags != nil {
			diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, map[string]*hcl.File{}, uint(width), true)
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		files := map[string]*hcl.File{
			fileName: file,
		}

		diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), true)

		if diags := hclv2yaml.DecodeBodyIntoMap(ctx, file.Body, schema, &result); diags.HasErrors() {
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stdout, "%v\n", result)
	}

	{
		type Result struct {
			Hello  string
			Intval int
			Foos   []struct {
				FooFirstLabel string
				Baz           string
			}
		}

		var result Result

		file, diags := hclv2yaml.Parse(yamlSource2, fileName)
		if diags.HasErrors() {
			diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, map[string]*hcl.File{}, uint(width), true)
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		files := map[string]*hcl.File{
			fileName: file,
		}

		diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), true)

		if diags := hclv2yaml.DecodeBodyIntoMap(ctx, file.Body, schema, &result); diags.HasErrors() {
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stdout, "%v\n", result)
	}

	{
		type Result struct {
			Hello  string `hcl:"hello,attr"`
			Intval int    `hcl:"intval,attr"`
			Foos   []struct {
				FooFirstLabel string `hcl:"fooFirstLabel,attr"`
				Baz           string `hcl:"baz,attr"`
			} `hcl:"foo,block"`
		}

		var result Result

		file, diags := hclv2yaml.Parse(yamlSource2, fileName)
		if diags.HasErrors() {
			diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, map[string]*hcl.File{}, uint(width), true)
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		files := map[string]*hcl.File{
			fileName: file,
		}

		diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), true)

		if diags := gohcl.DecodeBody(file.Body, ctx, &result); diags.HasErrors() {
			diagWriter.WriteDiagnostics(diags)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stdout, "#3: %v\n", result)
	}
}

