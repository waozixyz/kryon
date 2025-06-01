// render/raylib/renderer_processing.go
package raylib

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"path/filepath"
	//"strings" // Keep for PerformLayout logging condition

	rl "github.com/gen2brain/raylib-go/raylib" // For rl.Blank in expandComponent, default colors
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

func (r *RaylibRenderer) PrepareTree(
	doc *krb.Document,
	krbFilePath string,
) ([]*render.RenderElement, render.WindowConfig, error) {

	if doc == nil {
		log.Println("PrepareTree: KRB document is nil.")
		return nil, r.config, fmt.Errorf("PrepareTree: KRB document is nil")
	}
	r.docRef = doc

	var err error
	r.krbFileDir, err = filepath.Abs(filepath.Dir(krbFilePath))
	if err != nil {
		r.krbFileDir = filepath.Dir(krbFilePath)
		log.Printf("WARN PrepareTree: Failed to get abs path for KRB file dir '%s': %v. Using relative: %s", krbFilePath, err, r.krbFileDir)
	}
	log.Printf("PrepareTree: Resource Base Directory set to: %s", r.krbFileDir)

	// --- 1. Initialize WindowConfig with application defaults ---
	windowConfig := render.DefaultWindowConfig() // Gets struct with hardcoded defaults

	// --- 2. Apply App Element's Style and Direct Properties to WindowConfig ---
	isAppElementPresent := (doc.Header.Flags&krb.FlagHasApp) != 0 &&
		doc.Header.ElementCount > 0 &&
		doc.Elements[0].Type == krb.ElemTypeApp

	if isAppElementPresent {
		appElementKrbHeader := &doc.Elements[0]
		// Apply style from App element to windowConfig
		if appStyle, styleFound := findStyle(doc, appElementKrbHeader.StyleID); styleFound {
			r.applyStylePropertiesToWindowConfig(appStyle.Properties, doc, &windowConfig)
		} else if appElementKrbHeader.StyleID != 0 {
			log.Printf("Warn PrepareTree: App element has StyleID %d, but style was not found.", appElementKrbHeader.StyleID)
		}
		// Apply direct properties from App element to windowConfig
		if len(doc.Properties) > 0 && len(doc.Properties[0]) > 0 {
			r.applyDirectPropertiesToWindowConfig(doc.Properties[0], doc, &windowConfig)
		}
	} else {
		log.Println("PrepareTree: No App element found in KRB. Using default window configuration.")
	}
	// Finalize scale factor and store config in renderer
	r.scaleFactor = float32(math.Max(1.0, float64(windowConfig.ScaleFactor)))
	r.config = windowConfig
	log.Printf("PrepareTree: Final Window Config: W:%d, H:%d, Title:'%s', Scale:%.2f, Resizable:%t, DefBG:%v, DefFG:%v, DefBorder:%v",
		r.config.Width, r.config.Height, r.config.Title, r.scaleFactor, r.config.Resizable, r.config.DefaultBg, r.config.DefaultFgColor, r.config.DefaultBorderColor)

	// --- 3. Process KRB Elements into RenderElements ---
	initialElementCount := int(doc.Header.ElementCount)
	if initialElementCount == 0 {
		log.Println("PrepareTree: No elements in KRB document.")
		r.elements = nil
		r.roots = nil
		return nil, r.config, nil
	}
	r.elements = make([]render.RenderElement, initialElementCount, initialElementCount*2)

	// Initial properties that are not typically styled or inherited directly in the first pass
	defaultTextAlignment := uint8(krb.LayoutAlignStart)
	defaultIsVisible := true

	for i := 0; i < initialElementCount; i++ {
		renderEl := &r.elements[i]
		krbElHeader := doc.Elements[i]

		// Basic Initialization
		renderEl.Header = krbElHeader
		renderEl.OriginalIndex = i
		renderEl.DocRef = doc
		renderEl.BgColor = rl.Blank     // Default: transparent
		renderEl.FgColor = rl.Blank     // Default: "unset", to be filled by style, direct, or inheritance
		renderEl.BorderColor = rl.Blank // Default: "unset"
		renderEl.BorderWidths = [4]uint8{0, 0, 0, 0}
		renderEl.Padding = [4]uint8{0, 0, 0, 0}
		renderEl.TextAlignment = defaultTextAlignment // Base default, can be overridden
		renderEl.IsVisible = defaultIsVisible         // Base default, can be overridden
		renderEl.IsInteractive = (krbElHeader.Type == krb.ElemTypeButton || krbElHeader.Type == krb.ElemTypeInput)
		renderEl.ResourceIndex = render.InvalidResourceIndex

		// Source Element Name for Debugging
		elementIDString, _ := getStringValueByIdx(doc, renderEl.Header.ID)
		var componentName string
		if doc.CustomProperties != nil && i < len(doc.CustomProperties) {
			componentName, _ = GetCustomPropertyValue(renderEl, componentNameConventionKey, doc)
		}
		if componentName != "" {
			renderEl.SourceElementName = componentName
		} else if elementIDString != "" {
			renderEl.SourceElementName = elementIDString
		} else {
			renderEl.SourceElementName = fmt.Sprintf("Type0x%X_Idx%d", renderEl.Header.Type, renderEl.OriginalIndex)
		}

		// Styling Resolution Order (as per spec section 5)
		// 5.1. Basic Init (done above)
		// 5.2. Style Application
		elementStyle, styleFound := findStyle(doc, krbElHeader.StyleID)
		if styleFound {
			r.applyStylePropertiesToElement(elementStyle.Properties, doc, renderEl)
		} else if krbElHeader.StyleID != 0 && !(i == 0 && isAppElementPresent) {
			log.Printf("Warn PrepareTree: Element %s (Idx %d) has StyleID %d, but style was not found.",
				renderEl.SourceElementName, i, krbElHeader.StyleID)
		}

		// 5.3. Direct Property Application (overrides style)
		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			if i == 0 && isAppElementPresent { // App element has some visual props for its RenderElement
				r.applyDirectVisualPropertiesToAppElement(doc.Properties[0], doc, renderEl)
			} else {
				r.applyDirectPropertiesToElement(doc.Properties[i], doc, renderEl)
			}
		}

		// Resolve text and image source (might use values from style or direct props)
		r.resolveElementTextAndImage(doc, renderEl, elementStyle, styleFound)

		// 5.4. Contextual Default Resolution (e.g., borders)
		r.applyContextualDefaults(renderEl)

		// Event handlers (not styling, but part of element setup)
		resolveEventHandlers(doc, renderEl) // This can stay here or move to utils
	}

	// --- 4. Link Original KRB Children & Expand Components ---
	kryUsageChildrenMap := make(map[int][]*render.RenderElement)
	if err_link := r.linkOriginalKrbChildren(initialElementCount, kryUsageChildrenMap); err_link != nil {
		return nil, r.config, fmt.Errorf("PrepareTree: failed during initial child linking: %w", err_link)
	}

	nextMasterIndex := initialElementCount
	for i := 0; i < initialElementCount; i++ {
		instanceElement := &r.elements[i]
		componentName, _ := GetCustomPropertyValue(instanceElement, componentNameConventionKey, doc)
		if componentName != "" {
			compDef := r.findComponentDefinition(componentName)
			if compDef != nil {
				instanceKryChildren := kryUsageChildrenMap[instanceElement.OriginalIndex]
				err_expand := r.expandComponent(instanceElement, compDef, &r.elements, &nextMasterIndex, instanceKryChildren)
				if err_expand != nil {
					log.Printf("ERROR PrepareTree: Failed to expand component '%s' for instance '%s': %v", componentName, instanceElement.SourceElementName, err_expand)
				}
			} else {
				log.Printf("Warn PrepareTree: Component definition for '%s' (instance '%s') not found.", componentName, instanceElement.SourceElementName)
			}
		}
	}

	// Finalize tree structure (Parent pointers and finding roots) *after* expansion
	r.roots = nil
	if err_build := r.finalizeTreeStructureAndRoots(); err_build != nil {
		return nil, r.config, fmt.Errorf("failed to finalize full element tree: %w", err_build)
	}

	// --- 5. Resolve Property Inheritance ---
	// This must happen *after* the full tree is linked and components are expanded,
	// so parent properties are fully resolved before children try to inherit.
	r.resolvePropertyInheritance()

	// --- Done with Tree Preparation ---
	log.Printf("PrepareTree: Tree built. Roots: %d. Total elements (incl. expanded): %d.", len(r.roots), len(r.elements))
	for rootIdx, rootNode := range r.roots {
		logElementTree(rootNode, 0, fmt.Sprintf("Root[%d]", rootIdx))
	}

	return r.roots, r.config, nil
}

