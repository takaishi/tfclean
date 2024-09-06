package tfclean

import (
	"bytes"
	"fmt"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type MoveBlock struct {
	From string
	To   string
}

func (app *App) processMovedBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	from, _ := app.getValueFromAttribute(block.Body.Attributes["from"])
	to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])
	if state != nil {
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
	} else {
		data, err := app.cutMovedBlock(data, to, from)
		if err != nil {
			return data, err
		}
		return data, nil
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
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '-' || ch == '_' || ch == '.' || ch == '"' || ch == '[' || ch == ']' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	var lastPos int

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if s.TokenText() == "moved" && isAtLineStart(data, lastPos, s.Position.Offset) {
			found, data, err := app.readMovedBlock(&s, data, to, from, s.Offset)
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

func (app *App) readMovedBlock(s *scanner.Scanner, data []byte, to string, from string, lastPos int) (bool, []byte, error) {
	var spos, epos int
	var movedBlock MoveBlock
	var current string
	spos = lastPos
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "{":
			// Ignore
		case "}":
			// Remove moved block that includes `}` and newline
			epos = s.Offset + 2
			if movedBlock.To == to && movedBlock.From == from {
				data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
				return true, data, nil
			}
			return false, nil, nil
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
				return false, nil, fmt.Errorf("unexpected token: " + s.TokenText())
			}
		}
	}
	return false, nil, nil
}
