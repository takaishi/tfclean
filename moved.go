package tfclean

import (
	"bytes"
	"fmt"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"strings"
	"text/scanner"
	"unicode"
)

type MoveBlock struct {
	From string
	To   string
}

func (app *App) processMovedBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	from, _ := app.getValueFromAttribute(block.Body.Attributes["from"])
	to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])
	isApplied, err := app.movedBlockIsApplied(state, from, to)
	if err != nil {
		return data, err
	}
	if isApplied {
		data, err = app.cutMovedBlock(data, to, from)
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

func (app *App) cutMovedBlock(data []byte, to string, from string) ([]byte, error) {
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
