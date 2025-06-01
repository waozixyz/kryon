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
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

const baseFontSize = 18.0
const componentNameConventionKey = "_componentName"
const childrenSlotIDName = "children_host" // Convention for KRY-usage children slot

type RaylibRenderer struct {
	config          render.WindowConfig
	elements        []render.RenderElement
	roots           []*render.RenderElement
	loadedTextures  map[uint8]rl.Texture2D
	krbFileDir      string
	scaleFactor     float32
	docRef          *krb.Document
	eventHandlerMap map[string]func()
	customHandlers  map[string]render.CustomComponentHandler
}

func NewRaylibRenderer() *RaylibRenderer {
	return &RaylibRenderer{
		loadedTextures:  make(map[uint8]rl.Texture2D),
		scaleFactor:     1.0,
		eventHandlerMap: make(map[string]func()),
		customHandlers:  make(map[string]render.CustomComponentHandler),
	}
}

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
		rl.SetWindowSize(config.Width, config.Height)
	}

	rl.SetTargetFPS(60)

	if !rl.IsWindowReady() {
		return fmt.Errorf("RaylibRenderer Init: rl.InitWindow failed or window is not ready")
	}
	log.Println("RaylibRenderer Init: Raylib window is ready.")
	return nil
}

func (r *RaylibRenderer) PrepareTree(doc *krb.Document, krbFilePath string) ([]*render.RenderElement, render.WindowConfig, error) {
	if doc == nil {
		log.Println("PrepareTree: KRB document is nil.")
		return nil, r.config, fmt.Errorf("PrepareTree: KRB document is nil")
	}
	r.docRef = doc

	var err error
	r.krbFileDir, err = filepath.Abs(filepath.Dir(krbFilePath))
	if err != nil {
		r.krbFileDir = filepath.Dir(krbFilePath)
		log.Printf("WARN PrepareTree: Failed to get absolute path for KRB file dir '%s': %v. Using relative base: %s", krbFilePath, err, r.krbFileDir)
	}
	log.Printf("PrepareTree: Resource Base Directory set to: %s", r.krbFileDir)

	windowConfig := render.DefaultWindowConfig()
	windowConfig.DefaultBg = rl.Black

	defaultForegroundColor := rl.RayWhite
	defaultBorderColor := rl.Gray
	defaultBorderWidth := uint8(0)
	defaultTextAlignment := uint8(krb.LayoutAlignStart)
	defaultIsVisible := true

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
		r.scaleFactor = float32(math.Max(1.0, float64(windowConfig.ScaleFactor)))
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

	r.elements = make([]render.RenderElement, initialElementCount, initialElementCount*2)

	for i := 0; i < initialElementCount; i++ {
		renderEl := &r.elements[i]
		krbElHeader := doc.Elements[i]

		renderEl.Header = krbElHeader
		renderEl.OriginalIndex = i
		renderEl.DocRef = doc

		renderEl.BgColor = rl.Blank
		renderEl.FgColor = defaultForegroundColor
		renderEl.BorderColor = defaultBorderColor
		renderEl.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth}
		renderEl.Padding = [4]uint8{0, 0, 0, 0}
		renderEl.TextAlignment = defaultTextAlignment
		renderEl.IsVisible = defaultIsVisible
		renderEl.IsInteractive = (krbElHeader.Type == krb.ElemTypeButton || krbElHeader.Type == krb.ElemTypeInput)
		renderEl.ResourceIndex = render.InvalidResourceIndex

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

		elementStyle, styleFound := findStyle(doc, krbElHeader.StyleID)
		if styleFound {
			applyStylePropertiesToElement(elementStyle.Properties, doc, renderEl)
		} else if krbElHeader.StyleID != 0 && !(i == 0 && isAppElementPresent) {
			log.Printf("Warn PrepareTree: Element %d (Name: '%s', Type: %X) has StyleID %d, but style was not found.",
				i, renderEl.SourceElementName, krbElHeader.Type, krbElHeader.StyleID)
		}

		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			if i == 0 && isAppElementPresent {
				applyDirectVisualPropertiesToAppElement(doc.Properties[0], doc, renderEl)
			} else {
				applyDirectPropertiesToElement(doc.Properties[i], doc, renderEl)
			}
		}
		resolveElementText(doc, renderEl, elementStyle, styleFound)
		resolveElementImageSource(doc, renderEl, elementStyle, styleFound)
		resolveEventHandlers(doc, renderEl)
	}

	kryUsageChildrenMap := make(map[int][]*render.RenderElement)
	if err := r.linkOriginalKrbChildren(initialElementCount, kryUsageChildrenMap); err != nil {
		return nil, r.config, fmt.Errorf("PrepareTree: failed during initial child linking: %w", err)
	}

	nextMasterIndex := initialElementCount
	for i := 0; i < initialElementCount; i++ {
		instanceElement := &r.elements[i]
		componentName, _ := GetCustomPropertyValue(instanceElement, componentNameConventionKey, doc)

		if componentName != "" {
			compDef := r.findComponentDefinition(doc, componentName)
			if compDef != nil {
				log.Printf("PrepareTree: Expanding component '%s' for instance '%s' (OriginalIndex: %d)", componentName, instanceElement.SourceElementName, instanceElement.OriginalIndex)
				instanceKryChildren := kryUsageChildrenMap[instanceElement.OriginalIndex]
				err := r.expandComponent(instanceElement, compDef, doc, &r.elements, &nextMasterIndex, instanceKryChildren)
				if err != nil {
					log.Printf("ERROR PrepareTree: Failed to expand component '%s' for instance '%s': %v", componentName, instanceElement.SourceElementName, err)
				}
			} else {
				log.Printf("Warn PrepareTree: Component definition for '%s' (instance '%s') not found.", componentName, instanceElement.SourceElementName)
			}
		}
	}

	log.Println("PrepareTree: Finalizing element tree structure (setting Parent pointers and finding roots)...")
	r.roots = nil
	errBuild := r.finalizeTreeStructureAndRoots()
	if errBuild != nil {
		log.Printf("Error PrepareTree: Failed to finalize full element tree: %v", errBuild)
		return nil, r.config, fmt.Errorf("failed to finalize full element tree: %w", errBuild)
	}

	log.Printf("PrepareTree: Tree built successfully. Number of root nodes: %d. Total elements (including expanded): %d.",
		len(r.roots), len(r.elements))
	for rootIdx, rootNode := range r.roots {
		logElementTree(rootNode, 0, fmt.Sprintf("Root[%d]", rootIdx))
	}

	return r.roots, r.config, nil
}

