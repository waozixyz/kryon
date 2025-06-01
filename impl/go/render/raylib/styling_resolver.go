// render/raylib/styling_resolver.go
package raylib

import (
	"log" // For debug logging

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

// --- Methods for Applying Properties to WindowConfig ---

func (r *RaylibRenderer) applyStylePropertiesToWindowConfig(
	props []krb.Property,
	doc *krb.Document, // Needed for getColorValue which uses doc.Header.Flags
	config *render.WindowConfig,
) {
	if doc == nil || config == nil {
		return
	}
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultBg = c
			}
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultFgColor = c
			}
		case krb.PropIDBorderColor: // Less common for window, but could be a theme default
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultBorderColor = c
			}
			// Add PropIDFontSize here if App style can set default font size
		}
	}
}

func (r *RaylibRenderer) applyDirectPropertiesToWindowConfig(
	props []krb.Property,
	doc *krb.Document,
	config *render.WindowConfig,
) {
	if config == nil || doc == nil {
		return
	}
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDWindowWidth:
			if w, ok := getShortValue(&prop); ok && w > 0 {
				config.Width = int(w)
			}
		case krb.PropIDWindowHeight:
			if h, ok := getShortValue(&prop); ok && h > 0 {
				config.Height = int(h)
			}
		case krb.PropIDWindowTitle:
			if strIdx := getFirstByteValue(&prop); strIdx != 0 || (len(doc.Strings) > 0 && doc.Strings[0] != "") {
				if s, ok := getStringValueByIdx(doc, strIdx); ok {
					config.Title = s
				}
			}
		case krb.PropIDResizable:
			if rVal, ok := getByteValue(&prop); ok {
				config.Resizable = (rVal != 0)
			}
		case krb.PropIDScaleFactor:
			if sfRaw, ok := getShortValue(&prop); ok && sfRaw > 0 {
				config.ScaleFactor = float32(sfRaw) / 256.0
			}
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultBg = c
			}
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultFgColor = c
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultBorderColor = c
			}
			// Add PropIDFontSize here if App direct props can set default font size
		}
	}
}

// --- Methods for Applying Properties to RenderElement ---

func (r *RaylibRenderer) applyStylePropertiesToElement(
	props []krb.Property,
	doc *krb.Document,
	el *render.RenderElement,
) {
	if doc == nil || el == nil {
		return
	}
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.FgColor = c
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BorderColor = c
			}
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok {
				el.BorderWidths = [4]uint8{bw, bw, bw, bw}
			} else if edges, okEdges := getEdgeInsetsValue(&prop); okEdges {
				el.BorderWidths = edges
			}
		case krb.PropIDPadding:
			if p, ok := getEdgeInsetsValue(&prop); ok {
				el.Padding = p
			}
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				el.TextAlignment = align
			}
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
			// TODO: Add font properties if specified (e.g., PropIDFontSize)
		}
	}
}

func (r *RaylibRenderer) applyDirectPropertiesToElement(
	props []krb.Property,
	doc *krb.Document,
	el *render.RenderElement,
) {
	// This includes visual properties that override styles, and content properties.
	if doc == nil || el == nil {
		return
	}
	for _, prop := range props {
		switch prop.ID {
		// Visual properties (override style)
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.FgColor = c
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BorderColor = c
			}
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok {
				el.BorderWidths = [4]uint8{bw, bw, bw, bw}
			} else if edges, okEdges := getEdgeInsetsValue(&prop); okEdges {
				el.BorderWidths = edges
			}
		case krb.PropIDPadding:
			if p, ok := getEdgeInsetsValue(&prop); ok {
				el.Padding = p
			}
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				el.TextAlignment = align
			}
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
		// Content properties
		case krb.PropIDTextContent:
			if strIdx, ok := getByteValue(&prop); ok {
				if textVal, textOk := getStringValueByIdx(doc, strIdx); textOk {
					el.Text = textVal
				}
			}
		case krb.PropIDImageSource:
			if resIdx, ok := getByteValue(&prop); ok {
				el.ResourceIndex = resIdx
			}
		// Window config properties are ignored here
		case krb.PropIDWindowWidth, krb.PropIDWindowHeight, krb.PropIDWindowTitle, krb.PropIDResizable, krb.PropIDScaleFactor:
			continue
			// TODO: Add font properties if specified (e.g., PropIDFontSize)
		}
	}
}

