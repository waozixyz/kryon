// render/raylib/custom_markdownview.go
package raylib

import (
	"fmt"
	"io/ioutil" // Needed for file reading
	"log"
	"path/filepath" // Needed for path joining

	rl "github.com/gen2brain/raylib-go/raylib" // <<< IMPORT ADDED WITH ALIAS 'rl' >>>
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"

	// --- Potential Markdown Libraries ---
	// Choose one and uncomment when implementing:
	// "github.com/gomarkdown/markdown"
	// "github.com/gomarkdown/markdown/ast"
	// "github.com/gomarkdown/markdown/parser"
	// OR
	// "github.com/yuin/goldmark"
	// "github.com/yuin/goldmark/text"
)

// MarkdownViewHandler implements CustomComponentHandler for MarkdownView components.
type MarkdownViewHandler struct{}

// HandleLayoutAdjustment for MarkdownView.
// TODO: Implement the logic to parse markdown and generate children.
func (h *MarkdownViewHandler) HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error {
	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)
	log.Printf("DEBUG MarkdownHandler [%s]: Adjusting...", elIDStr)

	// --- Check if already processed ---
	if len(el.Children) > 0 && el.Children[0].OriginalIndex < 0 { // Check if placeholder/dynamic children exist
		log.Printf("DEBUG MarkdownHandler [%s]: Already has dynamic children (or placeholder). Skipping regeneration.", elIDStr)
		return nil
	}

	// 1. Get the 'source' custom property value (path string)
	sourcePath, ok := getCustomPropertyValue(el, "source", doc)
	if !ok {
		log.Printf("WARN MarkdownHandler [%s]: Missing 'source' custom property. Cannot render content.", elIDStr)
		addMarkdownPlaceholder(el, "Error: Missing 'source' property.")
		return nil // Not a fatal error for the whole layout process
	}

	// 2. Construct full path
	//    !!! THIS STILL REQUIRES ACCESS TO krbFileDir FROM THE RENDERER !!!
	krbBasePath := "." // <<< Placeholder - Needs real path somehow! >>>
	fullPath := filepath.Join(krbBasePath, sourcePath)
	log.Printf("DEBUG MarkdownHandler [%s]: Attempting to read markdown file: %s (Base: %s)", elIDStr, fullPath, krbBasePath)

	// 3. Read the Markdown file
	_, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Printf("ERROR MarkdownHandler [%s]: Failed to read file '%s': %v", elIDStr, fullPath, err)
		addMarkdownPlaceholder(el, fmt.Sprintf("Error: Cannot read '%s'", sourcePath))
		return nil // File not found/readable isn't fatal for layout
	}
	// mdContent := string(mdBytes) // <<< REMOVED UNUSED VARIABLE FOR NOW >>>

	// --- 4. Parse Markdown and Generate Children (Placeholder) ---
	log.Printf("WARN MarkdownHandler [%s]: Markdown parsing & element generation NOT IMPLEMENTED.", elIDStr)
	// TODO: Replace this placeholder section with actual Markdown processing
	// Use mdBytes or mdContent here with your chosen markdown parser library

	// If parsing/generation fails or is not implemented, add placeholder
	addMarkdownPlaceholder(el, fmt.Sprintf("Render '%s'...", sourcePath))

	// --- 5. Trigger Re-Layout (Crucial but Complex) ---
	log.Printf("WARN MarkdownHandler [%s]: Re-layout after adding children is NOT IMPLEMENTED.", elIDStr)
	// Example: PerformLayoutChildren(el, el.RenderX, el.RenderY, el.RenderW, el.RenderH, scaleFactor, doc)

	return nil
}

// addMarkdownPlaceholder adds a simple Text element as a child for errors/info.
func addMarkdownPlaceholder(parent *render.RenderElement, message string) {
	if parent == nil {
		return
	}
	// Ensure we don't add multiple placeholders
	for _, child := range parent.Children {
		if child != nil && child.OriginalIndex == -999 { // Use a specific placeholder index
			return
		}
	}

	placeholderChild := &render.RenderElement{
		OriginalIndex: -999, // Unique placeholder ID
		Header:        krb.ElementHeader{Type: krb.ElemTypeText},
		Text:          message,
		IsVisible:     true,
		FgColor:       rl.Red, // <<< FIXED: rl.Red is now defined >>>
		// Default layout/size will apply
	}
	parent.Children = append(parent.Children, placeholderChild)
	log.Printf("DEBUG addMarkdownPlaceholder [Parent Elem %d]: Added placeholder: '%s'", parent.OriginalIndex, message)
}

// TODO: Implement this function using a Markdown library
// func generateElementsFromMarkdown(node ast.Node, parentIndex int) []*render.RenderElement {
//    	children := []*render.RenderElement{}
//		// ... walk AST, create Text/Image/Container elements ...
//   	return children
// }