func (r *RaylibRenderer) linkOriginalKrbChildren(initialElementCount int, kryUsageChildrenMap map[int][]*render.RenderElement) error {
	if r.docRef == nil || r.docRef.ElementStartOffsets == nil {
		return fmt.Errorf("linkOriginalKrbChildren: docRef or ElementStartOffsets is nil")
	}

	offsetToInitialElementIndex := make(map[uint32]int)
	for i := 0; i < initialElementCount && i < len(r.docRef.ElementStartOffsets); i++ {
		offsetToInitialElementIndex[r.docRef.ElementStartOffsets[i]] = i
	}

	for i := 0; i < initialElementCount; i++ {
		currentEl := &r.elements[i]
		originalKrbHeader := &r.docRef.Elements[i]
		componentName, _ := GetCustomPropertyValue(currentEl, componentNameConventionKey, r.docRef)
		isPlaceholder := (componentName != "")

		if originalKrbHeader.ChildCount > 0 {
			if i >= len(r.docRef.ChildRefs) || r.docRef.ChildRefs[i] == nil {
				log.Printf("Warn linkOriginalKrbChildren: Elem %s (OrigIdx %d) has KRB ChildCount %d but no ChildRefs in doc.",
					currentEl.SourceElementName, i, originalKrbHeader.ChildCount)
				continue
			}

			krbChildRefs := r.docRef.ChildRefs[i]
			actualChildren := make([]*render.RenderElement, 0, len(krbChildRefs))

			parentStartOffset := uint32(0)
			if i < len(r.docRef.ElementStartOffsets) {
				parentStartOffset = r.docRef.ElementStartOffsets[i]
			} else {
				log.Printf("Error linkOriginalKrbChildren: Elem %s (OrigIdx %d) missing from ElementStartOffsets.", currentEl.SourceElementName, i)
				continue
			}

			for _, childRef := range krbChildRefs {
				childAbsoluteFileOffset := parentStartOffset + uint32(childRef.ChildOffset)
				childIndexInInitialElements, found := offsetToInitialElementIndex[childAbsoluteFileOffset]

				if !found {
					log.Printf("Error linkOriginalKrbChildren: Elem %s (OrigIdx %d) ChildRef offset %d (abs %d) does not map to known initial element.",
						currentEl.SourceElementName, i, childRef.ChildOffset, childAbsoluteFileOffset)
					continue
				}
				childEl := &r.elements[childIndexInInitialElements]
				actualChildren = append(actualChildren, childEl)
			}

			if isPlaceholder {
				kryUsageChildrenMap[i] = actualChildren
			} else {
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
	r.roots = nil
	for i := range r.elements {
		if r.elements[i].Parent == nil {
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
		log.Printf("Warn finalizeTreeStructureAndRoots: No root elements identified, but %d elements exist.", len(r.elements))
	}
	return nil
}


func (r *RaylibRenderer) findComponentDefinition(doc *krb.Document, name string) *krb.KrbComponentDefinition {
	if doc == nil || len(doc.ComponentDefinitions) == 0 || len(doc.Strings) == 0 {
		return nil
	}
	for i := range doc.ComponentDefinitions {
		compDef := &doc.ComponentDefinitions[i]
		if int(compDef.NameIndex) < len(doc.Strings) && doc.Strings[compDef.NameIndex] == name {
			return compDef
		}
	}
	return nil
}

func (r *RaylibRenderer) expandComponent(
	instanceElement *render.RenderElement,
	compDef *krb.KrbComponentDefinition,
	doc *krb.Document,
	allElements *[]render.RenderElement,
	nextMasterIndex *int,
	kryUsageChildren []*render.RenderElement,
) error {
	if compDef.RootElementTemplateData == nil || len(compDef.RootElementTemplateData) == 0 {
		log.Printf("Warn expandComponent: Component definition '%s' for instance '%s' has no RootElementTemplateData.", doc.Strings[compDef.NameIndex], instanceElement.SourceElementName)
		instanceElement.Children = nil
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

	defaultFgColor := rl.RayWhite
	defaultBorderColor := rl.Gray
	defaultTextAlignment := uint8(krb.LayoutAlignStart)
	defaultIsVisible := true
	templateDataStreamOffset := uint32(0)

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

		if newElGlobalIndex >= cap(*allElements) {
			newCap := cap(*allElements) * 2
			if newElGlobalIndex >= newCap {
				newCap = newElGlobalIndex + 10
			}
			tempSlice := make([]render.RenderElement, len(*allElements), newCap)
			copy(tempSlice, *allElements)
			*allElements = tempSlice
		}
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

		templateElIdStr, _ := getStringValueByIdx(doc, templateKrbHeader.ID)
		if templateElIdStr != "" {
			newEl.SourceElementName = templateElIdStr
		} else {
			newEl.SourceElementName = fmt.Sprintf("TplElem_Type0x%X_Idx%d", templateKrbHeader.Type, newEl.OriginalIndex)
		}

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
		applyDirectPropertiesToElement(templateDirectProps, doc, newEl)

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
		for _, cProp := range templateCustomProps {
			keyName, keyOk := getStringValueByIdx(doc, cProp.KeyIndex)
			if keyOk && keyName == componentNameConventionKey {
				if (cProp.ValueType == krb.ValTypeString || cProp.ValueType == krb.ValTypeResource) && cProp.Size == 1 {
					valueIndex := cProp.Value[0]
					if strVal, strOk := getStringValueByIdx(doc, valueIndex); strOk {
						nestedComponentName = strVal
						newEl.SourceElementName = nestedComponentName
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

		if len(templateRootsInThisExpansion) == 0 {
			templateRootsInThisExpansion = append(templateRootsInThisExpansion, newEl)
			newEl.Parent = instanceElement

			log.Printf("Debug expandComponent: Applying instance '%s' (OrigIdx %d) props to template root '%s' (GlobalIdx %d)",
				instanceElement.SourceElementName, instanceElement.OriginalIndex, newEl.SourceElementName, newEl.OriginalIndex)

			newEl.Header.ID = instanceElement.Header.ID
			newEl.Header.PosX = instanceElement.Header.PosX
			newEl.Header.PosY = instanceElement.Header.PosY
			newEl.Header.Width = instanceElement.Header.Width
			newEl.Header.Height = instanceElement.Header.Height
			newEl.Header.Layout = instanceElement.Header.Layout

			if instanceElement.Header.StyleID != 0 {
				newEl.Header.StyleID = instanceElement.Header.StyleID
			}
			newEl.SourceElementName = instanceElement.SourceElementName

			if instanceStyle, instanceStyleFound := findStyle(doc, instanceElement.Header.StyleID); instanceStyleFound {
				applyStylePropertiesToElement(instanceStyle.Properties, doc, newEl)
				log.Printf("   Applied instance style ID %d to template root.", instanceElement.Header.StyleID)
			}
			if doc != nil && instanceElement.OriginalIndex < len(doc.Properties) && len(doc.Properties[instanceElement.OriginalIndex]) > 0 {
				applyDirectPropertiesToElement(doc.Properties[instanceElement.OriginalIndex], doc, newEl)
				log.Printf("   Applied instance direct KRB properties to template root.")
			}
		}

		if nestedComponentName != "" {
			nestedCompDef := r.findComponentDefinition(doc, nestedComponentName)
			if nestedCompDef != nil {
				log.Printf("expandComponent: Expanding nested component '%s' for template element '%s' (GlobalIdx: %d)", nestedComponentName, newEl.SourceElementName, newEl.OriginalIndex)
				err := r.expandComponent(newEl, nestedCompDef, doc, allElements, nextMasterIndex, nil) // Nested instances don't take KRY children from this level
				if err != nil {
					return fmt.Errorf("expandComponent '%s': failed to expand nested component '%s': %w", instanceElement.SourceElementName, nestedComponentName, err)
				}
			} else {
				log.Printf("Warn expandComponent: Nested component definition '%s' for template element '%s' not found.", nestedComponentName, newEl.SourceElementName)
			}
		}
	}

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
				log.Printf("Error expandComponent '%s': Child for '%s' (GlobalIdx %d) at template offset %d (abs %d) not found in map. Parent template offset %d, child relative offset %d",
					instanceElement.SourceElementName, parentEl.SourceElementName, parentEl.OriginalIndex, childRef.ChildOffset, childAbsoluteOffsetInTemplate, info.parentHeaderOffsetInTemplate, childRef.ChildOffset)
				continue
			}
			childEl := &(*allElements)[childGlobalIndex]
			if childEl.Parent != nil && childEl.Parent != parentEl {
				log.Printf("Warn expandComponent: Template child '%s' (GlobalIdx %d) already has parent '%s'. Cannot set new parent '%s'.", childEl.SourceElementName, childEl.OriginalIndex, childEl.Parent.SourceElementName, parentEl.SourceElementName)
				continue
			}
			childEl.Parent = parentEl
			parentEl.Children = append(parentEl.Children, childEl)
		}
	}

	if instanceElement != nil {
		instanceElement.Children = make([]*render.RenderElement, 0, len(templateRootsInThisExpansion))
		for _, rootTplEl := range templateRootsInThisExpansion {
			if rootTplEl.Parent == instanceElement {
				instanceElement.Children = append(instanceElement.Children, rootTplEl)
			}
		}
	}

	if len(kryUsageChildren) > 0 {
		slotFound := false
		var slotElement *render.RenderElement
		queue := make([]*render.RenderElement, 0, len(instanceElement.Children))
		if instanceElement.Children != nil { // Check if instanceElement.Children is not nil before appending
		    queue = append(queue, instanceElement.Children...)
        }

		visitedInSearch := make(map[*render.RenderElement]bool)

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
			log.Printf("expandComponent '%s': Found slot '%s' (GlobalIdx %d). Re-parenting %d KRY-usage children.",
				instanceElement.SourceElementName, childrenSlotIDName, slotElement.OriginalIndex, len(kryUsageChildren))
			slotElement.Children = append(slotElement.Children, kryUsageChildren...)
			for _, kryChild := range kryUsageChildren {
				kryChild.Parent = slotElement
			}
		} else {
			log.Printf("Warn expandComponent '%s': No slot '%s' found in template. Appending %d KRY-usage children to first template root.",
				instanceElement.SourceElementName, childrenSlotIDName, len(kryUsageChildren))
			if len(instanceElement.Children) > 0 {
				firstRoot := instanceElement.Children[0]
				firstRoot.Children = append(firstRoot.Children, kryUsageChildren...)
				for _, kryChild := range kryUsageChildren {
					kryChild.Parent = firstRoot
				}
			} else {
				log.Printf("Error expandComponent '%s': No template root to append KRY-usage children to, and no slot found.", instanceElement.SourceElementName)
			}
		}
	}
	return nil
}

func GetCustomPropertyValue(el *render.RenderElement, keyName string, doc *krb.Document) (string, bool) {
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
			return "", false
		}
	}
	return "", false
}

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
	if elementIdentifier == "" && el.Header.ID != 0 && doc != nil {
		idStr, _ := getStringValueByIdx(doc, el.Header.ID)
		if idStr != "" {
			elementIdentifier = idStr
		}
	}
	if elementIdentifier == "" {
		elementIdentifier = fmt.Sprintf("Type0x%X_Idx%d_NoName", el.Header.Type, el.OriginalIndex)
	}

	isHelloWidgetRelated := strings.Contains(elementIdentifier, "HelloWidget") 
	if isHelloWidgetRelated {
		log.Printf(">>>>> PerformLayout for: %s (Type:0x%X, OrigIdx:%d) ParentCTX:%.0f,%.0f,%.0f,%.0f", elementIdentifier, el.Header.Type, el.OriginalIndex, parentContentX, parentContentY, parentContentW, parentContentH)
		log.Printf("      Hdr: W:%d,H:%d,PosX:%d,PosY:%d,Layout:0x%02X(Abs:%t,Grow:%t)", el.Header.Width, el.Header.Height, el.Header.PosX, el.Header.PosY, el.Header.Layout, el.Header.LayoutAbsolute(), el.Header.LayoutGrow())
	}

	isRootElement := (el.Parent == nil)
	scaledUint16Local := func(v uint16) float32 { return float32(v) * scale }

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
	if isHelloWidgetRelated {
		log.Printf("      S1 - Explicit Size: W:%.1f(exp:%t), H:%.1f(exp:%t)", desiredWidth, hasExplicitWidth, desiredHeight, hasExplicitHeight)
	}

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
			if isHelloWidgetRelated {
				log.Printf("      S2a - Intrinsic W (Text): %.1f (text:%.1f, hPad:%.1f)", desiredWidth, textWidthMeasuredInPixels, hPadding)
			}
		}
		if !hasExplicitHeight {
			textHeightMeasuredInPixels := finalFontSizePixels
			desiredHeight = textHeightMeasuredInPixels + vPadding
			if isHelloWidgetRelated {
				log.Printf("      S2a - Intrinsic H (Text): %.1f (text:%.1f, vPad:%.1f)", desiredHeight, textHeightMeasuredInPixels, vPadding)
			}
		}
	} else if el.Header.Type == krb.ElemTypeImage && el.ResourceIndex != render.InvalidResourceIndex {
		texWidth := float32(0)
		texHeight := float32(0)
		if el.TextureLoaded && el.Texture.ID > 0 {
			texWidth = float32(el.Texture.Width)
			texHeight = float32(el.Texture.Height)
		}
		if !hasExplicitWidth {
			desiredWidth = texWidth*scale + hPadding
			if isHelloWidgetRelated {
				log.Printf("      S2b - Intrinsic W (Image): %.1f (texW:%.1f, scale:%.1f, hPad:%.1f)", desiredWidth, texWidth, scale, hPadding)
			}
		}
		if !hasExplicitHeight {
			desiredHeight = texHeight*scale + vPadding
			if isHelloWidgetRelated {
				log.Printf("      S2b - Intrinsic H (Image): %.1f (texH:%.1f, scale:%.1f, vPad:%.1f)", desiredHeight, texHeight, scale, vPadding)
			}
		}
	}

	if !hasExplicitWidth && !isGrow && !isAbsolute {
		if desiredWidth == 0 && (el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp) {
			desiredWidth = parentContentW
			if isHelloWidgetRelated {
				log.Printf("      S2c - Default W (Container): %.1f from parent", desiredWidth)
			}
		}
	}
	if !hasExplicitHeight && !isGrow && !isAbsolute {
		if desiredHeight == 0 && (el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp) {
			desiredHeight = parentContentH
			if isHelloWidgetRelated {
				log.Printf("      S2c - Default H (Container): %.1f from parent", desiredHeight)
			}
		}
	}

	if isRootElement {
		if !hasExplicitWidth && desiredWidth == 0 {
			desiredWidth = parentContentW
			if isHelloWidgetRelated {
				log.Printf("      S2d - Default W (Root): %.1f from screen", desiredWidth)
			}
		}
		if !hasExplicitHeight && desiredHeight == 0 {
			desiredHeight = parentContentH
			if isHelloWidgetRelated {
				log.Printf("      S2d - Default H (Root): %.1f from screen", desiredHeight)
			}
		}
	}
	if isHelloWidgetRelated {
		log.Printf("      S2 - Final Desired: W:%.1f, H:%.1f", desiredWidth, desiredHeight)
	}
	el.RenderW = MaxF(0, desiredWidth)
	el.RenderH = MaxF(0, desiredHeight)
	if isHelloWidgetRelated {
		log.Printf("      S2 - Assigned RenderW/H: W:%.1f, H:%.1f", el.RenderW, el.RenderH)
	}

	el.RenderX = parentContentX
	el.RenderY = parentContentY
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
	}
	if isHelloWidgetRelated {
		log.Printf("      S3 - Position: X:%.1f, Y:%.1f (Abs:%t)", el.RenderX, el.RenderY, el.Header.LayoutAbsolute())
	}

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
	childAvailableWidth := el.RenderW - childBorderLeft - childBorderRight - childPaddingLeft - childPaddingRight
	childAvailableHeight := el.RenderH - childBorderTop - childBorderBottom - childPaddingTop - childPaddingBottom
	childAvailableWidth = MaxF(0, childAvailableWidth)
	childAvailableHeight = MaxF(0, childAvailableHeight)
	if isHelloWidgetRelated {
		log.Printf("      S4 - Child Content Area: X:%.1f,Y:%.1f, W:%.1f,H:%.1f", childContentAreaX, childContentAreaY, childAvailableWidth, childAvailableHeight)
	}

	if len(el.Children) > 0 {
		if isHelloWidgetRelated {
			log.Printf("      S5 - Layout Children for %s...", elementIdentifier)
		}
		PerformLayoutChildren(el, childContentAreaX, childContentAreaY, childAvailableWidth, childAvailableHeight, scale, doc)
		if !hasExplicitHeight && !isGrow && !isAbsolute {
			maxChildExtentMainAxis := float32(0.0)
			parentLayoutDir := el.Header.LayoutDirection()
			isParentVertical := (parentLayoutDir == krb.LayoutDirColumn || parentLayoutDir == krb.LayoutDirColumnReverse)
			parentGap := float32(0)
			numFlowChildren := 0
			for _, child := range el.Children {
				if child != nil && !child.Header.LayoutAbsolute() {
					if numFlowChildren > 0 {
						maxChildExtentMainAxis += parentGap
					}
					if isParentVertical {
						maxChildExtentMainAxis += child.RenderH
					} else {
						childRelativeY := child.RenderY - childContentAreaY
						currentChildExtentY := childRelativeY + child.RenderH
						if currentChildExtentY > maxChildExtentMainAxis {
							maxChildExtentMainAxis = currentChildExtentY
						}
					}
					numFlowChildren++
				}
			}
			var contentHeightFromChildren float32
			if isParentVertical {
				contentHeightFromChildren = maxChildExtentMainAxis + vPadding + childBorderTop + childBorderBottom
			} else {
				contentHeightFromChildren = maxChildExtentMainAxis + vPadding + childBorderTop + childBorderBottom
			}
			if contentHeightFromChildren > 0 && (desiredHeight == 0 || contentHeightFromChildren < desiredHeight || (el.Header.Type != krb.ElemTypeContainer && el.Header.Type != krb.ElemTypeApp)) {
				if desiredHeight == 0 || contentHeightFromChildren < desiredHeight && (el.Header.Type != krb.ElemTypeContainer && el.Header.Type != krb.ElemTypeApp) {
					el.RenderH = contentHeightFromChildren
					if isHelloWidgetRelated {
						log.Printf("      S6 - Content Hug H for %s: %.1f", elementIdentifier, el.RenderH)
					}
				} else if (el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp) && contentHeightFromChildren < el.RenderH {
					el.RenderH = contentHeightFromChildren
					if isHelloWidgetRelated {
						log.Printf("      S6 - Content Shrink H for Container %s: %.1f", elementIdentifier, el.RenderH)
					}
				}
			}
		}
	}
	if isHelloWidgetRelated {
		log.Printf("      S5/6 - After Children/Hugging for %s: W:%.1f, H:%.1f", elementIdentifier, el.RenderW, el.RenderH)
	}

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
	if isHelloWidgetRelated {
		log.Printf("      S7 - Min/Max Constraints for %s: W:%.1f, H:%.1f", elementIdentifier, el.RenderW, el.RenderH)
	}

	el.RenderW = MaxF(0, el.RenderW)
	if el.RenderW > 0 && el.RenderH == 0 {
		if el.Header.Type == krb.ElemTypeContainer || el.Header.Type == krb.ElemTypeApp || el.BgColor.A > 0 || (el.BorderWidths[0]+el.BorderWidths[1]+el.BorderWidths[2]+el.BorderWidths[3] > 0) {
			finalMinHeight := MaxF(ScaledF32(uint8(baseFontSize), scale), vPadding+childBorderTop+childBorderBottom)
			if finalMinHeight == 0 && (el.BgColor.A > 0 || (childBorderTop+childBorderBottom+childBorderLeft+childBorderRight > 0)) {
				finalMinHeight = 1.0 * scale
			}
			if finalMinHeight > 0 {
				el.RenderH = finalMinHeight
				if isHelloWidgetRelated {
					log.Printf("      S8 - Fallback Zero H for %s: %.1f", elementIdentifier, el.RenderH)
				}
			}
		}
	}
	el.RenderH = MaxF(0, el.RenderH)
	if isHelloWidgetRelated {
		log.Printf("<<<<< PerformLayout END for: %s -- Final Render: X:%.1f,Y:%.1f, W:%.1f,H:%.1f", elementIdentifier, el.RenderX, el.RenderY, el.RenderW, el.RenderH)
	}
}

