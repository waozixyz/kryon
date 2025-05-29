// render/raylib/custom_tabbar.go
package raylib

import (
	"fmt"
	"log"
	"strings"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/krb/utils"
	"github.com/waozixyz/kryon/impl/go/render"
)

// TabBarHandler implements render.CustomComponentHandler for TabBar components.
type TabBarHandler struct{}

// HandleLayoutAdjustment adjusts the TabBar's position based on the 'position'
// custom property and resizes its sibling (assumed main content area).
// It also re-layouts its own children within its new adjusted frame.
func (h *TabBarHandler) HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error {
	if el == nil {
		return fmt.Errorf("tabBar handler: received nil element")
	}
	elIDStr := fmt.Sprintf("ElemGlobalIdx %d Name '%s'", el.OriginalIndex, el.SourceElementName)

	if el.Parent == nil {
		log.Printf("WARN TabBarHandler [%s]: cannot adjust layout without a parent. TabBar might be a root element.", elIDStr)
		// If it's a root, it can't adjust relative to parent or siblings.
		// It can still re-layout its own children if its own frame was changed by window resize.
		// For now, we'll return if no parent, as typical usage involves a parent.
		return nil // Or return fmt.Errorf, depending on how strict this should be.
	}
	if doc == nil {
		return fmt.Errorf("tabBar %s: KRB document is nil", elIDStr)
	}

	// Use exported GetCustomPropertyValue from raylib_renderer.go
	position, posOk := GetCustomPropertyValue(el, "position", doc)
	if !posOk {
		position = "bottom" // Default if 'position' property is missing
	}
	orientation, orientOk := GetCustomPropertyValue(el, "orientation", doc)
	if !orientOk {
		orientation = "row" // Default if 'orientation' property is missing
	}

	parent := el.Parent
	parentIDStr := fmt.Sprintf("ElemGlobalIdx %d Name '%s'", parent.OriginalIndex, parent.SourceElementName)
	parentW, parentH := parent.RenderW, parent.RenderH
	parentX, parentY := parent.RenderX, parent.RenderY

	// TabBar's current dimensions (before this adjustment)
	// These might have been set by the initial layout pass.
	initialW, initialH := el.RenderW, el.RenderH
	initialX, initialY := el.RenderX, el.RenderY

	log.Printf("DEBUG TabBarHandler [%s]: Adjusting. Pos:'%s' Orient:'%s' | Initial Frame: X:%.1f,Y:%.1f W:%.1fxH:%.1f | Parent [%s] Frame: X:%.1f,Y:%.1f W:%.1fxH:%.1f",
		elIDStr, position, orientation, initialX, initialY, initialW, initialH, parentIDStr, parentX, parentY, parentW, parentH)

	newX, newY, newW, newH := initialX, initialY, initialW, initialH
	stretchWidth := (strings.ToLower(orientation) == "row")
	stretchHeight := (strings.ToLower(orientation) == "column")

	// Determine TabBar's new position and size based on 'position' and 'orientation'
	switch strings.ToLower(position) {
	case "top":
		newY = parentY // Align with parent's top
		newX = parentX // Align with parent's left
		if stretchWidth {
			newW = parentW // Stretch to parent's width
		}
		// Height (newH) remains initialH unless stretchHeight is true (uncommon for top/bottom bars)
	case "bottom":
		newY = parentY + parentH - initialH // Align bottom of TabBar with parent's bottom
		if newY < parentY { newY = parentY } // Prevent going above parent's top
		newX = parentX // Align with parent's left
		if stretchWidth {
			newW = parentW // Stretch to parent's width
		}
	case "left":
		newX = parentX // Align with parent's left
		newY = parentY // Align with parent's top
		if stretchHeight {
			newH = parentH // Stretch to parent's height
		}
		// Width (newW) remains initialW unless stretchWidth is true
	case "right":
		newX = parentX + parentW - initialW // Align right of TabBar with parent's right
		if newX < parentX { newX = parentX } // Prevent going left of parent's start
		newY = parentY // Align with parent's top
		if stretchHeight {
			newH = parentH // Stretch to parent's height
		}
	default:
		log.Printf("Warn TabBarHandler [%s]: Unknown position value '%s'. Defaulting to 'bottom'.", elIDStr, position)
		position = "bottom" // Ensure position variable matches default logic
		newY = parentY + parentH - initialH
		if newY < parentY { newY = parentY }
		newX = parentX
		if stretchWidth {
			newW = parentW
		}
	}

	// Ensure minimum dimensions for the TabBar itself
	finalW := maxF(1.0, newW)
	finalH := maxF(1.0, newH)

	frameChanged := (newX != el.RenderX || newY != el.RenderY || finalW != el.RenderW || finalH != el.RenderH)

	if frameChanged {
		el.RenderX = newX
		el.RenderY = newY
		el.RenderW = finalW
		el.RenderH = finalH
		log.Printf("DEBUG TabBarHandler [%s]: Frame *WAS* modified by custom adjustment to X:%.1f,Y:%.1f W:%.1fxH:%.1f.", elIDStr, el.RenderX, el.RenderY, el.RenderW, el.RenderH)
	} else {
		log.Printf("DEBUG TabBarHandler [%s]: Frame unchanged by custom adjustment. Re-layout of children might still be needed if parent resized.", elIDStr)
		// Even if the TabBar's frame relative to parent didn't change, if the parent resized,
		// the TabBar (if stretching) and its children might need re-layout.
		// The current frameChanged only checks if THIS adjustment changed it.
		// A more robust check would be if parent dimensions changed OR this adjustment changed it.
		// For now, if no change from THIS adjustment, we proceed to children re-layout as parent might have changed.
	}

	// Adjust sibling element (assumed to be the main content area)
	var mainContentSibling *render.RenderElement = nil
	if len(parent.Children) > 1 { // Need at least one other child to be the sibling
		for _, sibling := range parent.Children {
			if sibling != nil && sibling != el { // Find the first sibling that is not the TabBar itself
				mainContentSibling = sibling
				break
			}
		}
	}

	if mainContentSibling != nil {
		siblingIDStr := fmt.Sprintf("ElemGlobalIdx %d Name '%s'", mainContentSibling.OriginalIndex, mainContentSibling.SourceElementName)
		log.Printf("DEBUG TabBarHandler [%s]: Found main content sibling [%s] to adjust.", elIDStr, siblingIDStr)

		origSiblingX, origSiblingY := mainContentSibling.RenderX, mainContentSibling.RenderY
		origSiblingW, origSiblingH := mainContentSibling.RenderW, mainContentSibling.RenderH

		switch strings.ToLower(position) { // Use the resolved position
		case "bottom":
			// Sibling (content area) height should be from its current top to the TabBar's new top
			mainContentSibling.RenderH = maxF(1.0, el.RenderY - mainContentSibling.RenderY)
			// Sibling width and X position usually remain unchanged
		case "top":
			// Sibling (content area) new top is the TabBar's new bottom
			newSibY := el.RenderY + el.RenderH
			// Sibling's new height is its original bottom edge minus its new top edge
			mainContentSibling.RenderH = maxF(1.0, (origSiblingY + origSiblingH) - newSibY)
			mainContentSibling.RenderY = newSibY
			// Sibling width and X position usually remain unchanged
		case "left":
			newSibX := el.RenderX + el.RenderW
			mainContentSibling.RenderW = maxF(1.0, (origSiblingX + origSiblingW) - newSibX)
			mainContentSibling.RenderX = newSibX
			// Sibling height and Y position usually remain unchanged
		case "right":
			// Sibling (content area) width should be from its current left to the TabBar's new left
			mainContentSibling.RenderW = maxF(1.0, el.RenderX - mainContentSibling.RenderX)
			// Sibling height and Y position usually remain unchanged
		}
		// Ensure non-negative dimensions for sibling after adjustment
		mainContentSibling.RenderW = maxF(0, mainContentSibling.RenderW)
		mainContentSibling.RenderH = maxF(0, mainContentSibling.RenderH)

		log.Printf("DEBUG TabBarHandler [%s]: Sibling [%s] frame adjusted from (X:%.1f,Y:%.1f W:%.1fxH:%.1f) to (X:%.1f,Y:%.1f W:%.1fxH:%.1f)",
			elIDStr, siblingIDStr,
			origSiblingX, origSiblingY, origSiblingW, origSiblingH,
			mainContentSibling.RenderX, mainContentSibling.RenderY, mainContentSibling.RenderW, mainContentSibling.RenderH)
	} else {
		log.Printf("DEBUG TabBarHandler [%s]: No distinct sibling found to adjust, or TabBar is only child.", elIDStr)
	}

	// --- Re-Layout TabBar's Own Children ---
	// The TabBar (el) itself is the parent for its children.
	// Its RenderX, RenderY, RenderW, RenderH have now been adjusted.
	// PerformLayoutChildren will use these as the parent's bounds and apply el.Padding and el.BorderWidths
	// (scaled by the 'childLayoutScaleFactor') to determine the client area for el's children.

	// Attempt to get the global scale factor. This is a workaround as the handler interface
	// doesn't provide it directly. The best solution is to modify the interface.
	var childLayoutScaleFactor float32 = 1.0 // Default if not found
	if doc != nil && (doc.Header.Flags&krb.FlagHasApp) != 0 && doc.Header.ElementCount > 0 && len(doc.Properties) > 0 {
		// Try to derive from App element's config if it exists at index 0
		// This assumes the first element [0] is the App and has properties.
		// And that Properties[0] belongs to the App element.
		appConfig := render.DefaultWindowConfig() // Start with defaults
		if len(doc.Properties[0]) > 0 {
			// applyDirectPropertiesToConfig expects properties for the App element.
			// This assumes doc.Properties[0] are the App's direct properties.
			applyDirectPropertiesToConfig(doc.Properties[0], doc, &appConfig)
			childLayoutScaleFactor = appConfig.ScaleFactor
		}
	}
	// Ensure scale factor is at least 1.0
	childLayoutScaleFactor = maxF(1.0, childLayoutScaleFactor)

	log.Printf("DEBUG TabBarHandler [%s]: Relaying out its own children within its new frame (X:%.1f,Y:%.1f W:%.1fxH:%.1f). Effective scale for children: %.2f",
		elIDStr, el.RenderX, el.RenderY, el.RenderW, el.RenderH, childLayoutScaleFactor)

	// Call the exported PerformLayoutChildren function from raylib_renderer.go
	// Pass `el` as the parent, its newly adjusted RenderX/Y/W/H as the space for its children,
	// the derived scale factor, and the document.
	if len(el.Children) > 0 {
		PerformLayoutChildren(el, el.RenderX, el.RenderY, el.RenderW, el.RenderH, childLayoutScaleFactor, doc)
	} else {
		log.Printf("DEBUG TabBarHandler [%s]: TabBar has no children to re-layout.", elIDStr)
	}

	return nil
}

// Note: maxF, GetCustomPropertyValue, PerformLayoutChildren, applyDirectPropertiesToConfig
// are expected to be defined and exported in the raylib_renderer.go file within the same package.