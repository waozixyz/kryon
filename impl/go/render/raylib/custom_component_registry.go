// render/raylib/custom_component_registry.go
package raylib

import (
	"fmt"
	"log"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

// Convention Key for identifying the original component name in KRB custom properties.
// The compiler must add a custom property with this key and the component's Define name as the value.
const componentNameConventionKey = "_componentName"

// CustomComponentHandler defines the interface for implementing custom component logic
// within a specific renderer (like Raylib). Renderers will call these methods
// at appropriate times during the prepare, layout, draw, or event handling phases.
type CustomComponentHandler interface {
	// HandleLayoutAdjustment allows the handler to modify the element's layout
	// (RenderX/Y/W/H) and potentially its children's layout *after* the
	// standard layout pass has calculated initial values.
	HandleLayoutAdjustment(el *render.RenderElement, doc *krb.Document) error

	// --- Future/Optional Methods ---
	// Prepare allows the handler to perform initial setup for a specific
	// element instance when it's first processed (e.g., in PrepareTree).
	// Prepare(el *render.RenderElement, doc *krb.Document) error

	// Draw allows completely custom drawing logic for the component, potentially
	// replacing the standard drawing of the underlying element.
	// Draw(el *render.RenderElement, scale float32, rendererInstance render.Renderer) (skipStandardDraw bool, err error)

	// HandleEvent allows the component to react to specific input events
	// before or instead of standard event handlers attached via KRB events.
	// HandleEvent(el *render.RenderElement, eventType krb.EventType /* + other event details */) (handled bool, err error)
}

// --- Registry for Custom Component Handlers ---

// customComponentRegistry maps component identifiers (like "TabBar", "MarkdownView")
// defined in `.kry` via `Define` (and stored in KRB via the `_componentName` convention)
// to their corresponding Go handler implementations.
//
// NOTE: This is currently a global registry. For applications embedding Kryon,
// it might be preferable to make this registry part of the `RaylibRenderer` instance
// to avoid potential global state issues if multiple Kryon renderers were used
// simultaneously (though unlikely for Raylib).
var customComponentRegistry = make(map[string]CustomComponentHandler)

// RegisterCustomComponent adds or replaces a handler for a given component identifier.
// This should be called by the application *before* `PrepareTree` is called on the renderer.
func RegisterCustomComponent(identifier string, handler CustomComponentHandler) {
	if identifier == "" || handler == nil {
		log.Printf("WARN: Attempted to register invalid custom component handler (identifier: '%s', handler nil: %t)", identifier, handler == nil)
		return
	}
	if _, exists := customComponentRegistry[identifier]; exists {
		log.Printf("INFO: Overwriting existing custom component handler for identifier '%s'", identifier)
	}
	customComponentRegistry[identifier] = handler
	log.Printf("Registered custom component handler for '%s'", identifier)
}

// --- Dispatch Function ---

// ApplyCustomComponentLayoutAdjustments iterates through all rendered elements,
// explicitly identifies elements that originated from custom components using the
// `_componentName` convention, finds the registered handler, and calls its
// `HandleLayoutAdjustment` method. This allows custom components to modify their
// layout after the standard layout calculations are done.
func ApplyCustomComponentLayoutAdjustments(elements []*render.RenderElement, doc *krb.Document) {
	if doc == nil {
		// Cannot lookup custom properties without the document context (string table etc.)
		// log.Println("DEBUG CustomAdjust: KRB document is nil, skipping custom adjustments.")
		return
	}

	// Skip if no handlers are registered at all.
	if len(customComponentRegistry) == 0 {
		// log.Printf("DEBUG CustomAdjust: Custom component registry is empty. No adjustments will be applied.")
		return
	}

	// Iterate through all elements in the render tree.
	for _, el := range elements {
		if el == nil {
			continue // Skip nil elements if any exist.
		}

		// --- Explicit Component Identification ---
		// Look for the special "_componentName" custom property added by the compiler.
		componentIdentifier, found := getCustomPropertyValue(el, componentNameConventionKey, doc)

		if found && componentIdentifier != "" {
			// We found an element explicitly marked as a custom component instance (e.g., "TabBar").

			// --- Lookup and Dispatch Handler ---
			handler, handlerFound := customComponentRegistry[componentIdentifier] // Look up using the identified name.
			elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)                   // Element identifier for logging.

			if handlerFound {
				// A handler is registered for this component type. Call its layout adjustment method.
				// log.Printf("DEBUG CustomAdjust [%s]: Found handler for component '%s', applying layout adjustment.", elIDStr, componentIdentifier)
				err := handler.HandleLayoutAdjustment(el, doc) // <<< CALL HANDLER METHOD >>>
				if err != nil {
					// Log errors returned by the handler.
					log.Printf("ERROR: Custom layout handler for '%s' [%s] failed: %v", componentIdentifier, elIDStr, err)
				}
			} else {
				// The element is marked as a custom component, but no Go handler was registered for it.
				// This might be intentional (if only standard properties are used) or an application setup error.
				// log.Printf("DEBUG CustomAdjust [%s]: Component name '%s' found, but no layout handler registered.", elIDStr, componentIdentifier)
			}
		}
		// --- End Identification & Dispatch ---

	} // End loop through elements
}

