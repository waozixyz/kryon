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

// HandleLayoutAdjustment adjusts the TabBar's position based on custom properties
// AND triggers a re-layout of its children within the new parent frame.
func (h *TabBarHandler) HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error {
	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex) // Basic ID for logging
	if el == nil { return fmt.Errorf("tabBar Elem %d: received nil element", el.OriginalIndex) }
	if el.Parent == nil { return fmt.Errorf("tabBar Elem %d: cannot adjust layout without a parent", el.OriginalIndex) }
	if doc == nil { return fmt.Errorf("tabBar Elem %d: KRB document is nil", el.OriginalIndex) } // Need doc for PerformLayoutChildren

	// Get necessary custom properties
	position, posOk := getCustomPropertyValue(el, "position", doc); if !posOk { position = "bottom" }
	orientation, orientOk := getCustomPropertyValue(el, "orientation", doc); if !orientOk { orientation = "row" }

	parent := el.Parent
	parentW, parentH := parent.RenderW, parent.RenderH
	parentX, parentY := parent.RenderX, parent.RenderY
	elW, elH := el.RenderW, el.RenderH // Size from standard layout

	// Calculate New Position & Size (as before)
	newX, newY, newW, newH := el.RenderX, el.RenderY, elW, elH // Start with current values from standard layout
	stretchWidth := (orientation == "row")
	stretchHeight := (orientation == "column")
	switch strings.ToLower(position) {
	case "top":    newY = parentY; newX = parentX; if stretchWidth { newW = parentW }
	case "bottom": newY = parentY + parentH - elH; newX = parentX; if stretchWidth { newW = parentW }
	case "left":   newX = parentX; newY = parentY; if stretchHeight { newH = parentH }
	case "right":  newX = parentX + parentW - elW; newY = parentY; if stretchHeight { newH = parentH }
	default:       log.Printf("Warn TabBarHandler [%s]: Unknown position '%s'. Defaulting bottom.", elIDStr, position); newY = parentY + parentH - elH; newX = parentX; if stretchWidth { newW = parentW }
	}

	// Check if the frame actually changed
	frameChanged := (newX != el.RenderX || newY != el.RenderY || newW != el.RenderW || newH != el.RenderH)

	// Apply New Position & Size to the TabBar element itself
	el.RenderX = newX
	el.RenderY = newY
	el.RenderW = max(1, newW)
	el.RenderH = max(1, newH)


	// ===========================================================
	// >>>>>>>>>> MODIFY THIS PART: Re-Layout Children <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
	//
	// If the parent's frame was modified by the custom adjustment,
	// we need to re-calculate the layout of its children based on the *new* parent frame.
	if frameChanged {
		log.Printf("DEBUG TabBarHandler [%s]: Frame changed by custom adjustment. Re-laying out children...", elIDStr)

		// Need scale factor - how to get it here? Add it to RenderElement or pass Renderer?
		// For now, assume scale = 1.0 if not accessible. A better solution is needed.
		// TODO: Pass scale factor or renderer to HandleLayoutAdjustment if scale != 1.0 is needed.
		scaleFactor := float32(1.0) // <---- CAVEAT: Assumes scale 1.0!

		// Calculate the NEW client area origin and size based on the adjusted parent frame
		scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scaleFactor))) } // Use local scale factor
		borderL := scaledU8(el.BorderWidths[3]); borderT := scaledU8(el.BorderWidths[0])
		borderR := scaledU8(el.BorderWidths[1]); borderB := scaledU8(el.BorderWidths[2])

		// Use the NEW RenderX/Y for the client origin
		newClientAbsX := el.RenderX + borderL
		newClientAbsY := el.RenderY + borderT
		// Use the NEW RenderW/H for the client size
		newClientWidth := max(0, el.RenderW - borderL - borderR)
		newClientHeight := max(0, el.RenderH - borderT - borderB)

		log.Printf("DEBUG TabBarHandler [%s]: Calling PerformLayoutChildren with NEW Parent Frame: Origin=%d,%d Size=%dx%d",
			elIDStr, newClientAbsX, newClientAbsY, newClientWidth, newClientHeight)

		// Call PerformLayoutChildren again for THIS element, using its NEW frame
		// This will recursively call PerformLayout for its children, calculating
		// their positions relative to the NEW parent content area.
		PerformLayoutChildren(el, newClientAbsX, newClientAbsY, newClientWidth, newClientHeight, scaleFactor, doc)

	} else {
		// Optional log if frame didn't change
		// log.Printf("DEBUG TabBarHandler [%s]: Frame unchanged by custom adjustment. Skipping child re-layout.", elIDStr)
	}
	//
	// ===========================================================


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
