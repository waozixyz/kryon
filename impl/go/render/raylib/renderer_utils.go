// render/raylib/renderer_utils.go
package raylib

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

func GetCustomPropertyValue(
	el *render.RenderElement,
	keyName string,
	doc *krb.Document,
) (string, bool) {

	if doc == nil || el == nil {
		return "", false
	}

	var targetKeyIndex uint8 = 0xFF
	keyFoundInStrings := false

	for idx, str := range doc.Strings {

		if str == keyName {
			targetKeyIndex = uint8(idx)
			keyFoundInStrings = true
			break
		}
	}

	if !keyFoundInStrings {
		return "", false
	}

	if el.OriginalIndex < 0 || el.OriginalIndex >= len(doc.CustomProperties) {
		return "", false
	}
	elementCustomProps := doc.CustomProperties[el.OriginalIndex]

	if elementCustomProps == nil || len(elementCustomProps) == 0 {
		return "", false
	}

	for _, prop := range elementCustomProps {

		if prop.KeyIndex == targetKeyIndex {

			if (prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource) && prop.Size == 1 {

				if len(prop.Value) == 1 {
					valueStringIndex := prop.Value[0]

					if int(valueStringIndex) < len(doc.Strings) {
						return doc.Strings[valueStringIndex], true
					}
					log.Printf(
						"WARN GetCustomPropertyValue: Custom prop key '%s' for element '%s', value string index %d out of bounds (len %d).",
						keyName, el.SourceElementName, valueStringIndex, len(doc.Strings),
					)
				} else {
					log.Printf(
						"WARN GetCustomPropertyValue: Custom prop key '%s' for element '%s', value data empty despite Size=1.",
						keyName, el.SourceElementName,
					)
				}
			} else {
				log.Printf(
					"WARN GetCustomPropertyValue: Custom prop key '%s' for element '%s', unexpected ValueType %X or Size %d.",
					keyName, el.SourceElementName, prop.ValueType, prop.Size,
				)
			}
			return "", false // Property found but malformed
		}
	}
	return "", false // Key not found
}

func applyStylePropertiesToWindowDefaults(
	props []krb.Property,
	doc *krb.Document,
	defaultBg *rl.Color,
) {

	if doc == nil || defaultBg == nil {
		return
	}

	for _, prop := range props {

		if prop.ID == krb.PropIDBgColor {

			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBg = c
			}
		}
	}
}

func applyStylePropertiesToElement(
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
		}
	}
}

func applyDirectVisualPropertiesToAppElement(
	props []krb.Property,
	doc *krb.Document,
	el *render.RenderElement,
) {

	for _, prop := range props {

		switch prop.ID {

		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}

		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
		}
	}
}

func applyDirectPropertiesToElement(
	props []krb.Property,
	doc *krb.Document,
	el *render.RenderElement,
) {

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

		case krb.PropIDWindowWidth, krb.PropIDWindowHeight, krb.PropIDWindowTitle, krb.PropIDResizable, krb.PropIDScaleFactor:
			continue // Handled by applyDirectPropertiesToConfig or not applicable here
		}
	}
}

func applyDirectPropertiesToConfig(
	props []krb.Property,
	doc *krb.Document,
	config *render.WindowConfig,
) {

	if config == nil {
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
		}
	}
}

func resolveElementText(
	doc *krb.Document,
	el *render.RenderElement,
	style *krb.Style,
	styleOk bool,
) {

	if el.Header.Type != krb.ElemTypeText && el.Header.Type != krb.ElemTypeButton {
		return
	}

	if el.Text != "" { // Already set
		return
	}

	resolvedText := ""
	foundTextProp := false

	if doc != nil && el.OriginalIndex < len(doc.Properties) && len(doc.Properties[el.OriginalIndex]) > 0 {

		for _, prop := range doc.Properties[el.OriginalIndex] {

			if prop.ID == krb.PropIDTextContent {

				if strIdx, ok := getByteValue(&prop); ok {

					if s, textOk := getStringValueByIdx(doc, strIdx); textOk {
						resolvedText = s
						foundTextProp = true
						break
					}
				}
			}
		}
	}

	if !foundTextProp && styleOk && style != nil {

		if styleProp, propInStyleOk := getStylePropertyValue(style, krb.PropIDTextContent); propInStyleOk {

			if strIdx, ok := getByteValue(styleProp); ok {

				if s, textOk := getStringValueByIdx(doc, strIdx); textOk {
					resolvedText = s
				}
			}
		}
	}
	el.Text = resolvedText
}

