package tfclean

import (
	"bytes"
	"fmt"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"strings"
	"text/scanner"
	"unicode"
)

type ImportBlock struct {
	To string
	Id string
}

func (app *App) processImportBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	toAttr := block.Body.Attributes["to"]
	idAttr := block.Body.Attributes["id"]

	to := []string{}
	id := []string{}
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
	for _, traversals := range idAttr.Expr.(*hclsyntax.ScopeTraversalExpr).Variables() {
		for _, traversal := range traversals {
			switch traversal.(type) {
			case hcl.TraverseRoot:
				id = append(id, traversal.(hcl.TraverseRoot).Name)
			case hcl.TraverseAttr:
				id = append(id, traversal.(hcl.TraverseAttr).Name)
			}
		}
	}
	toS := strings.Join(to, ".")
	idS := strings.Join(id, ".")

	isApplied, err := app.movedImportIsApplied(state, toS)
	if err != nil {
		return data, err
	}
	if isApplied {
		data, err = app.cutImportBlock(data, toS, idS)
		if err != nil {
			return data, err
		}
	}
	return data, nil
}

func (app *App) movedImportIsApplied(state *tfstate.TFState, to string) (bool, error) {
	toAttrs, err := state.Lookup(to)
	if err != nil {
		return false, err
	}

	if toAttrs.String() != "null" {
		return true, nil
	}
	return false, nil
}

func (app *App) cutImportBlock(data []byte, to string, from string) ([]byte, error) {
	var s scanner.Scanner
	var spos, epos int
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '-' || ch == '_' || ch == '.' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "import":
			spos = s.Offset
			var importBlock ImportBlock
			var current string
			for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
				switch s.TokenText() {
				case "{":
					// Ignore
				case "}":
					// Remove moved block that includes `}` and newline
					epos = s.Offset + 2
					if importBlock.To == to && importBlock.Id == from {
						data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
						return data, nil
					}
				case "to":
					current = "to"
				case "id":
					current = "id"
				case "=":
				// Ignore
				default:
					switch current {
					case "to":
						importBlock.To = s.TokenText()
					case "id":
						importBlock.Id = s.TokenText()
					default:
						return nil, fmt.Errorf("unexpected token: " + s.TokenText())
					}
				}
			}
		}
	}

	return nil, nil
}
