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
	var spos, epos int
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '-' || ch == '_' || ch == '.' || ch == '"' || ch == '[' || ch == ']' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "removed":
			spos = s.Offset
			movedBlock := &RemovedBlock{}
			var current string
			for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
				fmt.Println(s.TokenText())
				switch s.TokenText() {
				case "{":
					// Ignore
				case "}":
					// Remove moved block that includes `}` and newline
					epos = s.Offset + 2
					if movedBlock.From == from {
						data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
						return data, nil
					}
				case "from":
					current = "from"
				case "lifecycle":
					lb, err := app.readLifecycleBlock(s)
					if err != nil {
						return nil, err
					}
					movedBlock.Lifecycle = lb
				case "=":
				// Ignore
				default:
					switch current {
					case "from":
						movedBlock.From = s.TokenText()
					default:
						return nil, fmt.Errorf("unexpected token: " + s.TokenText())
					}
				}
			}
		}
	}

	return nil, nil
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
