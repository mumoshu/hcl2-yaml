package integration

import (
	"bytes"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/terraform/lang/funcs"
	"github.com/mumoshu/hcl2-yaml"
	ctyyaml "github.com/zclconf/go-cty-yaml"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	"os"
	"testing"
)

func FailOnError(t *testing.T, files map[string]*hcl.File) func(hcl.Diagnostics) {
	return func(diags hcl.Diagnostics) {
		t.Helper()

		if diags.HasErrors() {
			var buf bytes.Buffer

			diagWriter := hcl.NewDiagnosticTextWriter(&buf, files, uint(200), true)
			diagWriter.WriteDiagnostics(diags)

			t.Fatal(buf.String())
		}
	}
}

func TestGohclIntegration(t *testing.T) {
	failOnError := FailOnError(t, map[string]*hcl.File{})

	fileName := "example.yaml"

	yamlSource2 := []byte(`
foo:
- fooFirstLabel: bar
  baz: BAZ

hoge:
  fuga: FUGA

hello: "x${var.one}y"

intval: 1
`)

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

	type Foo struct {
		FooFirstLabel string `hcl:"fooFirstLabel,attr"`
		Baz           string `hcl:"baz,attr"`
	}

	type Result struct {
		Hello  string `hcl:"hello,attr"`
		Intval int    `hcl:"intval,attr"`
		Foos   []Foo  `hcl:"foo,block"`
	}

	var result Result

	file, diags := hcl2yaml.Parse(yamlSource2, fileName)

	failOnError(diags)

	files := map[string]*hcl.File{
		fileName: file,
	}

	diags = gohcl.DecodeBody(file.Body, ctx, &result)

	FailOnError(t, files)(diags)

	want := Result{
		Hello:  "xONEy",
		Intval: 1,
		Foos: []Foo{
			{
				FooFirstLabel: "bar",
				Baz:           "BAZ",
			},
		},
	}

	if diff := cmp.Diff(want, result); diff != "" {
		t.Errorf("unexpected diff:\n%s", diff)
	}

	fmt.Fprintf(os.Stdout, "#3: %v\n", result)
}

