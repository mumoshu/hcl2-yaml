package main

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

func main() {
	width, _, err := terminal.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		panic(err)
	}

	attrs := []hcl.AttributeSchema{
		{Name: "hello", Required: true},
	}
	blocks := []hcl.BlockHeaderSchema{
		{Type: "foo", LabelNames: []string{"bar"}},
		{Type: "hoge", LabelNames: []string{}},
	}

	bodySchema := &hcl.BodySchema{
		Attributes: attrs,
		Blocks:     blocks,
	}

	fileName := "example.hcl"

	source := []byte(`foo "bar" {
  baz = "BAR"
}

hoge {
  fuga = "FUGA"
}

hello = "world"
`)
	file, diags := hclsyntax.ParseConfig(source, fileName, hcl.InitialPos)

	files := map[string]*hcl.File{
		fileName: file,
	}

	diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), true)

	if diags.HasErrors() {
		diagWriter.WriteDiagnostics(diags)

		os.Exit(1)
	}

	bodyContent, diags := file.Body.Content(bodySchema)

	if diags.HasErrors() {
		diagWriter.WriteDiagnostics(diags)

		os.Exit(1)
	}

	for _, b := range bodyContent.Blocks {
		attrs, diags := b.Body.JustAttributes()
		if diags.HasErrors() {
			diagWriter.WriteDiagnostics(diags)

			os.Exit(1)
		}

		for k, v := range attrs {
			summary := fmt.Sprintf("block %q (labels %v), attr %q = %v", b.Type, b.Labels, k, v)

			diagWriter.WriteDiagnostic(&hcl.Diagnostic{
				Severity:    hcl.DiagWarning,
				Summary:     summary,
				Detail:      "",
				Subject:     v.Range.Ptr(),
				Context:     b.DefRange.Ptr(),
				Expression:  v.Expr,
				EvalContext: nil,
			})
		}
	}

	for k, v := range bodyContent.Attributes {
		summary := fmt.Sprintf("attr %q = %v", k, v)

		diagWriter.WriteDiagnostic(&hcl.Diagnostic{
			Severity:    hcl.DiagWarning,
			Summary:     summary,
			Detail:      "details",
			Subject:     v.Range.Ptr(),
			Context:     nil,
			Expression:  v.Expr,
			EvalContext: nil,
		})
	}
}
