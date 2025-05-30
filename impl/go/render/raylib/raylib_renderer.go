// render/raylib/raylib_renderer.go
package raylib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

// baseFontSize defines the default size for text rendering.
const baseFontSize = 18.0

// componentNameConventionKey is the KRB custom property key used to identify
// the original KRY component name for instances. This must match the key
// used by the Kryon compiler when it generates KRB files.
const componentNameConventionKey = "_componentName"

// RaylibRenderer implements the render.Renderer interface using the Raylib graphics library.
// It handles window initialization, KRB document processing, layout, drawing, and event polling.
type RaylibRenderer struct {
	config          render.WindowConfig
	elements        []render.RenderElement // Slice of actual RenderElement structs, will grow
	roots           []*render.RenderElement
	loadedTextures  map[uint8]rl.Texture2D
	krbFileDir      string
	scaleFactor     float32
	docRef          *krb.Document
	eventHandlerMap map[string]func()
	customHandlers  map[string]render.CustomComponentHandler
}

// NewRaylibRenderer creates and initializes a new RaylibRenderer instance with default values.
func NewRaylibRenderer() *RaylibRenderer {
	return &RaylibRenderer{
		loadedTextures:  make(map[uint8]rl.Texture2D),
		scaleFactor:     1.0,
		eventHandlerMap: make(map[string]func()),
		customHandlers:  make(map[string]render.CustomComponentHandler),
	}
}

// Init initializes the Raylib window according to the provided configuration.
func (r *RaylibRenderer) Init(config render.WindowConfig) error {
	r.config = config
	r.scaleFactor = float32(math.Max(1.0, float64(config.ScaleFactor)))

	log.Printf("RaylibRenderer Init: Initializing window %dx%d. Title: '%s'. UI Scale: %.2f.",
		config.Width, config.Height, config.Title, r.scaleFactor)

	rl.InitWindow(int32(config.Width), int32(config.Height), config.Title)

	if config.Resizable {
		rl.SetWindowState(rl.FlagWindowResizable)
	} else {
		rl.ClearWindowState(rl.FlagWindowResizable)
		rl.SetWindowSize(config.Width, config.Height) // Enforce if not resizable
	}

	rl.SetTargetFPS(60)

	if !rl.IsWindowReady() {
		return fmt.Errorf("RaylibRenderer Init: rl.InitWindow failed or window is not ready")
	}
	log.Println("RaylibRenderer Init: Raylib window is ready.")
	return nil
}

// PrepareTree processes a parsed KRB document to build a tree of RenderElement objects.
func (r *RaylibRenderer) PrepareTree(doc *krb.Document, krbFilePath string) ([]*render.RenderElement, render.WindowConfig, error) {
	if doc == nil {
		log.Println("PrepareTree: KRB document is nil.")
		return nil, r.config, fmt.Errorf("PrepareTree: KRB document is nil")
	}
	r.docRef = doc

	var err error
	r.krbFileDir, err = filepath.Abs(filepath.Dir(krbFilePath))
	if err != nil {
		r.krbFileDir = filepath.Dir(krbFilePath) // Fallback
		log.Printf("WARN PrepareTree: Failed to get absolute path for KRB file dir '%s': %v. Using relative base: %s", krbFilePath, err, r.krbFileDir)
	}
	log.Printf("PrepareTree: Resource Base Directory set to: %s", r.krbFileDir)

	// Initialize window configuration with library defaults
	windowConfig := render.DefaultWindowConfig()
	windowConfig.DefaultBg = rl.Black // Default window clear color

	// Default visual properties for elements
	defaultForegroundColor := rl.RayWhite
	defaultBorderColor := rl.Gray
	defaultBorderWidth := uint8(0)
	defaultTextAlignment := uint8(krb.LayoutAlignStart)
	defaultIsVisible := true

	// Process the App element first (if present)
	isAppElementPresent := (doc.Header.Flags&krb.FlagHasApp) != 0 &&
		doc.Header.ElementCount > 0 &&
		doc.Elements[0].Type == krb.ElemTypeApp

	if isAppElementPresent {
		appElementKrbHeader := &doc.Elements[0]
		if appStyle, styleFound := findStyle(doc, appElementKrbHeader.StyleID); styleFound {
			applyStylePropertiesToWindowDefaults(appStyle.Properties, doc, &windowConfig.DefaultBg)
		} else if appElementKrbHeader.StyleID != 0 {
			log.Printf("Warn PrepareTree: App element has StyleID %d, but this style was not found.", appElementKrbHeader.StyleID)
		}

		if len(doc.Properties) > 0 && len(doc.Properties[0]) > 0 {
			applyDirectPropertiesToConfig(doc.Properties[0], doc, &windowConfig)
		}
		r.scaleFactor = float32(math.Max(1.0, float64(windowConfig.ScaleFactor))) // Update scale factor
		log.Printf("PrepareTree: Processed App element. Final Window Config: Width=%d, Height=%d, Title='%s', Scale=%.2f, Resizable=%t",
			windowConfig.Width, windowConfig.Height, windowConfig.Title, r.scaleFactor, windowConfig.Resizable)
	} else {
		log.Println("PrepareTree: No App element found in KRB. Using default window configuration.")
	}
	r.config = windowConfig

	initialElementCount := int(doc.Header.ElementCount)
	if initialElementCount == 0 {
		log.Println("PrepareTree: No elements in KRB document. Returning empty tree.")
		r.elements = nil
		r.roots = nil
		return nil, r.config, nil
	}

	// Allocate r.elements with initial capacity, it will grow as components are expanded.
	// Using a slice of structs directly for r.elements.
	r.elements = make([]render.RenderElement, initialElementCount, initialElementCount*2) // Heuristic for capacity

	// Pass 1: Create RenderElements for all top-level elements defined in doc.Elements
	for i := 0; i < initialElementCount; i++ {
		renderEl := &r.elements[i]     // Get pointer to the element in the slice
		krbElHeader := doc.Elements[i] // Corresponding KRB element header

		renderEl.Header = krbElHeader // Copy KRB header
		renderEl.OriginalIndex = i    // This is its index in the *original* doc.Elements
		renderEl.DocRef = doc         // Provide access to the full document for helpers

		// Set default visual properties for the element
		renderEl.BgColor = rl.Blank // Default to a fully transparent background
		renderEl.FgColor = defaultForegroundColor
		renderEl.BorderColor = defaultBorderColor
		renderEl.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth}
		renderEl.Padding = [4]uint8{0, 0, 0, 0} // Default padding to zero
		renderEl.TextAlignment = defaultTextAlignment
		renderEl.IsVisible = defaultIsVisible

		renderEl.IsInteractive = (krbElHeader.Type == krb.ElemTypeButton || krbElHeader.Type == krb.ElemTypeInput)
		renderEl.ResourceIndex = render.InvalidResourceIndex

		// Attempt to derive a SourceElementName for debugging and identification
		elementIDString, _ := getStringValueByIdx(doc, renderEl.Header.ID)
		// Check for component name using the ORIGINAL custom properties from doc.CustomProperties[i]
		var componentName string
		if doc.CustomProperties != nil && i < len(doc.CustomProperties) {
			// Use the RenderElement directly for GetCustomPropertyValue
			componentName, _ = GetCustomPropertyValue(renderEl, componentNameConventionKey, doc)
		}

		if componentName != "" {
			renderEl.SourceElementName = componentName
		} else if elementIDString != "" {
			renderEl.SourceElementName = elementIDString
		} else {
			renderEl.SourceElementName = fmt.Sprintf("Type0x%X_Idx%d", renderEl.Header.Type, renderEl.OriginalIndex)
		}

		// Apply styles to the element
		elementStyle, styleFound := findStyle(doc, krbElHeader.StyleID)
		if styleFound {
			applyStylePropertiesToElement(elementStyle.Properties, doc, renderEl)
		} else if krbElHeader.StyleID != 0 && !(i == 0 && isAppElementPresent) {
			log.Printf("Warn PrepareTree: Element %d (Name: '%s', Type: %X) has StyleID %d, but style was not found.",
				i, renderEl.SourceElementName, krbElHeader.Type, krbElHeader.StyleID)
		}

		// Apply direct properties from KRB
		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			if i == 0 && isAppElementPresent {
				applyDirectVisualPropertiesToAppElement(doc.Properties[0], doc, renderEl)
			} else {
				applyDirectPropertiesToElement(doc.Properties[i], doc, renderEl)
			}
		}

		resolveElementText(doc, renderEl, elementStyle, styleFound)
		resolveElementImageSource(doc, renderEl, elementStyle, styleFound)
		resolveEventHandlers(doc, renderEl) // Uses OriginalIndex to look up in doc.Events
	}

	// Pass 2: Expand Component Instances
	nextMasterIndex := initialElementCount // Next available index for globally unique OriginalIndex
	for i := 0; i < initialElementCount; i++ { // Iterate only original elements for expansion
		instanceElement := &r.elements[i] // This is the placeholder element

		var componentName string
		// Custom properties for instanceElement were resolved from doc.CustomProperties[i]
		componentName, _ = GetCustomPropertyValue(instanceElement, componentNameConventionKey, doc)

		if componentName != "" {
			compDef := r.findComponentDefinition(doc, componentName)
			if compDef != nil {
				log.Printf("PrepareTree: Expanding component '%s' for instance '%s' (OriginalIndex: %d)", componentName, instanceElement.SourceElementName, instanceElement.OriginalIndex)
				err := r.expandComponent(instanceElement, compDef, doc, &r.elements, &nextMasterIndex)
				if err != nil {
					log.Printf("ERROR PrepareTree: Failed to expand component '%s' for instance '%s': %v", componentName, instanceElement.SourceElementName, err)
					// Decide if this is a fatal error or if we can continue
				}
			} else {
				log.Printf("Warn PrepareTree: Component definition for '%s' (instance '%s') not found.", componentName, instanceElement.SourceElementName)
			}
		}
	}
	// After expansion, r.elements contains all elements (original + expanded from templates)

	// Pass 3: Build the tree structure (parent-child pointers)
	log.Println("PrepareTree: Building final element tree...")
	r.roots = nil // Reset roots before rebuilding
	errBuild := r.buildFullElementTree(initialElementCount)
	if errBuild != nil {
		log.Printf("Error PrepareTree: Failed to build full element tree: %v", errBuild)
		return nil, r.config, fmt.Errorf("failed to build full element tree: %w", errBuild)
	}

	log.Printf("PrepareTree: Tree built successfully. Number of root nodes: %d. Total elements (including expanded): %d.",
		len(r.roots), len(r.elements))
	for rootIdx, rootNode := range r.roots {
		logElementTree(rootNode, 0, fmt.Sprintf("Root[%d]", rootIdx))
	}

	return r.roots, r.config, nil
}

// findComponentDefinition looks up a component definition by name in the document.
func (r *RaylibRenderer) findComponentDefinition(doc *krb.Document, name string) *krb.KrbComponentDefinition {
	if doc == nil || len(doc.ComponentDefinitions) == 0 || len(doc.Strings) == 0 {
		return nil
	}
	for i := range doc.ComponentDefinitions {
		compDef := &doc.ComponentDefinitions[i] // Get pointer to the definition
		if int(compDef.NameIndex) < len(doc.Strings) && doc.Strings[compDef.NameIndex] == name {
			return compDef
		}
	}
	return nil
}

