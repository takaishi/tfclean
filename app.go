package tfclean

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
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
	var states []*tfstate.TFState

	if len(app.CLI.Tfstate) > 0 {
		// States given explicitly. A read failure is a hard error: silently
		// dropping a state would shrink the set we require agreement across and
		// could delete a block that is still unapplied in the dropped state.
		for _, url := range app.CLI.Tfstate {
			state, err := tfstate.ReadURL(ctx, url)
			if err != nil {
				return fmt.Errorf("could not read state from %s: %w", url, err)
			}
			states = append(states, state)
		}
	} else {
		detectedURL, err := app.detectBackendFromConfig()
		if err != nil {
			log.Printf("Warning: Could not auto-detect backend configuration: %v", err)
			log.Printf("Continuing without state file. Use --tfstate flag to specify state location manually.")
		} else if detectedURL != "" {
			log.Printf("Auto-detected state location: %s", detectedURL)
			state, err := tfstate.ReadURL(ctx, detectedURL)
			if err != nil {
				log.Printf("Warning: Could not read state from auto-detected location: %v", err)
				log.Printf("Continuing without state file.")
			} else {
				states = append(states, state)
			}
		}
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
			err := app.processFile(path, states)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (app *App) processFile(path string, states []*tfstate.TFState) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	data, err := app.applyAllDeletions(original, states)
	if err != nil {
		return err
	}

	if !bytes.Equal(data, original) {
		empty, err := app.isEmptyConfig(data)
		if err != nil {
			return err
		}
		if empty {
			return os.Remove(path)
		}
	}
	return os.WriteFile(path, data, 0644)
}

func (app *App) isEmptyConfig(data []byte) (bool, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return true, nil
	}
	parser := hclparse.NewParser()
	hclFile, diags := parser.ParseHCL(data, "memory.tf")
	if diags.HasErrors() {
		return false, fmt.Errorf("error parsing HCL: %s", diags)
	}
	body, ok := hclFile.Body.(*hclsyntax.Body)
	if !ok {
		return false, nil
	}
	return len(body.Blocks) == 0 && len(body.Attributes) == 0, nil
}

const (
	ignoreFileAnnotation = "tfclean-ignore-file"
	ignoreAnnotation     = "tfclean-ignore"
)

// collectIgnoreAnnotations scans data for tfclean-ignore and tfclean-ignore-file
// comment annotations. It reports whether the whole file should be left untouched,
// and the set of line numbers carrying a block-level ignore annotation (the line
// immediately preceding the block a user wants to preserve).
func (app *App) collectIgnoreAnnotations(data []byte) (bool, map[int]bool, error) {
	tokens, diags := hclsyntax.LexConfig(data, "memory.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return false, nil, fmt.Errorf("error lexing HCL: %s", diags)
	}

	fileIgnored := false
	ignoredLines := make(map[int]bool)
	for _, token := range tokens {
		if token.Type != hclsyntax.TokenComment {
			continue
		}
		text := string(token.Bytes)
		switch {
		case strings.Contains(text, ignoreFileAnnotation):
			fileIgnored = true
		case strings.Contains(text, ignoreAnnotation):
			ignoredLines[token.Range.Start.Line] = true
		}
	}
	return fileIgnored, ignoredLines, nil
}

// appliedInAll reports whether check returns true for every state. With no
// states it returns true so callers fall back to removing the block
// unconditionally (the "remove all" / forced mode). When several states are
// given, a block counts as applied only if it is applied in all of them, so a
// block still pending in any one state is preserved.
func (app *App) appliedInAll(states []*tfstate.TFState, check func(*tfstate.TFState) (bool, error)) (bool, error) {
	for _, state := range states {
		applied, err := check(state)
		if err != nil {
			return false, err
		}
		if !applied {
			return false, nil
		}
	}
	return true, nil
}

func (app *App) collectDeletionRanges(body *hclsyntax.Body, states []*tfstate.TFState, ignoredLines map[int]bool) ([]hcl.Range, error) {
	ranges := make([]hcl.Range, 0, len(body.Blocks))
	for _, block := range body.Blocks {
		if ignoredLines[block.Range().Start.Line-1] {
			continue
		}
		var applied bool
		var err error
		switch block.Type {
		case "import":
			to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])
			applied, err = app.appliedInAll(states, func(state *tfstate.TFState) (bool, error) {
				return app.movedImportIsApplied(state, to)
			})
		case "moved":
			from, _ := app.getValueFromAttribute(block.Body.Attributes["from"])
			to, _ := app.getValueFromAttribute(block.Body.Attributes["to"])
			applied, err = app.appliedInAll(states, func(state *tfstate.TFState) (bool, error) {
				return app.movedBlockIsApplied(state, from, to)
			})
		case "removed":
			from, _ := app.getValueFromAttribute(block.Body.Attributes["from"])
			applied, err = app.appliedInAll(states, func(state *tfstate.TFState) (bool, error) {
				return app.removedBlockIsApplied(state, from)
			})
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		if applied {
			ranges = append(ranges, block.Range())
		}
	}
	return ranges, nil
}

