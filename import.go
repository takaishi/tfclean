package tfclean

import (
	"bytes"
	"text/scanner"
	"unicode"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ImportBlock struct {
	To           string
	Id           string
	IdentityHash map[string]string
}

func (app *App) processImportBlock(block *hclsyntax.Block, state *tfstate.TFState, data []byte) ([]byte, error) {
	to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])

	var id string
	var identityHash map[string]string

	if idAttr, exists := block.Body.Attributes["id"]; exists {
		id, _ = app.getValueFromAttribute(idAttr)
	} else if identityAttr, exists := block.Body.Attributes["identity"]; exists {
		identityHash = app.generateIdentityHash(identityAttr)
	}

	if state != nil {
		isApplied, err := app.movedImportIsApplied(state, to)
		if err != nil {
			return data, err
		}
		if isApplied {
			data, err := app.cutImportBlock(data, to, id, identityHash)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	} else {
		data, err := app.cutImportBlock(data, to, id, identityHash)
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

func (app *App) generateIdentityHash(identityAttr *hclsyntax.Attribute) map[string]string {
	identityContent := make(map[string]string)

	for _, item := range identityAttr.Expr.(*hclsyntax.ObjectConsExpr).Items {
		if literalExpr, ok := item.ValueExpr.(*hclsyntax.LiteralValueExpr); ok {
			value := literalExpr.Val.AsString()
			identityContent[value] = value
		}
	}

	if len(identityContent) == 0 {
		return nil
	}

	return identityContent
}

func (app *App) cutImportBlock(data []byte, to string, id string, identityHash map[string]string) ([]byte, error) {
	var s scanner.Scanner
	s.Init(bytes.NewReader(data))
	s.Mode = scanner.ScanIdents | scanner.ScanFloats
	s.IsIdentRune = func(ch rune, i int) bool {
		return ch == '/' || ch == '-' || ch == '_' || ch == '.' || ch == '[' || ch == ']' || ch == ':' || ch == '"' || ch == '$' || ch == '{' || ch == '}' || unicode.IsLetter(ch) || unicode.IsDigit(ch) && i > 0
	}

	var lastPos int

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if s.TokenText() == "import" && isAtLineStart(data, lastPos, s.Position.Offset) {
			found, data, err := app.readImportBlock(&s, data, to, id, identityHash, s.Offset)
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

func (app *App) readImportBlock(s *scanner.Scanner, data []byte, to string, id string, identityHash map[string]string, lastPos int) (bool, []byte, error) {
	var spos, epos int
	var importBlock ImportBlock
	var current string
	var inIdentityBlock bool
	var braceDepth int

	spos = lastPos
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		switch s.TokenText() {
		case "{":
			if current == "identity" {
				inIdentityBlock = true
				braceDepth = 1
			}
		case "}":
			if inIdentityBlock {
				braceDepth--
				if braceDepth == 0 {
					inIdentityBlock = false
					// For identity blocks, we'll match based on the `to` field only
					// since the identity content is complex and we're focusing on basic support
				}
			} else {
				// End of import block
				epos = s.Offset + 2

				// Match logic for both import types
				var matches bool
				if identityHash != nil {
					// For identity-based imports, match only on 'to' field
					// In a more sophisticated implementation, we'd parse and compare identity content
					matches = importBlock.To == to
				} else {
					// For id-based imports, match on both 'to' and 'id'
					matches = importBlock.To == to && importBlock.Id == id
				}

				if matches {
					data = bytes.Join([][]byte{data[:spos], data[epos:]}, []byte(""))
					return true, data, nil
				}
				return false, nil, nil
			}
		case "to":
			if !inIdentityBlock {
				current = "to"
			}
		case "id":
			if !inIdentityBlock {
				current = "id"
			}
		case "identity":
			if !inIdentityBlock {
				current = "identity"
			}
		case "=":
			// Ignore assignment operator
		default:
			if !inIdentityBlock {
				switch current {
				case "to":
					importBlock.To = s.TokenText()
				case "id":
					tokenId := s.TokenText()
					if len(tokenId) > 0 && tokenId[0] == '"' && tokenId[len(tokenId)-1] == '"' {
						tokenId = tokenId[1 : len(tokenId)-1]
					}
					importBlock.Id = tokenId
				}
			}
			// Reset current after processing value
			current = ""
		}
	}
	return false, nil, nil
}