// expandComponent parses a component's template data, creates RenderElements for its contents,
// appends them to the main `allElements` slice, and links them as children to the `instanceElement`.
func (r *RaylibRenderer) expandComponent(
	instanceElement *render.RenderElement,
	compDef *krb.KrbComponentDefinition,
	doc *krb.Document,
	allElements *[]render.RenderElement, // Pointer to the renderer's main elements slice
	nextMasterIndex *int, // Pointer to the next available globally unique OriginalIndex
) error {
	if compDef.RootElementTemplateData == nil || len(compDef.RootElementTemplateData) == 0 {
		log.Printf("Warn expandComponent: Component definition '%s' for instance '%s' has no RootElementTemplateData.", doc.Strings[compDef.NameIndex], instanceElement.SourceElementName)
		instanceElement.Children = nil // Ensure it's an empty slice
		return nil
	}

	templateReader := bytes.NewReader(compDef.RootElementTemplateData)
	var templateRootsInThisExpansion []*render.RenderElement
	templateOffsetToGlobalIndex := make(map[uint32]int) // Maps offset *within template data* to global index in allElements
	type templateChildInfo struct {
		parentGlobalIndex            int
		childRefs                    []krb.ChildRef
		parentHeaderOffsetInTemplate uint32
	}
	var templateChildInfos []templateChildInfo

	// Default visual properties for template elements (can be overridden)
	defaultFgColor := rl.RayWhite
	defaultBorderColor := rl.Gray
	defaultTextAlignment := uint8(krb.LayoutAlignStart)
	defaultIsVisible := true

	templateDataStreamOffset := uint32(0) // Tracks current read position in templateReader

	for templateReader.Len() > 0 {
		currentElementOffsetInTemplate := templateDataStreamOffset

		headerBuf := make([]byte, krb.ElementHeaderSize)
		n, err := templateReader.Read(headerBuf)
		if err == io.EOF {
			break
		}
		if err != nil || n < krb.ElementHeaderSize {
			return fmt.Errorf("expandComponent '%s': failed to read template element header: %w (read %d bytes)", instanceElement.SourceElementName, err, n)
		}
		templateDataStreamOffset += uint32(n)

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

		// Grow allElements slice if needed
		if newElGlobalIndex >= cap(*allElements) {
			newCap := cap(*allElements) * 2
			if newElGlobalIndex >= newCap { // Ensure newCap is sufficient
				newCap = newElGlobalIndex + 10 // Add some buffer
			}
			tempSlice := make([]render.RenderElement, len(*allElements), newCap)
			copy(tempSlice, *allElements)
			*allElements = tempSlice
		}
		// Ensure slice has enough length for direct assignment
		if newElGlobalIndex >= len(*allElements) {
			*allElements = (*allElements)[:newElGlobalIndex+1]
		}

		newEl := &(*allElements)[newElGlobalIndex] // Get pointer to the element to modify
		newEl.OriginalIndex = newElGlobalIndex     // Global unique index
		newEl.Header = templateKrbHeader
		newEl.DocRef = doc

		// Initialize visual defaults
		newEl.BgColor = rl.Blank
		newEl.FgColor = defaultFgColor
		newEl.BorderColor = defaultBorderColor
		newEl.TextAlignment = defaultTextAlignment
		newEl.IsVisible = defaultIsVisible
		newEl.ResourceIndex = render.InvalidResourceIndex
		newEl.IsInteractive = (templateKrbHeader.Type == krb.ElemTypeButton || templateKrbHeader.Type == krb.ElemTypeInput)


		templateOffsetToGlobalIndex[currentElementOffsetInTemplate] = newElGlobalIndex

		templateElIdStr, _ := getStringValueByIdx(doc, templateKrbHeader.ID)
		if templateElIdStr != "" {
			newEl.SourceElementName = templateElIdStr
		} else {
			newEl.SourceElementName = fmt.Sprintf("TplElem_Type0x%X_Idx%d", templateKrbHeader.Type, newEl.OriginalIndex)
		}

		// Parse and Apply Properties from template data
		var templateDirectProps []krb.Property
		if templateKrbHeader.PropertyCount > 0 {
			templateDirectProps = make([]krb.Property, templateKrbHeader.PropertyCount)
			propHeaderBuf := make([]byte, 3)
			for j := uint8(0); j < templateKrbHeader.PropertyCount; j++ {
				nProp, errProp := templateReader.Read(propHeaderBuf)
				if errProp != nil || nProp < 3 {
					return fmt.Errorf("expandComponent '%s': failed to read template property header for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errProp)
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
						return fmt.Errorf("expandComponent '%s': failed to read template property value for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errVal)
					}
					templateDataStreamOffset += uint32(nVal)
				}
			}
		}
		templateStyle, templateStyleFound := findStyle(doc, templateKrbHeader.StyleID)
		if templateStyleFound {
			applyStylePropertiesToElement(templateStyle.Properties, doc, newEl)
		}
		applyDirectPropertiesToElement(templateDirectProps, doc, newEl) // Apply props from template

		// Parse and Apply Custom Properties from template data (primarily for _componentName of nested components)
		var templateCustomProps []krb.CustomProperty
		if templateKrbHeader.CustomPropCount > 0 {
			templateCustomProps = make([]krb.CustomProperty, templateKrbHeader.CustomPropCount)
			customPropHeaderBuf := make([]byte, 3)
			for j := uint8(0); j < templateKrbHeader.CustomPropCount; j++ {
				nCustomProp, errCustomProp := templateReader.Read(customPropHeaderBuf)
				if errCustomProp != nil || nCustomProp < 3 {
					return fmt.Errorf("expandComponent '%s': failed to read template custom property header for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errCustomProp)
				}
				templateDataStreamOffset += uint32(nCustomProp)
				cprop := &templateCustomProps[j]
				cprop.KeyIndex = customPropHeaderBuf[0]
				cprop.ValueType = krb.ValueType(customPropHeaderBuf[1])
				cprop.Size = customPropHeaderBuf[2]
				if cprop.Size > 0 {
					cprop.Value = make([]byte, cprop.Size)
					nVal, errVal := templateReader.Read(cprop.Value)
					if errVal != nil || nVal < int(cprop.Size) {
						return fmt.Errorf("expandComponent '%s': failed to read template custom property value for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errVal)
					}
					templateDataStreamOffset += uint32(nVal)
				}
			}
		}
		var nestedComponentName string
		for _, cProp := range templateCustomProps { // Check parsed custom props from template
			keyName, keyOk := getStringValueByIdx(doc, cProp.KeyIndex)
			if keyOk && keyName == componentNameConventionKey {
				if (cProp.ValueType == krb.ValTypeString || cProp.ValueType == krb.ValTypeResource) && cProp.Size == 1 {
					valueIndex := cProp.Value[0]
					if strVal, strOk := getStringValueByIdx(doc, valueIndex); strOk {
						nestedComponentName = strVal
						newEl.SourceElementName = nestedComponentName // Update if it's a nested component instance
						break
					}
				}
			}
		}

		resolveElementText(doc, newEl, templateStyle, templateStyleFound)
		resolveElementImageSource(doc, newEl, templateStyle, templateStyleFound)

		if templateKrbHeader.EventCount > 0 {
			eventDataSize := int(templateKrbHeader.EventCount) * krb.EventFileEntrySize
			eventBuf := make([]byte, eventDataSize)
			nEvent, errEvent := templateReader.Read(eventBuf)
			if errEvent != nil || nEvent < eventDataSize {
				return fmt.Errorf("expandComponent '%s': failed to read template event block for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errEvent)
			}
			templateDataStreamOffset += uint32(nEvent)
			newEl.EventHandlers = make([]render.EventCallbackInfo, templateKrbHeader.EventCount)
			for k := uint8(0); k < templateKrbHeader.EventCount; k++ {
				offset := int(k) * krb.EventFileEntrySize
				eventType := krb.EventType(eventBuf[offset])
				callbackID := eventBuf[offset+1]
				if handlerName, ok := getStringValueByIdx(doc, callbackID); ok {
					newEl.EventHandlers[k] = render.EventCallbackInfo{EventType: eventType, HandlerName: handlerName}
				} else {
					log.Printf("Warn expandComponent: Template element '%s' has invalid event callback string index %d.", newEl.SourceElementName, callbackID)
				}
			}
		}

		if templateKrbHeader.AnimationCount > 0 {
			animRefDataSize := int(templateKrbHeader.AnimationCount) * krb.AnimationRefSize
			bytesSkipped, errAnim := templateReader.Seek(int64(animRefDataSize), io.SeekCurrent)
			if errAnim != nil || bytesSkipped < int64(animRefDataSize) {
				return fmt.Errorf("expandComponent '%s': failed to seek past template animation refs for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errAnim)
			}
			templateDataStreamOffset += uint32(animRefDataSize)
		}

		// Store child references from template data for later linking
		if templateKrbHeader.ChildCount > 0 {
			templateChildRefs := make([]krb.ChildRef, templateKrbHeader.ChildCount)
			childRefDataSize := int(templateKrbHeader.ChildCount) * krb.ChildRefSize
			childRefBuf := make([]byte, childRefDataSize)
			nChildRef, errChildRef := templateReader.Read(childRefBuf)
			if errChildRef != nil || nChildRef < childRefDataSize {
				return fmt.Errorf("expandComponent '%s': failed to read template child refs for '%s': %w", instanceElement.SourceElementName, newEl.SourceElementName, errChildRef)
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

		// Consider this new element as a root of this template's expansion if it has no parent *within this template*.
		// The first element parsed is typically the root.
		if len(templateRootsInThisExpansion) == 0 { // Or a more robust check for parentlessness within template context
			templateRootsInThisExpansion = append(templateRootsInThisExpansion, newEl)
			newEl.Parent = instanceElement // Link this template root to the component instance element

			// Property inheritance from instance to template root
			// The instanceElement's ID should be used for the component as a whole.
			// Apply instance ID to the first root of the template.
			// Also, the SourceElementName should reflect the instance.
			newEl.Header.ID = instanceElement.Header.ID
			newEl.SourceElementName = instanceElement.SourceElementName // Override template's root name

			if instanceElement.Header.StyleID != 0 { // Instance style can override template root's style
				newEl.Header.StyleID = instanceElement.Header.StyleID
				if ovrStyle, ovrStyleFound := findStyle(doc, newEl.Header.StyleID); ovrStyleFound {
					applyStylePropertiesToElement(ovrStyle.Properties, doc, newEl) // Re-apply merged style
				}
			}
		}
		
		// Handle Nested Components recursively
		if nestedComponentName != "" {
			nestedCompDef := r.findComponentDefinition(doc, nestedComponentName)
			if nestedCompDef != nil {
				log.Printf("expandComponent: Expanding nested component '%s' for template element '%s' (GlobalIdx: %d)", nestedComponentName, newEl.SourceElementName, newEl.OriginalIndex)
				// newEl (the current element being processed from the parent template) acts as the instance placeholder for the nested component.
				err := r.expandComponent(newEl, nestedCompDef, doc, allElements, nextMasterIndex)
				if err != nil {
					return fmt.Errorf("expandComponent '%s': failed to expand nested component '%s': %w", instanceElement.SourceElementName, nestedComponentName, err)
				}
			} else {
				log.Printf("Warn expandComponent: Nested component definition '%s' for template element '%s' not found.", nestedComponentName, newEl.SourceElementName)
			}
		}
	}

	// Link children within the template using the recorded child_infos
	for _, info := range templateChildInfos {
		parentEl := &(*allElements)[info.parentGlobalIndex]

		// If parentEl's children were already populated by a nested component expansion, skip.
		if len(parentEl.Children) > 0 && parentEl.Children[0].Parent == parentEl {
			// This checks if children are already linked. Assumes expandComponent correctly set them.
			continue
		}
		
		parentEl.Children = make([]*render.RenderElement, 0, len(info.childRefs)) // Initialize if not already
		for _, childRef := range info.childRefs {
			childAbsoluteOffsetInTemplate := info.parentHeaderOffsetInTemplate + uint32(childRef.ChildOffset)
			childGlobalIndex, found := templateOffsetToGlobalIndex[childAbsoluteOffsetInTemplate]
			if !found {
				log.Printf("Error expandComponent '%s': Child for '%s' (GlobalIdx %d) at template offset %d (abs %d) not found in map. Parent template offset %d, child relative offset %d",
					instanceElement.SourceElementName, parentEl.SourceElementName, parentEl.OriginalIndex, childRef.ChildOffset, childAbsoluteOffsetInTemplate, info.parentHeaderOffsetInTemplate, childRef.ChildOffset)
				continue
			}
			childEl := &(*allElements)[childGlobalIndex]

			if childEl.Parent != nil && childEl.Parent != parentEl { // Check if already parented differently
                 log.Printf("Warn expandComponent: Template child '%s' (GlobalIdx %d) already has parent '%s'. Cannot set new parent '%s'.", childEl.SourceElementName, childEl.OriginalIndex, childEl.Parent.SourceElementName, parentEl.SourceElementName)
                 continue
            }
			childEl.Parent = parentEl
			parentEl.Children = append(parentEl.Children, childEl)
		}
	}

	// Set the instanceElement's children to the roots of this expansion
	if instanceElement != nil {
		instanceElement.Children = make([]*render.RenderElement, 0, len(templateRootsInThisExpansion)) // Clear any previous stubs
		for _, rootTplEl := range templateRootsInThisExpansion {
			if rootTplEl.Parent == instanceElement { // Ensure it was parented correctly to this instance
				instanceElement.Children = append(instanceElement.Children, rootTplEl)
			}
		}
	}
	return nil
}

// buildFullElementTree constructs the parent-child hierarchy for *all* elements in r.elements.
func (r *RaylibRenderer) buildFullElementTree(initialElementCount int) error {
	if len(r.elements) == 0 {
		r.roots = nil
		return nil
	}

	offsetToInitialElementIndex := make(map[uint32]int)
	if r.docRef.ElementStartOffsets != nil {
		for i := 0; i < initialElementCount && i < len(r.docRef.ElementStartOffsets); i++ {
			offsetToInitialElementIndex[r.docRef.ElementStartOffsets[i]] = i
		}
	}

	for i := 0; i < len(r.elements); i++ { // Iterate all elements, including expanded ones
		currentEl := &r.elements[i]

		// Only process original elements from doc.Elements for KRB ChildRef linking.
		// Component children are linked during expandComponent.
		if currentEl.OriginalIndex < initialElementCount && len(currentEl.Children) == 0 {
			// This element is one of the original top-level elements from krb.Document.Elements
			// and its children were NOT populated by component expansion (meaning it's not a component instance itself,
			// or it's an empty component instance).
			originalKrbHeader := &r.docRef.Elements[currentEl.OriginalIndex]

			if originalKrbHeader.ChildCount > 0 {
				if currentEl.OriginalIndex >= len(r.docRef.ChildRefs) || r.docRef.ChildRefs[currentEl.OriginalIndex] == nil {
					log.Printf("Warn buildFullElementTree: Original Elem %s (OrigIdx %d) has KRB ChildCount %d but no ChildRefs.",
						currentEl.SourceElementName, currentEl.OriginalIndex, originalKrbHeader.ChildCount)
					continue
				}

				krbChildRefs := r.docRef.ChildRefs[currentEl.OriginalIndex]
				// currentEl.Children should be pre-allocated or correctly sized
				currentEl.Children = make([]*render.RenderElement, 0, len(krbChildRefs))


				parentStartOffset := uint32(0)
				if currentEl.OriginalIndex < len(r.docRef.ElementStartOffsets) {
					parentStartOffset = r.docRef.ElementStartOffsets[currentEl.OriginalIndex]
				} else {
                     log.Printf("Error buildFullElementTree: Original Elem %s (OrigIdx %d) missing from ElementStartOffsets.", currentEl.SourceElementName, currentEl.OriginalIndex)
                     continue
                }


				for _, childRef := range krbChildRefs {
					childAbsoluteFileOffset := parentStartOffset + uint32(childRef.ChildOffset)
					childIndexInInitialElements, found := offsetToInitialElementIndex[childAbsoluteFileOffset]

					if !found {
						log.Printf("Error buildFullElementTree: Original Elem %s (OrigIdx %d) ChildRef offset %d (abs %d) does not map to known initial element.",
							currentEl.SourceElementName, currentEl.OriginalIndex, childRef.ChildOffset, childAbsoluteFileOffset)
						continue
					}
					// childIndexInInitialElements is an index into r.elements (since original elements are at the start)
					childEl := &r.elements[childIndexInInitialElements]
					childEl.Parent = currentEl
					currentEl.Children = append(currentEl.Children, childEl)
				}
			}
		}
	}

	// Populate r.roots based on Parent == nil
	r.roots = nil
	for i := range r.elements {
		if r.elements[i].Parent == nil {
			// Check if this root element is already in r.roots to avoid duplicates
			// (unlikely with current logic but good for robustness)
			isAlreadyRoot := false
			for _, existingRoot := range r.roots {
				if existingRoot == &r.elements[i] {
					isAlreadyRoot = true
					break
				}
			}
			if !isAlreadyRoot {
				r.roots = append(r.roots, &r.elements[i])
			}
		}
	}

	if len(r.roots) == 0 && len(r.elements) > 0 {
		log.Println("Warn buildFullElementTree: No root elements found after linking, but elements exist. This might be okay if App expands or is only element.")
	}
	return nil
}


// GetCustomPropertyValue retrieves the string value of a custom property for a given RenderElement.
func GetCustomPropertyValue(el *render.RenderElement, keyName string, doc *krb.Document) (string, bool) {
	if doc == nil || el == nil {
		// log.Printf("Debug GetCustomPropertyValue: Called with nil doc or el for key '%s'", keyName)
		return "", false
	}

	// 1. Find the targetKeyIndex for 'keyName' in the document's string table.
	var targetKeyIndex uint8 = 0xFF // Use 0xFF as a sentinel for not found
	keyFoundInStrings := false
	for idx, str := range doc.Strings {
		if str == keyName {
			targetKeyIndex = uint8(idx)
			keyFoundInStrings = true
			break
		}
	}

	if !keyFoundInStrings {
		// log.Printf("Debug GetCustomPropertyValue: Key '%s' not found in document string table.", keyName)
		return "", false // Key name itself isn't defined in the KRB strings.
	}

	// 2. Access custom properties for the element.
	// Custom properties are stored in doc.CustomProperties, indexed by el.OriginalIndex.
	// This assumes el.OriginalIndex correctly maps to the element's entry in doc.CustomProperties
	// if 'el' is one of the initial elements from doc.Elements.
	// If 'el' was instantiated from a template, its custom properties are not directly
	// obtained this way unless the expansion process specifically populates something equivalent
	// or merges them. For now, this function is most effective for instance placeholders.

	if el.OriginalIndex < 0 || el.OriginalIndex >= len(doc.CustomProperties) {
		// log.Printf("Debug GetCustomPropertyValue: el.OriginalIndex %d out of bounds for doc.CustomProperties (len %d) for key '%s', element '%s'", el.OriginalIndex, len(doc.CustomProperties), keyName, el.SourceElementName)
		return "", false
	}

	elementCustomProps := doc.CustomProperties[el.OriginalIndex]
	if elementCustomProps == nil || len(elementCustomProps) == 0 {
		// log.Printf("Debug GetCustomPropertyValue: No custom properties found for element '%s' (OrigIdx %d) for key '%s'", el.SourceElementName, el.OriginalIndex, keyName)
		return "", false
	}

	// 3. Iterate through the element's custom properties to find the one matching targetKeyIndex.
	for _, prop := range elementCustomProps {
		if prop.KeyIndex == targetKeyIndex {
			// Found the custom property by key. Now check its value type and size.
			// We expect it to be a string index (ValTypeString or ValTypeResource)
			// and its size to be 1 (the byte containing the string index).
			if (prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource) && prop.Size == 1 {
				if len(prop.Value) == 1 {
					valueStringIndex := prop.Value[0]
					if int(valueStringIndex) < len(doc.Strings) {
						// Successfully found the key and a valid string index for its value.
						return doc.Strings[valueStringIndex], true
					}
					log.Printf("WARN GetCustomPropertyValue: Custom prop key '%s' (idx %d) found for element '%s' (OrigIdx %d), but its value string index %d is out of bounds for doc.Strings (len %d).",
						keyName, targetKeyIndex, el.SourceElementName, el.OriginalIndex, valueStringIndex, len(doc.Strings))
				} else {
					log.Printf("WARN GetCustomPropertyValue: Custom prop key '%s' (idx %d) found for element '%s' (OrigIdx %d), but its value data is empty despite Size=1.",
						keyName, targetKeyIndex, el.SourceElementName, el.OriginalIndex)
				}
			} else {
				log.Printf("WARN GetCustomPropertyValue: Custom prop key '%s' (idx %d) found for element '%s' (OrigIdx %d), but it has an unexpected ValueType %X or Size %d. Expected String/Resource index type.",
					keyName, targetKeyIndex, el.SourceElementName, el.OriginalIndex, prop.ValueType, prop.Size)
			}
			return "", false // Found the key, but its value wasn't a valid string index.
		}
	}

	// log.Printf("Debug GetCustomPropertyValue: Custom prop key '%s' (idx %d) not found among custom properties of element '%s' (OrigIdx %d).", keyName, targetKeyIndex, el.SourceElementName, el.OriginalIndex)
	return "", false // Key was not among the custom properties for this element.
}

// PerformLayout calculates the final screen position and dimensions for an element and its descendants.
func PerformLayout(
	el *render.RenderElement,
	parentContentX, parentContentY, parentContentW, parentContentH float32,
	scale float32,
	doc *krb.Document,
) {
	if el == nil {
		return
	}

	elementIdentifier := el.SourceElementName
	if elementIdentifier == "" {
		elementIdentifier = fmt.Sprintf("Elem %d (Type %X)", el.OriginalIndex, el.Header.Type)
		if doc != nil && el.Header.ID != 0 {
			if idName, idFound := getStringValueByIdx(doc, el.Header.ID); idFound { // Assuming getStringValueByIdx is available
				elementIdentifier += fmt.Sprintf(" ID:'%s'", idName)
			}
		}
	}
	// Temporarily disable detailed layout logging per element to reduce noise, can be re-enabled for debugging
	// log.Printf("--- Layout Start for Elem[%d] Name='%s' --- ParentCW=%.1f, ParentCH=%.1f",
	// 	el.OriginalIndex, elementIdentifier, parentContentW, parentContentH)

	isRootElement := (el.Parent == nil)
	scaledUint16Local := func(v uint16) float32 { return float32(v) * scale } // Local helper for clarity

	// --- Step 1: Determine EXPLICIT Size from KRB Header or Properties ---
	hasExplicitWidth := false
	desiredWidth := float32(0.0)
	if el.Header.Width > 0 {
		desiredWidth = scaledUint16Local(el.Header.Width)
		hasExplicitWidth = true
		// log.Printf("Layout Elem[%d]: Has explicit KRB Header.Width: %.1f (scaled from %d)", el.OriginalIndex, desiredWidth, el.Header.Width)
	}

	hasExplicitHeight := false
	desiredHeight := float32(0.0)
	if el.Header.Height > 0 {
		desiredHeight = scaledUint16Local(el.Header.Height)
		hasExplicitHeight = true
		// log.Printf("Layout Elem[%d]: Has explicit KRB Header.Height: %.1f (scaled from %d)", el.OriginalIndex, desiredHeight, el.Header.Height)
	}

	// KRB Properties (MaxWidth/MaxHeight from direct properties) can also set/override explicit size
	if doc != nil && el.OriginalIndex < len(doc.Properties) && len(doc.Properties[el.OriginalIndex]) > 0 {
		elementDirectProps := doc.Properties[el.OriginalIndex]
		// MaxWidth property
		propWVal, propWType, _, propWErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMaxWidth, doc) // Assuming getNumericValueForSizeProp is available
		if propWErr == nil {
			explicitPropWidth := MuxFloat32(propWType == krb.ValTypePercentage, (propWVal/256.0)*parentContentW, propWVal*scale)
			if !hasExplicitWidth || explicitPropWidth < desiredWidth {
				desiredWidth = explicitPropWidth
				hasExplicitWidth = true
				// log.Printf("Layout Elem[%d]: Width set/capped by KRB PropIDMaxWidth: %.1f", el.OriginalIndex, desiredWidth)
			}
		}
		// MaxHeight property
		propHVal, propHType, _, propHErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMaxHeight, doc) // Assuming getNumericValueForSizeProp is available
		if propHErr == nil {
			explicitPropHeight := MuxFloat32(propHType == krb.ValTypePercentage, (propHVal/256.0)*parentContentH, propHVal*scale)
			if !hasExplicitHeight || explicitPropHeight < desiredHeight {
				desiredHeight = explicitPropHeight
				hasExplicitHeight = true
				// log.Printf("Layout Elem[%d]: Height set/capped by KRB PropIDMaxHeight: %.1f", el.OriginalIndex, desiredHeight)
			}
		}
	}

	// --- Step 2: Apply DEFAULT SIZING if not explicit and not growing/absolute ---
	if !hasExplicitWidth && !el.Header.LayoutGrow() && !el.Header.LayoutAbsolute() {
		desiredWidth = parentContentW // Default to parent's available content width
		// log.Printf("Layout Elem[%d]: Width NOT explicit, defaulting to parentContentW: %.1f", el.OriginalIndex, desiredWidth)
	}

	if !hasExplicitHeight && !el.Header.LayoutGrow() && !el.Header.LayoutAbsolute() {
		desiredHeight = parentContentH // Default to parent's available content height
		// log.Printf("Layout Elem[%d]: Height NOT explicit, defaulting to parentContentH: %.1f", el.OriginalIndex, desiredHeight)
	}

	// Root elements usually take full parent space (screen dimensions)
	if isRootElement {
		desiredWidth = parentContentW
		desiredHeight = parentContentH
		// log.Printf("Layout Elem[%d]: Is ROOT, taking parent space: W=%.1f, H=%.1f", el.OriginalIndex, desiredWidth, desiredHeight)
	}

	el.RenderW = MaxF(0, desiredWidth)  // Use exported MaxF
	el.RenderH = MaxF(0, desiredHeight) // Use exported MaxF
	// log.Printf("Layout Elem[%d]: After explicit/default: RenderW=%.1f, RenderH=%.1f", el.OriginalIndex, el.RenderW, el.RenderH)

	// --- Step 3: Determine Render Position ---
	el.RenderX = parentContentX // Default to parent's content origin
	el.RenderY = parentContentY

	if el.Header.LayoutAbsolute() || (!isRootElement && (el.Header.PosX != 0 || el.Header.PosY != 0)) {
		offsetX := scaledUint16Local(el.Header.PosX)
		offsetY := scaledUint16Local(el.Header.PosY)
		if el.Parent != nil { // Position relative to parent's top-left corner for absolute
			el.RenderX = el.Parent.RenderX + offsetX
			el.RenderY = el.Parent.RenderY + offsetY
		} else { // If root and absolute (uncommon, but handle), relative to initial parentContentX/Y
			el.RenderX = parentContentX + offsetX
			el.RenderY = parentContentY + offsetY
		}
	}
	// Note: For flow layout, final X/Y is determined by PerformLayoutChildren

	// --- Step 4: Calculate Content Area for Children (used by PerformLayoutChildren) ---
	// These are logical units, scaled within PerformLayoutChildren or when drawing
	paddingTopLogical := el.Padding[0]
	paddingRightLogical := el.Padding[1]
	paddingBottomLogical := el.Padding[2]
	paddingLeftLogical := el.Padding[3]
	borderTopLogical := el.BorderWidths[0]
	borderRightLogical := el.BorderWidths[1]
	borderBottomLogical := el.BorderWidths[2]
	borderLeftLogical := el.BorderWidths[3]

	// Scale them for calculating child content area
	scaledPaddingTop := ScaledF32(paddingTopLogical, scale) // Use exported ScaledF32
	scaledPaddingRight := ScaledF32(paddingRightLogical, scale)
	scaledPaddingBottom := ScaledF32(paddingBottomLogical, scale)
	scaledPaddingLeft := ScaledF32(paddingLeftLogical, scale)
	scaledBorderTop := ScaledF32(borderTopLogical, scale)
	scaledBorderRight := ScaledF32(borderRightLogical, scale)
	scaledBorderBottom := ScaledF32(borderBottomLogical, scale)
	scaledBorderLeft := ScaledF32(borderLeftLogical, scale)

	childContentAreaX := el.RenderX + scaledBorderLeft + scaledPaddingLeft
	childContentAreaY := el.RenderY + scaledBorderTop + scaledPaddingTop
	childAvailableWidth := el.RenderW - scaledBorderLeft - scaledBorderRight - scaledPaddingLeft - scaledPaddingRight
	childAvailableHeight := el.RenderH - scaledBorderTop - scaledBorderBottom - scaledPaddingTop - scaledPaddingBottom

	childAvailableWidth = MaxF(0, childAvailableWidth)   // Use exported MaxF
	childAvailableHeight = MaxF(0, childAvailableHeight) // Use exported MaxF

	// log.Printf("Layout Elem[%d]: Child Content Area: X=%.1f,Y=%.1f W=%.1f,H=%.1f (based on el.RenderW=%.1f, el.RenderH=%.1f)",
	// 	el.OriginalIndex, childContentAreaX, childContentAreaY, childAvailableWidth, childAvailableHeight, el.RenderW, el.RenderH)

	// --- Step 5: Layout Children Recursively ---
	// Children are laid out within the available space calculated above.
	// el.Header.ChildCount refers to children from original KRB, el.Children is the actual runtime tree.
	if len(el.Children) > 0 { // Check actual children in RenderElement tree
		// log.Printf("Layout Elem[%d]: Has %d RenderElement children. Calling PerformLayoutChildren.", el.OriginalIndex, len(el.Children))
		PerformLayoutChildren(el, childContentAreaX, childContentAreaY, childAvailableWidth, childAvailableHeight, scale, doc) // Call exported PerformLayoutChildren

		// --- Step 6: Content Hugging (Primarily for Height if not 'grow' or explicit height) ---
		if !hasExplicitHeight && !el.Header.LayoutGrow() && !el.Header.LayoutAbsolute() {
			maxChildExtentY := float32(0.0) // Relative to childContentAreaY
			for _, child := range el.Children {
				if child != nil && !child.Header.LayoutAbsolute() { // Only consider flow children for hugging
					// child.RenderY is absolute. Convert to relative to childContentAreaY for extent calc.
					childRelativeY := child.RenderY - childContentAreaY
					currentChildExtent := childRelativeY + child.RenderH
					if currentChildExtent > maxChildExtentY {
						maxChildExtentY = currentChildExtent
					}
				}
			}
			// The height needed by content is maxChildExtentY.
			// Add back borders and padding to get the new total desired height for the parent.
			contentHeightFromChildren := maxChildExtentY + scaledBorderTop + scaledBorderBottom + scaledPaddingTop + scaledPaddingBottom
			if contentHeightFromChildren > el.RenderH {
				// log.Printf("Layout Elem[%d]: Children make it TALLER (%.1f from content extent %.1f) than current RenderH (%.1f). Expanding height.",
				// 	el.OriginalIndex, contentHeightFromChildren, maxChildExtentY, el.RenderH)
				el.RenderH = contentHeightFromChildren
			}
		}
		// A similar logic could be applied for width if desired (e.g. for horizontal content hugging)
	} else {
		// log.Printf("Layout Elem[%d]: No children in RenderElement tree to layout.", el.OriginalIndex)
	}
	// log.Printf("Layout Elem[%d]: After children layout/content hugging: RenderW=%.1f, RenderH=%.1f", el.OriginalIndex, el.RenderW, el.RenderH)

	// --- Step 7: Apply Min-Width/Height Constraints from direct properties ---
	if doc != nil && el.OriginalIndex < len(doc.Properties) && len(doc.Properties[el.OriginalIndex]) > 0 {
		elementDirectProps := doc.Properties[el.OriginalIndex]
		// MinWidth
		minWVal, minWType, _, minWErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMinWidth, doc)
		if minWErr == nil {
			minWidthConstraint := MuxFloat32(minWType == krb.ValTypePercentage, (minWVal/256.0)*parentContentW, minWVal*scale)
			if el.RenderW < minWidthConstraint {
				// log.Printf("Layout Elem[%d]: Applying MinWidth constraint. %.1f -> %.1f", el.OriginalIndex, el.RenderW, minWidthConstraint)
				el.RenderW = minWidthConstraint
			}
		}
		// MinHeight
		minHVal, minHType, _, minHErr := getNumericValueForSizeProp(elementDirectProps, krb.PropIDMinHeight, doc)
		if minHErr == nil {
			minHeightConstraint := MuxFloat32(minHType == krb.ValTypePercentage, (minHVal/256.0)*parentContentH, minHVal*scale)
			if el.RenderH < minHeightConstraint {
				// log.Printf("Layout Elem[%d]: Applying MinHeight constraint. %.1f -> %.1f", el.OriginalIndex, el.RenderH, minHeightConstraint)
				el.RenderH = minHeightConstraint
			}
		}
	}

	// --- Step 8: Final Fallback for Zero Height (Ensuring Visibility for Styled Empty Containers) ---
	el.RenderW = MaxF(0, el.RenderW) // Ensure width is not negative

	if el.RenderW > 0 && el.RenderH == 0 {
		if el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp || el.BgColor.A > 0 {
			defaultMinVisibleHeight := ScaledF32(uint8(baseFontSize), scale) // e.g., base font size as min height
			
			// Min height should also accommodate borders and padding
			minHeightFromPaddingBorder := scaledBorderTop + scaledBorderBottom + scaledPaddingTop + scaledPaddingBottom
			finalMinHeight := MaxF(defaultMinVisibleHeight, minHeightFromPaddingBorder)

			if finalMinHeight == 0 && el.BgColor.A > 0 { // If still zero but has BG, give it 1 scaled pixel
				finalMinHeight = 1.0 * scale
			}
			if finalMinHeight == 0 && (el.BorderWidths[0]+el.BorderWidths[1]+el.BorderWidths[2]+el.BorderWidths[3] > 0) { // has borders
				finalMinHeight = 1.0 * scale
			}


			if finalMinHeight > 0 {
				// log.Printf("Layout Elem[%d]: RenderH was 0 with W>0. Setting to fallback min visible height: %.1f", el.OriginalIndex, finalMinHeight)
				el.RenderH = finalMinHeight
			}
		}
	}
	el.RenderH = MaxF(0, el.RenderH) // Ensure height is not negative

	// log.Printf("Layout Elem[%d] Name='%s': FINAL RenderRect=(X:%.1f, Y:%.1f, W:%.1f x H:%.1f)",
	// 	el.OriginalIndex, elementIdentifier, el.RenderX, el.RenderY, el.RenderW, el.RenderH)
	// log.Printf("--- Layout End for Elem[%d] Name='%s' ---", el.OriginalIndex, elementIdentifier)
}


// PerformLayoutChildren arranges the children of a parent element.
func PerformLayoutChildren(
	parent *render.RenderElement,
	parentClientOriginX, parentClientOriginY, // Top-left of the content area available for flow children
	availableClientWidth, availableClientHeight float32, // Size of the content area for flow children
	scale float32,
	doc *krb.Document,
) {
	if parent == nil || len(parent.Children) == 0 {
		return
	}

	flowChildren := make([]*render.RenderElement, 0, len(parent.Children))
	absoluteChildren := make([]*render.RenderElement, 0)

	// Separate children into flow and absolutely positioned
	for _, child := range parent.Children {
		if child != nil {
			if child.Header.LayoutAbsolute() {
				absoluteChildren = append(absoluteChildren, child)
			} else {
				flowChildren = append(flowChildren, child)
			}
		}
	}

	// Layout Flow Children
	if len(flowChildren) > 0 {
		layoutDirection := parent.Header.LayoutDirection() // e.g., krb.LayoutDirRow, krb.LayoutDirColumn
		layoutAlignment := parent.Header.LayoutAlignment() // e.g., krb.LayoutAlignStart, krb.LayoutAlignCenter
		isLayoutReversed := (layoutDirection == krb.LayoutDirRowReverse || layoutDirection == krb.LayoutDirColumnReverse)
		isMainAxisHorizontal := (layoutDirection == krb.LayoutDirRow || layoutDirection == krb.LayoutDirRowReverse)

		// Gap property (from style or direct)
		gapValue := float32(0)
		if parentStyle, styleFound := findStyle(doc, parent.Header.StyleID); styleFound { // findStyle should be available
			if gapProp, propFound := getStylePropertyValue(parentStyle, krb.PropIDGap); propFound { // getStylePropertyValue should be available
				if gVal, valOk := getShortValue(gapProp); valOk { // getShortValue should be available
					gapValue = float32(gVal) * scale
				}
			}
		}
		// Direct property overrides style for gap
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

		mainAxisEffectiveSpaceForElements := MaxF(0, MuxFloat32(isMainAxisHorizontal, availableClientWidth, availableClientHeight)-totalGapSpace)
		crossAxisEffectiveSizeForElements := MuxFloat32(isMainAxisHorizontal, availableClientHeight, availableClientWidth)

		totalFixedSizeOnMainAxis := float32(0)
		numberOfGrowChildren := 0

		// First pass on flow children: calculate their desired sizes without 'grow'
		// PerformLayout is called for each child. It will use its explicit header W/H,
		// or properties like MaxWidth/MaxHeight, or default to the parent's available space (availableClientWidth/Height here).
		for _, child := range flowChildren {
			// Children are laid out relative to the parentClientOriginX/Y initially.
			// Their final positions will be adjusted later based on flow logic.
			// The parentContentW/H passed here is the available space for this child before considering other flow children.
			PerformLayout(child, parentClientOriginX, parentClientOriginY, availableClientWidth, availableClientHeight, scale, doc) // Uppercase P
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

		// Second pass on flow children: apply 'grow' and constrain cross-axis
		totalFinalElementSizeOnMainAxis := float32(0)
		for _, child := range flowChildren {
			childCrossAxisAvailableSize := crossAxisEffectiveSizeForElements
			if child.Header.LayoutGrow() && sizePerGrowChild > 0 {
				if isMainAxisHorizontal {
					child.RenderW = sizePerGrowChild
					// If child doesn't have explicit height, or if its height allows, make it fill cross axis.
					// This depends on how "stretch" on cross-axis is handled.
					// For now, if it grows on main, let its cross-axis be what PerformLayout decided,
					// unless that exceeds childCrossAxisAvailableSize.
					if child.RenderH == 0 || child.RenderH > childCrossAxisAvailableSize {
                         child.RenderH = childCrossAxisAvailableSize
                    }
				} else { // Main axis is vertical
					child.RenderH = sizePerGrowChild
					if child.RenderW == 0 || child.RenderW > childCrossAxisAvailableSize {
                        child.RenderW = childCrossAxisAvailableSize
                    }
				}
			} else { // Not growing or no space to grow
				// Constrain to cross-axis size if it's larger and cross-axis size is positive
				if isMainAxisHorizontal {
					if child.RenderH > childCrossAxisAvailableSize && childCrossAxisAvailableSize > 0 {
						child.RenderH = childCrossAxisAvailableSize
					}
				} else { // Main axis is vertical
					if child.RenderW > childCrossAxisAvailableSize && childCrossAxisAvailableSize > 0 {
						child.RenderW = childCrossAxisAvailableSize
					}
				}
			}
			child.RenderW = MaxF(0, child.RenderW) // Ensure non-negative
			child.RenderH = MaxF(0, child.RenderH)
			totalFinalElementSizeOnMainAxis += MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
		}

		// Calculate alignment offsets and spacing
		totalUsedSpaceWithGaps := totalFinalElementSizeOnMainAxis + totalGapSpace
		startOffsetOnMainAxis, effectiveSpacingBetweenItems := calculateAlignmentOffsetsF(layoutAlignment,
			MuxFloat32(isMainAxisHorizontal, availableClientWidth, availableClientHeight), totalUsedSpaceWithGaps,
			len(flowChildren), isLayoutReversed, gapValue) // calculateAlignmentOffsetsF should be available

		// Third pass: Position flow children
		currentMainAxisPosition := startOffsetOnMainAxis
		childOrderIndices := make([]int, len(flowChildren))
		for i := range childOrderIndices {
			childOrderIndices[i] = i
		}
		if isLayoutReversed {
			ReverseSliceInt(childOrderIndices) // ReverseSliceInt should be available
		}

		for i, orderedChildIndex := range childOrderIndices {
			child := flowChildren[orderedChildIndex]
			childMainAxisSizeValue := MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
			childCrossAxisSizeValue := MuxFloat32(isMainAxisHorizontal, child.RenderH, child.RenderW)

			// Calculate cross-axis offset
			// This uses the parent's layoutAlignment for the cross-axis.
			// A more complete system might have separate cross-axis alignment properties.
			crossAxisOffset := calculateCrossAxisOffsetF(layoutAlignment, crossAxisEffectiveSizeForElements, childCrossAxisSizeValue) // NEW - CORRECT

			if isMainAxisHorizontal {
				child.RenderX = parentClientOriginX + currentMainAxisPosition
				child.RenderY = parentClientOriginY + crossAxisOffset
			} else { // Main axis is vertical
				child.RenderX = parentClientOriginX + crossAxisOffset
				child.RenderY = parentClientOriginY + currentMainAxisPosition
			}

			currentMainAxisPosition += childMainAxisSizeValue
			if i < len(flowChildren)-1 { // Add spacing if not the last item
				currentMainAxisPosition += effectiveSpacingBetweenItems
			}
		}
	}

	// Layout Absolutely Positioned Children
	if len(absoluteChildren) > 0 {
		for _, child := range absoluteChildren {
			// Absolute children are laid out relative to the parent's origin (parent.RenderX/Y),
			// not the client area origin. Their percentage sizes (if any)
			// are calculated based on the parent's overall dimensions (parent.RenderW/H).
			PerformLayout(child, parent.RenderX, parent.RenderY, parent.RenderW, parent.RenderH, scale, doc) // Uppercase P
		}
	}
}


// --- Helper function stubs (ensure these are defined and exported if needed, or kept lowercase if internal) ---

// getNumericValueForSizeProp: if only used by PerformLayout, can be lowercase.
// Otherwise, make it GetNumericValueForSizeProp.
func getNumericValueForSizeProp(props []krb.Property, propID krb.PropertyID, doc *krb.Document) (value float32, valueType krb.ValueType, rawSizeBytes uint8, err error) {
	for _, p := range props {
		if p.ID == propID {
			// Assuming getNumericValueFromKrbProp is also defined, possibly lowercase if internal
			return getNumericValueFromKrbProp(&p, doc)
		}
	}
	return 0, krb.ValTypeNone, 0, fmt.Errorf("property ID 0x%X not found in list", propID)
}

// getNumericValueFromKrbProp: internal helper for the above, can be lowercase.
func getNumericValueFromKrbProp(prop *krb.Property, doc *krb.Document) (value float32, valueType krb.ValueType, rawSizeBytes uint8, err error) {
	if prop == nil {
		return 0, krb.ValTypeNone, 0, fmt.Errorf("getNumericValueFromKrbProp: received nil property")
	}
	if prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 {
		return float32(binary.LittleEndian.Uint16(prop.Value)), krb.ValTypeShort, 2, nil
	}
	if prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 {
		// Value is 8.8 fixed point, stored as uint16.
		// The raw uint16 value is what's returned here (e.g., 256 for 100%).
		// The caller (e.g., getNumericValueForSizeProp) then interprets it as (val/256.0)*parentMeasure.
		return float32(binary.LittleEndian.Uint16(prop.Value)), krb.ValTypePercentage, 2, nil
	}
	// Add other types if needed e.g. ValTypeByte for fixed pixel values
	// if prop.ValueType == krb.ValTypeByte && len(prop.Value) == 1 {
	// return float32(prop.Value[0]), krb.ValTypeByte, 1, nil
	// }
	return 0, prop.ValueType, prop.Size, fmt.Errorf("unsupported KRB ValueType (%d) or Size (%d) for numeric size conversion (PropID: %X)", prop.ValueType, prop.Size, prop.ID)
}


// GetRenderTree returns a slice of pointers to all processed RenderElements.
func (r *RaylibRenderer) GetRenderTree() []*render.RenderElement {
	if len(r.elements) == 0 {
		return nil
	}
	pointers := make([]*render.RenderElement, len(r.elements))
	for i := range r.elements {
		pointers[i] = &r.elements[i]
	}
	return pointers
}

// RenderFrame orchestrates the entire rendering process for a single frame.
func (r *RaylibRenderer) RenderFrame(roots []*render.RenderElement) {
	windowResized := rl.IsWindowResized()
	currentWidth := r.config.Width
	currentHeight := r.config.Height

	if windowResized && r.config.Resizable {
		newWidth := int(rl.GetScreenWidth())
		newHeight := int(rl.GetScreenHeight())
		if newWidth != currentWidth || newHeight != currentHeight {
			r.config.Width = newWidth
			r.config.Height = newHeight
			currentWidth = newWidth
			currentHeight = newHeight
			log.Printf("RenderFrame: Window resized to %dx%d. Recalculating layout.", currentWidth, currentHeight)
		}
	} else if !r.config.Resizable {
		screenWidth := int(rl.GetScreenWidth())
		screenHeight := int(rl.GetScreenHeight())
		if currentWidth != screenWidth || currentHeight != screenHeight {
			rl.SetWindowSize(currentWidth, currentHeight)
		}
	}

	for _, root := range roots { // roots should be correctly populated by PrepareTree
		if root != nil {
			PerformLayout(root, 0, 0, float32(currentWidth), float32(currentHeight), r.scaleFactor, r.docRef)
		}
	}

	r.ApplyCustomComponentLayoutAdjustments(r.GetRenderTree(), r.docRef)

	for _, root := range roots {
		if root != nil {
			r.renderElementRecursiveWithCustomDraw(root, r.scaleFactor)
		}
	}
}

// Cleanup unloads all loaded textures and closes the Raylib window.
func (r *RaylibRenderer) Cleanup() {
	log.Println("RaylibRenderer Cleanup: Unloading textures...")
	unloadedCount := 0
	for resourceIdx, texture := range r.loadedTextures {
		if texture.ID > 0 {
			rl.UnloadTexture(texture)
			unloadedCount++
		}
		delete(r.loadedTextures, resourceIdx)
	}
	log.Printf("RaylibRenderer Cleanup: Unloaded %d textures from cache.", unloadedCount)
	r.loadedTextures = make(map[uint8]rl.Texture2D) // Clear the map

	if rl.IsWindowReady() {
		log.Println("RaylibRenderer Cleanup: Closing Raylib window...")
		rl.CloseWindow()
	} else {
		log.Println("RaylibRenderer Cleanup: Raylib window was already closed or not initialized.")
	}
}

// ShouldClose returns true if the Raylib window has been signaled to close.
func (r *RaylibRenderer) ShouldClose() bool {
	return rl.IsWindowReady() && rl.WindowShouldClose()
}

// BeginFrame prepares Raylib for a new frame of drawing.
func (r *RaylibRenderer) BeginFrame() {
	rl.BeginDrawing()
	rl.ClearBackground(r.config.DefaultBg)
}

// EndFrame finalizes the drawing for the current frame.
func (r *RaylibRenderer) EndFrame() {
	rl.EndDrawing()
}

// PollEvents handles window events and user input.
func (r *RaylibRenderer) PollEvents() {
	if !rl.IsWindowReady() {
		return
	}

	mousePos := rl.GetMousePosition()
	currentMouseCursor := rl.MouseCursorDefault
	isMouseButtonClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	clickHandledThisFrame := false

	for i := len(r.elements) - 1; i >= 0; i-- { // Iterate all elements, including expanded
		el := &r.elements[i]

		if !el.IsVisible || !el.IsInteractive || el.RenderW <= 0 || el.RenderH <= 0 {
			continue
		}

		elementBounds := rl.NewRectangle(el.RenderX, el.RenderY, el.RenderW, el.RenderH)
		isMouseHovering := rl.CheckCollisionPointRec(mousePos, elementBounds)

		if isMouseHovering {
			currentMouseCursor = rl.MouseCursorPointingHand
		}

		if isMouseHovering && isMouseButtonClicked && !clickHandledThisFrame {
			eventWasProcessedByCustomHandler := false
			componentID, isCustomInstance := GetCustomPropertyValue(el, componentNameConventionKey, r.docRef)

			if isCustomInstance && componentID != "" {
				if customHandler, handlerExists := r.customHandlers[componentID]; handlerExists {
					if eventInterface, implementsEvent := customHandler.(interface {
						HandleEvent(el *render.RenderElement, eventType krb.EventType) (bool, error)
					}); implementsEvent {
						handled, err := eventInterface.HandleEvent(el, krb.EventTypeClick)
						if err != nil {
							log.Printf("ERROR PollEvents: Custom click handler for '%s' [%s] returned error: %v",
								componentID, el.SourceElementName, err)
						}
						if handled {
							eventWasProcessedByCustomHandler = true
							clickHandledThisFrame = true
						}
					}
				}
			}

			if !eventWasProcessedByCustomHandler && len(el.EventHandlers) > 0 {
				for _, eventInfo := range el.EventHandlers {
					if eventInfo.EventType == krb.EventTypeClick {
						goHandlerFunc, found := r.eventHandlerMap[eventInfo.HandlerName]
						if found {
							goHandlerFunc()
							clickHandledThisFrame = true
						} else {
							log.Printf("Warn PollEvents: Standard KRB click handler named '%s' (for %s) is not registered.",
								eventInfo.HandlerName, el.SourceElementName)
						}
						goto ElementEventProcessingDone
					}
				}
			}
		}
	ElementEventProcessingDone:
		if isMouseHovering {
			break // Top-most interactive element handles hover cursor
		}
	}
	rl.SetMouseCursor(currentMouseCursor)
}

// RegisterEventHandler registers a Go function for a KRB event name.
func (r *RaylibRenderer) RegisterEventHandler(name string, handler func()) {
	if name == "" {
		log.Println("WARN RegisterEventHandler: Attempted to register handler with empty name.")
		return
	}
	if handler == nil {
		log.Printf("WARN RegisterEventHandler: Attempted to register nil handler for name '%s'.", name)
		return
	}
	if _, exists := r.eventHandlerMap[name]; exists {
		log.Printf("INFO RegisterEventHandler: Overwriting existing handler for event name '%s'", name)
	}
	r.eventHandlerMap[name] = handler
	log.Printf("Registered event handler for '%s'", name)
}

// RegisterCustomComponent registers a Go CustomComponentHandler for a specific component identifier.
func (r *RaylibRenderer) RegisterCustomComponent(identifier string, handler render.CustomComponentHandler) error {
	if identifier == "" {
		return fmt.Errorf("RegisterCustomComponent: identifier cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("RegisterCustomComponent: handler cannot be nil for identifier '%s'", identifier)
	}
	if _, exists := r.customHandlers[identifier]; exists {
		log.Printf("INFO RegisterCustomComponent: Overwriting existing custom component handler for identifier '%s'", identifier)
	}
	r.customHandlers[identifier] = handler
	log.Printf("Registered custom component handler for '%s'", identifier)
	return nil
}

// LoadAllTextures explicitly triggers the loading of all textures required by the UI elements.
func (r *RaylibRenderer) LoadAllTextures() error {
	if r.docRef == nil {
		return fmt.Errorf("cannot load textures, KRB document reference is nil")
	}
	if !rl.IsWindowReady() {
		return fmt.Errorf("cannot load textures, Raylib window is not ready")
	}
	log.Println("LoadAllTextures: Starting...")
	errCount := 0
	r.performTextureLoading(r.docRef, &errCount) // Iterates current r.elements
	log.Printf("LoadAllTextures: Complete. Encountered %d errors.", errCount)
	if errCount > 0 {
		return fmt.Errorf("encountered %d errors during texture loading", errCount)
	}
	return nil
}

// logElementTree recursively logs the structure of the render element tree.
func logElementTree(el *render.RenderElement, depth int, prefix string) {
	if el == nil {
		return
	}
	indentBuffer := make([]byte, depth*2)
	for i := 0; i < depth*2; i++ {
		indentBuffer[i] = ' '
	}
	indent := string(indentBuffer)

	parentId := -1
	if el.Parent != nil {
		parentId = el.Parent.OriginalIndex // Parent's global index in r.elements
	}

	log.Printf("%s%s ElemDX_Global[%d] Name='%s' Type=0x%02X Children:%d ParentDX_Global:%d RenderRect=(%.1f,%.1f %.1fwX%.1fh) Visible:%t StyleID:%d LayoutByte:0x%02X",
		indent, prefix, el.OriginalIndex, el.SourceElementName, el.Header.Type, len(el.Children), parentId,
		el.RenderX, el.RenderY, el.RenderW, el.RenderH, el.IsVisible, el.Header.StyleID, el.Header.Layout)

	for i, child := range el.Children {
		logElementTree(child, depth+1, fmt.Sprintf("Child[%d]", i))
	}
}

// ApplyCustomComponentLayoutAdjustments iterates elements and calls their layout adjustment handlers.
func (r *RaylibRenderer) ApplyCustomComponentLayoutAdjustments(elements []*render.RenderElement, doc *krb.Document) {
	if doc == nil || len(r.customHandlers) == 0 {
		return
	}
	for _, el := range elements { // elements is from GetRenderTree(), which is all current elements
		if el == nil {
			continue
		}
		componentIdentifier, found := GetCustomPropertyValue(el, componentNameConventionKey, doc)
		if found && componentIdentifier != "" {
			handler, handlerFound := r.customHandlers[componentIdentifier]
			if handlerFound {
				err := handler.HandleLayoutAdjustment(el, doc)
				if err != nil {
					log.Printf("ERROR ApplyCustomComponentLayoutAdjustments: Custom layout handler for '%s' [%s] failed: %v",
						componentIdentifier, el.SourceElementName, err)
				}
			}
		}
	}
}

// renderElementRecursiveWithCustomDraw decides if custom drawing should occur or standard.
func (r *RaylibRenderer) renderElementRecursiveWithCustomDraw(el *render.RenderElement, scale float32) {
	if el == nil || !el.IsVisible {
		return
	}

	skipStandardDraw := false
	var drawErr error
	componentIdentifier := ""
	foundName := false

	if r.docRef != nil {
		componentIdentifier, foundName = GetCustomPropertyValue(el, componentNameConventionKey, r.docRef)
	}

	if foundName && componentIdentifier != "" {
		if handler, foundHandler := r.customHandlers[componentIdentifier]; foundHandler {
			if drawer, ok := handler.(interface {
				Draw(el *render.RenderElement, scale float32, rendererInstance render.Renderer) (bool, error)
			}); ok {
				skipStandardDraw, drawErr = drawer.Draw(el, scale, r)
				if drawErr != nil {
					log.Printf("ERROR renderElementRecursiveWithCustomDraw: Custom Draw handler for component '%s' [%s] failed: %v",
						componentIdentifier, el.SourceElementName, drawErr)
				}
			}
		}
	}

	if !skipStandardDraw {
		r.renderElementRecursive(el, scale) // This will also draw its children
	} else {
		// If standard draw is skipped, the custom Draw method is responsible for drawing children if needed.
		// However, if the custom Draw only handles the parent and wants standard child drawing,
		// we need to explicitly draw them. This logic depends on the contract of Draw.
		// For now, assume if skipStandardDraw is true, the custom handler has drawn everything necessary
		// for this element and its subtree. If it wants standard children, it should call this function for its children.
		// A simpler model: if skipStandardDraw, then we manually recurse for children here.
		// This means the custom Draw is only for the element itself, not its children's standard rendering.
		for _, child := range el.Children {
			r.renderElementRecursiveWithCustomDraw(child, scale)
		}
	}
}

// renderElementRecursive draws an element and its children.
func (r *RaylibRenderer) renderElementRecursive(el *render.RenderElement, scale float32) {
	if el == nil || !el.IsVisible {
		return
	}

	renderXf, renderYf, renderWf, renderHf := el.RenderX, el.RenderY, el.RenderW, el.RenderH

	// If element has no size, still process children as they might be absolutely positioned.
	if renderWf <= 0 || renderHf <= 0 {
		for _, child := range el.Children {
			r.renderElementRecursive(child, scale) // Recurse for children
		}
		return
	}

	renderX, renderY := int32(renderXf), int32(renderYf)
	renderW, renderH := int32(renderWf), int32(renderHf)

	effectiveBgColor := el.BgColor
	effectiveFgColor := el.FgColor
	borderColor := el.BorderColor

	// Apply active/inactive styles if applicable
	if (el.Header.Type == krb.ElemTypeButton) && (el.ActiveStyleNameIndex != 0 || el.InactiveStyleNameIndex != 0) {
		targetStyleNameIndex := el.InactiveStyleNameIndex
		if el.IsActive {
			targetStyleNameIndex = el.ActiveStyleNameIndex
		}
		if r.docRef != nil && targetStyleNameIndex != 0 {
			targetStyleID := findStyleIDByNameIndex(r.docRef, targetStyleNameIndex)
			if targetStyleID != 0 {
				bg, fg, styleColorOk := getStyleColors(r.docRef, targetStyleID, r.docRef.Header.Flags)
				if styleColorOk {
					effectiveBgColor = bg
					effectiveFgColor = fg
				}
			}
		}
	}

	// Draw Background
	if el.Header.Type != krb.ElemTypeText && effectiveBgColor.A > 0 {
		rl.DrawRectangle(renderX, renderY, renderW, renderH, effectiveBgColor)
	}

	// Draw Borders
	topBorder := scaledI32(el.BorderWidths[0], scale)
	rightBorder := scaledI32(el.BorderWidths[1], scale)
	bottomBorder := scaledI32(el.BorderWidths[2], scale)
	leftBorder := scaledI32(el.BorderWidths[3], scale)

	clampedTop, clampedBottom := clampOpposingBorders(int(topBorder), int(bottomBorder), int(renderH))
	clampedLeft, clampedRight := clampOpposingBorders(int(leftBorder), int(rightBorder), int(renderW))
	drawBorders(int(renderX), int(renderY), int(renderW), int(renderH),
		clampedTop, clampedRight, clampedBottom, clampedLeft, borderColor)

	// Calculate Content Area
	paddingTop := scaledI32(el.Padding[0], scale)
	paddingRight := scaledI32(el.Padding[1], scale)
	paddingBottom := scaledI32(el.Padding[2], scale)
	paddingLeft := scaledI32(el.Padding[3], scale)

	contentX_f32 := renderXf + float32(clampedLeft) + float32(paddingLeft)
	contentY_f32 := renderYf + float32(clampedTop) + float32(paddingTop)
	contentWidth_f32 := renderWf - float32(clampedLeft) - float32(clampedRight) - float32(paddingLeft) - float32(paddingRight)
	contentHeight_f32 := renderHf - float32(clampedTop) - float32(clampedBottom) - float32(paddingTop) - float32(paddingBottom)

	contentX := int32(contentX_f32)
	contentY := int32(contentY_f32)
	contentWidth := maxI32(0, int32(contentWidth_f32))
	contentHeight := maxI32(0, int32(contentHeight_f32))

	// Draw Element Content (Text or Image)
	if contentWidth > 0 && contentHeight > 0 {
		rl.BeginScissorMode(contentX, contentY, contentWidth, contentHeight)
		r.drawContent(el, int(contentX), int(contentY), int(contentWidth), int(contentHeight), scale, effectiveFgColor)
		rl.EndScissorMode()
	}

	// Recursively Draw Children
	for _, child := range el.Children {
		r.renderElementRecursive(child, scale) // Standard recursion for children
	}
}

// performTextureLoading loads textures for elements that require them.
func (r *RaylibRenderer) performTextureLoading(doc *krb.Document, errorCounter *int) {
	if doc == nil {
		*errorCounter++
		return
	}
	if r.elements == nil { // Check if r.elements slice itself is nil
		*errorCounter++
		return
	}
	for i := range r.elements { // Iterate over the potentially expanded slice
		el := &r.elements[i]    // Get pointer to the element
		needsTexture := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) &&
			el.ResourceIndex != render.InvalidResourceIndex
		if !needsTexture {
			continue
		}
		resIndex := el.ResourceIndex
		if int(resIndex) >= len(doc.Resources) {
			log.Printf("Error performTextureLoading: Elem %s (GlobalIdx %d) ResourceIndex %d out of bounds for doc.Resources (len %d)",
				el.SourceElementName, el.OriginalIndex, resIndex, len(doc.Resources))
			*errorCounter++
			el.TextureLoaded = false
			continue
		}
		res := doc.Resources[resIndex]
		if loadedTex, exists := r.loadedTextures[resIndex]; exists {
			el.Texture = loadedTex
			el.TextureLoaded = (loadedTex.ID > 0)
			if !el.TextureLoaded {
				*errorCounter++ // Texture was in cache but invalid
			}
			continue
		}
		var texture rl.Texture2D
		loadedOk := false
		if res.Format == krb.ResFormatExternal {
			if resourceName, ok := getStringValueByIdx(doc, res.NameIndex); ok {
				fullPath := filepath.Join(r.krbFileDir, resourceName)
				if _, statErr := os.Stat(fullPath); !os.IsNotExist(statErr) {
					img := rl.LoadImage(fullPath)
					if img.Data != nil && img.Width > 0 && img.Height > 0 {
						if rl.IsWindowReady() {
							texture = rl.LoadTextureFromImage(img)
							if texture.ID > 0 {
								loadedOk = true
							} else {
								log.Printf("Error performTextureLoading: Failed to load texture from image for %s", fullPath)
								*errorCounter++
							}
						} else {
							log.Printf("Error performTextureLoading: Window not ready for texture loading for %s", fullPath)
							*errorCounter++
						}
						rl.UnloadImage(img)
					} else {
						log.Printf("Error performTextureLoading: Failed to load image data for external resource: %s", fullPath)
						*errorCounter++
					}
				} else {
					log.Printf("Error performTextureLoading: External resource file not found: %s", fullPath)
					*errorCounter++
				}
			} else {
				log.Printf("Error performTextureLoading: Could not get resource name for external resource index: %d", res.NameIndex)
				*errorCounter++
			}
		} else if res.Format == krb.ResFormatInline {
			if res.InlineData != nil && res.InlineDataSize > 0 {
				ext := ".png" // Default, should ideally be hinted by KRB or config
				img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData))) // Use actual length
				if img.Data != nil && img.Width > 0 && img.Height > 0 {
					if rl.IsWindowReady() {
						texture = rl.LoadTextureFromImage(img)
						if texture.ID > 0 {
							loadedOk = true
						} else {
							log.Printf("Error performTextureLoading: Failed to load texture from inline image data (name index %d)", res.NameIndex)
							*errorCounter++
						}
					} else {
						log.Printf("Error performTextureLoading: Window not ready for texture loading for inline image (name index %d)", res.NameIndex)
						*errorCounter++
					}
					rl.UnloadImage(img)
				} else {
					log.Printf("Error performTextureLoading: Failed to load image data for inline resource (name index: %d)", res.NameIndex)
					*errorCounter++
				}
			} else {
				log.Printf("Error performTextureLoading: Inline resource data is nil or size 0 (name index: %d)", res.NameIndex)
				*errorCounter++
			}
		} else {
			log.Printf("Error performTextureLoading: Unknown resource format for resource (name index: %d)", res.NameIndex)
			*errorCounter++
		}

		if loadedOk {
			el.Texture = texture
			el.TextureLoaded = true
			r.loadedTextures[resIndex] = texture
		} else {
			el.TextureLoaded = false
			// Do not add invalid texture to loadedTextures map
		}
	}
}

// drawContent draws the specific content (text, image) of an element.
func (r *RaylibRenderer) drawContent(el *render.RenderElement, cx, cy, cw, ch int, scale float32, effectiveFgColor rl.Color) {
	// Draw Text
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		fontSize := int32(math.Max(1.0, math.Round(baseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		textHeightMeasured := fontSize // For single line text

		textDrawX := int32(cx)
		textDrawY := int32(cy + (ch-int(textHeightMeasured))/2) // Center vertically by default

		switch el.TextAlignment {
		case krb.LayoutAlignCenter: // Horizontal center
			textDrawX = int32(cx + (cw-int(textWidthMeasured))/2)
		case krb.LayoutAlignEnd: // Horizontal end (right)
			textDrawX = int32(cx + cw - int(textWidthMeasured))
			// case krb.LayoutAlignStart: // Default, textDrawX is already cx
		}
		rl.DrawText(el.Text, textDrawX, textDrawY, fontSize, effectiveFgColor)
	}

	// Draw Image
	isImageElement := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton)
	if isImageElement && el.TextureLoaded && el.Texture.ID > 0 {
		texWidth := float32(el.Texture.Width)
		texHeight := float32(el.Texture.Height)

		sourceRec := rl.NewRectangle(0, 0, texWidth, texHeight)
		// Destination rectangle is the content area passed in (cx, cy, cw, ch)
		destRec := rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch))

		// Ensure valid dimensions for drawing
		if destRec.Width > 0 && destRec.Height > 0 && sourceRec.Width > 0 && sourceRec.Height > 0 {
			// DrawTexturePro allows scaling and fitting the texture into destRec
			rl.DrawTexturePro(el.Texture, sourceRec, destRec, rl.NewVector2(0, 0), 0.0, rl.White) // Tint white (no change)
		}
	}
}

// --- Property and Style Parsing Helpers ---
func applyStylePropertiesToWindowDefaults(props []krb.Property, doc *krb.Document, defaultBg *rl.Color) {
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

func applyStylePropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
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
			if bw, ok := getByteValue(&prop); ok { // Single value for all borders
				el.BorderWidths = [4]uint8{bw, bw, bw, bw}
			} else if edges, okEdges := getEdgeInsetsValue(&prop); okEdges { // Individual values
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
			// Note: TextContent and ImageSource are typically not set by styles directly,
			// but by direct properties or content fields in KRB (or resolved for components).
		}
	}
}

func applyDirectVisualPropertiesToAppElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor: // App can have a root visual background
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}
		case krb.PropIDVisibility: // App's root element can be hidden
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
		}
	}
}

func applyDirectPropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
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
			// Window configuration properties are skipped here as they are for App config
		case krb.PropIDWindowWidth, krb.PropIDWindowHeight, krb.PropIDWindowTitle, krb.PropIDResizable, krb.PropIDScaleFactor:
			continue
		}
	}
}

func applyDirectPropertiesToConfig(props []krb.Property, doc *krb.Document, config *render.WindowConfig) {
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
			if strIdx := getFirstByteValue(&prop); strIdx != 0 || (len(doc.Strings) > 0 && doc.Strings[0] != "") { // Allow index 0 if valid string
				if s, ok := getStringValueByIdx(doc, strIdx); ok {
					config.Title = s
				}
			}
		case krb.PropIDResizable:
			if rVal, ok := getByteValue(&prop); ok {
				config.Resizable = (rVal != 0)
			}
		case krb.PropIDScaleFactor:
			if sfRaw, ok := getShortValue(&prop); ok && sfRaw > 0 { // Ensure scale is positive
				config.ScaleFactor = float32(sfRaw) / 256.0
			}
		case krb.PropIDBgColor: // This is App's default background
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultBg = c
			}
		}
	}
}

