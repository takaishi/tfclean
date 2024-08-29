package tfclean

import (
	"bytes"
	"fmt"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"text/scanner"
	"unicode"
)

type ImportBlock struct {
	To string
	Id string
}

func (app *App) processImportBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])
	id, _ := app.getValueFromAttribute(block.Body.Attributes["id"])
	fmt.Printf("to: %s, id: %s\n", to, id)
	isApplied, err := app.movedImportIsApplied(state, to)
	if err != nil {
		return data, err
	}
	if isApplied {
		data, err := app.cutImportBlock(data, to, id)
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
			fmt.Printf("import block found\n")
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