// applyDirectVisualPropertiesToAppElement is specifically for the App element's RenderElement representation.
func (r *RaylibRenderer) applyDirectVisualPropertiesToAppElement(
	props []krb.Property,
	doc *krb.Document,
	el *render.RenderElement, // The App element as a RenderElement
) {
	// This applies visual properties that style the "canvas" of the app.
	// Window configuration properties are handled by applyDirectPropertiesToWindowConfig.
	if doc == nil || el == nil {
		return
	}
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}
		case krb.PropIDFgColor: // App's direct FgColor can also style its own "text" if it had any directly
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.FgColor = c
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BorderColor = c
			}
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok {
				el.BorderWidths = [4]uint8{bw, bw, bw, bw}
			} else if edges, okEdges := getEdgeInsetsValue(&prop); okEdges {
				el.BorderWidths = edges
			}
		case krb.PropIDPadding:
			if p, ok := getEdgeInsetsValue(&prop); ok {
				el.Padding = p
			}
		case krb.PropIDVisibility: // Though App is usually always visible
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
		}
	}
}

// --- Methods for Resolving Content and Contextual Defaults ---

func (r *RaylibRenderer) resolveElementTextAndImage(
	doc *krb.Document,
	el *render.RenderElement,
	style *krb.Style, // Style resolved for this element
	styleFound bool,
) {
	if doc == nil || el == nil {
		return
	}

	// Text Content (PropIDTextContent)
	if el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton {
		// Text is already set if a direct property PropIDTextContent was applied by applyDirectPropertiesToElement.
		// If el.Text is still empty, try to get it from style.
		if el.Text == "" && styleFound && style != nil {
			if styleProp, propInStyleOk := getStylePropertyValue(style, krb.PropIDTextContent); propInStyleOk {
				if strIdx, ok := getByteValue(styleProp); ok {
					if s, textOk := getStringValueByIdx(doc, strIdx); textOk {
						el.Text = s
					}
				}
			}
		}
	}

	// Image Source (PropIDImageSource)
	if el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton {
		// ResourceIndex is already set if a direct property PropIDImageSource was applied.
		// If el.ResourceIndex is still InvalidResourceIndex, try to get it from style.
		if el.ResourceIndex == render.InvalidResourceIndex && styleFound && style != nil {
			if styleProp, propInStyleOk := getStylePropertyValue(style, krb.PropIDImageSource); propInStyleOk {
				if idx, ok := getByteValue(styleProp); ok {
					el.ResourceIndex = idx
				}
			}
		}
	}
}

func (r *RaylibRenderer) applyContextualDefaults(el *render.RenderElement) {
	if el == nil {
		return
	}

	// --- Border Default Logic (as per spec) ---
	hasBorderColor := el.BorderColor.A > 0 // Check if color is not fully transparent

	allBorderWidthsZero := true
	for _, bw := range el.BorderWidths {
		if bw > 0 {
			allBorderWidthsZero = false
			break
		}
	}

	if hasBorderColor && allBorderWidthsZero {
		// If border_color is set BUT no border_width is specified (all sides are 0):
		// Default border_width for all sides to 1.
		el.BorderWidths = [4]uint8{1, 1, 1, 1}
		// log.Printf("Debug Styling: Elem '%s' (idx %d) - Applied default border width 1px (color was set, width was 0).", el.SourceElementName, el.OriginalIndex)
	} else if !allBorderWidthsZero && !hasBorderColor {
		// If border_width is specified (>0 on any side) BUT no border_color is set (or transparent):
		// Default border_color to WindowConfig.DefaultBorderColor.
		el.BorderColor = r.config.DefaultBorderColor
		// log.Printf("Debug Styling: Elem '%s' (idx %d) - Applied default border color (width was set, color was not).", el.SourceElementName, el.OriginalIndex)
	}
}

// --- Methods for Property Inheritance ---