func (r *RaylibRenderer) linkOriginalKrbChildren(
	initialElementCount int,
	kryUsageChildrenMap map[int][]*render.RenderElement,
) error {

	if r.docRef == nil || r.docRef.ElementStartOffsets == nil {
		return fmt.Errorf("linkOriginalKrbChildren: docRef or ElementStartOffsets is nil")
	}

	// Map KRB element file offsets to their index in the initial r.elements slice
	offsetToInitialElementIndex := make(map[uint32]int)

	for i := 0; i < initialElementCount && i < len(r.docRef.ElementStartOffsets); i++ {
		offsetToInitialElementIndex[r.docRef.ElementStartOffsets[i]] = i
	}

	for i := 0; i < initialElementCount; i++ {
		currentEl := &r.elements[i]
		originalKrbHeader := &r.docRef.Elements[i] // This is element from doc.Elements
		componentName, _ := GetCustomPropertyValue(currentEl, componentNameConventionKey, r.docRef)
		isPlaceholder := (componentName != "") // Is this element an instance of a component?

		if originalKrbHeader.ChildCount > 0 {
			// Ensure ChildRefs exist for this element in the KRB document

			if i >= len(r.docRef.ChildRefs) || r.docRef.ChildRefs[i] == nil {
				log.Printf(
					"Warn linkOriginalKrbChildren: Elem %s (OrigIdx %d) has KRB ChildCount %d but no ChildRefs in doc.",
					currentEl.SourceElementName, i, originalKrbHeader.ChildCount,
				)
				continue // Skip if child references are missing
			}

			krbChildRefs := r.docRef.ChildRefs[i]
			actualChildren := make([]*render.RenderElement, 0, len(krbChildRefs))

			parentStartOffset := uint32(0)

			if i < len(r.docRef.ElementStartOffsets) {
				parentStartOffset = r.docRef.ElementStartOffsets[i]
			} else {
				log.Printf(
					"Error linkOriginalKrbChildren: Elem %s (OrigIdx %d) missing from ElementStartOffsets.",
					currentEl.SourceElementName, i,
				)
				continue // Should not happen if initialElementCount is consistent
			}

			for _, childRef := range krbChildRefs {
				// ChildOffset in KRB is relative to the parent element's start in the file
				childAbsoluteFileOffset := parentStartOffset + uint32(childRef.ChildOffset)
				childIndexInInitialElements, found := offsetToInitialElementIndex[childAbsoluteFileOffset]

				if !found {
					log.Printf(
						"Error linkOriginalKrbChildren: Elem %s (OrigIdx %d) ChildRef offset %d (abs %d) does not map to known initial element.",
						currentEl.SourceElementName, i, childRef.ChildOffset, childAbsoluteFileOffset,
					)
					continue
				}
				childEl := &r.elements[childIndexInInitialElements]
				actualChildren = append(actualChildren, childEl)
			}

			if isPlaceholder {
				// For component instances, store these children temporarily. They will be slotted later.
				kryUsageChildrenMap[i] = actualChildren
			} else {
				// For regular elements, directly link children and set parent pointers
				currentEl.Children = actualChildren
				for _, child := range actualChildren {
					child.Parent = currentEl
				}
			}
		}
	}
	return nil
}

func (r *RaylibRenderer) finalizeTreeStructureAndRoots() error {

	if len(r.elements) == 0 {
		r.roots = nil
		return nil
	}
	r.roots = nil // Clear any existing roots

	for i := range r.elements {
		el := &r.elements[i] // Get pointer to the element

		if el.Parent == nil {
			r.roots = append(r.roots, el)
		}
	}

	if len(r.roots) == 0 && len(r.elements) > 0 {
		log.Printf(
			"Warn finalizeTreeStructureAndRoots: No root elements identified, but %d elements exist. This might indicate a problem in parent linking or an unusual KRB structure.",
			len(r.elements),
		)
	}
	return nil
}

func (r *RaylibRenderer) findComponentDefinition(name string) *krb.KrbComponentDefinition {

	if r.docRef == nil || len(r.docRef.ComponentDefinitions) == 0 || len(r.docRef.Strings) == 0 {
		return nil
	}

	for i := range r.docRef.ComponentDefinitions {
		compDef := &r.docRef.ComponentDefinitions[i]

		if int(compDef.NameIndex) < len(r.docRef.Strings) && r.docRef.Strings[compDef.NameIndex] == name {
			return compDef
		}
	}
	return nil
}