func (app *App) applyAllDeletions(data []byte, states []*tfstate.TFState) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	fileIgnored, ignoredLines, err := app.collectIgnoreAnnotations(data)
	if err != nil {
		return nil, err
	}
	if fileIgnored {
		return data, nil
	}
	// Create a new parser each time to avoid the influence of previous parse results
	parser := hclparse.NewParser()
	hclFile, diags := parser.ParseHCL(data, "memory.tf")
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing HCL: %s", diags)
	}
	body, ok := hclFile.Body.(*hclsyntax.Body)
	if !ok {
		return data, nil
	}
	ranges, err := app.collectDeletionRanges(body, states, ignoredLines)
	if err != nil {
		return nil, err
	}
	if len(ranges) == 0 {
		return data, nil
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Start.Byte > ranges[j].Start.Byte })
	for _, r := range ranges {
		start := r.Start.Byte
		end := r.End.Byte
		// Remove the newline after the deletion range
		if end < len(data) {
			if data[end] == '\n' {
				end++
			} else if data[end] == '\r' {
				if end+1 < len(data) && data[end+1] == '\n' {
					end += 2
				} else {
					end++
				}
			}
		}
		data = append(append([]byte{}, data[:start]...), data[end:]...)
		// Normalize consecutive newlines around the deletion range (collapse 3 or more consecutive newlines into 2)
		data = app.normalizeNewlinesAround(data, start)
	}
	return data, nil
}

func (app *App) normalizeNewlinesAround(data []byte, pos int) []byte {
	if len(data) == 0 {
		return data
	}
	// Find consecutive newlines before and after the deletion position
	// Search backwards for newlines
	beforeStart := pos
	for beforeStart > 0 && (data[beforeStart-1] == '\n' || data[beforeStart-1] == '\r') {
		if beforeStart > 1 && data[beforeStart-2] == '\r' && data[beforeStart-1] == '\n' {
			beforeStart -= 2
		} else {
			beforeStart--
		}
	}
	// Search forwards for newlines
	afterEnd := pos
	for afterEnd < len(data) && (data[afterEnd] == '\n' || data[afterEnd] == '\r') {
		if afterEnd+1 < len(data) && data[afterEnd] == '\r' && data[afterEnd+1] == '\n' {
			afterEnd += 2
		} else {
			afterEnd++
		}
	}
	// Count the number of consecutive newlines
	newlineCount := 0
	for i := beforeStart; i < afterEnd; i++ {
		if data[i] == '\n' {
			newlineCount++
		} else if data[i] == '\r' {
			newlineCount++
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
			}
		}
	}
	// Collapse 3 or more consecutive newlines into 2
	if newlineCount > 2 {
		// Previous part + 2 newlines + following part
		result := make([]byte, 0, len(data))
		result = append(result, data[:beforeStart]...)
		// Add 2 newlines (match the newline format of the previous line)
		if beforeStart > 0 {
			if beforeStart > 1 && data[beforeStart-2] == '\r' && data[beforeStart-1] == '\n' {
				result = append(result, '\r', '\n', '\r', '\n')
			} else {
				result = append(result, '\n', '\n')
			}
		} else {
			result = append(result, '\n', '\n')
		}
		result = append(result, data[afterEnd:]...)
		return result
	}
	return data
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

