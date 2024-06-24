package tfclean

import (
	"bytes"
	"context"
	"fmt"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"os"
	"path/filepath"
	"strings"
	"text/scanner"
	"unicode"
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

type MoveBlock struct {
	From string
	To   string
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
		if block.Type == "moved" {
			data, err = app.processMovedBlock(block, state, data)
			if err != nil {
				return err
			}
		}
	}
	return os.WriteFile(path, data, 0644)
}

func (app *App) processMovedBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	fromAttr := block.Body.Attributes["from"]
	toAttr := block.Body.Attributes["to"]

	from := []string{}
	to := []string{}
	for _, traversals := range fromAttr.Expr.(*hclsyntax.ScopeTraversalExpr).Variables() {
		for _, traversal := range traversals {
			switch traversal.(type) {
			case hcl.TraverseRoot:
				from = append(from, traversal.(hcl.TraverseRoot).Name)
			case hcl.TraverseAttr:
				from = append(from, traversal.(hcl.TraverseAttr).Name)
			}
		}
	}
	for _, traversals := range toAttr.Expr.(*hclsyntax.ScopeTraversalExpr).Variables() {
		for _, traversal := range traversals {
			switch traversal.(type) {
			case hcl.TraverseRoot:
				to = append(to, traversal.(hcl.TraverseRoot).Name)
			case hcl.TraverseAttr:
				to = append(to, traversal.(hcl.TraverseAttr).Name)
			}
		}
	}
	fromS := strings.Join(from, ".")
	toS := strings.Join(to, ".")

	isApplied, err := app.movedBlockIsApplied(state, fromS, toS)
	if err != nil {
		return data, err
	}
	if isApplied {
		data, err = cutMovedBlock(data, strings.Join(to, "."), strings.Join(from, "."))
		if err != nil {
			return data, err
		}
	}
	return data, nil
}

func (app *App) movedBlockIsApplied(state *tfstate.TFState, from string, to string) (bool, error) {
	if len(strings.Split(from, ".")) == 2 && len(strings.Split(to, ".")) == 2 {
		// from and to is module
		names, err := state.List()
		if err != nil {
			return false, err
		}
		existsFrom := false
		for _, name := range names {
			if strings.HasPrefix(name, from+".") {
				existsFrom = true
				break
			}
		}
		existsTo := false
		for _, name := range names {
			if strings.HasPrefix(name, to+".") {
				existsTo = true
				break
			}
		}
		if !existsFrom && existsTo {
			return true, nil
		}
		if !existsFrom && !existsTo {
			return true, nil
		}
		return false, nil
	} else {
		// from and to is resource
		fromAttrs, err := state.Lookup(from)
		if err != nil {
			return false, err
		}

		toAttrs, err := state.Lookup(to)
		if err != nil {
			return false, err
		}

		if fromAttrs.String() == "null" && toAttrs.String() != "null" {
			return true, nil
		}
		return false, nil
	}
}

func cutMovedBlock(data []byte, to string, from string) ([]byte, error) {
	var s scanner.Scanner
	var spos, epos int
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '-' || ch == '_' || ch == '.' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "moved":
			spos = s.Offset
			var movedBlock MoveBlock
			var current string
			for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
				switch s.TokenText() {
				case "{":
					// Ignore
				case "}":
					// Remove moved block that includes `}` and newline
					epos = s.Offset + 2
					if movedBlock.To == to && movedBlock.From == from {
						data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
						return data, nil
					}
				case "from":
					current = "from"
				case "to":
					current = "to"
				case "=":
				// Ignore
				default:
					switch current {
					case "from":
						movedBlock.From = s.TokenText()
					case "to":
						movedBlock.To = s.TokenText()
					default:
						return nil, fmt.Errorf("unexpected token: " + s.TokenText())
					}
				}
			}
		}
	}

	return nil, nil
}