// calculateAlignmentOffsetsF calculates the starting offset and the effective spacing
// between items for flow layouts based on the alignment mode.
// - alignment: Main axis alignment (e.g., krb.LayoutAlignStart, krb.LayoutAlignCenter).
// - availableSpaceOnMainAxis: Total space available for elements and gaps.
// - totalUsedSpaceByChildrenAndGaps: Sum of all children's sizes on the main axis plus all fixed gaps.
// - numberOfChildren: Number of flow children being laid out.
// - isLayoutReversed: True if the layout direction is reversed (e.g., RowReverse).
// - fixedGapBetweenChildren: The base gap value between items.

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
		if isLayoutReversed {
			startOffset = unusedSpace 
		} else {
			startOffset = 0 
		}
	case krb.LayoutAlignCenter: 
		startOffset = unusedSpace / 2.0
	case krb.LayoutAlignEnd: 
		if isLayoutReversed {
			startOffset = 0 
		} else {
			startOffset = unusedSpace 
		}
	case krb.LayoutAlignSpaceBetween:
		if numberOfChildren > 1 {
			spacingToApplyBetweenChildren += unusedSpace / float32(numberOfChildren-1)
		} else {
			startOffset = unusedSpace / 2.0
		}
	// REMOVED krb.LayoutAlignSpaceAround and krb.LayoutAlignSpaceEvenly cases
	default: 
		// Log if an unexpected alignment value is encountered, but still provide a default behavior.
		if alignment != krb.LayoutAlignStart && alignment != krb.LayoutAlignCenter && alignment != krb.LayoutAlignEnd && alignment != krb.LayoutAlignSpaceBetween {
			log.Printf("Warn calculateAlignmentOffsetsF: Unknown or non-standard alignment value %d. Defaulting to LayoutAlignStart behavior.", alignment)
		}
		// Default to LayoutAlignStart behavior
		if isLayoutReversed {
			startOffset = unusedSpace
		} else {
			startOffset = 0
		}
	}
	return startOffset, spacingToApplyBetweenChildren
}

