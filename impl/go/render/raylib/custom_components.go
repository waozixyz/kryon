// render/raylib/custom_components.go
package raylib

import (
	"fmt"
	"log"
	"strings"
	"math"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

// CustomComponentHandler defines the interface for handling custom component logic.
type CustomComponentHandler interface {
	// Prepare allows initialization or resource loading based on the element's KRB data.
	// This could be called during the PrepareTree phase if needed.
	// Prepare(el *render.RenderElement, doc *krb.Document, renderer *RaylibRenderer) error

	// HandleLayoutAdjustment applies layout modifications *after* standard layout.
	HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error
}

// --- Registry for Custom Component Handlers ---

var customComponentRegistry = make(map[string]CustomComponentHandler)

// RegisterCustomComponent links a component identifier (e.g., original tag name)
// to its specific handler implementation. Call this during application setup.
func RegisterCustomComponent(identifier string, handler CustomComponentHandler) {
	if identifier == "" || handler == nil {
		log.Printf("WARN: Attempted to register invalid custom component handler for identifier '%s'", identifier)
		return
	}
	if _, exists := customComponentRegistry[identifier]; exists {
		log.Printf("WARN: Overwriting custom component handler for identifier '%s'", identifier)
	}
	customComponentRegistry[identifier] = handler
	log.Printf("Registered custom component handler for '%s'", identifier)
}

// --- Dispatch Function ---
// ApplyCustomComponentLayoutAdjustments iterates through elements, identifies known
// custom components via the registry, and calls their layout adjustment handlers.
func ApplyCustomComponentLayoutAdjustments(elements []*render.RenderElement, doc *krb.Document) {
	if doc == nil { return }

	log.Printf("DEBUG CustomAdjust: == Starting Custom Adjustment Pass for %d elements ==", len(elements)) // Keep this log

	// --- Added: Check if registry is populated ---
	if len(customComponentRegistry) == 0 {
		log.Printf("WARN CustomAdjust: Custom component registry is empty. No handlers registered?")
	} else {
		// Optional: Log registered handlers
		// registeredKeys := make([]string, 0, len(customComponentRegistry))
		// for k := range customComponentRegistry { registeredKeys = append(registeredKeys, k) }
		// log.Printf("DEBUG CustomAdjust: Registered handlers: %v", registeredKeys)
	}
	// --- End Added ---


	for _, el := range elements {
        // --- Added: Log element being checked ---
        if el == nil { continue } // Safety first
        elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)
        log.Printf("DEBUG CustomAdjust: Checking %s...", elIDStr)
        // --- End Added ---

		// --- Identify the Component ---
		identifier := ""
		// Try to identify specifically as TabBar
		posVal, posOk := getCustomPropertyValue(el, "position", doc) // Check for "position" prop
        // --- Added: Log property lookup result ---
        log.Printf("DEBUG CustomAdjust [%s]: Checked for 'position' custom prop. Found: %t, Value: '%s'", elIDStr, posOk, posVal)
        // --- End Added ---

		if posOk { // If "position" property exists, assume it's a TabBar for now
			identifier = "TabBar"
            log.Printf("DEBUG CustomAdjust [%s]: Identified as potential '%s' based on 'position' prop.", elIDStr, identifier) // Added log
		} else {
            // Optional: Add checks for other component identifiers here
            // log.Printf("DEBUG CustomAdjust [%s]: Did not find 'position' prop.", elIDStr) // Optional log
        }


		// --- Lookup and Dispatch ---
		if identifier != "" {
			handler, found := customComponentRegistry[identifier]
            // --- Added: Log handler lookup ---
            log.Printf("DEBUG CustomAdjust [%s]: Looking up handler for '%s'. Found: %t", elIDStr, identifier, found)
            // --- End Added ---

			if found {
				log.Printf("DEBUG CustomAdjust [%s]: Calling HandleLayoutAdjustment for '%s'...", elIDStr, identifier) // Added log
				preAdjustX, preAdjustY, preAdjustW, preAdjustH := el.RenderX, el.RenderY, el.RenderW, el.RenderH // Store pre-state

				err := handler.HandleLayoutAdjustment(el, doc) // CALL THE HANDLER

				// --- Added: Log post-adjustment results ---
				if err != nil {
					log.Printf("ERROR: Custom handler for '%s' [%s] failed: %v", identifier, elIDStr, err)
				} else {
					if el.RenderX != preAdjustX || el.RenderY != preAdjustY || el.RenderW != preAdjustW || el.RenderH != preAdjustH {
						log.Printf("DEBUG CustomAdjust [%s]: Handler '%s' MODIFIED frame: (%d,%d %dx%d) -> (%d,%d %dx%d)",
							elIDStr, identifier, preAdjustX, preAdjustY, preAdjustW, preAdjustH, el.RenderX, el.RenderY, el.RenderW, el.RenderH)
					} else {
						log.Printf("DEBUG CustomAdjust [%s]: Handler '%s' did NOT modify frame.", elIDStr, identifier)
					}
				}
				// --- End Added Log ---

			} else {
				// Identified as potentially custom, but no handler registered
                log.Printf("WARN CustomAdjust [%s]: Identified as '%s', but NO HANDLER registered for this identifier.", elIDStr, identifier) // Added log
			}
		}
		// else { log.Printf("DEBUG CustomAdjust [%s]: Not identified as a known custom component.", elIDStr) } // Optional log
	}
	log.Printf("DEBUG CustomAdjust: == Custom Adjustment Pass Complete ==") // Keep this log
}

