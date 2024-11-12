package tfclean

import (
	"bytes"
	"fmt"
	"text/scanner"
	"unicode"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ImportBlock struct {
	To string
	Id string
}

func (app *App) processImportBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])
	id, _ := app.getValueFromAttribute(block.Body.Attributes["id"])
	if state != nil {
		isApplied, err := app.movedImportIsApplied(state, to)
		if err != nil {
			return data, err
		}
		if isApplied {
			data, err := app.cutImportBlock(data, to, id)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	} else {
		data, err := app.cutImportBlock(data, to, id)
		if err != nil {
			return nil, err
		}
		return data, nil
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

func (app *App) cutImportBlock(data []byte, to string, id string) ([]byte, error) {
	var s scanner.Scanner
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '-' || ch == '_' || ch == '.' || ch == '[' || ch == ']' || ch == ':' || ch == '"' || ch == '$' || ch == '{' || ch == '}' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	var lastPos int

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if s.TokenText() == "import" && isAtLineStart(data, lastPos, s.Position.Offset) {
			found, data, err := app.readImportBlock(&s, data, to, id, s.Offset)
			if err != nil {
				return nil, err
			}
			if found {
				return data, nil
			}
		}
		lastPos = s.Position.Offset
	}

	return nil, nil
}

func (app *App) readImportBlock(s *scanner.Scanner, data []byte, to string, id string, lastPos int) (bool, []byte, error) {
	var spos, epos int
	var importBlock ImportBlock
	var current string
	spos = lastPos
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "{":
			// Ignore
		case "}":
			// Remove moved block that includes `}` and newline
			epos = s.Offset + 2
			if importBlock.To == to && importBlock.Id == id {
				data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
				return true, data, nil
			}
			return false, nil, nil
		case "to":
			current = "to"
		case "id":
			current = "id"
		case "=":
		//case "\"":
		// Ignore
		default:
			switch current {
			case "to":
				importBlock.To = s.TokenText()
			case "id":
				id = s.TokenText()
				if id[0] == '"' && id[len(id)-1] == '"' {
					id = id[1 : len(id)-1]
				}
				importBlock.Id = id
			default:
				return false, nil, fmt.Errorf("unexpected token: " + s.TokenText())
			}
		}
	}
	return false, nil, nil
}