func (app *App) movedBlockIsApplied(state *tfstate.TFState, from string, to string) (bool, error) {
	if strings.HasPrefix(from, "module.") && strings.HasPrefix(to, "module.") && len(strings.Split(from, ".")) == 2 && len(strings.Split(to, ".")) == 2 {
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

func (app *App) removedBlockIsApplied(state *tfstate.TFState, from string) (bool, error) {
	if strings.HasPrefix(from, "module.") && len(strings.Split(from, ".")) == 2 {
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
		// resource
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

func (app *App) getValueFromAttribute(attr *hclsyntax.Attribute) (string, error) {
	switch attr.Expr.(type) {
	case *hclsyntax.TemplateExpr:
		result := []string{}
		for _, part := range attr.Expr.(*hclsyntax.TemplateExpr).Parts {
			switch part.(type) {
			case *hclsyntax.LiteralValueExpr:
				result = append(result, part.(*hclsyntax.LiteralValueExpr).Val.AsString())
			case *hclsyntax.ScopeTraversalExpr:
				valueSlice := []string{"\"", "${"}
				for _, traversals := range part.(*hclsyntax.ScopeTraversalExpr).Variables() {
					tl := len(traversals)
					for i, traversal := range traversals {
						switch traversal.(type) {
						case hcl.TraverseRoot:
							valueSlice = append(valueSlice, traversal.(hcl.TraverseRoot).Name)
							valueSlice = append(valueSlice, ".")
							if i == tl-1 {
								valueSlice = valueSlice[:len(valueSlice)-1]
							}
						case hcl.TraverseAttr:
							valueSlice = append(valueSlice, traversal.(hcl.TraverseAttr).Name)
							valueSlice = append(valueSlice, ".")
							if i == tl-1 {
								valueSlice = valueSlice[:len(valueSlice)-1]
							}
						}
					}
				}
				valueSlice = append(valueSlice, "}")
				result = append(result, strings.Join(valueSlice, ""))
			default:
				return "", fmt.Errorf("unexpected type: %T", part)
			}
		}
		result = append(result, "\"")
		return strings.Join(result, ""), nil
	case *hclsyntax.ScopeTraversalExpr:
		valueSlice := []string{}
		for _, traversals := range attr.Expr.(*hclsyntax.ScopeTraversalExpr).Variables() {
			tl := len(traversals)
			for i, traversal := range traversals {
				switch traversal.(type) {
				case hcl.TraverseRoot:
					valueSlice = append(valueSlice, traversal.(hcl.TraverseRoot).Name)
					valueSlice = append(valueSlice, ".")
					if i == tl-1 {
						valueSlice = valueSlice[:len(valueSlice)-1]
						return strings.Join(valueSlice, ""), nil
					}
				case hcl.TraverseAttr:
					valueSlice = append(valueSlice, traversal.(hcl.TraverseAttr).Name)
					valueSlice = append(valueSlice, ".")
					if i == tl-1 {
						valueSlice = valueSlice[:len(valueSlice)-1]
						return strings.Join(valueSlice, ""), nil
					}
				case hcl.TraverseIndex:
					valueSlice = valueSlice[:len(valueSlice)-1]
					switch traversal.(hcl.TraverseIndex).Key.Type().FriendlyName() {
					case "string":
						valueSlice = append(valueSlice, fmt.Sprintf("[\"%s\"]", traversal.(hcl.TraverseIndex).Key.AsString()))
						if i == tl-1 {
							return strings.Join(valueSlice, ""), nil
						}
						valueSlice = append(valueSlice, ".")
					case "number":
						valueSlice = append(valueSlice, fmt.Sprintf("[%s]", traversal.(hcl.TraverseIndex).Key.AsBigFloat().String()))
						if i == tl-1 {
							return strings.Join(valueSlice, ""), nil
						}
						valueSlice = append(valueSlice, ".")
					default:
						return "", fmt.Errorf("unexpected type: %T", traversal.(hcl.TraverseIndex).Key.Type().FriendlyName())
					}
				}
			}
		}
		return strings.Join(valueSlice, ""), nil
	default:
		return "", fmt.Errorf("unexpected type: %T", attr.Expr)
	}
	return "", nil
}

func (app *App) detectBackendFromConfig() (string, error) {
	files, err := os.ReadDir(app.CLI.Dir)
	if err != nil {
		return "", err
	}

	parser := hclparse.NewParser()
	var terraformBlocks []*hclsyntax.Block

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) == ".tf" {
			path := filepath.Join(app.CLI.Dir, file.Name())
			hclFile, diags := parser.ParseHCLFile(path)
			if diags.HasErrors() {
				continue
			}
			body, ok := hclFile.Body.(*hclsyntax.Body)
			if !ok {
				continue
			}
			for _, block := range body.Blocks {
				if block.Type == "terraform" {
					terraformBlocks = append(terraformBlocks, block)
				}
			}
		}
	}

	for _, terraformBlock := range terraformBlocks {
		for _, block := range terraformBlock.Body.Blocks {
			if block.Type == "backend" {
				return app.buildStateURLFromBackend(block)
			}
		}
	}

	return "", fmt.Errorf("no backend configuration found")
}

func (app *App) buildStateURLFromBackend(backendBlock *hclsyntax.Block) (string, error) {
	if len(backendBlock.Labels) == 0 {
		return "", fmt.Errorf("backend block has no type label")
	}

	backendType := backendBlock.Labels[0]

	if backendType != "s3" {
		return "", fmt.Errorf("unsupported backend type: %s (only S3 backend is supported for auto-detection)", backendType)
	}

	return app.buildS3URL(backendBlock)
}

func (app *App) buildS3URL(backendBlock *hclsyntax.Block) (string, error) {
	bucket, err := app.getStringAttribute(backendBlock.Body, "bucket")
	if err != nil {
		return "", fmt.Errorf("s3 backend: bucket attribute is required: %w", err)
	}

	key, err := app.getStringAttribute(backendBlock.Body, "key")
	if err != nil {
		return "", fmt.Errorf("s3 backend: key attribute is required: %w", err)
	}

	return fmt.Sprintf("s3://%s/%s", bucket, key), nil
}

func (app *App) getStringAttribute(body *hclsyntax.Body, name string) (string, error) {
	attr, ok := body.Attributes[name]
	if !ok {
		return "", fmt.Errorf("attribute %s not found", name)
	}

	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return "", fmt.Errorf("error evaluating attribute %s: %v", name, diags)
	}

	if val.Type() != cty.String {
		return "", fmt.Errorf("attribute %s is not a string", name)
	}

	return val.AsString(), nil
}