// calculateCrossAxisOffsetF calculates the offset for a child on the cross-axis
// based on the parent's cross-axis alignment and the child's size.
// - alignment: Cross-axis alignment (e.g., krb.LayoutAlignStart, krb.LayoutAlignCenter).
//              This uses the same enum as main-axis alignment for simplicity here.
// - parentCrossAxisSize: The total available size on the cross axis in the parent.
// - childCrossAxisSize: The size of the child element on the cross axis.
func calculateCrossAxisOffsetF(
	alignment uint8, 
	parentCrossAxisSize float32,
	childCrossAxisSize float32,
) float32 {
	offset := float32(0.0)
	availableSpace := parentCrossAxisSize - childCrossAxisSize

	switch alignment {
	case krb.LayoutAlignStart: // Align to the start of the cross axis (top/left)
		offset = 0.0
	case krb.LayoutAlignCenter: // Center on the cross axis
		if availableSpace > 0 {
			offset = availableSpace / 2.0
		}
	case krb.LayoutAlignEnd: // Align to the end of the cross axis (bottom/right)
		if availableSpace > 0 {
			offset = availableSpace
		}
	// krb.LayoutAlignSpaceBetween and similar are typically for main axis distribution,
	// for cross-axis, 'stretch' is a common behavior if child has no fixed size.
	// This function assumes child has a determined cross-axis size.
	// 'Stretch' would typically be handled by PerformLayout setting child's cross-axis size to parentCrossAxisSize.
	default: // Default to start alignment on cross-axis
		offset = 0.0
	}
	return MaxF(0, offset) // Ensure offset is not negative
}