func PerformLayoutChildren(
	parent *render.RenderElement,
	parentClientOriginX, parentClientOriginY,
	availableClientWidth, availableClientHeight float32,
	scale float32,
	doc *krb.Document,
) {
	if parent == nil || len(parent.Children) == 0 {
		return
	}
	parentIdentifier := parent.SourceElementName
	if parentIdentifier == "" {
		parentIdentifier = fmt.Sprintf("ParentType0x%X_Idx%d", parent.Header.Type, parent.OriginalIndex)
	}

	isParentHelloWidgetRelated := strings.Contains(parentIdentifier, "HelloWidget")
	if isParentHelloWidgetRelated {
		log.Printf(">>>>> PerformLayoutChildren for PARENT: %s (Content: X:%.0f,Y:%.0f, AvailW:%.0f,AvailH:%.0f, Layout:0x%02X)",
			parentIdentifier, parentClientOriginX, parentClientOriginY, availableClientWidth, availableClientHeight, parent.Header.Layout)
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

		mainAxisEffectiveSpaceForParent := MuxFloat32(isMainAxisHorizontal, availableClientWidth, availableClientHeight)
		mainAxisEffectiveSpaceForElements := MaxF(0, mainAxisEffectiveSpaceForParent-totalGapSpace)
		crossAxisEffectiveSizeForParent := MuxFloat32(isMainAxisHorizontal, availableClientHeight, availableClientWidth)

		for _, child := range flowChildren {
			childIdentifier := child.SourceElementName
			if childIdentifier == "" {
				childIdentifier = fmt.Sprintf("ChildType0x%X_Idx%d", child.Header.Type, child.OriginalIndex)
			}
			if isParentHelloWidgetRelated {
				log.Printf("      PLC Pass 1 - PerformLayout for child: %s", childIdentifier)
			}
			PerformLayout(child, parentClientOriginX, parentClientOriginY, availableClientWidth, availableClientHeight, scale, doc)
		}

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

		totalFinalElementSizeOnMainAxis := float32(0)
		for _, child := range flowChildren {
			childIdentifier := child.SourceElementName
			if childIdentifier == "" {
				childIdentifier = fmt.Sprintf("ChildType0x%X_Idx%d", child.Header.Type, child.OriginalIndex)
			}

			if child.Header.LayoutGrow() && sizePerGrowChild > 0 {
				if isMainAxisHorizontal {
					child.RenderW = sizePerGrowChild
				} else {
					child.RenderH = sizePerGrowChild
				}
			}

			if crossAxisAlignment == krb.LayoutAlignStretch {
				if isMainAxisHorizontal {
					if child.RenderH == 0 && crossAxisEffectiveSizeForParent > 0 {
						child.RenderH = crossAxisEffectiveSizeForParent
						if isParentHelloWidgetRelated {
							log.Printf("      PLC Pass 3 - Child %s stretched H to %.1f", childIdentifier, child.RenderH)
						}
					}
				} else {
					if child.RenderW == 0 && crossAxisEffectiveSizeForParent > 0 {
						child.RenderW = crossAxisEffectiveSizeForParent
						if isParentHelloWidgetRelated {
							log.Printf("      PLC Pass 3 - Child %s stretched W to %.1f", childIdentifier, child.RenderW)
						}
					}
				}
			}

			child.RenderW = MaxF(0, child.RenderW)
			child.RenderH = MaxF(0, child.RenderH)
			totalFinalElementSizeOnMainAxis += MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
		}

		totalUsedSpaceWithGaps := totalFinalElementSizeOnMainAxis + totalGapSpace
		startOffsetOnMainAxis, effectiveSpacingBetweenItems := calculateAlignmentOffsetsF(layoutAlignment,
			mainAxisEffectiveSpaceForParent, totalUsedSpaceWithGaps,
			len(flowChildren), isLayoutReversed, gapValue)

		if isParentHelloWidgetRelated {
			log.Printf("      PLC Details: mainEffSpaceForElems:%.0f, crossEffSizeForParent:%.0f", mainAxisEffectiveSpaceForElements, crossAxisEffectiveSizeForParent)
			log.Printf("      PLC Details: totalFixed:%.0f, numGrow:%d, spaceForGrow:%.0f, sizePerGrow:%.0f", totalFixedSizeOnMainAxis, numberOfGrowChildren, spaceAvailableForGrowingChildren, sizePerGrowChild)
			log.Printf("      PLC Details: totalFinalMainAxis:%.0f, totalUsedWithGaps:%.0f", totalFinalElementSizeOnMainAxis, totalUsedSpaceWithGaps)
			log.Printf("      PLC Details: startOffMain:%.0f, effSpacing:%.0f", startOffsetOnMainAxis, effectiveSpacingBetweenItems)
		}

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
			childIdentifier := child.SourceElementName
			if childIdentifier == "" {
				childIdentifier = fmt.Sprintf("ChildType0x%X_Idx%d", child.Header.Type, child.OriginalIndex)
			}

			childMainAxisSizeValue := MuxFloat32(isMainAxisHorizontal, child.RenderW, child.RenderH)
			childCrossAxisSizeValue := MuxFloat32(isMainAxisHorizontal, child.RenderH, child.RenderW)
			crossAxisOffset := calculateCrossAxisOffsetF(crossAxisAlignment, crossAxisEffectiveSizeForParent, childCrossAxisSizeValue)

			if isMainAxisHorizontal {
				child.RenderX = parentClientOriginX + currentMainAxisPosition
				child.RenderY = parentClientOriginY + crossAxisOffset
			} else {
				child.RenderX = parentClientOriginX + crossAxisOffset
				child.RenderY = parentClientOriginY + currentMainAxisPosition
			}
			if isParentHelloWidgetRelated {
				log.Printf("      PLC Pass 4 - Pos Child %s: MainPos:%.0f, CrossOff:%.0f => X:%.0f,Y:%.0f (Child W:%.0f,H:%.0f)",
					childIdentifier, currentMainAxisPosition, crossAxisOffset, child.RenderX, child.RenderY, child.RenderW, child.RenderH)
			}

			currentMainAxisPosition += childMainAxisSizeValue
			if i < len(flowChildren)-1 {
				currentMainAxisPosition += effectiveSpacingBetweenItems
			}
		}
	}

	if len(absoluteChildren) > 0 {
		for _, child := range absoluteChildren {
			childIdentifier := child.SourceElementName
			if childIdentifier == "" {
				childIdentifier = fmt.Sprintf("ChildType0x%X_Idx%d", child.Header.Type, child.OriginalIndex)
			}
			if isParentHelloWidgetRelated {
				log.Printf("      PLC - Layout Abs Child: %s", childIdentifier)
			}
			PerformLayout(child, parent.RenderX, parent.RenderY, parent.RenderW, parent.RenderH, scale, doc)
		}
	}
	if isParentHelloWidgetRelated {
		log.Printf("<<<<< PerformLayoutChildren END for PARENT: %s", parentIdentifier)
	}
}

