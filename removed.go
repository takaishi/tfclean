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

type RemovedBlock struct {
	From      string
	Lifecycle *LifecycleBlock
}

type LifecycleBlock struct {
	Destroy string
}

func (app *App) processRemovedBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	from, _ := app.getValueFromAttribute(block.Body.Attributes["from"])
	if state != nil {
		isApplied, err := app.removedBlockIsApplied(state, from)
		if err != nil {
			return data, err
		}
		if isApplied {
			data, err = app.cutRemovedBlock(data, from)
			if err != nil {
				return data, err
			}
		}
	} else {
		data, err := app.cutRemovedBlock(data, from)
		if err != nil {
			return data, err
		}
		return data, nil
	}
	return data, nil
}

func (app *App) removedBlockIsApplied(state *tfstate.TFState, from string) (bool, error) {
	if len(strings.Split(from, ".")) == 2 {
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
		if !existsFrom {
			return true, nil
		}
		return false, nil
	} else {
		// from and to is resource
		fromAttrs, err := state.Lookup(from)
		if err != nil {
			return false, err
		}
		if fromAttrs.String() == "null" {
			return true, nil
		}
		return false, nil
	}
}

func (app *App) cutRemovedBlock(data []byte, from string) ([]byte, error) {
	s := &scanner.Scanner{}
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '-' || ch == '_' || ch == '.' || ch == '"' || ch == '[' || ch == ']' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	var lastPos int

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if s.TokenText() == "removed" && isAtLineStart(data, lastPos, s.Position.Offset) {
			found, data, err := app.readRemovedBlock(s, data, from, s.Offset)
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

func (app *App) readRemovedBlock(s *scanner.Scanner, data []byte, from string, lastPos int) (bool, []byte, error) {
	var spos, epos int
	movedBlock := &RemovedBlock{}
	var current string
	spos = lastPos
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "{":
			// Ignore
		case "}":
			// Remove moved block that includes `}` and newline
			epos = s.Offset + 2
			if movedBlock.From == from {
				data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
				return true, data, nil
			}
			return false, nil, nil
		case "from":
			current = "from"
		case "lifecycle":
			lb, err := app.readLifecycleBlock(s)
			if err != nil {
				return false, nil, err
			}
			movedBlock.Lifecycle = lb
		case "=":
		// Ignore
		default:
			switch current {
			case "from":
				movedBlock.From = s.TokenText()
			default:
				return false, nil, fmt.Errorf("unexpected token: " + s.TokenText())
			}
		}
	}
	return false, nil, nil
}

func (app *App) readLifecycleBlock(s *scanner.Scanner) (*LifecycleBlock, error) {
	lifecycleBlock := &LifecycleBlock{}
	var current string
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "{":
			// Ignore
		case "}":
			return lifecycleBlock, nil
		case "destroy":
			current = "destroy"
		case "=":
			// Ignore
		default:
			switch current {
			case "destroy":
				lifecycleBlock.Destroy = s.TokenText()
			default:
				return nil, fmt.Errorf("unexpected token: " + s.TokenText())
			}
		}
	}

	return lifecycleBlock, nil
}

func isAtLineStart(data []byte, lastPos int, currentPos int) bool {
	if lastPos == 0 {
		return true
	}
	return bytes.LastIndexByte(data[lastPos:currentPos], '\n') == len(data[lastPos:currentPos])-1
}
