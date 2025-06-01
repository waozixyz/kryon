// render/raylib/custom_markdownview.go
package raylib

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
	// Markdown libraries...
)

type MarkdownViewHandler struct{}

// GetKrbFileDir is an interface that a Renderer might implement
// to provide its base path for resource loading by custom components.
type KrbDirectoryProvider interface {
	GetKrbFileDir() string
}

func (h *MarkdownViewHandler) HandleLayoutAdjustment(
	el *render.RenderElement,
	doc *krb.Document,
	rendererInstance render.Renderer, // Renderer instance
) error {
	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)
	log.Printf("DEBUG MarkdownHandler [%s]: Adjusting...", elIDStr)

	if len(el.Children) > 0 && el.Children[0].OriginalIndex < 0 {
		log.Printf("DEBUG MarkdownHandler [%s]: Already has dynamic children. Skipping.", elIDStr)
		return nil
	}

	sourcePath, ok := GetCustomPropertyValue(el, "source", doc)
	if !ok {
		log.Printf("WARN MarkdownHandler [%s]: Missing 'source' custom property.", elIDStr)
		addMarkdownPlaceholder(el, "Error: Missing 'source' property.")
		return nil
	}

	krbBasePath := "."
	if provider, ok := rendererInstance.(KrbDirectoryProvider); ok {
		krbBasePath = provider.GetKrbFileDir()
		log.Printf("DEBUG MarkdownHandler [%s]: Got krbFileDir from provider: %s", elIDStr, krbBasePath)
	} else {
		log.Printf("WARN MarkdownHandler [%s]: Renderer does not provide KrbFileDir. Using default base path '%s'.", elIDStr, krbBasePath)
		if rRenderer, castOk := rendererInstance.(*RaylibRenderer); castOk { // Last resort direct cast
			krbBasePath = rRenderer.krbFileDir // Access the field directly if it's our RaylibRenderer
			log.Printf("DEBUG MarkdownHandler [%s]: Got krbFileDir via direct cast: %s", elIDStr, krbBasePath)
		}
	}

	fullPath := filepath.Join(krbBasePath, sourcePath)
	log.Printf("DEBUG MarkdownHandler [%s]: Reading markdown: %s", elIDStr, fullPath)

	mdBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Printf("ERROR MarkdownHandler [%s]: Failed to read '%s': %v", elIDStr, fullPath, err)
		addMarkdownPlaceholder(el, fmt.Sprintf("Error: Cannot read '%s'", sourcePath))
		return nil
	}
	_ = mdBytes

	log.Printf("WARN MarkdownHandler [%s]: Markdown parsing & element generation NOT IMPLEMENTED.", elIDStr)

	addMarkdownPlaceholder(el, fmt.Sprintf("Render '%s'...\n(Content Area: %.0fx%.0f)", sourcePath, el.RenderW, el.RenderH))

	if len(el.Children) > 0 {
		log.Printf("INFO MarkdownHandler [%s]: Requesting re-layout of children for element.", elIDStr)
		var scaleFactor float32 = 1.0
		if rr, ok := rendererInstance.(*RaylibRenderer); ok {
			scaleFactor = rr.scaleFactor
		}

		elPaddingTop := ScaledF32(el.Padding[0], scaleFactor)
		elPaddingRight := ScaledF32(el.Padding[1], scaleFactor)
		elPaddingBottom := ScaledF32(el.Padding[2], scaleFactor)
		elPaddingLeft := ScaledF32(el.Padding[3], scaleFactor)
		elBorderTop := ScaledF32(el.BorderWidths[0], scaleFactor)
		elBorderRight := ScaledF32(el.BorderWidths[1], scaleFactor)
		elBorderBottom := ScaledF32(el.BorderWidths[2], scaleFactor)
		elBorderLeft := ScaledF32(el.BorderWidths[3], scaleFactor)

		childrenClientOriginX := el.RenderX + elBorderLeft + elPaddingLeft
		childrenClientOriginY := el.RenderY + elBorderTop + elPaddingTop
		childrenAvailableClientWidth := el.RenderW - (elBorderLeft + elBorderRight + elPaddingLeft + elPaddingRight)
		childrenAvailableClientHeight := el.RenderH - (elBorderTop + elBorderBottom + elPaddingTop + elPaddingBottom)

		childrenAvailableClientWidth = MaxF(0, childrenAvailableClientWidth)
		childrenAvailableClientHeight = MaxF(0, childrenAvailableClientHeight)

		rendererInstance.PerformLayoutChildrenOfElement(
			el,
			childrenClientOriginX,
			childrenClientOriginY,
			childrenAvailableClientWidth,
			childrenAvailableClientHeight,
		)
	}
	return nil
}

func addMarkdownPlaceholder(parent *render.RenderElement, message string) {
	if parent == nil {
		return
	}
	parent.Children = nil

	placeholderChild := &render.RenderElement{
		OriginalIndex: -999,
		// ***** FIX APPLIED HERE: Use krb.LayoutGrowBit *****
		Header:            krb.ElementHeader{Type: krb.ElemTypeText, Layout: krb.LayoutGrowBit},
		Text:              message,
		IsVisible:         true,
		FgColor:           rl.Red,
		BgColor:           rl.NewColor(50, 0, 0, 100),
		DocRef:            parent.DocRef,
		Parent:            parent,
		SourceElementName: "MarkdownPlaceholder",
	}
	parent.Children = append(parent.Children, placeholderChild)
	log.Printf("DEBUG addMarkdownPlaceholder [Parent Elem %d, Name '%s']: Added placeholder: '%s'", parent.OriginalIndex, parent.SourceElementName, message)
}