func resolveElementImageSource(
	doc *krb.Document,
	el *render.RenderElement,
	style *krb.Style,
	styleOk bool,
) {

	if el.Header.Type != krb.ElemTypeImage && el.Header.Type != krb.ElemTypeButton {
		return
	}

	if el.ResourceIndex != render.InvalidResourceIndex { // Already set
		return
	}

	resolvedResIdx := uint8(render.InvalidResourceIndex)
	foundResProp := false

	if doc != nil && el.OriginalIndex < len(doc.Properties) && len(doc.Properties[el.OriginalIndex]) > 0 {

		for _, prop := range doc.Properties[el.OriginalIndex] {

			if prop.ID == krb.PropIDImageSource {

				if idx, ok := getByteValue(&prop); ok {
					resolvedResIdx = idx
					foundResProp = true
					break
				}
			}
		}
	}

	if !foundResProp && styleOk && style != nil {

		if styleProp, propInStyleOk := getStylePropertyValue(style, krb.PropIDImageSource); propInStyleOk {

			if idx, ok := getByteValue(styleProp); ok {
				resolvedResIdx = idx
			}
		}
	}
	el.ResourceIndex = resolvedResIdx
}

func resolveEventHandlers(doc *krb.Document, el *render.RenderElement) {
	el.EventHandlers = nil // Clear/initialize

	if doc != nil && doc.Events != nil &&
		el.OriginalIndex < len(doc.Events) && doc.Events[el.OriginalIndex] != nil {
		krbEvents := doc.Events[el.OriginalIndex]

		if len(krbEvents) > 0 {
			el.EventHandlers = make([]render.EventCallbackInfo, 0, len(krbEvents))

			for _, krbEvent := range krbEvents {

				if handlerName, ok := getStringValueByIdx(doc, krbEvent.CallbackID); ok {
					el.EventHandlers = append(el.EventHandlers, render.EventCallbackInfo{
						EventType:   krbEvent.EventType,
						HandlerName: handlerName,
					})
				} else {
					log.Printf(
						"Warn resolveEventHandlers: Elem %s (OrigIdx %d) invalid event callback string index %d.",
						el.SourceElementName, el.OriginalIndex, krbEvent.CallbackID,
					)
				}
			}
		}
	}
}

func findStyle(doc *krb.Document, styleID uint8) (*krb.Style, bool) {

	if doc == nil || styleID == 0 { // StyleID 0 is "no style"
		return nil, false
	}
	styleIndex := int(styleID - 1) // StyleID is 1-based

	if styleIndex < 0 || styleIndex >= len(doc.Styles) {
		return nil, false
	}
	return &doc.Styles[styleIndex], true
}

func getStylePropertyValue(style *krb.Style, propID krb.PropertyID) (*krb.Property, bool) {

	if style == nil {
		return nil, false
	}

	for i := range style.Properties {

		if style.Properties[i].ID == propID {
			return &style.Properties[i], true
		}
	}
	return nil, false
}

func findStyleIDByNameIndex(doc *krb.Document, nameIndex uint8) uint8 {

	if doc == nil {
		return 0
	}

	if int(nameIndex) >= len(doc.Strings) { // Check bounds for safety
		return 0
	}

	for i := range doc.Styles {

		if doc.Styles[i].NameIndex == nameIndex {
			return doc.Styles[i].ID // ID is 1-based
		}
	}
	return 0 // Not found
}

