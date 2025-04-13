// render/raylib/custom_component_registry.go
package raylib

import (
	"fmt"
	"log"
	// "math" // Not needed directly here
	// "strings" // Not needed directly here

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

// CustomComponentHandler defines the interface for handling custom component logic.
type CustomComponentHandler interface {
	HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error
	// Other methods like Prepare, Draw could be added later
}

// --- Registry for Custom Component Handlers ---
var customComponentRegistry = make(map[string]CustomComponentHandler)

// RegisterCustomComponent links a component identifier to its handler.
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
// ApplyCustomComponentLayoutAdjustments iterates through elements, identifies components, and calls handlers.
func ApplyCustomComponentLayoutAdjustments(elements []*render.RenderElement, doc *krb.Document) {
	if doc == nil {
		log.Println("WARN CustomAdjust: KRB document is nil, skipping custom adjustments.")
		return
	}

	if len(customComponentRegistry) == 0 {
		log.Printf("DEBUG CustomAdjust: Custom component registry is empty. No adjustments will be applied.")
	} else {
		registeredKeys := make([]string, 0, len(customComponentRegistry))
		for k := range customComponentRegistry { registeredKeys = append(registeredKeys, k) }
	}

	// Use index 'i' explicitly if needed for debugging, otherwise range is fine
	for _, el := range elements { // Correctly using range without index variable 'i'
		if el == nil { continue }

		identifier := ""
		elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)

		// --- Component Identification (Heuristic Based) ---
		_, posOk := getCustomPropertyValue(el, "position", doc)
		if posOk {
			identifier = "TabBar"
			log.Printf("DEBUG CustomAdjust [%s]: Identified as potential '%s' based on 'position' prop.", elIDStr, identifier)
		}

		if identifier == "" {
			_, srcOk := getCustomPropertyValue(el, "source", doc)
			if srcOk {
				identifier = "MarkdownView"
			}
		}
		// --- End Identification ---


		// --- Lookup and Dispatch ---
		if identifier != "" {
			handler, found := customComponentRegistry[identifier]

			if found {
				err := handler.HandleLayoutAdjustment(el, doc) // CALL HANDLER

				if err != nil {
					log.Printf("ERROR: Custom handler for '%s' [%s] failed: %v", identifier, elIDStr, err)
				}
			}
		}
	} // End loop through elements

}


// --- Helper Functions ---

// getCustomPropertyValue retrieves the string value of a custom property.
// Assumes the value type in KRB is ValTypeString or ValTypeResource (an index into the string table).
func getCustomPropertyValue(el *render.RenderElement, keyName string, doc *krb.Document) (string, bool) {
	if doc == nil || el == nil || el.OriginalIndex >= len(doc.CustomProperties) {
		return "", false
	}

	var targetKeyIndex uint8 = 0xFF
	for idx, str := range doc.Strings {
		if str == keyName {
			targetKeyIndex = uint8(idx)
			break
		}
	}
	if targetKeyIndex == 0xFF { return "", false } // Key not in string table

	// --- Check if custom properties slice exists for this element index ---
	if doc.CustomProperties[el.OriginalIndex] == nil {
	    // log.Printf("DEBUG getCustomPropertyValue: No custom props map entry for Elem %d.", el.OriginalIndex)
		return "", false
	}
	customProps := doc.CustomProperties[el.OriginalIndex]
	if len(customProps) == 0 { // Check if the slice is empty
	    // log.Printf("DEBUG getCustomPropertyValue: Custom props slice is empty for Elem %d.", el.OriginalIndex)
	    return "", false
	}
	// --- End Check ---


	for _, prop := range customProps {
		if prop.KeyIndex == targetKeyIndex {
			// Handle String Index or Resource Index (both store string index for value)
			if (prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource) && prop.Size == 1 {
				valueIndex := prop.Value[0]
				if int(valueIndex) < len(doc.Strings) {
					return doc.Strings[valueIndex], true // Return resolved string
				} else {
					log.Printf("Warn getCustomPropertyValue: Custom prop '%s' has invalid value string index %d for Elem %d", keyName, valueIndex, el.OriginalIndex)
				}
			} else {
				log.Printf("Warn getCustomPropertyValue: Custom prop '%s' for Elem %d is not expected ValueType (Type:%d, Size:%d). Cannot get string value.", keyName, el.OriginalIndex, prop.ValueType, prop.Size)
			}
			return "", false // Found key, but couldn't get value as expected type
		}
	}
	return "", false // Property key not found for this element
}