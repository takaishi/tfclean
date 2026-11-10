package tfclean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// stateResource is one managed resource in a test state.
type stateResource struct {
	Module string // e.g. `module.x["a"]`, "" for the root module
	Type   string
	Name   string
}

// writeState writes a minimal Terraform state JSON containing the given
// resources and returns its path.
func writeState(t *testing.T, dir string, resources ...stateResource) string {
	t.Helper()
	rs := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		m := map[string]any{
			"mode":     "managed",
			"type":     r.Type,
			"name":     r.Name,
			"provider": `provider["registry.terraform.io/hashicorp/aws"]`,
			"instances": []map[string]any{
				{"schema_version": 0, "attributes": map[string]any{"id": r.Name}},
			},
		}
		if r.Module != "" {
			m["module"] = r.Module
		}
		rs = append(rs, m)
	}
	b, err := json.Marshal(map[string]any{"version": 4, "resources": rs})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "terraform.tfstate")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The "split a module" refactoring from
// https://developer.hashicorp.com/terraform/language/modules/develop/refactoring#split-a-module
// leaves moved blocks inside the old module, whose addresses are relative to
// each instance of that module.
const splitModuleX = `module "a" {
  source = "../a"
}

moved {
  from = aws_instance.a
  to   = module.a.aws_instance.a
}
`

const splitModuleXCleaned = `module "a" {
  source = "../a"
}

`

const moduleA = `resource "aws_instance" "a" {}

moved {
  from = aws_instance.old
  to   = aws_instance.a
}
`

const moduleACleaned = `resource "aws_instance" "a" {}

`

func TestApp_Run_childModules(t *testing.T) {
	singleRoot := `module "x" {
  source = "./modules/x"
}
`
	forEachRoot := `module "x" {
  for_each = toset(["p", "q"])
  source   = "./modules/x"
}
`
	tests := []struct {
		name      string
		root      string
		resources []stateResource
		wantX     string
		wantA     string
	}{
		{
			name: "applied in the child and the nested module: remove",
			root: singleRoot,
			resources: []stateResource{
				{Module: "module.x.module.a", Type: "aws_instance", Name: "a"},
			},
			wantX: splitModuleXCleaned,
			wantA: moduleACleaned,
		},
		{
			name: "not applied yet: keep",
			root: singleRoot,
			resources: []stateResource{
				{Module: "module.x", Type: "aws_instance", Name: "a"},
			},
			wantX: splitModuleX,
			wantA: moduleA,
		},
		{
			name: "module not in state: keep",
			root: singleRoot,
			resources: []stateResource{
				{Type: "aws_instance", Name: "unrelated"},
			},
			wantX: splitModuleX,
			wantA: moduleA,
		},
		{
			name: "for_each: applied in every instance: remove",
			root: forEachRoot,
			resources: []stateResource{
				{Module: `module.x["p"].module.a`, Type: "aws_instance", Name: "a"},
				{Module: `module.x["q"].module.a`, Type: "aws_instance", Name: "a"},
			},
			wantX: splitModuleXCleaned,
			wantA: moduleACleaned,
		},
		{
			name: "for_each: pending in one instance: keep",
			root: forEachRoot,
			resources: []stateResource{
				{Module: `module.x["p"].module.a`, Type: "aws_instance", Name: "a"},
				{Module: `module.x["q"]`, Type: "aws_instance", Name: "a"},
			},
			wantX: splitModuleX,
			wantA: moduleA,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "root")
			writeFiles(t, root, map[string]string{
				"main.tf": tt.root,
				// Remote modules must be skipped, not followed.
				"remote.tf": `module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
}
`,
				"modules/x/main.tf": splitModuleX,
				"modules/a/main.tf": moduleA,
			})
			statePath := writeState(t, dir, tt.resources...)

			app := New(&CLI{Tfstate: []string{statePath}, Dir: root})
			if err := app.Run(t.Context()); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := readFile(t, filepath.Join(root, "modules/x/main.tf")); got != tt.wantX {
				t.Errorf("modules/x/main.tf = %q, want %q", got, tt.wantX)
			}
			if got := readFile(t, filepath.Join(root, "modules/a/main.tf")); got != tt.wantA {
				t.Errorf("modules/a/main.tf = %q, want %q", got, tt.wantA)
			}
		})
	}
}

// A module called from several places must be applied for every caller.
func TestApp_Run_childModules_multipleCallers(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	writeFiles(t, root, map[string]string{
		"main.tf": `module "x1" {
  source = "./modules/x"
}

module "x2" {
  source = "./modules/x/"
}
`,
		"modules/x/main.tf": splitModuleX,
		"modules/a/main.tf": moduleA,
	})
	statePath := writeState(t, dir,
		stateResource{Module: "module.x1.module.a", Type: "aws_instance", Name: "a"},
		stateResource{Module: "module.x2", Type: "aws_instance", Name: "a"},
	)

	app := New(&CLI{Tfstate: []string{statePath}, Dir: root})
	if err := app.Run(t.Context()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readFile(t, filepath.Join(root, "modules/x/main.tf")); got != splitModuleX {
		t.Errorf("modules/x/main.tf = %q, want unchanged", got)
	}
	// module.x2.module.a has nothing in state yet, so module a is kept too.
	if got := readFile(t, filepath.Join(root, "modules/a/main.tf")); got != moduleA {
		t.Errorf("modules/a/main.tf = %q, want unchanged", got)
	}
}

// Modules outside the given directory may be shared with other roots, so they
// are only cleaned with --include-external-modules.
func TestApp_Run_externalModules(t *testing.T) {
	tests := []struct {
		name            string
		includeExternal bool
		wantX           string
		wantA           string
	}{
		{name: "default: untouched", includeExternal: false, wantX: splitModuleX, wantA: moduleA},
		{name: "--include-external-modules: cleaned", includeExternal: true, wantX: splitModuleXCleaned, wantA: moduleACleaned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"envs/dev/main.tf": `module "x" {
  source = "../../modules/x"
}
`,
				"modules/x/main.tf": splitModuleX,
				"modules/a/main.tf": moduleA,
			})
			statePath := writeState(t, dir,
				stateResource{Module: "module.x.module.a", Type: "aws_instance", Name: "a"},
			)

			app := New(&CLI{Tfstate: []string{statePath}, Dir: filepath.Join(dir, "envs/dev"), IncludeExternalModules: tt.includeExternal})
			if err := app.Run(t.Context()); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := readFile(t, filepath.Join(dir, "modules/x/main.tf")); got != tt.wantX {
				t.Errorf("modules/x/main.tf = %q, want %q", got, tt.wantX)
			}
			if got := readFile(t, filepath.Join(dir, "modules/a/main.tf")); got != tt.wantA {
				t.Errorf("modules/a/main.tf = %q, want %q", got, tt.wantA)
			}
		})
	}
}