func (r *RaylibRenderer) expandComponent(
	instanceElement *render.RenderElement,
	compDef *krb.KrbComponentDefinition,
	allElements *[]render.RenderElement,
	nextMasterIndex *int,
	kryUsageChildren []*render.RenderElement,
) error {
	doc := r.docRef // Use doc from renderer context

	if compDef.RootElementTemplateData == nil || len(compDef.RootElementTemplateData) == 0 {
		log.Printf(
			"Warn expandComponent: Component definition '%s' for instance '%s' has no RootElementTemplateData. Instance will have no template children.",
			getStringValueByIdxFallback(doc, compDef.NameIndex, "UnnamedComponent"),
			instanceElement.SourceElementName,
		)
		instanceElement.Children = nil
		// KRY-usage children slotting will be handled later.
		return nil
	}

	templateReader := bytes.NewReader(compDef.RootElementTemplateData)
	var templateRootsInThisExpansion []*render.RenderElement
	templateOffsetToGlobalIndex := make(map[uint32]int)

	type templateChildInfo struct {
		parentGlobalIndex            int
		childRefs                    []krb.ChildRef
		parentHeaderOffsetInTemplate uint32
	}
	var templateChildInfos []templateChildInfo

	// Default visual properties for template elements
	defaultFgColor := rl.RayWhite
	defaultBorderColor := rl.Gray
	defaultTextAlignment := uint8(krb.LayoutAlignStart)
	defaultIsVisible := true
	templateDataStreamOffset := uint32(0)

	// Loop to read and create elements from the template data
	for templateReader.Len() > 0 {
		currentElementOffsetInTemplate := templateDataStreamOffset
		headerBuf := make([]byte, krb.ElementHeaderSize)
		n, err := templateReader.Read(headerBuf)

		if err == io.EOF {
			break // End of template data
		}

		if err != nil || n < krb.ElementHeaderSize {
			return fmt.Errorf(
				"expandComponent '%s': failed to read template element header: %w (read %d bytes)",
				instanceElement.SourceElementName, err, n,
			)
		}
		templateDataStreamOffset += uint32(n)

		// Deserialize header
		templateKrbHeader := krb.ElementHeader{
			Type:            krb.ElementType(headerBuf[0]),
			ID:              headerBuf[1],
			PosX:            krb.ReadU16LE(headerBuf[2:4]),
			PosY:            krb.ReadU16LE(headerBuf[4:6]),
			Width:           krb.ReadU16LE(headerBuf[6:8]),
			Height:          krb.ReadU16LE(headerBuf[8:10]),
			Layout:          headerBuf[10],
			StyleID:         headerBuf[11],
			PropertyCount:   headerBuf[12],
			ChildCount:      headerBuf[13],
			EventCount:      headerBuf[14],
			AnimationCount:  headerBuf[15],
			CustomPropCount: headerBuf[16],
		}

		newElGlobalIndex := *nextMasterIndex
		(*nextMasterIndex)++

		// Grow the allElements slice if needed
		if newElGlobalIndex >= cap(*allElements) {
			newCap := cap(*allElements) * 2
			if newElGlobalIndex >= newCap { // Ensure newCap is sufficient
				newCap = newElGlobalIndex + 20 // Add some buffer
			}
			tempSlice := make([]render.RenderElement, len(*allElements), newCap)
			copy(tempSlice, *allElements)
			*allElements = tempSlice
		}

		// Extend the length of the slice if newElGlobalIndex is at the current end
		if newElGlobalIndex >= len(*allElements) {
			*allElements = (*allElements)[:newElGlobalIndex+1]
		}

		newEl := &(*allElements)[newElGlobalIndex]
		newEl.OriginalIndex = newElGlobalIndex
		newEl.Header = templateKrbHeader
		newEl.DocRef = doc
		newEl.BgColor = rl.Blank
		newEl.FgColor = defaultFgColor
		newEl.BorderColor = defaultBorderColor
		newEl.TextAlignment = defaultTextAlignment
		newEl.IsVisible = defaultIsVisible
		newEl.ResourceIndex = render.InvalidResourceIndex
		newEl.IsInteractive = (templateKrbHeader.Type == krb.ElemTypeButton || templateKrbHeader.Type == krb.ElemTypeInput)
		templateOffsetToGlobalIndex[currentElementOffsetInTemplate] = newElGlobalIndex

		// Set SourceElementName for the new template element
		templateElIdStr, _ := getStringValueByIdx(doc, templateKrbHeader.ID)

		if templateElIdStr != "" {
			newEl.SourceElementName = templateElIdStr
		} else {
			newEl.SourceElementName = fmt.Sprintf("TplElem_Type0x%X_Idx%d", templateKrbHeader.Type, newEl.OriginalIndex)
		}

		// Read and apply direct properties from template
		var templateDirectProps []krb.Property

		if templateKrbHeader.PropertyCount > 0 {
			templateDirectProps = make([]krb.Property, templateKrbHeader.PropertyCount)
			propHeaderBuf := make([]byte, 3) // Property header: ID, ValueType, Size

			for j := uint8(0); j < templateKrbHeader.PropertyCount; j++ {
				nProp, errProp := templateReader.Read(propHeaderBuf)

				if errProp != nil || nProp < 3 {
					return fmt.Errorf(
						"expandComponent '%s': failed to read template property header for '%s': %w",
						instanceElement.SourceElementName, newEl.SourceElementName, errProp,
					)
				}
				templateDataStreamOffset += uint32(nProp)
				prop := &templateDirectProps[j]
				prop.ID = krb.PropertyID(propHeaderBuf[0])
				prop.ValueType = krb.ValueType(propHeaderBuf[1])
				prop.Size = propHeaderBuf[2]

				if prop.Size > 0 {
					prop.Value = make([]byte, prop.Size)
					nVal, errVal := templateReader.Read(prop.Value)

					if errVal != nil || nVal < int(prop.Size) {
						return fmt.Errorf(
							"expandComponent '%s': failed to read template property value for '%s': %w",
							instanceElement.SourceElementName, newEl.SourceElementName, errVal,
						)
					}
					templateDataStreamOffset += uint32(nVal)
				}
			}
		}
		// Apply style and direct properties from template
		templateStyle, templateStyleFound := findStyle(doc, templateKrbHeader.StyleID)

		if templateStyleFound {
			applyStylePropertiesToElement(templateStyle.Properties, doc, newEl)
		}
		applyDirectPropertiesToElement(templateDirectProps, doc, newEl) // Direct template props override template style

		// Read and process custom properties from template
		var nestedComponentName string // If this template element itself is a nested component

		if templateKrbHeader.CustomPropCount > 0 {
			customPropHeaderBuf := make([]byte, 3) // KeyIndex, ValueType, Size

			for j := uint8(0); j < templateKrbHeader.CustomPropCount; j++ {
				nCustomProp, errCustomProp := templateReader.Read(customPropHeaderBuf)

				if errCustomProp != nil || nCustomProp < 3 {
					return fmt.Errorf(
						"expandComponent '%s': failed to read template custom property header for '%s': %w",
						instanceElement.SourceElementName, newEl.SourceElementName, errCustomProp,
					)
				}
				templateDataStreamOffset += uint32(nCustomProp)
				cpropKeyIndex := customPropHeaderBuf[0]
				cpropValueType := krb.ValueType(customPropHeaderBuf[1])
				cpropSize := customPropHeaderBuf[2]
				var cpropValue []byte

				if cpropSize > 0 {
					cpropValue = make([]byte, cpropSize)
					nVal, errVal := templateReader.Read(cpropValue)

					if errVal != nil || nVal < int(cpropSize) {
						return fmt.Errorf(
							"expandComponent '%s': failed to read template custom property value for '%s': %w",
							instanceElement.SourceElementName, newEl.SourceElementName, errVal,
						)
					}
					templateDataStreamOffset += uint32(nVal)
				}

				// Check if this custom prop defines a nested component
				keyName, keyOk := getStringValueByIdx(doc, cpropKeyIndex)

				if keyOk && keyName == componentNameConventionKey {

					if (cpropValueType == krb.ValTypeString || cpropValueType == krb.ValTypeResource) && cpropSize == 1 {
						valueIndex := cpropValue[0]

						if strVal, strOk := getStringValueByIdx(doc, valueIndex); strOk {
							nestedComponentName = strVal
							newEl.SourceElementName = nestedComponentName // Update name to nested component's name
							break
						}
					}
				}
			}
		}
		// Resolve text and image for the template element
		resolveElementText(doc, newEl, templateStyle, templateStyleFound)
		resolveElementImageSource(doc, newEl, templateStyle, templateStyleFound)

		// Read Event Handlers from template
		if templateKrbHeader.EventCount > 0 {
			eventDataSize := int(templateKrbHeader.EventCount) * krb.EventFileEntrySize
			eventBuf := make([]byte, eventDataSize)
			nEvent, errEvent := templateReader.Read(eventBuf)

			if errEvent != nil || nEvent < eventDataSize {
				return fmt.Errorf(
					"expandComponent '%s': failed to read template event block for '%s': %w",
					instanceElement.SourceElementName, newEl.SourceElementName, errEvent,
				)
			}
			templateDataStreamOffset += uint32(nEvent)
			newEl.EventHandlers = make([]render.EventCallbackInfo, templateKrbHeader.EventCount)

			for k := uint8(0); k < templateKrbHeader.EventCount; k++ {
				offset := int(k) * krb.EventFileEntrySize
				eventType := krb.EventType(eventBuf[offset])
				callbackID := eventBuf[offset+1] // String table index for handler name

				if handlerName, ok := getStringValueByIdx(doc, callbackID); ok {
					newEl.EventHandlers[k] = render.EventCallbackInfo{EventType: eventType, HandlerName: handlerName}
				} else {
					log.Printf(
						"Warn expandComponent: Template element '%s' has invalid event callback string index %d.",
						newEl.SourceElementName, callbackID,
					)
				}
			}
		}

		// Skip Animation Refs from template
		if templateKrbHeader.AnimationCount > 0 {
			animRefDataSize := int(templateKrbHeader.AnimationCount) * krb.AnimationRefSize
			bytesSkipped, errAnim := templateReader.Seek(int64(animRefDataSize), io.SeekCurrent)

			if errAnim != nil || bytesSkipped < int64(animRefDataSize) {
				return fmt.Errorf(
					"expandComponent '%s': failed to seek past template animation refs for '%s': %w",
					instanceElement.SourceElementName, newEl.SourceElementName, errAnim,
				)
			}
			templateDataStreamOffset += uint32(animRefDataSize) // Update stream offset
		}

		// Read Child Refs from template (to be linked later)
		if templateKrbHeader.ChildCount > 0 {
			templateChildRefs := make([]krb.ChildRef, templateKrbHeader.ChildCount)
			childRefDataSize := int(templateKrbHeader.ChildCount) * krb.ChildRefSize
			childRefBuf := make([]byte, childRefDataSize)
			nChildRef, errChildRef := templateReader.Read(childRefBuf)

			if errChildRef != nil || nChildRef < childRefDataSize {
				return fmt.Errorf(
					"expandComponent '%s': failed to read template child refs for '%s': %w",
					instanceElement.SourceElementName, newEl.SourceElementName, errChildRef,
				)
			}
			templateDataStreamOffset += uint32(nChildRef)

			for k := uint8(0); k < templateKrbHeader.ChildCount; k++ {
				offset := int(k) * krb.ChildRefSize
				templateChildRefs[k] = krb.ChildRef{ChildOffset: krb.ReadU16LE(childRefBuf[offset : offset+krb.ChildRefSize])}
			}
			templateChildInfos = append(templateChildInfos, templateChildInfo{
				parentGlobalIndex:            newElGlobalIndex,
				childRefs:                    templateChildRefs,
				parentHeaderOffsetInTemplate: currentElementOffsetInTemplate,
			})
		}

		// If this is the first element from the template, it's a root of this component's template.
		if len(templateRootsInThisExpansion) == 0 {
			templateRootsInThisExpansion = append(templateRootsInThisExpansion, newEl)
			newEl.Parent = instanceElement // Link to the component instance

			log.Printf(
				"Debug expandComponent: Applying instance '%s' (OrigIdx %d) props to template root '%s' (GlobalIdx %d)",
				instanceElement.SourceElementName, instanceElement.OriginalIndex, newEl.SourceElementName, newEl.OriginalIndex,
			)

			// Override template root's Header fields with instance's values
			newEl.Header.ID = instanceElement.Header.ID // ID comes from instance
			newEl.Header.PosX = instanceElement.Header.PosX
			newEl.Header.PosY = instanceElement.Header.PosY
			newEl.Header.Width = instanceElement.Header.Width
			newEl.Header.Height = instanceElement.Header.Height
			newEl.Header.Layout = instanceElement.Header.Layout
			if instanceElement.Header.StyleID != 0 {
				newEl.Header.StyleID = instanceElement.Header.StyleID
			}
			newEl.SourceElementName = instanceElement.SourceElementName // Name comes from instance

			// Re-apply styles and direct properties from the instanceElement to newEl
			if instanceStyle, instanceStyleFound := findStyle(doc, instanceElement.Header.StyleID); instanceStyleFound {
				applyStylePropertiesToElement(instanceStyle.Properties, doc, newEl)
				log.Printf("   Applied instance style ID %d to template root.", instanceElement.Header.StyleID)
			}

			if doc != nil && instanceElement.OriginalIndex < len(doc.Properties) && len(doc.Properties[instanceElement.OriginalIndex]) > 0 {
				applyDirectPropertiesToElement(doc.Properties[instanceElement.OriginalIndex], doc, newEl)
				log.Printf("   Applied instance direct KRB properties to template root.")
			}

			currentStyleForNewEl, currentStyleFoundForNewEl := findStyle(doc, newEl.Header.StyleID)
			resolveElementText(doc, newEl, currentStyleForNewEl, currentStyleFoundForNewEl)
			resolveElementImageSource(doc, newEl, currentStyleForNewEl, currentStyleFoundForNewEl)

		}

		// If this template element is a nested component, expand it recursively
		if nestedComponentName != "" {
			nestedCompDef := r.findComponentDefinition(nestedComponentName) // Uses r.docRef

			if nestedCompDef != nil {
				log.Printf(
					"expandComponent: Expanding nested component '%s' for template element '%s' (GlobalIdx: %d)",
					nestedComponentName, newEl.SourceElementName, newEl.OriginalIndex,
				)
				err := r.expandComponent(newEl, nestedCompDef, allElements, nextMasterIndex, nil)

				if err != nil {
					return fmt.Errorf(
						"expandComponent '%s': failed to expand nested component '%s': %w",
						instanceElement.SourceElementName, nestedComponentName, err,
					)
				}
			} else {
				log.Printf(
					"Warn expandComponent: Nested component definition '%s' for template element '%s' not found.",
					nestedComponentName, newEl.SourceElementName,
				)
			}
		}
	} // End of loop reading elements from template data

	// Link children within the expanded template structure
	for _, info := range templateChildInfos {
		parentEl := &(*allElements)[info.parentGlobalIndex]

		if len(parentEl.Children) > 0 && parentEl.Children[0].Parent == parentEl {
			continue
		}

		parentEl.Children = make([]*render.RenderElement, 0, len(info.childRefs))

		for _, childRef := range info.childRefs {
			childAbsoluteOffsetInTemplate := info.parentHeaderOffsetInTemplate + uint32(childRef.ChildOffset)
			childGlobalIndex, found := templateOffsetToGlobalIndex[childAbsoluteOffsetInTemplate]

			if !found {
				log.Printf(
					"Error expandComponent '%s': Child for '%s' (GlobalIdx %d) at template offset %d (abs %d) not found in map. Parent template offset %d, child relative offset %d",
					instanceElement.SourceElementName, parentEl.SourceElementName, parentEl.OriginalIndex,
					childRef.ChildOffset, childAbsoluteOffsetInTemplate, info.parentHeaderOffsetInTemplate, childRef.ChildOffset,
				)
				continue
			}
			childEl := &(*allElements)[childGlobalIndex]

			if childEl.Parent != nil && childEl.Parent != parentEl {
				log.Printf(
					"Warn expandComponent: Template child '%s' (GlobalIdx %d) already has parent '%s' (GlobalIdx %d). Cannot set new parent '%s' (GlobalIdx %d).",
					childEl.SourceElementName, childEl.OriginalIndex,
					childEl.Parent.SourceElementName, childEl.Parent.OriginalIndex,
					parentEl.SourceElementName, parentEl.OriginalIndex,
				)
				continue
			}
			childEl.Parent = parentEl
			parentEl.Children = append(parentEl.Children, childEl)
		}
	}

	// Set the children of the original instanceElement to be the roots found in this template expansion
	if instanceElement != nil {
		instanceElement.Children = make([]*render.RenderElement, 0, len(templateRootsInThisExpansion))

		for _, rootTplEl := range templateRootsInThisExpansion {

			if rootTplEl.Parent == instanceElement { // Double check parentage
				instanceElement.Children = append(instanceElement.Children, rootTplEl)
			}
		}
	}

	// Slot KRY-usage children (children defined at the component's usage site)
	if len(kryUsageChildren) > 0 {
		slotFound := false
		var slotElement *render.RenderElement // The element in the template marked as children_host

		queue := make([]*render.RenderElement, 0, len(instanceElement.Children))

		if instanceElement.Children != nil {
			queue = append(queue, instanceElement.Children...)
		}

		visitedInSearch := make(map[*render.RenderElement]bool) // Prevent cycles

		for len(queue) > 0 {
			currentSearchNode := queue[0]
			queue = queue[1:]

			if visitedInSearch[currentSearchNode] {
				continue
			}
			visitedInSearch[currentSearchNode] = true

			idName, _ := getStringValueByIdx(doc, currentSearchNode.Header.ID)

			if idName == childrenSlotIDName {
				slotElement = currentSearchNode
				slotFound = true
				break
			}

			for _, childOfSearchNode := range currentSearchNode.Children {

				if !visitedInSearch[childOfSearchNode] {
					queue = append(queue, childOfSearchNode)
				}
			}
		}

		if slotFound && slotElement != nil {
			log.Printf(
				"expandComponent '%s': Found slot '%s' (GlobalIdx %d). Re-parenting %d KRY-usage children.",
				instanceElement.SourceElementName, childrenSlotIDName, slotElement.OriginalIndex, len(kryUsageChildren),
			)
			slotElement.Children = append(slotElement.Children, kryUsageChildren...)

			for _, kryChild := range kryUsageChildren {
				kryChild.Parent = slotElement // Re-parent KRY children to the slot
			}
		} else {
			log.Printf(
				"Warn expandComponent '%s': No slot '%s' found in template. Appending %d KRY-usage children to first template root (if any).",
				instanceElement.SourceElementName, childrenSlotIDName, len(kryUsageChildren),
			)

			if len(instanceElement.Children) > 0 {
				firstRoot := instanceElement.Children[0]
				firstRoot.Children = append(firstRoot.Children, kryUsageChildren...)

				for _, kryChild := range kryUsageChildren {
					kryChild.Parent = firstRoot
				}
			} else {
				log.Printf(
					"Error expandComponent '%s': No template root to append KRY-usage children to, and no slot found. KRY children are unparented from this component instance.",
					instanceElement.SourceElementName,
				)
			}
		}
	}
	return nil
}