func resolveElementText(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeText && el.Header.Type != krb.ElemTypeButton {
		return
	}
	// Text might already be set by direct properties (from KRB or template)
	if el.Text != "" {
		return
	}

	resolvedText := ""
	foundTextProp := false

	// Check direct properties from the original KRB document if this element is from there
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

	// If not found in direct properties, check style (if applicable)
	if !foundTextProp && styleOk && style != nil {
		if styleProp, propInStyleOk := getStylePropertyValue(style, krb.PropIDTextContent); propInStyleOk {
			if strIdx, ok := getByteValue(styleProp); ok {
				if s, textOk := getStringValueByIdx(doc, strIdx); textOk {
					resolvedText = s
					// No foundTextProp = true here, direct props take precedence if they set el.Text
				}
			}
		}
	}
	el.Text = resolvedText
}

func resolveElementImageSource(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeImage && el.Header.Type != krb.ElemTypeButton {
		return
	}
	// ResourceIndex might already be set by direct properties
	if el.ResourceIndex != render.InvalidResourceIndex {
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
	// This function is primarily for elements originating from doc.Elements.
	// Template elements have their events resolved during expandComponent.
	el.EventHandlers = nil // Clear existing, if any

	// Check if el.OriginalIndex is valid for doc.Events (i.e., it's an original element)
	if doc != nil && doc.Events != nil &&
		el.OriginalIndex < len(doc.Events) && // Ensure OriginalIndex is within bounds of doc.Events
		doc.Events[el.OriginalIndex] != nil {

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
					log.Printf("Warn resolveEventHandlers: Elem %s (OrigIdx %d) has invalid event callback string index %d.",
						el.SourceElementName, el.OriginalIndex, krbEvent.CallbackID)
				}
			}
		}
	}
}


