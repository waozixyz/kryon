// render/raylib/custom_tabbar.go
package raylib

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
	// rl "github.com/gen2brain/raylib-go/raylib" // Not needed directly
)

// TabBarHandler implements CustomComponentHandler for TabBar components.
type TabBarHandler struct{}

// HandleLayoutAdjustment adjusts the TabBar's position based on the 'position'
// custom property and resizes its sibling (assumed main content area).
func (h *TabBarHandler) HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error {
	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)
	if el == nil { return fmt.Errorf("tabBar %s: received nil element", elIDStr) }
	if el.Parent == nil { return fmt.Errorf("tabBar %s: cannot adjust layout without a parent", elIDStr) }
	if doc == nil { return fmt.Errorf("tabBar %s: KRB document is nil", elIDStr) }

	// Get necessary custom properties
	position, posOk := getCustomPropertyValue(el, "position", doc)
	if !posOk { position = "bottom" } // Default if property missing
	orientation, orientOk := getCustomPropertyValue(el, "orientation", doc)
	if !orientOk { orientation = "row" } // Default if property missing

	parent := el.Parent
	parentIDStr := fmt.Sprintf("Elem %d", parent.OriginalIndex)
	parentW, parentH := parent.RenderW, parent.RenderH
	parentX, parentY := parent.RenderX, parent.RenderY

	// Use the size calculated by standard layout
	initialW, initialH := el.RenderW, el.RenderH
	initialX, initialY := el.RenderX, el.RenderY

	log.Printf("DEBUG TabBarHandler [%s]: Adjusting. Pos:'%s' | Initial Frame: %d,%d %dx%d | Parent [%s] Frame: %d,%d %dx%d",
		elIDStr, position, initialX, initialY, initialW, initialH, parentIDStr, parentX, parentY, parentW, parentH)

	// Calculate New Position & Size based on 'position' property
	newX, newY, newW, newH := initialX, initialY, initialW, initialH
	stretchWidth := (orientation == "row")
	stretchHeight := (orientation == "column")

	switch strings.ToLower(position) {
	case "top":
		newY = parentY; newX = parentX; if stretchWidth { newW = parentW }
	case "bottom":
		newY = parentY + parentH - initialH; newX = parentX; if stretchWidth { newW = parentW }
	case "left":
		newX = parentX; newY = parentY; if stretchHeight { newH = parentH }
	case "right":
		newX = parentX + parentW - initialW; newY = parentY; if stretchHeight { newH = parentH }
	default:
		log.Printf("Warn TabBarHandler [%s]: Unknown position '%s'. Defaulting bottom.", elIDStr, position)
		newY = parentY + parentH - initialH; newX = parentX; if stretchWidth { newW = parentW }
	}

	finalW := max(1, newW); finalH := max(1, newH)
	frameChanged := (newX != el.RenderX || newY != el.RenderY || finalW != el.RenderW || finalH != el.RenderH)

	// Store the final calculated frame before potentially skipping
	calculatedX, calculatedY, calculatedW, calculatedH := newX, newY, finalW, finalH

	// Apply the new frame to the TabBar element IF it changed
	if frameChanged {
		el.RenderX = calculatedX
		el.RenderY = calculatedY
		el.RenderW = calculatedW
		el.RenderH = calculatedH
		log.Printf("DEBUG TabBarHandler [%s]: Frame *will be* modified.", elIDStr)
	} else {
		log.Printf("DEBUG TabBarHandler [%s]: Frame unchanged by custom adjustment. Skipping sibling/child updates.", elIDStr)
		return nil // No changes needed
	}


	// --- Adjust Siblings ---
	log.Printf("DEBUG TabBarHandler [%s]: Adjusting siblings to accommodate frame (%d,%d %dx%d)", elIDStr, el.RenderX, el.RenderY, el.RenderW, el.RenderH)
	var mainContentSibling *render.RenderElement = nil
	for _, sibling := range parent.Children {
		if sibling != nil && sibling != el { mainContentSibling = sibling; break } // Simple: first non-self sibling is content
	}

	if mainContentSibling != nil {
		siblingIDStr := fmt.Sprintf("Elem %d", mainContentSibling.OriginalIndex)
		log.Printf("DEBUG TabBarHandler [%s]: Found main content sibling [%s] to adjust.", elIDStr, siblingIDStr)

		// Adjust sibling based on position (Simplified for column layout)
		switch strings.ToLower(position) {
		case "bottom":
			originalSiblingH := mainContentSibling.RenderH
			// Use el.RenderY which now holds the calculated top Y of the bottom bar
			newSiblingH := max(1, el.RenderY-mainContentSibling.RenderY) // New height is distance from sibling top to tab top
			if newSiblingH != originalSiblingH {
				mainContentSibling.RenderH = newSiblingH
				log.Printf("DEBUG TabBarHandler [%s]: Resized sibling [%s] height from %d to %d.", elIDStr, siblingIDStr, originalSiblingH, newSiblingH)
			}
		case "top":
			originalSiblingY := mainContentSibling.RenderY
			originalSiblingH := mainContentSibling.RenderH
			// Use el.RenderY and el.RenderH which hold the calculated top bar frame
			newSiblingY := el.RenderY + el.RenderH // New top is below the tab bar
			// *** FIX: Correct calculation for new height ***
			newSiblingH := max(1, (originalSiblingY+originalSiblingH)-newSiblingY) // Original bottom edge - new top edge
			if newSiblingY != originalSiblingY || newSiblingH != originalSiblingH {
				mainContentSibling.RenderY = newSiblingY
				mainContentSibling.RenderH = newSiblingH
				log.Printf("DEBUG TabBarHandler [%s]: Adjusted sibling [%s] pos Y %d->%d / height %d->%d for top bar.",
					elIDStr, siblingIDStr, originalSiblingY, newSiblingY, originalSiblingH, newSiblingH)
			}
		// TODO: Add logic for "left", "right" if needed (adjust X and Width)
		default:
			log.Printf("DEBUG TabBarHandler [%s]: Sibling adjustment not implemented for position '%s'.", elIDStr, position)
		}
	}
	// --- Re-Layout TabBar's Own Children ---
	// Assume scale = 1.0 for simplicity here. Pass scale factor if needed.
	scaleFactor := float32(1.0)
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scaleFactor))) }
	borderL := scaledU8(el.BorderWidths[3]); borderT := scaledU8(el.BorderWidths[0])
	borderR := scaledU8(el.BorderWidths[1]); borderB := scaledU8(el.BorderWidths[2])
	newClientAbsX := el.RenderX + borderL; newClientAbsY := el.RenderY + borderT
	newClientWidth := max(0, el.RenderW-borderL-borderR)
	newClientHeight := max(0, el.RenderH-borderT-borderB)

	// PerformLayoutChildren needs access to the original PerformLayout function.
	// Assuming PerformLayout is accessible in this package scope.
	PerformLayoutChildren(el, newClientAbsX, newClientAbsY, newClientWidth, newClientHeight, scaleFactor, doc)


	return nil
} // End HandleLayoutAdjustment

// --- Helper Functions (max already exists elsewhere) ---