func (r *RaylibRenderer) PerformLayout(
	el *render.RenderElement,
	parentContentX, parentContentY, parentContentW, parentContentH float32,
) {

	if el == nil {
		return
	}
	doc := r.docRef
	scale := r.scaleFactor

	elementIdentifier := el.SourceElementName

	if elementIdentifier == "" && el.Header.ID != 0 && doc != nil {
		idStr, _ := getStringValueByIdx(doc, el.Header.ID)

		if idStr != "" {
			elementIdentifier = idStr
		}
	}

	if elementIdentifier == "" {
		elementIdentifier = fmt.Sprintf("Type0x%X_Idx%d_NoName", el.Header.Type, el.OriginalIndex)
	}

	//isSpecificElementToLog := strings.Contains(elementIdentifier, "HelloWidget") || elementIdentifier == "Type0x1_Idx1"
	isSpecificElementToLog := false

	if isSpecificElementToLog {
		log.Printf(
			">>>>> PerformLayout for: %s (Type:0x%X, OrigIdx:%d) ParentCTX:%.0f,%.0f,%.0f,%.0f",
			elementIdentifier, el.Header.Type, el.OriginalIndex, parentContentX, parentContentY, parentContentW, parentContentH,
		)
		log.Printf(
			"      Hdr: W:%d,H:%d,PosX:%d,PosY:%d,Layout:0x%02X(Abs:%t,Grow:%t)",
			el.Header.Width, el.Header.Height, el.Header.PosX, el.Header.PosY,
			el.Header.Layout, el.Header.LayoutAbsolute(), el.Header.LayoutGrow(),
		)
	}

	isRootElement := (el.Parent == nil)
	scaledUint16Local := func(v uint16) float32 { return float32(v) * scale }

	// --- Step 1: Determine EXPLICIT Size ---
	hasExplicitWidth := false
	desiredWidth := float32(0.0)

	if el.Header.Width > 0 {
		desiredWidth = scaledUint16Local(el.Header.Width)
		hasExplicitWidth = true
	}

	hasExplicitHeight := false
	desiredHeight := float32(0.0)

	if el.Header.Height > 0 {
		desiredHeight = scaledUint16Local(el.Header.Height)
		hasExplicitHeight = true
	}

	if doc != nil && el.OriginalIndex < len(doc.Properties) && doc.Properties[el.OriginalIndex] != nil {
		elementDirectProps := doc.Properties[el.OriginalIndex]
		propWVal, propWType, _, propWErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMaxWidth, doc)

		if propWErr == nil {
			explicitPropWidth := MuxFloat32(propWType == krb.ValTypePercentage, (propWVal/256.0)*parentContentW, propWVal*scale)

			if !hasExplicitWidth || (explicitPropWidth > 0 && explicitPropWidth < desiredWidth) {
				desiredWidth = explicitPropWidth
				hasExplicitWidth = true
			} else if !hasExplicitWidth && explicitPropWidth > 0 {
				desiredWidth = explicitPropWidth
				hasExplicitWidth = true
			}
		}
		propHVal, propHType, _, propHErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMaxHeight, doc)

		if propHErr == nil {
			explicitPropHeight := MuxFloat32(propHType == krb.ValTypePercentage, (propHVal/256.0)*parentContentH, propHVal*scale)

			if !hasExplicitHeight || (explicitPropHeight > 0 && explicitPropHeight < desiredHeight) {
				desiredHeight = explicitPropHeight
				hasExplicitHeight = true
			} else if !hasExplicitHeight && explicitPropHeight > 0 {
				desiredHeight = explicitPropHeight
				hasExplicitHeight = true
			}
		}
	}

	if isSpecificElementToLog {
		log.Printf("      S1 - Explicit Size: W:%.1f(exp:%t), H:%.1f(exp:%t)", desiredWidth, hasExplicitWidth, desiredHeight, hasExplicitHeight)
	}

	// --- Step 2: Apply INTRINSIC and DEFAULT SIZING ---
	hPadding := ScaledF32(el.Padding[1], scale) + ScaledF32(el.Padding[3], scale)
	vPadding := ScaledF32(el.Padding[0], scale) + ScaledF32(el.Padding[2], scale)
	isGrow := el.Header.LayoutGrow()
	isAbsolute := el.Header.LayoutAbsolute()

	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		var elementFontSize uint16 = uint16(baseFontSize)

		if doc != nil && el.OriginalIndex < len(doc.Properties) && doc.Properties[el.OriginalIndex] != nil {

			for _, prop := range doc.Properties[el.OriginalIndex] {

				if prop.ID == krb.PropIDFontSize {

					if fsVal, fsOk := getShortValue(&prop); fsOk {
						elementFontSize = fsVal
						break
					}
				}
			}
		}
		finalFontSizePixels := MaxF(1.0, ScaledF32(uint8(elementFontSize), scale))

		if !hasExplicitWidth {
			textWidthMeasuredInPixels := float32(rl.MeasureText(el.Text, int32(finalFontSizePixels)))
			desiredWidth = textWidthMeasuredInPixels + hPadding

			if isSpecificElementToLog {
				log.Printf("      S2a - Intrinsic W (Text): %.1f (text:%.1f, hPad:%.1f)", desiredWidth, textWidthMeasuredInPixels, hPadding)
			}
		}

		if !hasExplicitHeight {
			textHeightMeasuredInPixels := finalFontSizePixels
			desiredHeight = textHeightMeasuredInPixels + vPadding

			if isSpecificElementToLog {
				log.Printf("      S2a - Intrinsic H (Text): %.1f (text:%.1f, vPad:%.1f)", desiredHeight, textHeightMeasuredInPixels, vPadding)
			}
		}
	} else if el.Header.Type == krb.ElemTypeImage && el.ResourceIndex != render.InvalidResourceIndex {
		texWidthPx := float32(0)
		texHeightPx := float32(0)

		if el.TextureLoaded && el.Texture.ID > 0 {
			texWidthPx = float32(el.Texture.Width)
			texHeightPx = float32(el.Texture.Height)
		}

		if !hasExplicitWidth {
			desiredWidth = texWidthPx*scale + hPadding

			if isSpecificElementToLog {
				log.Printf("      S2b - Intrinsic W (Image): %.1f (texW_native:%.1f, scale:%.1f, hPad:%.1f)", desiredWidth, texWidthPx, scale, hPadding)
			}
		}

		if !hasExplicitHeight {
			desiredHeight = texHeightPx*scale + vPadding

			if isSpecificElementToLog {
				log.Printf("      S2b - Intrinsic H (Image): %.1f (texH_native:%.1f, scale:%.1f, vPad:%.1f)", desiredHeight, texHeightPx, scale, vPadding)
			}
		}
	}

	if !hasExplicitWidth && !isGrow && !isAbsolute {

		if desiredWidth == 0 && (el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp) {
			desiredWidth = parentContentW

			if isSpecificElementToLog {
				log.Printf("      S2c - Default W (Container/App): %.1f from parent content area", desiredWidth)
			}
		}
	}

	if !hasExplicitHeight && !isGrow && !isAbsolute {

		if desiredHeight == 0 && (el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp) {
			desiredHeight = parentContentH

			if isSpecificElementToLog {
				log.Printf("      S2c - Default H (Container/App): %.1f from parent content area", desiredHeight)
			}
		}
	}

	if isRootElement {
		if !hasExplicitWidth {
			el.RenderW = parentContentW
		} else {
			el.RenderW = desiredWidth
		}

		if !hasExplicitHeight {
			el.RenderH = parentContentH
		} else {
			el.RenderH = desiredHeight
		}

		if isSpecificElementToLog || el.Header.Type == krb.ElemTypeApp {
			// log.Printf("      S2d - Default/Explicit Size (Root/App): W:%.1f H:%.1f from screen/parent context or explicit", el.RenderW, el.RenderH)
		}
	} else {
		el.RenderW = MaxF(0, desiredWidth)
		el.RenderH = MaxF(0, desiredHeight)
	}

	if isSpecificElementToLog {
		log.Printf("      S2 - Final Desired Before Assignment: W:%.1f, H:%.1f", desiredWidth, desiredHeight)
		log.Printf("      S2 - Assigned RenderW/H: W:%.1f, H:%.1f", el.RenderW, el.RenderH)
	}

	// --- Step 3: Determine Base Render Position ---
	if el.Header.LayoutAbsolute() {
		offsetX := scaledUint16Local(el.Header.PosX)
		offsetY := scaledUint16Local(el.Header.PosY)

		if el.Parent != nil {
			el.RenderX = el.Parent.RenderX + offsetX
			el.RenderY = el.Parent.RenderY + offsetY
		} else {
			el.RenderX = parentContentX + offsetX
			el.RenderY = parentContentY + offsetY
		}
	} else {
		el.RenderX = parentContentX
		el.RenderY = parentContentY
	}

	if isSpecificElementToLog {
		log.Printf("      S3 - Initial Position: X:%.1f, Y:%.1f (Abs:%t)", el.RenderX, el.RenderY, el.Header.LayoutAbsolute())
	}

	// --- Step 4: Calculate Content Area for Children ---
	childPaddingTop := ScaledF32(el.Padding[0], scale)
	childPaddingRight := ScaledF32(el.Padding[1], scale)
	childPaddingBottom := ScaledF32(el.Padding[2], scale)
	childPaddingLeft := ScaledF32(el.Padding[3], scale)
	childBorderTop := ScaledF32(el.BorderWidths[0], scale)
	childBorderRight := ScaledF32(el.BorderWidths[1], scale)
	childBorderBottom := ScaledF32(el.BorderWidths[2], scale)
	childBorderLeft := ScaledF32(el.BorderWidths[3], scale)

	childContentAreaX := el.RenderX + childBorderLeft + childPaddingLeft
	childContentAreaY := el.RenderY + childBorderTop + childPaddingTop
	childAvailableWidth := el.RenderW - (childBorderLeft + childBorderRight + childPaddingLeft + childPaddingRight)
	childAvailableHeight := el.RenderH - (childBorderTop + childBorderBottom + childPaddingTop + childPaddingBottom)
	childAvailableWidth = MaxF(0, childAvailableWidth)
	childAvailableHeight = MaxF(0, childAvailableHeight)

	if isSpecificElementToLog {
		log.Printf(
			"      S4 - Child Content Area (relative to el.RenderX/Y): X_offset_to_content:%.1f, Y_offset_to_content:%.1f",
			childBorderLeft+childPaddingLeft, childBorderTop+childPaddingTop,
		)
		log.Printf(
			"           Absolute Child Content Origin: X:%.1f, Y:%.1f. Available W:%.1f, H:%.1f for children flow.",
			childContentAreaX, childContentAreaY, childAvailableWidth, childAvailableHeight,
		)
	}

	// --- Step 5 & 6: Layout Children & Content Hugging ---
	if len(el.Children) > 0 {

		if isSpecificElementToLog {
			log.Printf("      S5 - Layout Children for %s...", elementIdentifier)
		}
		r.PerformLayoutChildren(el, childContentAreaX, childContentAreaY, childAvailableWidth, childAvailableHeight)

		if !isRootElement && !hasExplicitHeight && !isGrow && !isAbsolute {
			maxChildExtentY := float32(0.0)
			parentLayoutDir := el.Header.LayoutDirection()
			isParentVertical := (parentLayoutDir == krb.LayoutDirColumn || parentLayoutDir == krb.LayoutDirColumnReverse)

			// Simplified: assumes children contribute to Y extent for hugging height
			if isParentVertical {
				currentY := float32(0)
				numFlowChildren := 0
				gapValue := float32(0) // TODO: Get parent's gap property from style/direct
				// This requires a utility or direct lookup similar to PerformLayoutChildren
				for _, child := range el.Children {

					if child != nil && !child.Header.LayoutAbsolute() {

						if numFlowChildren > 0 {
							currentY += gapValue
						}
						currentY += child.RenderH
						numFlowChildren++
					}
				}
				maxChildExtentY = currentY
			} else { // Horizontal flow or other complex cases
				for _, child := range el.Children {

					if child != nil && !child.Header.LayoutAbsolute() {
						childRelativeY := child.RenderY - childContentAreaY
						currentChildBottom := childRelativeY + child.RenderH

						if currentChildBottom > maxChildExtentY {
							maxChildExtentY = currentChildBottom
						}
					}
				}
			}

			contentHeightFromChildren := maxChildExtentY + vPadding + childBorderTop + childBorderBottom

			if contentHeightFromChildren > 0 {

				if el.RenderH == 0 || (contentHeightFromChildren < el.RenderH && (el.Header.Type != krb.ElemTypeContainer && el.Header.Type != krb.ElemTypeApp)) {
					el.RenderH = contentHeightFromChildren

					if isSpecificElementToLog {
						log.Printf("      S6 - Content Hug H for %s: %.1f", elementIdentifier, el.RenderH)
					}
				} else if (el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp) && contentHeightFromChildren < el.RenderH {
					el.RenderH = contentHeightFromChildren

					if isSpecificElementToLog {
						log.Printf("      S6 - Content Shrink H for Container %s: %.1f", elementIdentifier, el.RenderH)
					}
				}
			}
		}
	}

	if isSpecificElementToLog {
		log.Printf(
			"      S5/6 - After Children/Hugging for %s: W:%.1f, H:%.1f, X:%.1f, Y:%.1f",
			elementIdentifier, el.RenderW, el.RenderH, el.RenderX, el.RenderY,
		)
	}

	// --- Step 7: Apply Min-Width/Height Constraints ---
	if doc != nil && el.OriginalIndex < len(doc.Properties) && doc.Properties[el.OriginalIndex] != nil {
		elementDirectProps := doc.Properties[el.OriginalIndex]
		minWVal, minWType, _, minWErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMinWidth, doc)

		if minWErr == nil {
			minWidthConstraint := MuxFloat32(minWType == krb.ValTypePercentage, (minWVal/256.0)*parentContentW, minWVal*scale)

			if el.RenderW < minWidthConstraint {
				el.RenderW = minWidthConstraint
			}
		}
		minHVal, minHType, _, minHErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMinHeight, doc)

		if minHErr == nil {
			minHeightConstraint := MuxFloat32(minHType == krb.ValTypePercentage, (minHVal/256.0)*parentContentH, minHVal*scale)

			if el.RenderH < minHeightConstraint {
				el.RenderH = minHeightConstraint
			}
		}
	}

	if isSpecificElementToLog {
		log.Printf("      S7 - Min/Max Constraints Applied for %s: W:%.1f, H:%.1f", elementIdentifier, el.RenderW, el.RenderH)
	}

	// --- Step 8: Final Fallback for Zero Size ---
	el.RenderW = MaxF(0, el.RenderW)
	el.RenderH = MaxF(0, el.RenderH)

	if el.RenderW > 0 && el.RenderH == 0 {
		isVisualPlaceholder := el.Header.Type == krb.ElemTypeContainer ||
			el.Header.Type == krb.ElemTypeApp ||
			el.BgColor.A > 0 ||
			(el.BorderWidths[0]+el.BorderWidths[1]+el.BorderWidths[2]+el.BorderWidths[3] > 0)

		if isVisualPlaceholder {
			finalMinHeight := MaxF(ScaledF32(uint8(baseFontSize), scale), vPadding+childBorderTop+childBorderBottom)

			if finalMinHeight == 0 && (el.BgColor.A > 0 || (childBorderTop+childBorderBottom > 0)) {
				finalMinHeight = 1.0 * scale
			}

			if finalMinHeight > 0 {
				el.RenderH = finalMinHeight

				if isSpecificElementToLog {
					log.Printf("      S8 - Fallback Zero H for %s: %.1f applied", elementIdentifier, el.RenderH)
				}
			}
		}
	}
	el.RenderH = MaxF(0, el.RenderH)

	if isSpecificElementToLog {
		log.Printf(
			"<<<<< PerformLayout END for: %s -- Final Render: X:%.1f,Y:%.1f, W:%.1f,H:%.1f",
			elementIdentifier, el.RenderX, el.RenderY, el.RenderW, el.RenderH,
		)
	}
}

