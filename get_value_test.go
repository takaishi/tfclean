package tfclean

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// parseToAttr extracts the `to` attribute from a single-block HCL snippet
// (e.g. a bare `import {}` block). Kept tiny so the table below stays focused
// on the traversal string we want to assert.
func parseToAttr(t *testing.T, src string) *hclsyntax.Attribute {
	t.Helper()
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCL([]byte(src), "test.tf")
	if diags.HasErrors() {
		t.Fatalf("parse: %s", diags)
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok || len(body.Blocks) != 1 {
		t.Fatalf("expected exactly one block, got body=%T blocks=%d", f.Body, len(body.Blocks))
	}
	attr, ok := body.Blocks[0].Body.Attributes["to"]
	if !ok {
		t.Fatal("expected a `to` attribute on the block")
	}
	return attr
}

// TestApp_getValueFromAttribute_scopeTraversal covers the `to = ...` /
// `from = ...` expressions on import / moved / removed blocks. The
// interesting edge cases are string / number indices that are *not* the
// final traversal step — those used to drop the separator between the
// bracketed key and the following attribute, so
// `module.foo["bar"].baz.qux` was stringified as
// `module.foo["bar"]baz.qux` and missed in state lookups.
func TestApp_getValueFromAttribute_scopeTraversal(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "bare module attr",
			src:  "import {\n  to = module.foo.bar\n  id = \"x\"\n}\n",
			want: "module.foo.bar",
		},
		{
			name: "string index is final step",
			src:  "import {\n  to = module.foo[\"bar\"]\n  id = \"x\"\n}\n",
			want: "module.foo[\"bar\"]",
		},
		{
			name: "number index is final step",
			src:  "import {\n  to = module.foo[0]\n  id = \"x\"\n}\n",
			want: "module.foo[0]",
		},
		{
			name: "string index followed by attrs",
			src:  "import {\n  to = module.foo[\"bar\"].baz.qux\n  id = \"x\"\n}\n",
			want: "module.foo[\"bar\"].baz.qux",
		},
		{
			name: "number index followed by attrs",
			src:  "import {\n  to = module.foo[0].baz.qux\n  id = \"x\"\n}\n",
			want: "module.foo[0].baz.qux",
		},
	}

	app := New(&CLI{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attr := parseToAttr(t, tc.src)
			got, err := app.getValueFromAttribute(attr)
			if err != nil {
				t.Fatalf("getValueFromAttribute: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