func getNumericValueForSizeProp(props []krb.Property, propID krb.PropertyID, doc *krb.Document) (value float32, valueType krb.ValueType, rawSizeBytes uint8, err error) {
	for _, p := range props {
		if p.ID == propID {
			return getNumericValueFromKrbProp(&p, doc)
		}
	}
	return 0, krb.ValTypeNone, 0, fmt.Errorf("property ID 0x%X not found in list", propID)
}

func getNumericValueFromKrbProp(prop *krb.Property, doc *krb.Document) (value float32, valueType krb.ValueType, rawSizeBytes uint8, err error) {
	if prop == nil {
		return 0, krb.ValTypeNone, 0, fmt.Errorf("getNumericValueFromKrbProp: received nil property")
	}
	if prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 {
		return float32(binary.LittleEndian.Uint16(prop.Value)), krb.ValTypeShort, 2, nil
	}
	if prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 {
		return float32(binary.LittleEndian.Uint16(prop.Value)), krb.ValTypePercentage, 2, nil
	}
	return 0, prop.ValueType, prop.Size, fmt.Errorf("unsupported KRB ValueType (%d) or Size (%d) for numeric size conversion (PropID: %X)", prop.ValueType, prop.Size, prop.ID)
}

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

	for _, root := range roots {
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
	r.loadedTextures = make(map[uint8]rl.Texture2D)

	if rl.IsWindowReady() {
		log.Println("RaylibRenderer Cleanup: Closing Raylib window...")
		rl.CloseWindow()
	} else {
		log.Println("RaylibRenderer Cleanup: Raylib window was already closed or not initialized.")
	}
}