func findStyle(doc *krb.Document, styleID uint8) (*krb.Style, bool) {
	if doc == nil || styleID == 0 || int(styleID) > len(doc.Styles) { // StyleID is 1-based
		return nil, false
	}
	return &doc.Styles[styleID-1], true // Access 0-based slice
}

func getStylePropertyValue(style *krb.Style, propID krb.PropertyID) (*krb.Property, bool) {
	if style == nil {
		return nil, false
	}
	for i := range style.Properties { // Iterate by index to get pointer
		if style.Properties[i].ID == propID {
			return &style.Properties[i], true
		}
	}
	return nil, false
}

func findStyleIDByNameIndex(doc *krb.Document, nameIndex uint8) uint8 {
	if doc == nil {
		return 0 // Invalid style ID
	}
	// Allow index 0 if it's a valid non-empty string, though style names are usually non-empty
	if nameIndex == 0 {
		if len(doc.Strings) == 0 || doc.Strings[0] == "" {
			return 0 // Index 0 is not a valid style name if empty or table empty
		}
	}
	for i := range doc.Styles {
		if doc.Styles[i].NameIndex == nameIndex {
			return doc.Styles[i].ID // Return 1-based Style ID
		}
	}
	return 0 // Not found
}

func getStyleColors(doc *krb.Document, styleID uint8, flags uint16) (bg rl.Color, fg rl.Color, ok bool) {
	if doc == nil || styleID == 0 { // StyleID is 1-based
		return rl.Blank, rl.White, false // Default colors, not OK
	}
	styleIndex := int(styleID - 1) // Convert to 0-based
	if styleIndex < 0 || styleIndex >= len(doc.Styles) {
		return rl.Blank, rl.White, false
	}
	style := &doc.Styles[styleIndex]
	bg, fg = rl.Blank, rl.White // Initialize with defaults
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
		if foundBg && foundFg { // Optimization: stop if both found
			break
		}
	}
	// Return true if style was valid, even if colors weren't specifically in it (defaults used)
	return bg, fg, true
}