// --- Specific Component Implementations ---

// TabBarHandler implements CustomComponentHandler for TabBar components.
type TabBarHandler struct{}

// Prepare (Optional): Could pre-load icons or set defaults if needed.
// func (h *TabBarHandler) Prepare(el *render.RenderElement, doc *krb.Document, renderer *RaylibRenderer) error {
//     // ... implementation ...
//     return nil
// }
func (h *TabBarHandler) HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error {
	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)
	if el == nil { return fmt.Errorf("tabBar %s: received nil element", elIDStr) }
	if el.Parent == nil { return fmt.Errorf("tabBar %s: cannot adjust layout without a parent", elIDStr) }
	if doc == nil { return fmt.Errorf("tabBar %s: KRB document is nil", elIDStr) }

	// Get necessary custom properties
	position, posOk := getCustomPropertyValue(el, "position", doc); if !posOk { position = "bottom" }
	orientation, orientOk := getCustomPropertyValue(el, "orientation", doc); if !orientOk { orientation = "row" }

	parent := el.Parent
	parentIDStr := fmt.Sprintf("Elem %d", parent.OriginalIndex)
	parentW, parentH := parent.RenderW, parent.RenderH
	parentX, parentY := parent.RenderX, parent.RenderY

	// Use the size calculated by standard layout (which includes style height/width)
	// If auto-sized, PerformLayout should have updated RenderW/H based on children.
	initialW, initialH := el.RenderW, el.RenderH
	initialX, initialY := el.RenderX, el.RenderY // Store initial pos calculated by standard layout

	log.Printf("DEBUG TabBarHandler [%s]: Adjusting. Pos:'%s' | Initial Frame: %d,%d %dx%d | Parent [%s] Frame: %d,%d %dx%d",
		elIDStr, position, initialX, initialY, initialW, initialH, parentIDStr, parentX, parentY, parentW, parentH)

	// Calculate New Position & Size based on 'position' property
	newX, newY, newW, newH := initialX, initialY, initialW, initialH // Start with standard layout results
	stretchWidth := (orientation == "row")
	stretchHeight := (orientation == "column")

	switch strings.ToLower(position) {
	case "top":
		newY = parentY // Align Y to parent top
		newX = parentX // Align X to parent left
		if stretchWidth { newW = parentW }
	case "bottom":
		// Calculate Y position based on parent bottom edge and TabBar's height
		newY = parentY + parentH - initialH // Use initialH calculated from standard layout/style
		newX = parentX
		if stretchWidth { newW = parentW }
	case "left":
		newX = parentX; newY = parentY; if stretchHeight { newH = parentH }
	case "right":
		newX = parentX + parentW - initialW; newY = parentY; if stretchHeight { newH = parentH }
	default:
		log.Printf("Warn TabBarHandler [%s]: Unknown position '%s'. Defaulting bottom.", elIDStr, position)
		newY = parentY + parentH - initialH; newX = parentX; if stretchWidth { newW = parentW }
	}

	// Apply final size (ensuring minimum 1x1)
	finalW := max(1, newW)
	finalH := max(1, newH)

	// Check if the frame actually needs modification
	frameChanged := (newX != el.RenderX || newY != el.RenderY || finalW != el.RenderW || finalH != el.RenderH)

	if !frameChanged {
		log.Printf("DEBUG TabBarHandler [%s]: Frame unchanged by custom adjustment. Skipping updates.", elIDStr)
		return nil // No changes needed
	}

	// Apply the new frame to the TabBar element
	el.RenderX = newX
	el.RenderY = newY
	el.RenderW = finalW
	el.RenderH = finalH


	// --- >>> Adjust Siblings <<< ---
	// Find siblings (other children of the SAME parent) and adjust their size/pos
	// This is a simple implementation assuming one main growing content area sibling.
	log.Printf("DEBUG TabBarHandler [%s]: Adjusting siblings to accommodate frame (%d,%d %dx%d)", elIDStr, el.RenderX, el.RenderY, el.RenderW, el.RenderH)
	var mainContentSibling *render.RenderElement = nil // Assuming one primary sibling to adjust

	for _, sibling := range parent.Children {
		if sibling == nil || sibling == el { continue } // Skip self and nil siblings
		// Simple heuristic: assume the non-TabBar sibling is the main content
        // TODO: Make this more robust if multiple siblings exist (e.g., check for 'grow' flag?)
        mainContentSibling = sibling
        break // Found the presumed sibling
	}

	if mainContentSibling != nil {
        siblingIDStr := fmt.Sprintf("Elem %d", mainContentSibling.OriginalIndex)
        log.Printf("DEBUG TabBarHandler [%s]: Found main content sibling [%s] to adjust.", elIDStr, siblingIDStr)

        // Reduce height of the sibling to make space for the bottom TabBar
        // Assumption: App layout is Column.
        if strings.ToLower(position) == "bottom" {
			originalSiblingH := mainContentSibling.RenderH
            // The new height should be the parent's height minus the TabBar's height
            // Or more simply, its bottom edge should be the TabBar's top edge.
            newSiblingH := el.RenderY - mainContentSibling.RenderY // Y is top edge
			newSiblingH = max(1, newSiblingH) // Ensure minimum height
            if newSiblingH != originalSiblingH {
                mainContentSibling.RenderH = newSiblingH
			    log.Printf("DEBUG TabBarHandler [%s]: Resized sibling [%s] height from %d to %d.", elIDStr, siblingIDStr, originalSiblingH, newSiblingH)
                // TODO: If sibling height changed, might need to re-layout ITS children too! (Recursive complexity)
                // For now, we assume simple text content in the sibling doesn't need re-layout.
            }
        }
        // TODO: Add similar logic for "top", "left", "right" positions, adjusting sibling width/height/pos accordingly.

	} else {
        log.Printf("WARN TabBarHandler [%s]: Could not find a sibling element to adjust.", elIDStr)
    }


	// --- >>> Re-Layout Children (As before) <<< ---
	// Recalculate layout for the TabBar's *own* children using its *new* final frame.
	log.Printf("DEBUG TabBarHandler [%s]: Re-laying out own children...", elIDStr)
	scaleFactor := float32(1.0) // <<< CAVEAT: Assumes scale 1.0! Pass if needed.
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scaleFactor))) }
	borderL := scaledU8(el.BorderWidths[3]); borderT := scaledU8(el.BorderWidths[0])
	borderR := scaledU8(el.BorderWidths[1]); borderB := scaledU8(el.BorderWidths[2])
	newClientAbsX := el.RenderX + borderL
	newClientAbsY := el.RenderY + borderT
	newClientWidth := max(0, el.RenderW - borderL - borderR)
	newClientHeight := max(0, el.RenderH - borderT - borderB)

	log.Printf("DEBUG TabBarHandler [%s]: Calling PerformLayoutChildren with NEW Parent Frame: Origin=%d,%d Size=%dx%d",
		elIDStr, newClientAbsX, newClientAbsY, newClientWidth, newClientHeight)
	PerformLayoutChildren(el, newClientAbsX, newClientAbsY, newClientWidth, newClientHeight, scaleFactor, doc)


	log.Printf("DEBUG TabBarHandler [%s]: Adjustment Complete. Final Frame: %d,%d %dx%d",
		elIDStr, el.RenderX, el.RenderY, el.RenderW, el.RenderH)

	return nil
} // End HandleLayoutAdjustment