func (r *RaylibRenderer) ShouldClose() bool {
	return rl.IsWindowReady() && rl.WindowShouldClose()
}

func (r *RaylibRenderer) BeginFrame() {
	rl.BeginDrawing()
	rl.ClearBackground(r.config.DefaultBg)
}

func (r *RaylibRenderer) EndFrame() {
	rl.EndDrawing()
}

func (r *RaylibRenderer) PollEvents() {
	if !rl.IsWindowReady() {
		return
	}

	mousePos := rl.GetMousePosition()
	currentMouseCursor := rl.MouseCursorDefault
	isMouseButtonClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	clickHandledThisFrame := false

	for i := len(r.elements) - 1; i >= 0; i-- {
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
			break
		}
	}
	rl.SetMouseCursor(currentMouseCursor)
}

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

func (r *RaylibRenderer) LoadAllTextures() error {
	if r.docRef == nil {
		return fmt.Errorf("cannot load textures, KRB document reference is nil")
	}
	if !rl.IsWindowReady() {
		return fmt.Errorf("cannot load textures, Raylib window is not ready")
	}
	log.Println("LoadAllTextures: Starting...")
	errCount := 0
	r.performTextureLoading(r.docRef, &errCount)
	log.Printf("LoadAllTextures: Complete. Encountered %d errors.", errCount)
	if errCount > 0 {
		return fmt.Errorf("encountered %d errors during texture loading", errCount)
	}
	return nil
}

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
		parentId = el.Parent.OriginalIndex
	}

	log.Printf("%s%s ElemDX_Global[%d] Name='%s' Type=0x%02X Children:%d ParentDX_Global:%d RenderRect=(%.1f,%.1f %.1fwX%.1fh) Visible:%t StyleID:%d LayoutByte:0x%02X",
		indent, prefix, el.OriginalIndex, el.SourceElementName, el.Header.Type, len(el.Children), parentId,
		el.RenderX, el.RenderY, el.RenderW, el.RenderH, el.IsVisible, el.Header.StyleID, el.Header.Layout)

	for i, child := range el.Children {
		logElementTree(child, depth+1, fmt.Sprintf("Child[%d]", i))
	}
}