func (r *RaylibRenderer) resolvePropertyInheritance() {
	if len(r.roots) == 0 {
		return
	}
	log.Println("PrepareTree: Resolving property inheritance...")

	// These are the "document root" level inheritable style values.
	// They come from the final WindowConfig, which could have been influenced by App's style/props.
	initialFgColor := r.config.DefaultFgColor
	initialFontSize := r.config.DefaultFontSize // Assuming DefaultFontSize in WindowConfig
	// initialFontFamily := r.config.DefaultFontFamily
	// initialTextAlignment := uint8(krb.LayoutAlignStart) // Or from WindowConfig if made configurable

	for _, rootEl := range r.roots {
		// Apply/Resolve inheritable properties for the root element itself first.
		// If the root's FgColor is "unset" (transparent or Blank), it takes the initialFgColor.
		effectiveRootFgColor := rootEl.FgColor
		isTextBearingRoot := (rootEl.Header.Type == krb.ElemTypeText || rootEl.Header.Type == krb.ElemTypeButton || rootEl.Header.Type == krb.ElemTypeInput)

		if isTextBearingRoot && (rootEl.FgColor == rl.Blank || rootEl.FgColor.A == 0) {
			if initialFgColor.A > 0 { // Ensure initialFgColor is valid
				rootEl.FgColor = initialFgColor
			} else {
				rootEl.FgColor = rl.RayWhite // Ultimate fallback for root text
			}
		}
		effectiveRootFgColor = rootEl.FgColor // Use the now resolved FgColor of the root

		// TODO: Handle FontSize for root similarly, using initialFontSize
		// TODO: Handle TextAlignment for root, if it's considered inheritable from App level

		// Start recursion for children of this root
		r.applyInheritanceRecursive(rootEl, effectiveRootFgColor, initialFontSize /*, initialTextAlignment */)
	}
}

func (r *RaylibRenderer) applyInheritanceRecursive(
	el *render.RenderElement,
	inheritedFgColor rl.Color,
	inheritedFontSize float32,
	// inheritedTextAlignment uint8,
) {
	if el == nil {
		return
	}

	// --- 1. ForegroundColor (text_color) ---
	currentElFgColor := el.FgColor // Color set by element's own style/direct props
	isTextBearing := (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton || el.Header.Type == krb.ElemTypeInput)

	if isTextBearing {
		if currentElFgColor == rl.Blank || currentElFgColor.A == 0 { // If FgColor is "unset" for this text element
			if inheritedFgColor.A > 0 { // And parent/ancestor had a valid color
				el.FgColor = inheritedFgColor
			} else {
				// This case should be rare if App/WindowConfig.DefaultFgColor is always valid.
				el.FgColor = rl.RayWhite // Ultimate fallback for text elements
			}
		}
	}
	// The FgColor to pass to children is the one now resolved for 'el' (or what it inherited if non-text-bearing and unset)
	fgColorForChildren := el.FgColor
	if fgColorForChildren.A == 0 { // If still unset (e.g. non-text-bearing container)
		fgColorForChildren = inheritedFgColor // Pass down what this element inherited
	}

	// --- 2. FontSize ---
	// Assuming RenderElement has a ResolvedFontSize float32 field, or we use a temp var.
	// For now, let's assume direct setting of a property for font size if supported by KRB PropIDFontSize.
	// If el.ResolvedFontSize == 0 (or some "unset" sentinel)
	//    el.ResolvedFontSize = inheritedFontSize
	// fontSizeForChildren := el.ResolvedFontSize
	// If PropIDFontSize is used, applyDirectPropertiesToElement would have set it.
	// Inheritance would apply if that property was missing.
	// For this example, we'll just pass it down.
	fontSizeForChildren := inheritedFontSize // Placeholder - real logic depends on how font size is stored on RenderElement

	// --- 3. TextAlignment ---
	// currentElTextAlignment := el.TextAlignment (already set by style/direct or defaultLayoutAlignment)
	// if currentElTextAlignment is some "unset_alignment_sentinel"
	//    el.TextAlignment = inheritedTextAlignment
	// textAlignmentForChildren := el.TextAlignment

	// Recurse for children
	for _, child := range el.Children {
		r.applyInheritanceRecursive(child, fgColorForChildren, fontSizeForChildren /*, textAlignmentForChildren */)
	}
}