// --- Add other component handlers here ---

// type DateTimePickerHandler struct {}
// func (h *DateTimePickerHandler) HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error { ... }

// --- Helper Functions ---

// getCustomPropertyValue retrieves the string value of a custom property.
// (Implementation remains the same as provided before)
func getCustomPropertyValue(el *render.RenderElement, keyName string, doc *krb.Document) (string, bool) {
	if doc == nil || el.OriginalIndex >= len(doc.CustomProperties) { return "", false }
	var targetKeyIndex uint8 = 0xFF
	for idx, str := range doc.Strings { if str == keyName { targetKeyIndex = uint8(idx); break } }
	if targetKeyIndex == 0xFF { return "", false } // Key not in string table

	customProps := doc.CustomProperties[el.OriginalIndex]
	for _, prop := range customProps {
		if prop.KeyIndex == targetKeyIndex {
			// TODO: Enhance this to handle different ValueTypes based on component needs
			if prop.ValueType == krb.ValTypeString && prop.Size == 1 {
				valueIndex := prop.Value[0]
				if int(valueIndex) < len(doc.Strings) {
					return doc.Strings[valueIndex], true // Return resolved string
				} else { log.Printf("Warn: Custom prop '%s' has invalid value string index %d for Elem %d", keyName, valueIndex, el.OriginalIndex) }
			} else { log.Printf("Warn: Custom prop '%s' for Elem %d is not ValTypeString/Size=1 (Type:%d, Size:%d). Cannot get string value.", keyName, el.OriginalIndex, prop.ValueType, prop.Size) }
			return "", false // Found key, but couldn't get value as expected string
		}
	}
	return "", false // Property key not found for this element
}

// max and min helpers (if not already available globally)
// func max(a, b int) int { if a > b { return a }; return b }
// func min(a, b int) int { if a < b { return a }; return b }