func TestGohclIntegration_Expr(t *testing.T) {
	failOnError := FailOnError(t, map[string]*hcl.File{})

	fileName := "example.yaml"

	yamlSource2 := []byte(`
ary1:
- a: "x${var.one}y"

ary2: !!exp list(map("a", "x${var.one}y"))

map1:
  foo: "x${var.one}y"

map2: !!exp map("foo", "x${var.one}y")

str1: "x${var.one}y"

int1: !!exp 1 + 2
`)

	vs := map[string]cty.Value{
		"one": cty.StringVal("ONE"),
	}

	vars := map[string]cty.Value{
		"var": cty.MapVal(vs),
	}

	ctx := &hcl.EvalContext{
		Variables: vars,
		Functions: Functions("."),
	}

	type Dynamic struct {
		Ary1 hcl.Expression `hcl:"ary1,attr"`
		Ary2 hcl.Expression `hcl:"ary2,attr"`
		Map1 hcl.Expression `hcl:"map1,attr"`
		Map2 hcl.Expression `hcl:"map2,attr"`
		Str1 hcl.Expression `hcl:"str1,attr"`
		Int1 hcl.Expression `hcl:"int1,attr"`
	}

	type Static struct {
		Ary1 []map[string]string `hcl:"ary1,attr"`
		Ary2 []map[string]string `hcl:"ary2,attr"`
		Map1 map[string]string    `hcl:"map1,attr"`
		Map2 map[string]string    `hcl:"map2,attr"`
		Str1 string               `hcl:"str1,attr"`
		Int1 int                  `hcl:"int1,attr"`
	}

	var dynamic Dynamic

	file, diags := hcl2yaml.Parse(yamlSource2, fileName)

	failOnError(diags)

	files := map[string]*hcl.File{
		fileName: file,
	}

	want := Static{
		Ary1: []map[string]string{map[string]string{"a": "xONEy"}},
		Ary2: []map[string]string{map[string]string{"a": "xONEy"}},
		Map1: map[string]string{"foo": "xONEy"},
		Map2: map[string]string{"foo": "xONEy"},
		Int1: 3,
		Str1: "xONEy",
	}

	f := FailOnError(t, files)

	f(gohcl.DecodeBody(file.Body, ctx, &dynamic))

	got1 := Static{
		Ary1: [] map[string]string{},
		Ary2: [] map[string]string{},
		Map1: map[string]string{},
		Map2: map[string]string{},
	}

	f(gohcl.DecodeBody(file.Body, ctx, &got1))

	if diff := cmp.Diff(want, got1); diff != "" {
		t.Errorf("unexpected diff for got1:\n%s", diff)
	}

	got2 := Static{
		Ary1: [] map[string]string{},
		Ary2: [] map[string]string{},
		Map1: map[string]string{},
		Map2: map[string]string{},
	}

	f(gohcl.DecodeExpression(dynamic.Ary1, ctx, &got2.Ary1))
	f(gohcl.DecodeExpression(dynamic.Ary2, ctx, &got2.Ary2))
	f(gohcl.DecodeExpression(dynamic.Map1, ctx, &got2.Map1))
	f(gohcl.DecodeExpression(dynamic.Map2, ctx, &got2.Map2))
	f(gohcl.DecodeExpression(dynamic.Int1, ctx, &got2.Int1))
	f(gohcl.DecodeExpression(dynamic.Str1, ctx, &got2.Str1))

	if diff := cmp.Diff(want, got2); diff != "" {
		t.Errorf("unexpected diff for got2:\n%s", diff)
	}

	fmt.Fprintf(os.Stdout, "#3: %v\n", got2)
}