func (r *RaylibRenderer) ApplyCustomComponentLayoutAdjustments(elements []*render.RenderElement, doc *krb.Document) {
	if doc == nil || len(r.customHandlers) == 0 {
		return
	}
	for _, el := range elements {
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
		r.renderElementRecursive(el, scale)
	} else {
		for _, child := range el.Children {
			r.renderElementRecursiveWithCustomDraw(child, scale)
		}
	}
}

func (r *RaylibRenderer) renderElementRecursive(el *render.RenderElement, scale float32) {
	if el == nil || !el.IsVisible {
		return
	}

	renderXf, renderYf, renderWf, renderHf := el.RenderX, el.RenderY, el.RenderW, el.RenderH

	if renderWf <= 0 || renderHf <= 0 {
		for _, child := range el.Children {
			r.renderElementRecursive(child, scale)
		}
		return
	}

	renderX, renderY := int32(renderXf), int32(renderYf)
	renderW, renderH := int32(renderWf), int32(renderHf)

	effectiveBgColor := el.BgColor
	effectiveFgColor := el.FgColor
	borderColor := el.BorderColor

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

	if el.Header.Type != krb.ElemTypeText && effectiveBgColor.A > 0 {
		rl.DrawRectangle(renderX, renderY, renderW, renderH, effectiveBgColor)
	}

	topBorder := scaledI32(el.BorderWidths[0], scale)
	rightBorder := scaledI32(el.BorderWidths[1], scale)
	bottomBorder := scaledI32(el.BorderWidths[2], scale)
	leftBorder := scaledI32(el.BorderWidths[3], scale)

	clampedTop, clampedBottom := clampOpposingBorders(int(topBorder), int(bottomBorder), int(renderH))
	clampedLeft, clampedRight := clampOpposingBorders(int(leftBorder), int(rightBorder), int(renderW))
	drawBorders(int(renderX), int(renderY), int(renderW), int(renderH),
		clampedTop, clampedRight, clampedBottom, clampedLeft, borderColor)

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

	if contentWidth > 0 && contentHeight > 0 {
		rl.BeginScissorMode(contentX, contentY, contentWidth, contentHeight)
		r.drawContent(el, int(contentX), int(contentY), int(contentWidth), int(contentHeight), scale, effectiveFgColor)
		rl.EndScissorMode()
	}

	for _, child := range el.Children {
		r.renderElementRecursive(child, scale)
	}
}

