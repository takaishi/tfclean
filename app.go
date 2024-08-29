package tfclean

import (
	"context"
	"fmt"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"os"
	"path/filepath"
	"strings"
)

type App struct {
	hclParser *hclparse.Parser
	CLI       *CLI
}

func New(cli *CLI) *App {
	return &App{
		hclParser: hclparse.NewParser(),
		CLI:       cli,
	}
}

func (app *App) Run(ctx context.Context) error {
	state, err := tfstate.ReadURL(ctx, app.CLI.Tfstate)
	if err != nil {
		return err
	}

	files, err := os.ReadDir(app.CLI.Dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) == ".tf" {
			path := filepath.Join(app.CLI.Dir, file.Name())
			err := app.processFile(path, state)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (app *App) processFile(path string, state *tfstate.TFState) error {
	hclFile, diags := app.hclParser.ParseHCLFile(path)
	if diags.HasErrors() {
		return fmt.Errorf("error parsing HCL hclFile: %s", diags)
	}
	body, ok := hclFile.Body.(*hclsyntax.Body)
	if !ok {
		return fmt.Errorf("not an HCL syntax body")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, _ := os.ReadFile(path)
	file.Close()

	for _, block := range body.Blocks {
		switch block.Type {
		case "moved":
			data, err = app.processMovedBlock(block, state, data)
			if err != nil {
				return err
			}
		case "import":
			data, err = app.processImportBlock(block, state, data)
			if err != nil {
				return err
			}
		}
	}
	return os.WriteFile(path, data, 0644)
}

func (app *App) getValueFromAttribute(attr *hclsyntax.Attribute) (string, error) {
	switch attr.Expr.(type) {
	case *hclsyntax.TemplateExpr:
		for _, part := range attr.Expr.(*hclsyntax.TemplateExpr).Parts {
			switch part.(type) {
			case *hclsyntax.LiteralValueExpr:
				return part.(*hclsyntax.LiteralValueExpr).Val.AsString(), nil
			default:
				return "", fmt.Errorf("unexpected type: %T", part)
			}
		}
	case *hclsyntax.ScopeTraversalExpr:
		valueSlice := []string{}
		for _, traversals := range attr.Expr.(*hclsyntax.ScopeTraversalExpr).Variables() {
			for _, traversal := range traversals {
				switch traversal.(type) {
				case hcl.TraverseRoot:
					valueSlice = append(valueSlice, traversal.(hcl.TraverseRoot).Name)
					valueSlice = append(valueSlice, ".")
				case hcl.TraverseAttr:
					valueSlice = append(valueSlice, traversal.(hcl.TraverseAttr).Name)
					valueSlice = append(valueSlice, ".")
				case hcl.TraverseIndex:
					valueSlice = valueSlice[:len(valueSlice)-1]
					valueSlice = append(valueSlice, fmt.Sprintf("[\"%s\"]", traversal.(hcl.TraverseIndex).Key.AsString()))
				}
			}
		}
		return strings.Join(valueSlice, ""), nil
	default:
		return "", fmt.Errorf("unexpected type: %T", attr.Expr)
	}
	return "", nil
}