func getStyleColors(
	doc *krb.Document,
	styleID uint8,
	flags uint16,
) (bg rl.Color, fg rl.Color, ok bool) {
	style, styleFound := findStyle(doc, styleID)

	if !styleFound {
		return rl.Blank, rl.White, false
	}

	bg, fg = rl.Blank, rl.White // Defaults
	foundBg, foundFg := false, false

	for _, prop := range style.Properties {

		if prop.ID == krb.PropIDBgColor {

			if c, pOk := getColorValue(&prop, flags); pOk {
				bg = c
				foundBg = true
			}
		}

		if prop.ID == krb.PropIDFgColor {

			if c, pOk := getColorValue(&prop, flags); pOk {
				fg = c
				foundFg = true
			}
		}

		if foundBg && foundFg {
			break
		}
	}
	return bg, fg, true // Processed successfully
}

func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {

	if prop == nil || prop.ValueType != krb.ValTypeColor {
		return rl.Color{}, false
	}
	useExtended := (flags & krb.FlagExtendedColor) != 0

	if useExtended { // RGBA

		if len(prop.Value) == 4 {
			return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true
		}
	} else { // Palette index

		if len(prop.Value) == 1 {
			log.Printf(
				"Warn getColorValue: Palette color (index %d) requested, palette system not implemented. Returning Magenta.",
				prop.Value[0],
			)
			return rl.Magenta, true // Placeholder for palette
		}
	}
	log.Printf(
		"Warn getColorValue: Invalid color data for PropID %X, ValueType %X, Size %d, ExtendedFlag %t",
		prop.ID, prop.ValueType, prop.Size, useExtended,
	)
	return rl.Color{}, false
}

func getByteValue(prop *krb.Property) (uint8, bool) {

	if prop != nil &&
		(prop.ValueType == krb.ValTypeByte ||
			prop.ValueType == krb.ValTypeString ||
			prop.ValueType == krb.ValTypeResource ||
			prop.ValueType == krb.ValTypeEnum) &&
		len(prop.Value) == 1 {
		return prop.Value[0], true
	}
	return 0, false
}

func getFirstByteValue(prop *krb.Property) uint8 {

	if prop != nil && len(prop.Value) > 0 {
		return prop.Value[0]
	}
	return 0
}

func getShortValue(prop *krb.Property) (uint16, bool) {

	if prop != nil && prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 {
		return binary.LittleEndian.Uint16(prop.Value), true
	}
	return 0, false
}

func getStringValueByIdx(doc *krb.Document, stringIndex uint8) (string, bool) {

	if doc != nil && int(stringIndex) < len(doc.Strings) {
		return doc.Strings[stringIndex], true
	}
	return "", false
}

func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) { // TRBL

	if prop != nil && prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 {
		return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true
	}
	return [4]uint8{}, false
}

func getNumericValueForSizeProp(
	props []krb.Property,
	propID krb.PropertyID,
	doc *krb.Document,
) (value float32, valueType krb.ValueType, rawSizeBytes uint8, err error) {

	for i := range props {

		if props[i].ID == propID {
			return getNumericValueFromKrbProp(&props[i], doc)
		}
	}
	return 0, krb.ValTypeNone, 0, fmt.Errorf("property ID 0x%X not found in list", propID)
}

func getNumericValueFromKrbProp(
	prop *krb.Property,
	doc *krb.Document,
) (value float32, valueType krb.ValueType, rawSizeBytes uint8, err error) {

	if prop == nil {
		return 0, krb.ValTypeNone, 0, fmt.Errorf("getNumericValueFromKrbProp: received nil property")
	}

	if prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 {
		return float32(binary.LittleEndian.Uint16(prop.Value)), krb.ValTypeShort, 2, nil
	}

	if prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 {
		return float32(binary.LittleEndian.Uint16(prop.Value)), krb.ValTypePercentage, 2, nil
	}
	return 0, prop.ValueType, prop.Size, fmt.Errorf(
		"unsupported KRB ValueType (%d) or Size (%d for PropID %X) for numeric size conversion",
		prop.ValueType, prop.Size, prop.ID,
	)
}