// Functions is
func Functions(baseDir string) map[string]function.Function {
	return map[string]function.Function{
		"abs":              stdlib.AbsoluteFunc,
		"abspath":          funcs.AbsPathFunc,
		"basename":         funcs.BasenameFunc,
		"base64decode":     funcs.Base64DecodeFunc,
		"base64encode":     funcs.Base64EncodeFunc,
		"base64gzip":       funcs.Base64GzipFunc,
		"base64sha256":     funcs.Base64Sha256Func,
		"base64sha512":     funcs.Base64Sha512Func,
		"bcrypt":           funcs.BcryptFunc,
		"ceil":             funcs.CeilFunc,
		"chomp":            funcs.ChompFunc,
		"cidrhost":         funcs.CidrHostFunc,
		"cidrnetmask":      funcs.CidrNetmaskFunc,
		"cidrsubnet":       funcs.CidrSubnetFunc,
		"cidrsubnets":      funcs.CidrSubnetsFunc,
		"coalesce":         stdlib.CoalesceFunc,
		"coalescelist":     funcs.CoalesceListFunc,
		"compact":          funcs.CompactFunc,
		"concat":           stdlib.ConcatFunc,
		"contains":         funcs.ContainsFunc,
		"csvdecode":        stdlib.CSVDecodeFunc,
		"dirname":          funcs.DirnameFunc,
		"distinct":         funcs.DistinctFunc,
		"element":          funcs.ElementFunc,
		"chunklist":        funcs.ChunklistFunc,
		"file":             funcs.MakeFileFunc(baseDir, false),
		"fileexists":       funcs.MakeFileExistsFunc(baseDir),
		"fileset":          funcs.MakeFileSetFunc(baseDir),
		"filebase64":       funcs.MakeFileFunc(baseDir, true),
		"filebase64sha256": funcs.MakeFileBase64Sha256Func(baseDir),
		"filebase64sha512": funcs.MakeFileBase64Sha512Func(baseDir),
		"filemd5":          funcs.MakeFileMd5Func(baseDir),
		"filesha1":         funcs.MakeFileSha1Func(baseDir),
		"filesha256":       funcs.MakeFileSha256Func(baseDir),
		"filesha512":       funcs.MakeFileSha512Func(baseDir),
		"flatten":          funcs.FlattenFunc,
		"floor":            funcs.FloorFunc,
		"format":           stdlib.FormatFunc,
		"formatdate":       stdlib.FormatDateFunc,
		"formatlist":       stdlib.FormatListFunc,
		"indent":           funcs.IndentFunc,
		"index":            funcs.IndexFunc,
		"join":             funcs.JoinFunc,
		"jsondecode":       stdlib.JSONDecodeFunc,
		"jsonencode":       stdlib.JSONEncodeFunc,
		"keys":             funcs.KeysFunc,
		"length":           funcs.LengthFunc,
		"list":             funcs.ListFunc,
		"log":              funcs.LogFunc,
		"lookup":           funcs.LookupFunc,
		"lower":            stdlib.LowerFunc,
		"map":              funcs.MapFunc,
		"matchkeys":        funcs.MatchkeysFunc,
		"max":              stdlib.MaxFunc,
		"md5":              funcs.Md5Func,
		"merge":            funcs.MergeFunc,
		"min":              stdlib.MinFunc,
		"parseint":         funcs.ParseIntFunc,
		"pathexpand":       funcs.PathExpandFunc,
		"pow":              funcs.PowFunc,
		"range":            stdlib.RangeFunc,
		"regex":            stdlib.RegexFunc,
		"regexall":         stdlib.RegexAllFunc,
		"replace":          funcs.ReplaceFunc,
		"reverse":          funcs.ReverseFunc,
		"rsadecrypt":       funcs.RsaDecryptFunc,
		"setintersection":  stdlib.SetIntersectionFunc,
		"setproduct":       funcs.SetProductFunc,
		"setunion":         stdlib.SetUnionFunc,
		"sha1":             funcs.Sha1Func,
		"sha256":           funcs.Sha256Func,
		"sha512":           funcs.Sha512Func,
		"signum":           funcs.SignumFunc,
		"slice":            funcs.SliceFunc,
		"sort":             funcs.SortFunc,
		"split":            funcs.SplitFunc,
		"strrev":           stdlib.ReverseFunc,
		"substr":           stdlib.SubstrFunc,
		"timestamp":        funcs.TimestampFunc,
		"timeadd":          funcs.TimeAddFunc,
		"title":            funcs.TitleFunc,
		"tostring":         funcs.MakeToFunc(cty.String),
		"tonumber":         funcs.MakeToFunc(cty.Number),
		"tobool":           funcs.MakeToFunc(cty.Bool),
		"toset":            funcs.MakeToFunc(cty.Set(cty.DynamicPseudoType)),
		"tolist":           funcs.MakeToFunc(cty.List(cty.DynamicPseudoType)),
		"tomap":            funcs.MakeToFunc(cty.Map(cty.DynamicPseudoType)),
		"transpose":        funcs.TransposeFunc,
		"trim":             funcs.TrimFunc,
		"trimprefix":       funcs.TrimPrefixFunc,
		"trimspace":        funcs.TrimSpaceFunc,
		"trimsuffix":       funcs.TrimSuffixFunc,
		"upper":            stdlib.UpperFunc,
		"urlencode":        funcs.URLEncodeFunc,
		"uuid":             funcs.UUIDFunc,
		"uuidv5":           funcs.UUIDV5Func,
		"values":           funcs.ValuesFunc,
		"yamldecode":       ctyyaml.YAMLDecodeFunc,
		"yamlencode":       ctyyaml.YAMLEncodeFunc,
		"zipmap":           funcs.ZipmapFunc,
		"try":              tryfunc.TryFunc,
		"can":              tryfunc.CanFunc,
		"convert":          typeexpr.ConvertFunc,
	}
}
