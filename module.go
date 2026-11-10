package tfclean

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// moduleScope says where the module whose files are being cleaned sits in the
// state. Addresses in moved/removed blocks of a child module are relative to
// each instance of that module, so they have to be prefixed before lookup.
type moduleScope struct {
	// callPaths lists the module call names leading from the root to this
	// module, one entry per place it is called from (e.g. [["x", "a"]] for
	// module.x.module.a). nil means the root module.
	callPaths [][]string
}

var rootScope = moduleScope{}

// instanceKey matches an optional count/for_each key such as [0] or ["a"].
const instanceKey = `(?:\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?`

// instancePrefixes returns the address prefixes ("" for the root module, e.g.
// `module.foo["a"].` for a child) of every instance of the module in state.
//
// It returns nil, so the caller keeps the block, when some instance of a
// parent module on a call path has no instance of the next module below it:
// its resources may still sit under an old address that a pending moved block
// elsewhere will bring here, and judging only the instances we can see would
// drop a block that move still needs.
func (s moduleScope) instancePrefixes(state *tfstate.TFState) ([]string, error) {
	if s.callPaths == nil {
		return []string{""}, nil
	}
	names, err := state.List()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, callPath := range s.callPaths {
		parents := []string{""}
		for _, call := range callPath {
			var children []string
			for _, parent := range parents {
				found := childInstances(names, parent, call)
				if len(found) == 0 {
					return nil, nil
				}
				children = append(children, found...)
			}
			parents = children
		}
		for _, prefix := range parents {
			seen[prefix] = true
		}
	}
	prefixes := make([]string, 0, len(seen))
	for prefix := range seen {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes, nil
}

// childInstances returns the distinct prefixes of instances of module call
// `call` directly under the module instance at parent, e.g. parent
// `module.x["p"].` and call "a" yield `module.x["p"].module.a.` or
// `module.x["p"].module.a[0].`.
func childInstances(names []string, parent, call string) []string {
	re := regexp.MustCompile("^" + regexp.QuoteMeta(parent+"module."+call) + instanceKey + `\.`)
	seen := make(map[string]bool)
	var found []string
	for _, name := range names {
		if prefix := re.FindString(name); prefix != "" && !seen[prefix] {
			seen[prefix] = true
			found = append(found, prefix)
		}
	}
	return found
}

// discoverLocalModules follows module calls with a local source ("./" or
// "../") starting from rootDir and returns the scope of every module directory
// reached, keyed by its cleaned path. Remote modules are skipped because their
// files are not ours to edit.
//
// Modules outside rootDir (e.g. ../../modules/x) are skipped unless
// includeExternal is set: they are often shared with other root modules whose
// states tfclean cannot see, so cleaning them with this root's states alone
// could drop a block another root still needs.
func (app *App) discoverLocalModules(rootDir string, includeExternal bool) (map[string]moduleScope, error) {
	scopes := make(map[string]moduleScope)
	root := filepath.Clean(rootDir)
	w := moduleWalker{root: root, includeExternal: includeExternal, visiting: map[string]bool{root: true}, scopes: scopes}
	if err := w.walk(root, nil); err != nil {
		return nil, err
	}
	return scopes, nil
}

type moduleWalker struct {
	root            string
	includeExternal bool
	visiting        map[string]bool
	scopes          map[string]moduleScope
}

// isInside reports whether dir is root itself or below it.
func isInside(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (w *moduleWalker) walk(dir string, callPath []string) error {
	calls, err := localModuleCalls(dir)
	if err != nil {
		return err
	}
	for _, call := range calls {
		// A module that (indirectly) calls itself is invalid Terraform; skip it
		// rather than recursing forever.
		if w.visiting[call.dir] {
			continue
		}
		if !w.includeExternal && !isInside(w.root, call.dir) {
			continue
		}
		childPath := append(append([]string{}, callPath...), call.name)
		scope := w.scopes[call.dir]
		scope.callPaths = append(scope.callPaths, childPath)
		w.scopes[call.dir] = scope

		w.visiting[call.dir] = true
		err := w.walk(call.dir, childPath)
		delete(w.visiting, call.dir)
		if err != nil {
			return err
		}
	}
	return nil
}

type moduleCall struct {
	name string
	dir  string
}

func localModuleCalls(dir string) ([]moduleCall, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	parser := hclparse.NewParser()
	var calls []moduleCall
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".tf" {
			continue
		}
		path := filepath.Join(dir, file.Name())
		hclFile, diags := parser.ParseHCLFile(path)
		if diags.HasErrors() {
			return nil, fmt.Errorf("error parsing %s: %s", path, diags)
		}
		body, ok := hclFile.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range body.Blocks {
			if block.Type != "module" || len(block.Labels) == 0 {
				continue
			}
			attr, ok := block.Body.Attributes["source"]
			if !ok {
				continue
			}
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() || val.Type() != cty.String || val.IsNull() {
				continue
			}
			source := val.AsString()
			if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
				continue
			}
			calls = append(calls, moduleCall{
				name: block.Labels[0],
				dir:  filepath.Clean(filepath.Join(dir, source)),
			})
		}
	}
	return calls, nil
}