func (r *RaylibRenderer) PerformLayoutChildren(
	parent *render.RenderElement,
	parentClientOriginX, parentClientOriginY,
	availableClientWidth, availableClientHeight float32,
) {

	if parent == nil || len(parent.Children) == 0 {
		return
	}
	doc := r.docRef
	scale := r.scaleFactor

	parentIdentifier := parent.SourceElementName

	if parentIdentifier == "" {
		parentIdentifier = fmt.Sprintf("ParentType0x%X_Idx%d", parent.Header.Type, parent.OriginalIndex)
	}

	//isParentSpecificToLog := strings.Contains(parentIdentifier, "HelloWidget") || parentIdentifier == "Type0x0_Idx0"
	isParentSpecificToLog := false
	if isParentSpecificToLog {
		log.Printf(
			">>>>> PerformLayoutChildren for PARENT: %s (ContentOrigin: X:%.0f,Y:%.0f, AvailW:%.0f,AvailH:%.0f, LayoutByte:0x%02X)",
			parentIdentifier, parentClientOriginX, parentClientOriginY, availableClientWidth, availableClientHeight, parent.Header.Layout,
		)
	}

	flowChildren := make([]*render.RenderElement, 0, len(parent.Children))
	absoluteChildren := make([]*render.RenderElement, 0)

	for _, child := range parent.Children {

		if child != nil {

			if child.Header.LayoutAbsolute() {
				absoluteChildren = append(absoluteChildren, child)
			} else {
				flowChildren = append(flowChildren, child)
			}
		}
	}

	scaledUint16Local := func(v uint16) float32 { return float32(v) * scale }

	// --- Layout Flow Children ---
	if len(flowChildren) > 0 {
		layoutDirection := parent.Header.LayoutDirection()
		layoutAlignment := parent.Header.LayoutAlignment()
		crossAxisAlignment := parent.Header.LayoutCrossAlignment()
		isLayoutReversed := (layoutDirection == krb.LayoutDirRowReverse || layoutDirection == krb.LayoutDirColumnReverse)
		isMainAxisHorizontal := (layoutDirection == krb.LayoutDirRow || layoutDirection == krb.LayoutDirRowReverse)

		gapValue := float32(0)

		if parentStyle, styleFound := findStyle(doc, parent.Header.StyleID); styleFound {

			if gapProp, propFound := getStylePropertyValue(parentStyle, krb.PropIDGap); propFound {

				if gVal, valOk := getShortValue(gapProp); valOk {
					gapValue = float32(gVal) * scale
				}
			}
		}

		if doc != nil && parent.OriginalIndex < len(doc.Properties) && len(doc.Properties[parent.OriginalIndex]) > 0 {

			for _, prop := range doc.Properties[parent.OriginalIndex] {

				if prop.ID == krb.PropIDGap {

					if gVal, valOk := getShortValue(&prop); valOk {
						gapValue = float32(gVal) * scale
						break
					}
				}
			}
		}

		totalGapSpace := float32(0)

		if len(flowChildren) > 1 {
			totalGapSpace = gapValue * float32(len(flowChildren)-1)
		}

		mainAxisEffectiveSpaceForParentLayout := MuxFloat32(isMainAxisHorizontal, availableClientWidth, availableClientHeight)
		mainAxisEffectiveSpaceForElements := MaxF(0, mainAxisEffectiveSpaceForParentLayout-totalGapSpace)
		crossAxisEffectiveSizeForParentLayout := MuxFloat32(isMainAxisHorizontal, availableClientHeight, availableClientWidth)

		// Pass 1: Sizing
		for _, child := range flowChildren {

			if isParentSpecificToLog {
				log.Printf("      PLC Pass 1 (Sizing) - Calling PerformLayout for child: %s", child.SourceElementName)
			}
			r.PerformLayout(child, parentClientOriginX, parentClientOriginY, availableClientWidth, availableClientHeight)
		}

		// Pass 2: Calculate fixed size and grow children
		totalFixedSizeOnMainAxis := float32(0)
		numberOfGrowChildren := 0

		for _, child := range flowChildren {

			if child.Header.LayoutGrow() {
				numberOfGrowChildren++
			} else {
				totalFixedSizeOnMainAxis += MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
			}
		}
		totalFixedSizeOnMainAxis = MaxF(0, totalFixedSizeOnMainAxis)

		spaceAvailableForGrowingChildren := MaxF(0, mainAxisEffectiveSpaceForElements-totalFixedSizeOnMainAxis)
		sizePerGrowChild := float32(0)

		if numberOfGrowChildren > 0 && spaceAvailableForGrowingChildren > 0 {
			sizePerGrowChild = spaceAvailableForGrowingChildren / float32(numberOfGrowChildren)
		}

		// Pass 3: Apply grow and cross-axis stretch
		totalFinalElementSizeOnMainAxis := float32(0)

		for _, child := range flowChildren {

			if child.Header.LayoutGrow() && sizePerGrowChild > 0 {

				if isMainAxisHorizontal {
					child.RenderW = sizePerGrowChild
				} else {
					child.RenderH = sizePerGrowChild
				}

				if isParentSpecificToLog {
					log.Printf(
						"      PLC Pass 3 (Grow) - Child %s grew to main-axis size: %.1f",
						child.SourceElementName, MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH),
					)
				}
			}

			if crossAxisAlignment == krb.LayoutAlignStretch {

				if isMainAxisHorizontal {

					if child.Header.Height == 0 && child.RenderH < crossAxisEffectiveSizeForParentLayout {
						child.RenderH = crossAxisEffectiveSizeForParentLayout

						if isParentSpecificToLog {
							log.Printf("      PLC Pass 3 (Stretch) - Child %s stretched H to %.1f", child.SourceElementName, child.RenderH)
						}
					}
				} else {

					if child.Header.Width == 0 && child.RenderW < crossAxisEffectiveSizeForParentLayout {
						child.RenderW = crossAxisEffectiveSizeForParentLayout

						if isParentSpecificToLog {
							log.Printf("      PLC Pass 3 (Stretch) - Child %s stretched W to %.1f", child.SourceElementName, child.RenderW)
						}
					}
				}
			}
			child.RenderW = MaxF(0, child.RenderW)
			child.RenderH = MaxF(0, child.RenderH)
			totalFinalElementSizeOnMainAxis += MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
		}

		totalUsedSpaceWithGaps := totalFinalElementSizeOnMainAxis + totalGapSpace
		startOffsetOnMainAxis, effectiveSpacingBetweenItems := calculateAlignmentOffsetsF(
			layoutAlignment,
			mainAxisEffectiveSpaceForParentLayout,
			totalUsedSpaceWithGaps,
			len(flowChildren), isLayoutReversed, gapValue,
		)

		if isParentSpecificToLog {
			log.Printf("      PLC Details: mainEffSpaceForElems:%.0f, crossEffSizeForParent:%.0f", mainAxisEffectiveSpaceForElements, crossAxisEffectiveSizeForParentLayout)
			log.Printf("      PLC Details: totalFixed:%.0f, numGrow:%d, spaceForGrow:%.0f, sizePerGrow:%.0f", totalFixedSizeOnMainAxis, numberOfGrowChildren, spaceAvailableForGrowingChildren, sizePerGrowChild)
			log.Printf("      PLC Details: totalFinalMainAxis:%.0f, totalUsedWithGaps:%.0f", totalFinalElementSizeOnMainAxis, totalUsedSpaceWithGaps)
			log.Printf("      PLC Details: startOffMain:%.0f, effSpacing:%.0f", startOffsetOnMainAxis, effectiveSpacingBetweenItems)
		}

		// Pass 4: Position and recurse
		currentMainAxisPosition := startOffsetOnMainAxis
		childOrderIndices := make([]int, len(flowChildren))

		for i := range childOrderIndices {
			childOrderIndices[i] = i
		}

		if isLayoutReversed {
			ReverseSliceInt(childOrderIndices)
		}

		for i, orderedChildIndex := range childOrderIndices {
			child := flowChildren[orderedChildIndex]
			childMainAxisSizeValue := MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
			childCrossAxisSizeValue := MuxFloat32(isMainAxisHorizontal, child.RenderH, child.RenderW)
			crossAxisOffset := calculateCrossAxisOffsetF(crossAxisAlignment, crossAxisEffectiveSizeForParentLayout, childCrossAxisSizeValue)

			if isMainAxisHorizontal {
				child.RenderX = parentClientOriginX + currentMainAxisPosition
				child.RenderY = parentClientOriginY + crossAxisOffset
			} else {
				child.RenderX = parentClientOriginX + crossAxisOffset
				child.RenderY = parentClientOriginY + currentMainAxisPosition
			}

			if !child.Header.LayoutAbsolute() && (child.Header.PosX != 0 || child.Header.PosY != 0) {
				childOwnOffsetX := scaledUint16Local(child.Header.PosX)
				childOwnOffsetY := scaledUint16Local(child.Header.PosY)
				child.RenderX += childOwnOffsetX
				child.RenderY += childOwnOffsetY
				if isParentSpecificToLog || child.SourceElementName == "Type0x1_Idx1" {
					log.Printf("      PLC Pass 4 - Child %s applied its own PosX/Y offset: dX:%.1f, dY:%.1f. New pos: X:%.1f,Y:%.1f",
						child.SourceElementName, childOwnOffsetX, childOwnOffsetY, child.RenderX, child.RenderY)
				}
			}

			if isParentSpecificToLog {
				log.Printf(
					"      PLC Pass 4 - Positioned Child %s: Final X:%.0f,Y:%.0f (Child W:%.0f,H:%.0f)",
					child.SourceElementName, child.RenderX, child.RenderY, child.RenderW, child.RenderH,
				)
			}

			if len(child.Children) > 0 {
				childPaddingTop := ScaledF32(child.Padding[0], scale)
				childPaddingRight := ScaledF32(child.Padding[1], scale)
				childPaddingBottom := ScaledF32(child.Padding[2], scale)
				childPaddingLeft := ScaledF32(child.Padding[3], scale)
				childBorderTop := ScaledF32(child.BorderWidths[0], scale)
				childBorderRight := ScaledF32(child.BorderWidths[1], scale)
				childBorderBottom := ScaledF32(child.BorderWidths[2], scale)
				childBorderLeft := ScaledF32(child.BorderWidths[3], scale)

				grandChildContentAreaX := child.RenderX + childBorderLeft + childPaddingLeft
				grandChildContentAreaY := child.RenderY + childBorderTop + childPaddingTop
				grandChildAvailableWidth := child.RenderW - (childBorderLeft + childBorderRight + childPaddingLeft + childPaddingRight)
				grandChildAvailableHeight := child.RenderH - (childBorderTop + childBorderBottom + childPaddingTop + childPaddingBottom)
				grandChildAvailableWidth = MaxF(0, grandChildAvailableWidth)
				grandChildAvailableHeight = MaxF(0, grandChildAvailableHeight)

				r.PerformLayoutChildren(child, grandChildContentAreaX, grandChildContentAreaY, grandChildAvailableWidth, grandChildAvailableHeight)
			}

			currentMainAxisPosition += childMainAxisSizeValue

			if i < len(flowChildren)-1 {
				currentMainAxisPosition += effectiveSpacingBetweenItems
			}
		}
	}

	// --- Layout Absolute Children ---
	if len(absoluteChildren) > 0 {

		for _, child := range absoluteChildren {

			if isParentSpecificToLog {
				log.Printf(
					"      PLC - Calling PerformLayout for Absolute Child: %s (Parent Frame: X:%.0f,Y:%.0f W:%.0f,H:%.0f)",
					child.SourceElementName, parent.RenderX, parent.RenderY, parent.RenderW, parent.RenderH,
				)
			}
			r.PerformLayout(child, parent.RenderX, parent.RenderY, parent.RenderW, parent.RenderH)
		}
	}

	if isParentSpecificToLog {
		log.Printf("<<<<< PerformLayoutChildren END for PARENT: %s", parentIdentifier)
	}
}

func getStringValueByIdxFallback(doc *krb.Document, idx uint8, fallback string) string {
	s, ok := getStringValueByIdx(doc, idx)

	if !ok {
		return fallback
	}
	return s
}