func calculateAlignmentOffsetsF(
	alignment uint8,
	availableSpaceOnMainAxis float32,
	totalUsedSpaceByChildrenAndGaps float32,
	numberOfChildren int,
	isLayoutReversed bool,
	fixedGapBetweenChildren float32,
) (startOffset float32, spacingToApplyBetweenChildren float32) {
	unusedSpace := MaxF(0, availableSpaceOnMainAxis-totalUsedSpaceByChildrenAndGaps)
	startOffset = 0.0
	spacingToApplyBetweenChildren = fixedGapBetweenChildren

	switch alignment {

	case krb.LayoutAlignStart:
		startOffset = MuxFloat32(isLayoutReversed, unusedSpace, 0)

	case krb.LayoutAlignCenter:
		startOffset = unusedSpace / 2.0

	case krb.LayoutAlignEnd:
		startOffset = MuxFloat32(isLayoutReversed, 0, unusedSpace)

	case krb.LayoutAlignSpaceBetween:
		if numberOfChildren > 1 {
			spacingToApplyBetweenChildren += unusedSpace / float32(numberOfChildren-1)
		} else { // Center single child
			startOffset = unusedSpace / 2.0
		}

	default:
		log.Printf("Warn calculateAlignmentOffsetsF: Unknown alignment %d. Defaulting to Start.", alignment)
		startOffset = MuxFloat32(isLayoutReversed, unusedSpace, 0)
	}
	return startOffset, spacingToApplyBetweenChildren
}

func calculateCrossAxisOffsetF(
	alignment uint8,
	parentCrossAxisSize float32,
	childCrossAxisSize float32,
) float32 {

	if alignment == krb.LayoutAlignStretch { // Stretch handled by size, not offset
		return 0.0
	}
	availableSpace := parentCrossAxisSize - childCrossAxisSize

	if availableSpace <= 0 {
		return 0.0
	}

	offset := float32(0.0)
	switch alignment {

	case krb.LayoutAlignStart:
		offset = 0.0

	case krb.LayoutAlignCenter:
		offset = availableSpace / 2.0

	case krb.LayoutAlignEnd:
		offset = availableSpace

	default: // Fallback for unknown
		offset = 0.0
	}
	return MaxF(0, offset)
}

func logElementTree(el *render.RenderElement, depth int, prefix string) {

	if el == nil {
		return
	}
	indent := make([]byte, depth*2)

	for i := range indent {
		indent[i] = ' '
	}

	parentId := -1

	if el.Parent != nil {
		parentId = el.Parent.OriginalIndex
	}

	log.Printf(
		"%s%s ElemDX_Global[%d] Name='%s' Type=0x%02X Children:%d ParentDX_Global:%d Render=(%.0f,%.0f %.0fwX%.0fh) Vis:%t StyleID:%d Layout:0x%02X",
		string(indent), prefix, el.OriginalIndex, el.SourceElementName, el.Header.Type, len(el.Children), parentId,
		el.RenderX, el.RenderY, el.RenderW, el.RenderH, el.IsVisible, el.Header.StyleID, el.Header.Layout,
	)

	for i, child := range el.Children {
		logElementTree(child, depth+1, fmt.Sprintf("Child[%d]", i))
	}
}

// --- Math & Slice Utilities ---

func ReverseSliceInt(s []int) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func ScaledF32(value uint8, scale float32) float32 {
	return float32(value) * scale
}

func scaledI32(value uint8, scale float32) int32 {
	return int32(math.Round(float64(value) * float64(scale)))
}

func MinF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func MaxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func MuxFloat32(cond bool, valTrue, valFalse float32) float32 {
	if cond {
		return valTrue
	}
	return valFalse
}

func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