// --- Helper Function ---

// getCustomPropertyValue retrieves the string value of a custom property associated with a RenderElement.
// It looks up the key name in the document's string table, then searches the element's
// KrbCustomProperty list for a matching key index.
// It expects the value to be stored as a ValTypeString or ValTypeResource (which both contain a 1-byte string table index).
// Returns the resolved string value and a boolean indicating if the property was found with the expected type/size.
func getCustomPropertyValue(el *render.RenderElement, keyName string, doc *krb.Document) (string, bool) {
	// Basic validation.
	if doc == nil || el == nil {
		return "", false
	}

	// Find the string table index for the requested key name.
	var targetKeyIndex uint8 = 0xFF // Use 0xFF as a sentinel for "not found".
	for idx, str := range doc.Strings {
		if str == keyName {
			targetKeyIndex = uint8(idx)
			break
		}
	}
	// If the key name itself isn't in the string table, the property cannot exist.
	if targetKeyIndex == 0xFF {
		// log.Printf("DEBUG getCustomPropertyValue: Key '%s' not found in string table.", keyName)
		return "", false
	}

	// Check if the CustomProperties data exists for this element's index.
	// Note: KRB parsing should populate this structure correctly.
	if el.OriginalIndex >= len(doc.CustomProperties) || doc.CustomProperties[el.OriginalIndex] == nil {
		// log.Printf("DEBUG getCustomPropertyValue: No custom props map/slice entry for Elem %d.", el.OriginalIndex)
		return "", false
	}
	customProps := doc.CustomProperties[el.OriginalIndex] // Get the slice of custom props for this element.
	if len(customProps) == 0 {
		// log.Printf("DEBUG getCustomPropertyValue: Custom props slice is empty for Elem %d.", el.OriginalIndex)
		return "", false // No custom props defined for this element.
	}

	// Iterate through the custom properties defined for this specific element.
	for _, prop := range customProps {
		// Check if the property's key index matches the one we're looking for.
		if prop.KeyIndex == targetKeyIndex {
			// Found the key. Now check if the value type and size match our expectation
			// for retrieving a string value (which is stored as a string table index).
			if (prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource) && prop.Size == 1 {
				// Correct type and size. The value is a 1-byte index into the string table.
				valueIndex := prop.Value[0]
				// Validate the index against the actual string table size.
				if int(valueIndex) < len(doc.Strings) {
					// Valid index, return the corresponding string and true.
					return doc.Strings[valueIndex], true
				} else {
					// Index is out of bounds, log a warning.
					log.Printf("WARN getCustomPropertyValue: Custom prop '%s' (KeyIdx:%d) for Elem %d has invalid value string index %d (String Table Size: %d).", keyName, targetKeyIndex, el.OriginalIndex, valueIndex, len(doc.Strings))
				}
			} else {
				// Found the key, but the value isn't stored as expected (e.g., it's a number, color, or different size).
				log.Printf("WARN getCustomPropertyValue: Custom prop '%s' (KeyIdx:%d) for Elem %d has unexpected ValueType (%d) or Size (%d). Cannot retrieve as string index.", keyName, targetKeyIndex, el.OriginalIndex, prop.ValueType, prop.Size)
			}
			// Found the key but couldn't get the value as expected. Return false.
			return "", false
		}
	}

	// The property key was not found in the list of custom properties for this element.
	return "", false
}