func (r *RaylibRenderer) performTextureLoading(doc *krb.Document, errorCounter *int) {
	if doc == nil {
		*errorCounter++
		return
	}
	if r.elements == nil {
		*errorCounter++
		return
	}
	for i := range r.elements {
		el := &r.elements[i]
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
				*errorCounter++
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
				ext := ".png"
				img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData)))
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
		}
	}
}

func (r *RaylibRenderer) drawContent(el *render.RenderElement, cx, cy, cw, ch int, scale float32, effectiveFgColor rl.Color) {
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		fontSize := int32(math.Max(1.0, math.Round(baseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		textHeightMeasured := fontSize

		textDrawX := int32(cx)
		textDrawY := int32(cy + (ch-int(textHeightMeasured))/2)

		switch el.TextAlignment {
		case krb.LayoutAlignCenter:
			textDrawX = int32(cx + (cw-int(textWidthMeasured))/2)
		case krb.LayoutAlignEnd:
			textDrawX = int32(cx + cw - int(textWidthMeasured))
		}
		rl.DrawText(el.Text, textDrawX, textDrawY, fontSize, effectiveFgColor)
	}

	isImageElement := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton)
	if isImageElement && el.TextureLoaded && el.Texture.ID > 0 {
		texWidth := float32(el.Texture.Width)
		texHeight := float32(el.Texture.Height)
		sourceRec := rl.NewRectangle(0, 0, texWidth, texHeight)
		destRec := rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch))
		if destRec.Width > 0 && destRec.Height > 0 && sourceRec.Width > 0 && sourceRec.Height > 0 {
			rl.DrawTexturePro(el.Texture, sourceRec, destRec, rl.NewVector2(0, 0), 0.0, rl.White)
		}
	}
}

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

func applyDirectVisualPropertiesToAppElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
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
	default:
		if alignment != krb.LayoutAlignStart && alignment != krb.LayoutAlignCenter && alignment != krb.LayoutAlignEnd && alignment != krb.LayoutAlignSpaceBetween {
			log.Printf("Warn calculateAlignmentOffsetsF: Unknown or non-standard alignment value %d. Defaulting to LayoutAlignStart behavior.", alignment)
		}
		if isLayoutReversed {
			startOffset = unusedSpace
		} else {
			startOffset = 0
		}
	}
	return startOffset, spacingToApplyBetweenChildren
}

func calculateCrossAxisOffsetF(
	alignment uint8,
	parentCrossAxisSize float32,
	childCrossAxisSize float32,
) float32 {
	offset := float32(0.0)
	availableSpace := parentCrossAxisSize - childCrossAxisSize
	switch alignment {
	case krb.LayoutAlignStart:
		offset = 0.0
	case krb.LayoutAlignCenter:
		if availableSpace > 0 {
			offset = availableSpace / 2.0
		}
	case krb.LayoutAlignEnd:
		if availableSpace > 0 {
			offset = availableSpace
		}
	default:
		offset = 0.0
	}
	return MaxF(0, offset)
}

func resolveElementText(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeText && el.Header.Type != krb.ElemTypeButton {
		return
	}
	if el.Text != "" {
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

func resolveElementImageSource(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeImage && el.Header.Type != krb.ElemTypeButton {
		return
	}
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
	el.EventHandlers = nil
	if doc != nil && doc.Events != nil &&
		el.OriginalIndex < len(doc.Events) &&
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
	if doc == nil || styleID == 0 || int(styleID) > len(doc.Styles) {
		return nil, false
	}
	return &doc.Styles[styleID-1], true
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
	if nameIndex == 0 {
		if len(doc.Strings) == 0 || doc.Strings[0] == "" {
			return 0
		}
	}
	for i := range doc.Styles {
		if doc.Styles[i].NameIndex == nameIndex {
			return doc.Styles[i].ID
		}
	}
	return 0
}

func getStyleColors(doc *krb.Document, styleID uint8, flags uint16) (bg rl.Color, fg rl.Color, ok bool) {
	if doc == nil || styleID == 0 {
		return rl.Blank, rl.White, false
	}
	styleIndex := int(styleID - 1)
	if styleIndex < 0 || styleIndex >= len(doc.Styles) {
		return rl.Blank, rl.White, false
	}
	style := &doc.Styles[styleIndex]
	bg, fg = rl.Blank, rl.White
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
	return bg, fg, true
}

func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {
	if prop == nil || prop.ValueType != krb.ValTypeColor {
		return rl.Color{}, false
	}
	useExtended := (flags & krb.FlagExtendedColor) != 0
	if useExtended {
		if len(prop.Value) == 4 {
			return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true
		}
	} else {
		if len(prop.Value) == 1 {
			log.Printf("Warn getColorValue: Palette color (index %d) requested, but palette system not implemented. Returning Magenta.", prop.Value[0])
			return rl.Magenta, true
		}
	}
	log.Printf("Warn getColorValue: Invalid color data for PropID %X, ValueType %X, Size %d, ExtendedFlag %t", prop.ID, prop.ValueType, prop.Size, useExtended)
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

func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 {
		return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true
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
	if borderA+borderB > totalSize {
		borderA = totalSize / 2
		borderB = totalSize - borderA
	}
	return borderA, borderB
}

func drawBorders(x, y, w, h, top, right, bottom, left int, color rl.Color) {
	if color.A == 0 {
		return
	}
	if top > 0 {
		rl.DrawRectangle(int32(x), int32(y), int32(w), int32(top), color)
	}
	if bottom > 0 {
		rl.DrawRectangle(int32(x), int32(y+h-bottom), int32(w), int32(bottom), color)
	}
	sideY := y + top
	sideH := h - top - bottom
	if sideH > 0 {
		if left > 0 {
			rl.DrawRectangle(int32(x), int32(sideY), int32(left), int32(sideH), color)
		}
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

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