func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {
	if prop == nil || prop.ValueType != krb.ValTypeColor {
		return rl.Color{}, false
	}
	useExtended := (flags & krb.FlagExtendedColor) != 0
	if useExtended {
		if len(prop.Value) == 4 { // RGBA
			return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true
		}
	} else {
		if len(prop.Value) == 1 { // Palette index
			log.Printf("Warn getColorValue: Palette color (index %d) requested, but palette system not implemented. Returning Magenta.", prop.Value[0])
			return rl.Magenta, true // Placeholder for palette
		}
	}
	log.Printf("Warn getColorValue: Invalid color data for PropID %X, ValueType %X, Size %d, ExtendedFlag %t", prop.ID, prop.ValueType, prop.Size, useExtended)
	return rl.Color{}, false
}

func getByteValue(prop *krb.Property) (uint8, bool) {
	if prop != nil &&
		(prop.ValueType == krb.ValTypeByte ||
			prop.ValueType == krb.ValTypeString || // If used as index
			prop.ValueType == krb.ValTypeResource || // If used as index
			prop.ValueType == krb.ValTypeEnum) && // Enum value
		len(prop.Value) == 1 {
		return prop.Value[0], true
	}
	return 0, false
}

func getFirstByteValue(prop *krb.Property) uint8 {
	// Used for properties where the value is a single byte index (e.g., string index for title)
	if prop != nil && len(prop.Value) > 0 {
		return prop.Value[0]
	}
	return 0 // Or an invalid index marker if appropriate
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

func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 {
		return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true // T, R, B, L
	}
	return [4]uint8{}, false
}

func clampOpposingBorders(borderA, borderB, totalSize int) (int, int) {
	if totalSize <= 0 {
		return 0, 0
	}
	if borderA < 0 {
		borderA = 0
	}
	if borderB < 0 {
		borderB = 0
	}
	if borderA+borderB > totalSize { // If sum of borders exceeds total available size
		// Proportional scaling (or simpler: cap each at half)
		// For now, simple cap:
		borderA = totalSize / 2
		borderB = totalSize - borderA // Ensures sum is totalSize, handles odd totalSize
	}
	return borderA, borderB
}

func drawBorders(x, y, w, h, top, right, bottom, left int, color rl.Color) {
	if color.A == 0 { // Don't draw fully transparent borders
		return
	}
	// Top border
	if top > 0 {
		rl.DrawRectangle(int32(x), int32(y), int32(w), int32(top), color)
	}
	// Bottom border
	if bottom > 0 {
		rl.DrawRectangle(int32(x), int32(y+h-bottom), int32(w), int32(bottom), color)
	}
	// Calculate remaining height for side borders
	sideY := y + top
	sideH := h - top - bottom
	if sideH > 0 { // Only draw side borders if there's height for them
		// Left border
		if left > 0 {
			rl.DrawRectangle(int32(x), int32(sideY), int32(left), int32(sideH), color)
		}
		// Right border
		if right > 0 {
			rl.DrawRectangle(int32(x+w-right), int32(sideY), int32(right), int32(sideH), color)
		}
	}
}

func ReverseSliceInt(s []int) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// --- Math and Scaling Helpers ---
func ScaledF32(value uint8, scale float32) float32 {
	return float32(value) * scale
}

func scaledI32(value uint8, scale float32) int32 {
	// Round to nearest integer for pixel values
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

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}