// render/raylib/raylib_renderer.go
package raylib

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	// "strings" // Not currently needed here

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

const baseFontSize = 18.0 // Base font size for text rendering

type RaylibRenderer struct {
	config          render.WindowConfig
	elements        []render.RenderElement
	roots           []*render.RenderElement
	loadedTextures  map[uint8]rl.Texture2D
	krbFileDir      string
	scaleFactor     float32
	docRef          *krb.Document

	// --- Instance-specific maps ---
	eventHandlerMap map[string]func()                      // Map standard callback names to Go funcs
	customHandlers  map[string]render.CustomComponentHandler // Map component names to Go handlers
}

// NewRaylibRenderer creates a new Raylib renderer instance.
func NewRaylibRenderer() *RaylibRenderer {
	return &RaylibRenderer{
		loadedTextures:  make(map[uint8]rl.Texture2D),
		scaleFactor:     1.0,
		eventHandlerMap: make(map[string]func()),
		customHandlers:  make(map[string]render.CustomComponentHandler), // Initialize map
	}
}

// Init initializes the Raylib window based on the provided configuration.
func (r *RaylibRenderer) Init(config render.WindowConfig) error {
	r.config = config
	// Use scale factor from config, ensuring it's at least 1.0
	r.scaleFactor = float32(math.Max(1.0, float64(config.ScaleFactor)))
	log.Printf("Raylib Init: Window %dx%d Title: '%s' Scale: %.2f", config.Width, config.Height, config.Title, r.scaleFactor)

	// Optional: Configure Raylib window flags before InitWindow
	// rl.SetConfigFlags(rl.FlagMsaa4xHint | rl.FlagVsyncHint)

	rl.InitWindow(int32(config.Width), int32(config.Height), config.Title)

	// Apply resizable flag
	if config.Resizable {
		rl.SetWindowState(rl.FlagWindowResizable)
	} else {
		rl.ClearWindowState(rl.FlagWindowResizable)
		// Ensure initial size is set correctly if not resizable
		rl.SetWindowSize(config.Width, config.Height)
	}

	rl.SetTargetFPS(60) // Set a target frame rate
	// rl.SetExitKey(0) // Optional: Disable default ESC key closing the window

	// Check if window initialized successfully (important before GPU operations)
	if !rl.IsWindowReady() {
		return fmt.Errorf("raylib InitWindow failed or window is not ready")
	}
	log.Println("Raylib Init: Window is ready.") // Added confirmation log
	return nil
}

// PrepareTree processes the KRB document into a renderable tree structure.
// It initializes RenderElement structs, applies styles and standard properties,
// resolves text/images/events, and builds the parent-child hierarchy.
// Custom properties are NOT interpreted here.
func (r *RaylibRenderer) PrepareTree(doc *krb.Document, krbFilePath string) ([]*render.RenderElement, render.WindowConfig, error) {
	if doc == nil || doc.Header.ElementCount == 0 {
		log.Println("PrepareTree: No elements in document.")
		return nil, r.config, nil
	}
	r.docRef = doc // Store reference for helper functions

	// --- Determine Base Directory for Resources ---
	var err error
	if krbFilePath == "." || krbFilePath == "" {
		r.krbFileDir, err = os.Getwd()
		if err != nil {
			log.Printf("WARN PrepareTree: Could not get CWD: %v. Using '.'", err)
			r.krbFileDir = "."
		}
	} else {
		absDir, err := filepath.Abs(filepath.Dir(krbFilePath))
		if err != nil {
			log.Printf("WARN PrepareTree: Failed to get abs path for '%s': %v. Using provided dir.", krbFilePath, err)
			r.krbFileDir = filepath.Dir(krbFilePath)
		} else {
			r.krbFileDir = absDir
		}
	}
	log.Printf("PrepareTree: Resource Base Directory: %s", r.krbFileDir)

	// --- Initialize Data Structures ---
	r.elements = make([]render.RenderElement, doc.Header.ElementCount)
	r.roots = nil // Reset roots slice

	// --- Set Up Defaults ---
	windowConfig := render.DefaultWindowConfig() // Start with library defaults
	// *** NOTE: We still determine the window's clear color here ***
	windowConfig.DefaultBg = rl.Black // Start with black
	// Other visual defaults that might be inherited IF NOT transparent
	defaultFg := rl.RayWhite
	defaultBorder := rl.Gray
	defaultBorderWidth := uint8(0)
	defaultTextAlign := uint8(krb.LayoutAlignStart) // Default to Start alignment
	defaultVisible := true                          // Default visibility

	// --- Pass 1: Process App Element (if present) to override defaults/config ---
	hasAppElement := (doc.Header.Flags&krb.FlagHasApp) != 0 && doc.Header.ElementCount > 0 && doc.Elements[0].Type == krb.ElemTypeApp
	if hasAppElement {
		appElement := &r.elements[0] // App is always the first element if present
		appElement.Header = doc.Elements[0]
		appElement.OriginalIndex = 0

		// Apply App's style props -> ONLY affects window defaults now
		if style, ok := findStyle(doc, appElement.Header.StyleID); ok {
			// Pass only windowConfig.DefaultBg to be modified by style
			applyStylePropertiesToWindowDefaults(style.Properties, doc, &windowConfig.DefaultBg)
		} else if appElement.Header.StyleID != 0 {
			log.Printf("Warn: App element has invalid StyleID %d", appElement.Header.StyleID)
		}

		// Apply App's direct KRB properties -> ONLY affects window config now
		if len(doc.Properties) > 0 && len(doc.Properties[0]) > 0 {
			applyDirectPropertiesToConfig(doc.Properties[0], doc, &windowConfig)
		}

		// Scale factor update remains
		r.scaleFactor = float32(math.Max(1.0, float64(windowConfig.ScaleFactor)))
		log.Printf("PrepareTree: Processed App. Config: %dx%d '%s' Scale:%.2f Resizable:%t", windowConfig.Width, windowConfig.Height, windowConfig.Title, r.scaleFactor, windowConfig.Resizable)
	} else {
		log.Println("PrepareTree: No App element found, using default window config.")
	}
	// Store final window config (primarily for size, title, resizable, DefaultBg)
	r.config = windowConfig

	// --- Pass 2: Initialize All RenderElements ---
	// Apply defaults, styles, and direct standard properties to each element.
	for i := 0; i < int(doc.Header.ElementCount); i++ {
		currentEl := &r.elements[i]
		currentEl.Header = doc.Elements[i]
		currentEl.OriginalIndex = i

		// --- >>> MODIFICATION: Default Background is TRANSPARENT <<< ---
		currentEl.BgColor = rl.Blank // Use Raylib's fully transparent color
		// --- Default other visual properties ---
		currentEl.FgColor = defaultFg
		currentEl.BorderColor = defaultBorder
		currentEl.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth} // Uniform default border
		currentEl.TextAlignment = defaultTextAlign
		currentEl.IsVisible = defaultVisible
		// --- END MODIFICATION ---

		// Set other basic properties derived from KRB header
		currentEl.IsInteractive = (currentEl.Header.Type == krb.ElemTypeButton || currentEl.Header.Type == krb.ElemTypeInput)
		currentEl.ResourceIndex = render.InvalidResourceIndex // Initialize resource index

		// Apply Style Properties (overrides defaults, including the transparent BgColor if style defines one)
		style, styleOk := findStyle(doc, currentEl.Header.StyleID)
		if styleOk {
			// This function now correctly overrides the rl.Blank default if the style has a background_color
			applyStylePropertiesToElement(style.Properties, doc, currentEl)
		} else if currentEl.Header.StyleID != 0 && i != 0 { // Don't warn again for App if already warned
			log.Printf("Warn: Elem %d (Type %d) has invalid StyleID %d", i, currentEl.Header.Type, currentEl.Header.StyleID)
		}

		// Apply Direct Standard Properties (overrides style and defaults)
		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			// Special handling for App element's OWN background if needed
			if i == 0 && hasAppElement {
				// If App element itself should have a visible background (rare, usually just sets window default)
				applyDirectVisualPropertiesToAppElement(doc.Properties[0], doc, currentEl)
			} else if i != 0 || !hasAppElement {
				// Overrides style/defaults (including potentially overriding BgColor back to non-transparent)
				applyDirectPropertiesToElement(doc.Properties[i], doc, currentEl)
			}
		}

		// Resolve Content and Events (using potentially overridden properties)
		resolveElementText(doc, currentEl, style, styleOk)
		resolveElementImageSource(doc, currentEl, style, styleOk)
		resolveEventHandlers(doc, currentEl) // Stores callback info

	} // End loop initializing elements

	// --- Pass 3: Build Parent/Child Tree Hierarchy ---
	log.Println("PrepareTree: Building element tree...")

	errBuild := buildElementTree(doc, r.elements, &r.roots, doc.ElementStartOffsets)
	if errBuild != nil {
		log.Printf("Error: Failed to build element tree: %v", errBuild)
		// Handle error appropriately, maybe return it from PrepareTree
		return nil, r.config, fmt.Errorf("failed to build element tree: %w", errBuild)
	}

	return r.roots, r.config, nil
}

// GetRenderTree returns the flat list of all processed render elements.
// This implements the method added to the render.Renderer interface.
func (r *RaylibRenderer) GetRenderTree() []*render.RenderElement {
	// Create a new slice of pointers referencing the elements stored in the renderer
	pointers := make([]*render.RenderElement, len(r.elements))
	for i := range r.elements {
		pointers[i] = &r.elements[i]
	}
	return pointers
}

// RenderFrame performs layout, custom adjustments, and drawing.
func (r *RaylibRenderer) RenderFrame(roots []*render.RenderElement) {
	windowResized := rl.IsWindowResized()
	currentWidth := r.config.Width
	currentHeight := r.config.Height

	// --- Handle Window Resizing ---
	if windowResized && r.config.Resizable {
		newWidth, newHeight := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
		if newWidth != currentWidth || newHeight != currentHeight {
			// Update config and current dimensions if changed
			r.config.Width, r.config.Height = newWidth, newHeight
			currentWidth, currentHeight = newWidth, newHeight
			log.Printf("Window resized to %dx%d", currentWidth, currentHeight)
			// Layout will recalculate based on new dimensions
		}
	} else if !r.config.Resizable {
		// If not resizable, ensure window size matches config (user might have forced resize)
		screenWidth, screenHeight := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
		if currentWidth != screenWidth || currentHeight != screenHeight {
			rl.SetWindowSize(currentWidth, currentHeight)
		}
	}

	// --- 1. Standard Layout Pass ---
	for _, root := range roots {
		if root != nil { // Safety check
			PerformLayout(root, 0, 0, currentWidth, currentHeight, r.scaleFactor, r.docRef)
		}
	}

	// --- 2. Custom Component Layout Adjustment Pass ---
	r.ApplyCustomComponentLayoutAdjustments(r.GetRenderTree(), r.docRef) 
	// --- 3. Draw Pass ---

	for _, root := range roots {
		if root != nil { // Safety check
			r.renderElementRecursiveWithCustomDraw(root, r.scaleFactor) // Use the instance method
		}
	}
}

// PerformLayout calculates layout based on standard KRB rules (Layout byte, standard sizes).
// It does NOT interpret custom properties for layout.
func PerformLayout(el *render.RenderElement, parentContentX, parentContentY, parentContentW, parentContentH int, scale float32, doc *krb.Document) {
	if el == nil {
		log.Printf("DEBUG Layout: Skipping nil element.")
		return // Safety check
	}

	// --- Element Identification ---
	elIDStr := fmt.Sprintf("Elem %d (Type %d)", el.OriginalIndex, el.Header.Type)
	if doc != nil && el.Header.ID != 0 && int(el.Header.ID) < len(doc.Strings) { // Check doc != nil
		elIDStr += fmt.Sprintf(" ID:'%s'", doc.Strings[el.Header.ID])
	}

	isRoot := (el.Parent == nil)
	scaled := func(v uint16) int { return int(math.Round(float64(v) * float64(scale))) }
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

	// --- 1. Determine Base Size ---
	baseW, baseH := calculateBaseSize(el, scale, doc)
	el.IntrinsicW = baseW
	el.IntrinsicH = baseH

	// Apply explicit size from KRB header
	explicitW, explicitH := false, false // Define flags HERE to capture header explicitness
	if el.Header.Width > 0 {
		baseW = scaled(el.Header.Width)
		explicitW = true // Set flag
	}
	if el.Header.Height > 0 {
		baseH = scaled(el.Header.Height)
		explicitH = true // Set flag
	}

	// --- 2. Determine Initial Render Size ---
	initialW, initialH := baseW, baseH
	isFlowElement := !el.Header.LayoutAbsolute()

	if isRoot {
		initialW, initialH = parentContentW, parentContentH
	} else if isFlowElement {
		// Clamp size to the available space from the parent *if* the element doesn't have an explicit size itself
		if !explicitW && initialW > parentContentW { // Use the flag from Step 1
			initialW = max(0, parentContentW)
		}
		if !explicitH && initialH > parentContentH { // Use the flag from Step 1
			initialH = max(0, parentContentH)
		}
	}

	el.RenderW = initialW
	el.RenderH = initialH // Store potentially clamped size

	// --- 3. Determine Initial Render Position ---
	el.RenderX = parentContentX
	el.RenderY = parentContentY

	if isEffectivelyAbsolute(el) {
		offsetX, offsetY := scaled(el.Header.PosX), scaled(el.Header.PosY)
		if el.Parent != nil {
			el.RenderX = el.Parent.RenderX + offsetX
			el.RenderY = el.Parent.RenderY + offsetY
		} else if !isRoot {
			el.RenderX = offsetX
			el.RenderY = offsetY
		} else {
			el.RenderX = offsetX
			el.RenderY = offsetY
		}
	} 

	// --- 4. Layout Children (Recursive Step) ---
	if el.Header.ChildCount > 0 && len(el.Children) > 0 {
		borderL := scaledU8(el.BorderWidths[3])
		borderT := scaledU8(el.BorderWidths[0])
		borderR := scaledU8(el.BorderWidths[1])
		borderB := scaledU8(el.BorderWidths[2])
		clientAbsX := el.RenderX + borderL
		clientAbsY := el.RenderY + borderT

		// Determine the available width and height FOR THE CHILDREN.
		potentialChildW := parentContentW
		if explicitW { // Use flag from Step 1
			potentialChildW = baseW
		}
		potentialChildH := parentContentH
		if explicitH { // Use flag from Step 1
			potentialChildH = baseH
		}

		clientWidth := max(0, potentialChildW-borderL-borderR)
		clientHeight := max(0, potentialChildH-borderT-borderB)

		PerformLayoutChildren(el, clientAbsX, clientAbsY, clientWidth, clientHeight, scale, doc)
	} // End Child Layout Step

	// --- 5. Final Size Adjustment (Auto-Sizing) ---
	shouldGrow := el.Header.LayoutGrow() && isFlowElement && !isRoot
	// Check style for max/min size constraints
	hasExplicitW_StyleCheck := hasStyleSize(doc, el, krb.PropIDMaxWidth, krb.PropIDMinWidth)
	hasExplicitH_StyleCheck := hasStyleSize(doc, el, krb.PropIDMaxHeight, krb.PropIDMinHeight)

	hasExplicitW := explicitW || hasExplicitW_StyleCheck
	hasExplicitH := explicitH || hasExplicitH_StyleCheck

	// Determine if auto-sizing should occur
	shouldAutoSizeW := !shouldGrow && !hasExplicitW
	shouldAutoSizeH := !shouldGrow && !hasExplicitH

	if (shouldAutoSizeW || shouldAutoSizeH) && !isRoot && isFlowElement {
		log.Printf("DEBUG Layout [%s]: Attempting Auto-Size (IsRoot:%t, IsFlow:%t | Grow:%t | ExplicitW:%t [Hdr:%t Stl:%t] | ExplicitH:%t [Hdr:%t Stl:%t] => AutoW:%t, AutoH:%t)",
			elIDStr, isRoot, isFlowElement, shouldGrow,
			hasExplicitW, explicitW, hasExplicitW_StyleCheck, // explicitW is flag from Step 1 header check
			hasExplicitH, explicitH, hasExplicitH_StyleCheck, // explicitH is flag from Step 1 header check
			shouldAutoSizeW, shouldAutoSizeH)

		borderL, borderT := scaledU8(el.BorderWidths[3]), scaledU8(el.BorderWidths[0])
		borderR, borderB := scaledU8(el.BorderWidths[1]), scaledU8(el.BorderWidths[2])
		clientAbsX := el.RenderX + borderL
		clientAbsY := el.RenderY + borderT
		preAutoSizeW, preAutoSizeH := el.RenderW, el.RenderH

		fitToChildren(el, clientAbsX, clientAbsY, borderL, borderT, borderR, borderB, shouldAutoSizeW, shouldAutoSizeH)

		if el.RenderW != preAutoSizeW || el.RenderH != preAutoSizeH {
			log.Printf("DEBUG Layout [%s]: Auto-Sized from %dx%d to %dx%d", elIDStr, preAutoSizeW, preAutoSizeH, el.RenderW, el.RenderH)
		} else {
			log.Printf("DEBUG Layout [%s]: Auto-Size did not change dimensions (%dx%d)", elIDStr, el.RenderW, el.RenderH)
		}
	} // End Auto-Size block

	// --- 6. Ensure Minimum Size ---
	finalW, finalH := max(1, el.RenderW), max(1, el.RenderH)
	if finalW != el.RenderW || finalH != el.RenderH {
		log.Printf("DEBUG Layout [%s]: Clamping to minimum size 1x1. Prev: %dx%d -> Final: %dx%d", elIDStr, el.RenderW, el.RenderH, finalW, finalH)
	}
	el.RenderW, el.RenderH = finalW, finalH

} // End PerformLayout

// PerformLayoutChildren lays out flow and absolute children.
func PerformLayoutChildren(parent *render.RenderElement, parentClientOriginX, parentClientOriginY, availableW, availableH int, scale float32, doc *krb.Document) {
	if parent == nil || len(parent.Children) == 0 {
		return
	}

	parentIDStr := fmt.Sprintf("Elem %d", parent.OriginalIndex)
	if doc != nil && parent.Header.ID != 0 && int(parent.Header.ID) < len(doc.Strings) {
		parentIDStr += fmt.Sprintf(" ID:'%s'", doc.Strings[parent.Header.ID])
	}

	flowChildren := make([]*render.RenderElement, 0, len(parent.Children))
	absoluteChildren := make([]*render.RenderElement, 0)
	for _, child := range parent.Children {
		if child == nil {
			continue
		}
		if isEffectivelyAbsolute(child) {
			absoluteChildren = append(absoluteChildren, child)
		} else {
			flowChildren = append(flowChildren, child)
		}
	}

	// --- Layout Flow Children ---
	if len(flowChildren) > 0 {
		// Get layout settings from the parent's KRB header Layout byte
		direction := parent.Header.LayoutDirection()
		alignment := parent.Header.LayoutAlignment()
		isReversed := (direction == krb.LayoutDirRowReverse || direction == krb.LayoutDirColumnReverse)
		mainAxisIsHorizontal := (direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse)

		// Determine available space along the main and cross axes
		mainAxisAvailableSpace := MuxInt(mainAxisIsHorizontal, availableW, availableH)
		crossAxisSize := MuxInt(mainAxisIsHorizontal, availableH, availableW)

		// --- Pass 1: Calculate Initial Sizes & Fixed Space ---
		totalFixedSizeMainAxis := 0
		growChildrenCount := 0
		for _, child := range flowChildren {
			PerformLayout(child, parentClientOriginX, parentClientOriginY, availableW, availableH, scale, doc)

			if child.Header.LayoutGrow() {
				growChildrenCount++
			} else {
				fixedSize := MuxInt(mainAxisIsHorizontal, child.RenderW, child.RenderH)
				totalFixedSizeMainAxis += fixedSize
			}
		}

		// --- Calculate & Distribute Growth Space ---
		spaceForGrowth := max(0, mainAxisAvailableSpace-totalFixedSizeMainAxis)
		growSizePerChild := 0
		remainderForLastGrowChild := 0
		// *** ADDED FLAG: Track if growth calculation applies to ALL available space ***
		allSpaceIsGrowth := (totalFixedSizeMainAxis == 0 && growChildrenCount > 0 && spaceForGrowth > 0)

		if growChildrenCount > 0 && spaceForGrowth > 0 {
			growSizePerChild = spaceForGrowth / growChildrenCount
			remainderForLastGrowChild = spaceForGrowth % growChildrenCount
		}

		// --- Pass 2: Finalize Sizes ---
		totalFinalFlowSizeMainAxis := 0
		tempGrowCount := 0
		for _, child := range flowChildren {
			isGrowing := child.Header.LayoutGrow()

			if isGrowing {
				growAmount := growSizePerChild
				// Distribute remainder to the last growing child encountered
				if tempGrowCount == growChildrenCount-1 {
					growAmount += remainderForLastGrowChild
				}
				tempGrowCount++ // Increment after checking remainder condition

				// --- >>> CORRECTED GROWTH APPLICATION <<< ---
				if allSpaceIsGrowth {
					// If ALL available space was distributed via growth, SET the size directly.
					if mainAxisIsHorizontal {
						child.RenderW = growAmount    // SET width
						child.RenderH = crossAxisSize // Stretch height
					} else {
						child.RenderH = growAmount    // SET height
						child.RenderW = crossAxisSize // Stretch width
					}
				} else {
					// If only *some* space was for growth (mixed fixed/grow children), ADD the growth amount.
					if mainAxisIsHorizontal {
						child.RenderW += growAmount   // ADD width
						child.RenderH = crossAxisSize // Stretch height
					} else {
						child.RenderH += growAmount   // ADD height
						child.RenderW = crossAxisSize // Stretch width
					}
				}
				// --- >>> END CORRECTION <<< ---

			} else { // Not growing
				// Clamp cross axis size if needed (stretch not applicable)
				if mainAxisIsHorizontal {
					if child.RenderH > crossAxisSize {
						child.RenderH = crossAxisSize
					}
				} else {
					if child.RenderW > crossAxisSize {
						child.RenderW = crossAxisSize
					}
				}
			}

			// Ensure minimum dimensions AFTER applying growth/clamping
			child.RenderW = max(1, child.RenderW)
			child.RenderH = max(1, child.RenderH)
			totalFinalFlowSizeMainAxis += MuxInt(mainAxisIsHorizontal, child.RenderW, child.RenderH)
		}

		// --- Calculate Alignment Offsets ---
		startOffset, spacing := calculateAlignmentOffsets(alignment, mainAxisAvailableSpace, totalFinalFlowSizeMainAxis, len(flowChildren), isReversed)

		// --- Pass 3: Position Flow Children ---
		currentMainAxisPos := startOffset
		indices := make([]int, len(flowChildren))
		for i := range indices {
			indices[i] = i
		}
		if isReversed {
			ReverseSliceInt(indices)
		}

		for i, childIndex := range indices {
			child := flowChildren[childIndex]
			childW, childH := child.RenderW, child.RenderH
			// Initialize childX/Y with the parent's content area origin
			childX, childY := parentClientOriginX, parentClientOriginY
			nextMainAxisPos := currentMainAxisPos

			// Calculate the offset along the cross-axis based on alignment
			crossOffset := calculateCrossAxisOffset(alignment, crossAxisSize, MuxInt(mainAxisIsHorizontal, childH, childW))

			// Calculate FINAL ABSOLUTE Screen Coordinates by adding relative position to parent's origin
			if mainAxisIsHorizontal {
				// Add relative position along main axis (currentMainAxisPos) to parent's X origin
				childX = parentClientOriginX + currentMainAxisPos
				// Add relative position along cross axis (crossOffset) to parent's Y origin
				childY = parentClientOriginY + crossOffset
				// Prepare next main axis position
				nextMainAxisPos += childW
			} else { // Column
				// Add relative position along main axis (currentMainAxisPos) to parent's Y origin
				childY = parentClientOriginY + currentMainAxisPos
				// Add relative position along cross axis (crossOffset) to parent's X origin
				childX = parentClientOriginX + crossOffset
				// Prepare next main axis position
				nextMainAxisPos += childH
			}

			// Assign FINAL ABSOLUTE Screen Coordinates to the child element
			child.RenderX, child.RenderY = childX, childY

			// Advance main axis position for the next child
			currentMainAxisPos = nextMainAxisPos
			// Add spacing if using space-between alignment
			if alignment == krb.LayoutAlignSpaceBetween && i < len(flowChildren)-1 {
				currentMainAxisPos += spacing
			}
		} // End loop positioning children
	} // End if len(flowChildren) > 0

	// --- Layout Absolute Children ---
	if len(absoluteChildren) > 0 {
		for _, child := range absoluteChildren {
			// Pass parent's top-left corner (RenderX/Y) and full available space initially.
			// PerformLayout will use child.Header.PosX/Y relative to parent.RenderX/Y if absolute.
			PerformLayout(child, parent.RenderX, parent.RenderY, availableW, availableH, scale, doc)
		}
	}
} // End PerformLayoutChildren

// renderElementRecursive draws an element and its children recursively based on final layout.
// It determines effective colors based on IsActive state before drawing.

func (r *RaylibRenderer) renderElementRecursive(el *render.RenderElement, scale float32) {
	if el == nil || !el.IsVisible {
		return
	}

	renderX, renderY := el.RenderX, el.RenderY
	renderW, renderH := el.RenderW, el.RenderH

	if renderW <= 0 || renderH <= 0 {
		for _, child := range el.Children {
			r.renderElementRecursive(child, scale) // Recurse using the method
		}
		return
	}

	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

	// --- Determine Effective Colors based on IsActive state ---
	effectiveBgColor := el.BgColor
	effectiveFgColor := el.FgColor

	// --- MODIFIED HEURISTIC ---
	// Apply dynamic styles if it's a Button AND has style indices set.
	// No longer checks el.Parent.IsComponentInstance.
	isPotentiallyDynamicButton := (el.Header.Type == krb.ElemTypeButton)

	if isPotentiallyDynamicButton && (el.ActiveStyleNameIndex != 0 || el.InactiveStyleNameIndex != 0) {
		targetStyleNameIndex := el.InactiveStyleNameIndex
		if el.IsActive {
			targetStyleNameIndex = el.ActiveStyleNameIndex
		}

		// *** MODIFIED: Access doc through r.docRef ***
		if r.docRef != nil && targetStyleNameIndex != 0 {
			// Pass r.docRef to helpers
			targetStyleID := findStyleIDByNameIndex(r.docRef, targetStyleNameIndex)
			if targetStyleID != 0 {
				// Pass r.docRef to helpers
				bg, fg, ok := getStyleColors(r.docRef, targetStyleID, r.docRef.Header.Flags)
				if ok {
					effectiveBgColor = bg
					effectiveFgColor = fg
				} else { /* Log Warning */ }
			} else { /* Log Warning */ }
		} else if r.docRef == nil { /* Log Warning */ }
	}
	// --- End Determining Effective Colors ---

	borderColor := el.BorderColor
	topBW := scaledU8(el.BorderWidths[0]); rightBW := scaledU8(el.BorderWidths[1])
	bottomBW := scaledU8(el.BorderWidths[2]); leftBW := scaledU8(el.BorderWidths[3])
	topBW, bottomBW = clampOpposingBorders(topBW, bottomBW, renderH)
	leftBW, rightBW = clampOpposingBorders(leftBW, rightBW, renderW)

	// Draw Background using EFFECTIVE color
	shouldDrawBackground := el.Header.Type != krb.ElemTypeText && effectiveBgColor.A > 0
	if shouldDrawBackground {
		rl.DrawRectangle(int32(renderX), int32(renderY), int32(renderW), int32(renderH), effectiveBgColor)
	}

	// Draw Borders
	drawBorders(renderX, renderY, renderW, renderH, topBW, rightBW, bottomBW, leftBW, borderColor)

	// Calculate Content Area
	contentX, contentY, contentWidth, contentHeight := calculateContentArea(renderX, renderY, renderW, renderH, topBW, rightBW, bottomBW, leftBW)

	// Draw Content using EFFECTIVE foreground color
	if contentWidth > 0 && contentHeight > 0 {
		rl.BeginScissorMode(int32(contentX), int32(contentY), int32(contentWidth), int32(contentHeight))
		// *** MODIFIED: Pass r.docRef needed by drawContent's helpers ***
		r.drawContent(el, contentX, contentY, contentWidth, contentHeight, scale, effectiveFgColor)
		rl.EndScissorMode()
	}

	// Recursively Draw Children
	for _, child := range el.Children {
		r.renderElementRecursive(child, scale) // Call the method recursively
	}
}


// Cleanup unloads resources and closes the Raylib window.
func (r *RaylibRenderer) Cleanup() {
	log.Println("Raylib Cleanup: Unloading textures...")
	unloadedCount := 0
	// Unload textures from the cache
	for resIndex, texture := range r.loadedTextures {
		if rl.IsTextureReady(texture) {
			rl.UnloadTexture(texture)
			unloadedCount++
		}
		delete(r.loadedTextures, resIndex) // Remove from map
	}
	log.Printf("Raylib Cleanup: Unloaded %d textures from cache.", unloadedCount) // <<<--- IMPROVED LOG ---<<<
	r.loadedTextures = make(map[uint8]rl.Texture2D) // Clear map just in case

	// Also ensure elements don't hold dangling references (though GC should handle this)
	// for i := range r.elements {
	// 	r.elements[i].Texture = rl.Texture2D{} // Zero out the texture struct
	// }

	if rl.IsWindowReady() {
		log.Println("Raylib Cleanup: Closing window...")
		rl.CloseWindow()
	} else {
		log.Println("Raylib Cleanup: Window was already closed or not ready.") // <<<--- ADDED ELSE CASE ---<<<
	}
}
// ShouldClose checks if the window close button or ESC key (if enabled) was pressed.
func (r *RaylibRenderer) ShouldClose() bool {
	// Check IsWindowReady first to avoid potential issues if called after Cleanup
	return rl.IsWindowReady() && rl.WindowShouldClose()
}

// BeginFrame prepares Raylib for drawing.
func (r *RaylibRenderer) BeginFrame() {
	rl.BeginDrawing()
	// Clear background using the default color (potentially set by App element)
	rl.ClearBackground(r.config.DefaultBg)
}

// EndFrame finishes the Raylib drawing sequence for the current frame.
func (r *RaylibRenderer) EndFrame() {
	rl.EndDrawing()
}

// PollEvents handles window events and user input.
func (r *RaylibRenderer) PollEvents() {
	if !rl.IsWindowReady() {
		return // Don't process if window isn't ready
	}

	// Get current input state
	mousePos := rl.GetMousePosition()
	cursor := rl.MouseCursorDefault
	mouseClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)

	// Track if a click has been handled to prevent multiple triggers
	clickedElementFound := false

	// Iterate elements top-down (visually)
	for i := len(r.elements) - 1; i >= 0; i-- {
		el := &r.elements[i]

		// Skip non-visible, non-interactive, or zero-size elements
		if el == nil || !el.IsVisible || !el.IsInteractive || el.RenderW <= 0 || el.RenderH <= 0 {
			continue
		}

		// Check for mouse hover
		bounds := rl.NewRectangle(float32(el.RenderX), float32(el.RenderY), float32(el.RenderW), float32(el.RenderH))
		isHovering := rl.CheckCollisionPointRec(mousePos, bounds)

		if isHovering {
			cursor = rl.MouseCursorPointingHand // Change cursor if hovering interactive element
		}

		// --- Primary Event Check: Mouse Click ---
		if isHovering && mouseClicked && !clickedElementFound {
			eventHandledByCustom := false

			// 1. Check for Custom Component Handler first
			componentIdentifier, foundName := getCustomPropertyValue(el, componentNameConventionKey, r.docRef)
			if foundName && componentIdentifier != "" {
				if handler, foundHandler := r.customHandlers[componentIdentifier]; foundHandler {
					// Attempt to call HandleEvent if the handler implements it
					if eventer, ok := handler.(interface {
						HandleEvent(el *render.RenderElement, eventType krb.EventType) (bool, error)
					}); ok {
						handled, err := eventer.HandleEvent(el, krb.EventTypeClick) // Pass Click event type
						if err != nil {
							log.Printf("ERROR: Custom click handler '%s' [Elem %d] error: %v", componentIdentifier, el.OriginalIndex, err)
						}
						if handled {
							eventHandledByCustom = true
							clickedElementFound = true // Mark click as handled
						}
					}
				}
			}

			// 2. Check for Standard KRB Event Handlers (if not handled by custom)
			if !eventHandledByCustom && len(el.EventHandlers) > 0 {
				for _, eventInfo := range el.EventHandlers {
					if eventInfo.EventType == krb.EventTypeClick {
						// Look up and execute the registered Go function
						handlerFunc, found := r.eventHandlerMap[eventInfo.HandlerName]
						if found {
							handlerFunc()
							clickedElementFound = true // Mark click as handled
						} else {
							log.Printf("Warn: Standard click handler named '%s' (for Elem %d) not registered.", eventInfo.HandlerName, el.OriginalIndex)
						}
						goto StopProcessingElementEvents // Click handled, move to next element
					}
					// Add checks for other standard event types here if needed
				}
			}
		} // End Mouse Click Check

		StopProcessingElementEvents:

		// Optimization: If hovering, stop checking elements below
		if isHovering {
			break
		}

	} // End loop through elements

	rl.SetMouseCursor(cursor) // Set final cursor style
}

// --- Helper Functions ---

// buildElementTree links elements based on KRB child references.
func buildElementTree(doc *krb.Document, elements []render.RenderElement, roots *[]*render.RenderElement, elementStartOffsets []uint32) error {
	if len(elements) != len(elementStartOffsets) {
		return fmt.Errorf("buildElementTree: mismatch element count (%d) vs start offsets (%d)", len(elements), len(elementStartOffsets))
	}

	// Phase 1: Reset links and build offset lookup map
	offsetToIndex := make(map[uint32]int, len(elements))
	for i := range elements {
		elements[i].Parent = nil
		elements[i].Children = nil
		if i < len(elementStartOffsets) {
			offsetToIndex[elementStartOffsets[i]] = i
		} else {
			log.Printf("Error: buildElementTree: Index %d out of bounds for offsets (len %d)", i, len(elementStartOffsets))
		}
	}
	log.Printf("Debug buildElementTree: Built offset map with %d entries.", len(offsetToIndex))

	// Phase 2: Link elements based on ChildRef offsets
	linkErrors := 0
	for parentIndex := 0; parentIndex < len(elements); parentIndex++ {
		parentEl := &elements[parentIndex]
		parentOffset := elementStartOffsets[parentIndex]
		if doc.ChildRefs != nil && parentIndex < len(doc.ChildRefs) && doc.ChildRefs[parentIndex] != nil {
			for _, childRef := range doc.ChildRefs[parentIndex] {
				childHeaderOffset := parentOffset + uint32(childRef.ChildOffset)
				childIndex, found := offsetToIndex[childHeaderOffset]

				// Validation
				if !found { log.Printf("Error: buildElementTree: Parent Elem %d (Off %d) refs child at %d, not found. Skipping.", parentIndex, parentOffset, childHeaderOffset); linkErrors++; continue }
				if childIndex == parentIndex { log.Printf("Error: buildElementTree: Parent Elem %d refs self. Skipping.", parentIndex); linkErrors++; continue }
				if childIndex < 0 || childIndex >= len(elements) { log.Printf("Error: buildElementTree: Invalid child index %d for parent %d. Skipping.", childIndex, parentIndex); linkErrors++; continue }

				// Perform Linking
				childEl := &elements[childIndex]
				if childEl.Parent != nil { log.Printf("Warn: buildElementTree: Child Elem %d already has Parent Elem %d. Overwriting with Parent Elem %d.", childIndex, childEl.Parent.OriginalIndex, parentIndex) }
				childEl.Parent = parentEl
				parentEl.Children = append(parentEl.Children, childEl)
			}
		}
	}
	if linkErrors > 0 { log.Printf("Warn: buildElementTree: Encountered %d linking errors.", linkErrors) }

	// Phase 3: Identify root elements
	*roots = (*roots)[:0] // Clear efficiently
	foundRoots := 0
	for i := range elements {
		if elements[i].Parent == nil {
			*roots = append(*roots, &elements[i])
			foundRoots++
		}
	}
	log.Printf("Debug buildElementTree: Linking complete. Found %d root elements.", foundRoots)
	if len(elements) > 0 && foundRoots == 0 { log.Printf("Error: buildElementTree: No root elements identified. Potential cycle.") }

	return nil
}

// applyStylePropertiesToWindowDefaults applies BG color from style to window defaults.
func applyStylePropertiesToWindowDefaults(props []krb.Property, doc *krb.Document, defaultBg *rl.Color) {
	for _, prop := range props {
		if prop.ID == krb.PropIDBgColor { // Use prop.ID
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBg = c
			}
		}
		// Add other window-level style properties if needed later.
	}
}


func applyStylePropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	if doc == nil || el == nil {
		return
	}

	for _, prop := range props {
		switch prop.ID { // Use prop.ID
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
			}
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				// Basic validation could be added: if align <= krb.LayoutAlignmentSpaceBtn { ... }
				el.TextAlignment = align
			}
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
		case krb.PropIDOpacity:
			if opacity, ok := getByteValue(&prop); ok {
				// TODO: Decide how style opacity interacts with color alpha.
				// Currently ignored, assuming alpha is handled by color values.
				_ = opacity // Placeholder to avoid unused variable error
				// Example: Modify alpha directly if needed:
				// el.BgColor.A = opacity
				// el.FgColor.A = opacity
			}
		// Add other styleable properties here (FontSize, Padding, etc.) as needed
		// case krb.PropIDFontSize:
		//	 if fs, ok := getShortValue(&prop); ok { el.FontSize = fs } // Assuming RenderElement has FontSize
		// case krb.PropIDPadding:
		//	 if edges, ok := getEdgeInsetsValue(&prop); ok { el.Padding = edges } // Assuming RenderElement has Padding

		}
	}
}

// applyDirectVisualPropertiesToAppElement applies direct visual props to App element.
func applyDirectVisualPropertiesToAppElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BgColor = c }
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok { el.IsVisible = (vis != 0) }
		// Add other visual overrides if App element itself should be drawn differently
		}
	}
}

// applyDirectPropertiesToElement applies direct KRB properties.
func applyDirectPropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BgColor = c }
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.FgColor = c }
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BorderColor = c }
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok { el.BorderWidths = [4]uint8{bw, bw, bw, bw} } else if edges, ok := getEdgeInsetsValue(&prop); ok { el.BorderWidths = edges }
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok { el.TextAlignment = align }
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok { el.IsVisible = (vis != 0) }
		case krb.PropIDTextContent:
			if strIdx, ok := getByteValue(&prop); ok { if doc != nil && int(strIdx) < len(doc.Strings) { el.Text = doc.Strings[strIdx] } else { log.Printf("Warn: Elem %d invalid text content str index %d", el.OriginalIndex, strIdx) } }
		case krb.PropIDImageSource:
			if resIdx, ok := getByteValue(&prop); ok { el.ResourceIndex = resIdx }
		case krb.PropIDWindowWidth, krb.PropIDWindowHeight, krb.PropIDWindowTitle, krb.PropIDResizable, krb.PropIDScaleFactor, krb.PropIDIcon, krb.PropIDVersion, krb.PropIDAuthor:
			continue // Ignore app config props on non-app elements
		}
	}
}

// applyDirectPropertiesToConfig applies App properties to window config.
func applyDirectPropertiesToConfig(props []krb.Property, doc *krb.Document, config *render.WindowConfig) {
	if config == nil { return }
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDWindowWidth:
			if w, ok := getShortValue(&prop); ok && w > 0 { config.Width = int(w) }
		case krb.PropIDWindowHeight:
			if h, ok := getShortValue(&prop); ok && h > 0 { config.Height = int(h) }
		case krb.PropIDWindowTitle:
			if s, ok := getStringValue(&prop, doc); ok { config.Title = s }
		case krb.PropIDResizable:
			if r, ok := getByteValue(&prop); ok { config.Resizable = (r != 0) }
		case krb.PropIDScaleFactor:
			if sf, ok := getFixedPointValue(&prop); ok && sf > 0 { config.ScaleFactor = sf }
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { config.DefaultBg = c }
		}
	}
}

// resolveElementText determines final text content.
func resolveElementText(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeText && el.Header.Type != krb.ElemTypeButton { return }
	resolvedText := ""; foundTextProp := false
	if len(doc.Properties) > el.OriginalIndex {
		for _, prop := range doc.Properties[el.OriginalIndex] {
			if prop.ID == krb.PropIDTextContent {
				if s, ok := getStringValue(&prop, doc); ok { resolvedText = s; foundTextProp = true; break }
			}
		}
	}
	if !foundTextProp && styleOk {
		if prop, ok := getStylePropertyValue(style, krb.PropIDTextContent); ok {
			if s, valOk := getStringValue(prop, doc); valOk { resolvedText = s }
		}
	}
	el.Text = resolvedText
}

// resolveElementImageSource determines final image resource index.
func resolveElementImageSource(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeImage && el.Header.Type != krb.ElemTypeButton { return }
	resolvedResIdx := uint8(render.InvalidResourceIndex); foundResProp := false
	if len(doc.Properties) > el.OriginalIndex {
		for _, prop := range doc.Properties[el.OriginalIndex] {
			if prop.ID == krb.PropIDImageSource {
				if idx, ok := getByteValue(&prop); ok { resolvedResIdx = idx; foundResProp = true; break }
			}
		}
	}
	if !foundResProp && styleOk {
		if prop, ok := getStylePropertyValue(style, krb.PropIDImageSource); ok {
			if idx, valOk := getByteValue(prop); valOk { resolvedResIdx = idx }
		}
	}
	el.ResourceIndex = resolvedResIdx
}

// resolveEventHandlers resolves event callback names.
func resolveEventHandlers(doc *krb.Document, el *render.RenderElement) {
	el.EventHandlers = nil
	if doc.Events != nil && el.OriginalIndex < len(doc.Events) && doc.Events[el.OriginalIndex] != nil {
		krbEvents := doc.Events[el.OriginalIndex]
		if len(krbEvents) > 0 {
			el.EventHandlers = make([]render.EventCallbackInfo, 0, len(krbEvents))
		}
		for _, krbEvent := range krbEvents {
			if int(krbEvent.CallbackID) < len(doc.Strings) {
				el.EventHandlers = append(el.EventHandlers, render.EventCallbackInfo{
					EventType:   krbEvent.EventType,
					HandlerName: doc.Strings[krbEvent.CallbackID],
				})
			} else {
				log.Printf("Warn: Elem %d has invalid event callback string index %d", el.OriginalIndex, krbEvent.CallbackID)
			}
		}
	}
}

// LoadAllTextures loads all required textures identified during PrepareTree.
// This MUST be called AFTER rl.InitWindow() has successfully completed.
func (r *RaylibRenderer) LoadAllTextures() error {
	if r.docRef == nil {
		return fmt.Errorf("cannot load textures, KRB document reference is nil (PrepareTree not called or failed?)")
	}
	if !rl.IsWindowReady() {
		// This is a critical check before attempting any GPU operations
		return fmt.Errorf("cannot load textures, Raylib window is not ready")
	}

	log.Println("LoadAllTextures: Starting texture loading...")
	errCount := 0 // Optional: Count errors instead of returning on first one

	// Call the actual loading logic
	r.performTextureLoading(r.docRef, &errCount) // Pass pointer to error counter

	log.Printf("LoadAllTextures: Texture loading complete. Encountered %d errors.", errCount)
	if errCount > 0 {
		// Return an error if any texture failed to load, or just log the warning
		return fmt.Errorf("encountered %d errors during texture loading", errCount)
	}
	return nil
}

func (r *RaylibRenderer) performTextureLoading(doc *krb.Document, errorCounter *int) {
	if doc == nil {
		log.Println("Error(performTextureLoading): doc is nil")
		*errorCounter++
		return
	}
	if r.elements == nil {
		log.Println("Error(performTextureLoading): r.elements is nil")
		*errorCounter++
		return
	}

	for i := range r.elements {
		el := &r.elements[i] // Use pointer directly

		// Skip if not an image/button or no valid resource index
		needsTexture := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) &&
			el.ResourceIndex != render.InvalidResourceIndex
		if !needsTexture {
			continue
		}

		resIndex := el.ResourceIndex

		// Validate resource index against document resources
		if int(resIndex) >= len(doc.Resources) {
			log.Printf("Error: Elem %d has invalid resource index %d (max %d)", el.OriginalIndex, resIndex, len(doc.Resources)-1)
			*errorCounter++
			el.TextureLoaded = false // Ensure it's marked as not loaded
			continue
		}
		res := doc.Resources[resIndex]

		// Check cache first
		if loadedTex, exists := r.loadedTextures[resIndex]; exists {
			el.Texture = loadedTex
			el.TextureLoaded = rl.IsTextureReady(loadedTex)
			if !el.TextureLoaded {
				log.Printf("Warn: Cached texture for Res %d (Elem %d) is no longer ready.", resIndex, el.OriginalIndex)
				*errorCounter++
			} else {
				// <<<--- ADDED CACHE HIT LOG ---<<<
				log.Printf("DEBUG [Elem %d]: Using cached texture Res %d (ID: %d)", el.OriginalIndex, resIndex, loadedTex.ID)
			}
			continue // Move to next element
		}

		// --- Texture not in cache, attempt loading ---
		var texture rl.Texture2D
		loadedOk := false

		if res.Format == krb.ResFormatExternal {
			// Validate name index
			if int(res.NameIndex) >= len(doc.Strings) {
				log.Printf("Error: External Resource %d (Elem %d) has invalid name index %d", resIndex, el.OriginalIndex, res.NameIndex)
				*errorCounter++
				el.TextureLoaded = false
				continue
			}
			resourceName := doc.Strings[res.NameIndex]
			fullPath := filepath.Join(r.krbFileDir, resourceName)
			log.Printf("INFO: Attempting to load external texture: %s (for Res %d, Elem %d)", fullPath, resIndex, el.OriginalIndex)

			// Check file existence first
			if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
				log.Printf("Error: Texture file not found: %s (for Res %d, Elem %d)", fullPath, resIndex, el.OriginalIndex)
				*errorCounter++
				el.TextureLoaded = false
			} else {
				// Load image data from file to CPU memory
				img := rl.LoadImage(fullPath)
				// <<<--- ADDED LOG ---<<<
				log.Printf("DEBUG [Elem %d]: rl.LoadImage returned. IsImageReady(img): %t", el.OriginalIndex, rl.IsImageReady(img)) // Log LoadImage success

				if rl.IsImageReady(img) {
					// --- GPU Upload Attempt ---
					if !rl.IsWindowReady() {
						log.Printf("CRITICAL ERROR: Window became not ready before LoadTextureFromImage for %s", fullPath)
						*errorCounter++
						el.TextureLoaded = false
					} else {
						// <<<--- ADDED LOG ---<<<
						log.Printf("DEBUG [Elem %d]: Calling LoadTextureFromImage for %s...", el.OriginalIndex, fullPath)
						texture = rl.LoadTextureFromImage(img)
						// <<<--- ADDED LOG ---<<<
						log.Printf("DEBUG [Elem %d]: Returned from LoadTextureFromImage. Texture ID: %d", el.OriginalIndex, texture.ID) // Log the ID

						isReady := rl.IsTextureReady(texture)
						// <<<--- ADDED LOG ---<<<
						log.Printf("DEBUG [Elem %d]: rl.IsTextureReady(texture) returned: %t", el.OriginalIndex, isReady) // Log readiness

						if isReady {
							loadedOk = true
							// log.Printf("  OK: Loaded external texture Res %d ('%s') -> ID:%d", resIndex, resourceName, texture.ID) // Keep OK log minimal for now
						} else {
							log.Printf("Error: Failed IsTextureReady check for texture Res %d (Elem %d) after LoadTextureFromImage.", resIndex, el.OriginalIndex)
							*errorCounter++
							// el.TextureLoaded = false // This will be set later based on loadedOk
						}
					}
					// --- End GPU Upload Attempt ---
					rl.UnloadImage(img)
				} else {
					log.Printf("Error: Failed LoadImage (CPU) for %s (Res %d, Elem %d). Cannot create texture.", fullPath, resIndex, el.OriginalIndex)
					*errorCounter++
					// el.TextureLoaded = false // Set later
				}
			}
		} else if res.Format == krb.ResFormatInline {
			// Handling for inline data
			if res.InlineData != nil && res.InlineDataSize > 0 {
				ext := ".png"; if int(res.NameIndex) < len(doc.Strings) { nameHint := doc.Strings[res.NameIndex]; if nameExt := filepath.Ext(nameHint); nameExt != "" { ext = nameExt } }
				log.Printf("INFO: Attempting to load inline texture Res %d (Elem %d) (hint: %s, size: %d bytes)", resIndex, el.OriginalIndex, ext, len(res.InlineData))

				img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData)))
				// <<<--- ADDED LOG ---<<<
				log.Printf("DEBUG [Elem %d]: rl.LoadImageFromMemory returned. IsImageReady(img): %t", el.OriginalIndex, rl.IsImageReady(img)) // Log LoadImage success

				if rl.IsImageReady(img) {
					if !rl.IsWindowReady() {
						log.Printf("CRITICAL ERROR: Window became not ready before LoadTextureFromImage for inline Res %d", resIndex)
						*errorCounter++
						el.TextureLoaded = false
					} else {
						// <<<--- ADDED LOG ---<<<
						log.Printf("DEBUG [Elem %d]: Calling LoadTextureFromImage for inline Res %d...", el.OriginalIndex, resIndex)
						texture = rl.LoadTextureFromImage(img)
						// <<<--- ADDED LOG ---<<<
						log.Printf("DEBUG [Elem %d]: Returned from LoadTextureFromImage. Texture ID: %d", el.OriginalIndex, texture.ID) // Log the ID

						isReady := rl.IsTextureReady(texture)
						// <<<--- ADDED LOG ---<<<
						log.Printf("DEBUG [Elem %d]: rl.IsTextureReady(texture) returned: %t", el.OriginalIndex, isReady) // Log readiness

						if isReady {
							loadedOk = true
							// log.Printf("  OK: Loaded inline texture Res %d -> ID:%d (hint: %s)", resIndex, texture.ID, ext)
						} else {
							log.Printf("Error: Failed IsTextureReady check for inline texture Res %d (Elem %d) after LoadTextureFromImage.", resIndex, el.OriginalIndex)
							*errorCounter++
							// el.TextureLoaded = false // Set later
						}
					}
					rl.UnloadImage(img)
				} else {
					log.Printf("Error: Failed LoadImageFromMemory (CPU) for inline Res %d (Elem %d) (hint: %s)", resIndex, el.OriginalIndex, ext)
					*errorCounter++
					// el.TextureLoaded = false // Set later
				}
			} else {
				log.Printf("Error: Inline Resource %d (Elem %d) has no data or zero size", resIndex, el.OriginalIndex)
				*errorCounter++
				// el.TextureLoaded = false // Set later
			}
		} else {
			log.Printf("Warn: Unknown resource format %d for Res %d (Elem %d)", res.Format, resIndex, el.OriginalIndex)
			*errorCounter++
			// el.TextureLoaded = false // Set later
		}

		// Store texture in element and cache if loaded successfully
		if loadedOk { // Check the flag we set based on IsTextureReady
			// <<<--- ADDED LOG ---<<<
			log.Printf("DEBUG [Elem %d]: Setting TextureLoaded=true and caching Res %d (ID: %d)", el.OriginalIndex, resIndex, texture.ID)
			el.Texture = texture
			el.TextureLoaded = true
			r.loadedTextures[resIndex] = texture // Add to cache
		} else {
			// <<<--- ADDED LOG ---<<<
			log.Printf("DEBUG [Elem %d]: Setting TextureLoaded=false for Res %d because loadedOk is false.", el.OriginalIndex, resIndex)
			el.TextureLoaded = false
		}
	} // End loop through elements
} // End of performTextureLoading function

// hasStyleSize checks if style includes size properties.
func hasStyleSize(doc *krb.Document, el *render.RenderElement, propIDMax, propIDNormal krb.PropertyID) bool {
	if style, ok := findStyle(doc, el.Header.StyleID); ok {
		if _, maxOk := getStylePropertyValue(style, propIDMax); maxOk { return true }
		// Optionally check for normal size prop if needed
	}
	return false
}

// findStyle gets style by 1-based ID.
func findStyle(doc *krb.Document, styleID uint8) (*krb.Style, bool) {
	if styleID == 0 || int(styleID) > len(doc.Styles) { return nil, false }
	return &doc.Styles[styleID-1], true
}

// getStylePropertyValue gets a property from a resolved style.
func getStylePropertyValue(style *krb.Style, propID krb.PropertyID) (*krb.Property, bool) {
	if style == nil { return nil, false }
	for i := range style.Properties { if style.Properties[i].ID == propID { return &style.Properties[i], true } }
	return nil, false
}

func findStyleIDByNameIndex(doc *krb.Document, nameIndex uint8) uint8 {
	if doc == nil || nameIndex == 0 {
		return 0
	}
	// KRB Style Blocks are 1-based indexed in the header, but stored 0-based in the slice.
	// The krb.Style struct holds the 1-based ID.
	for i := range doc.Styles {
		// Compare the NameIndex field of the Style Block Header
		if doc.Styles[i].NameIndex == nameIndex {
			return doc.Styles[i].ID // Return the 1-based ID
		}
	}
	return 0 // Not found
}


func getStyleColors(doc *krb.Document, styleID uint8, flags uint16) (bg rl.Color, fg rl.Color, ok bool) {
	if doc == nil || styleID == 0 {
		return rl.Blank, rl.White, false // Cannot look up style 0 or without doc
	}
	// Adjust ID to be 0-based index for the slice
	styleIndex := int(styleID - 1)
	if styleIndex < 0 || styleIndex >= len(doc.Styles) {
		log.Printf("WARN getStyleColors: Invalid style index %d (derived from ID %d)", styleIndex, styleID)
		return rl.Blank, rl.White, false // Invalid index
	}

	style := &doc.Styles[styleIndex]
	// Default colors if not found in style
	bg = rl.Blank // Default to transparent background
	fg = rl.White // Default to white foreground

	foundBg := false
	foundFg := false

	// Iterate through the resolved properties of the target style
	for _, prop := range style.Properties {
		// *** MODIFIED: Use prop.ID instead of prop.PropertyID ***
		if prop.ID == krb.PropIDBgColor {
			if c, propOk := getColorValue(&prop, flags); propOk {
				bg = c
				foundBg = true
			}
		// *** MODIFIED: Use prop.ID instead of prop.PropertyID ***
		} else if prop.ID == krb.PropIDFgColor {
			if c, propOk := getColorValue(&prop, flags); propOk {
				fg = c
				foundFg = true
			}
		}
		if foundBg && foundFg { break } // Optimization
	}
	ok = true // Consider successful even if defaults used
	return bg, fg, ok
}

// --- Value Parsing Helper Functions ---
func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {
	if prop == nil || prop.ValueType != krb.ValTypeColor { return rl.Color{}, false }
	useExtended := (flags&krb.FlagExtendedColor) != 0
	if useExtended { if len(prop.Value) == 4 { return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true }
	} else { if len(prop.Value) == 1 { log.Printf("Warn: Palette color index %d used, but palettes not implemented.", prop.Value[0]); return rl.Magenta, true } }
	log.Printf("Warn: Invalid color data size for prop ID %d.", prop.ID)
	return rl.Color{}, false
}
func getByteValue(prop *krb.Property) (uint8, bool) {
	if prop != nil && (prop.ValueType == krb.ValTypeByte || prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource || prop.ValueType == krb.ValTypeEnum) && len(prop.Value) == 1 { return prop.Value[0], true }
	return 0, false
}
func getShortValue(prop *krb.Property) (uint16, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 { return binary.LittleEndian.Uint16(prop.Value), true }
	return 0, false
}
func getStringValue(prop *krb.Property, doc *krb.Document) (string, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeString && len(prop.Value) == 1 {
		idx := prop.Value[0]
		if doc != nil && int(idx) < len(doc.Strings) { return doc.Strings[idx], true } else { log.Printf("Warn: Invalid string index %d for prop %d.", idx, prop.ID) }
	}
	return "", false
}
func getFixedPointValue(prop *krb.Property) (float32, bool) {
	if prop != nil && prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 { return float32(binary.LittleEndian.Uint16(prop.Value)) / 256.0, true }
	return 0, false
}
func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 { return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true }
	return [4]uint8{}, false
}

// --- Generic Math/Slice Helpers ---
func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }
func MuxInt(cond bool, a, b int) int { if cond { return a }; return b }
func ReverseSliceInt(s []int) { for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 { s[i], s[j] = s[j], s[i] } }

// --- Drawing Helper Functions ---
func isEffectivelyAbsolute(el *render.RenderElement) bool {
	if el == nil { return false }
	return el.Header.LayoutAbsolute() || el.Header.PosX != 0 || el.Header.PosY != 0
}

func calculateBaseSize(el *render.RenderElement, scale float32, doc *krb.Document) (baseW int, baseH int) {
	scaled := func(v uint16) int { return int(math.Round(float64(v) * float64(scale))) }
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

	if el.Header.Width > 0 { baseW = scaled(el.Header.Width) }
	if el.Header.Height > 0 { baseH = scaled(el.Header.Height) }

	if baseW == 0 || baseH == 0 {
		if style, ok := findStyle(doc, el.Header.StyleID); ok {
			if baseH == 0 { if prop, ok := getStylePropertyValue(style, krb.PropIDMaxHeight); ok { if h, valOk := getShortValue(prop); valOk { baseH = scaled(h) } } }
			if baseW == 0 { if prop, ok := getStylePropertyValue(style, krb.PropIDMaxWidth); ok { if w, valOk := getShortValue(prop); valOk { baseW = scaled(w) } } }
		}
	}

	if baseW == 0 || baseH == 0 {
		intrinsicW, intrinsicH := 0, 0
		if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
			fontSize := int32(math.Max(1, math.Round(baseFontSize*float64(scale))))
			textWidthMeasured := rl.MeasureText(el.Text, fontSize)
			hPadding := scaledU8(el.BorderWidths[1]) + scaledU8(el.BorderWidths[3])
			vPadding := scaledU8(el.BorderWidths[0]) + scaledU8(el.BorderWidths[2])
			intrinsicW = int(textWidthMeasured) + hPadding
			intrinsicH = int(fontSize) + vPadding
		} else if el.Header.Type == krb.ElemTypeImage && el.TextureLoaded {
			intrinsicW = int(math.Round(float64(el.Texture.Width) * float64(scale)))
			intrinsicH = int(math.Round(float64(el.Texture.Height) * float64(scale)))
		}
		if baseW == 0 { baseW = intrinsicW }
		if baseH == 0 { baseH = intrinsicH }
	}
	return baseW, baseH
}

func fitToChildren(el *render.RenderElement, clientAbsX, clientAbsY, borderL, borderT, borderR, borderB int, shouldAutoSizeW, shouldAutoSizeH bool) {
	maxChildRelXExtent := 0; maxChildRelYExtent := 0; hasFlowChildren := false
	for _, child := range el.Children {
		if !isEffectivelyAbsolute(child) {
			hasFlowChildren = true
			relX := child.RenderX - clientAbsX; relY := child.RenderY - clientAbsY
			xExtent := relX + child.RenderW; yExtent := relY + child.RenderH
			if xExtent > maxChildRelXExtent { maxChildRelXExtent = xExtent }
			if yExtent > maxChildRelYExtent { maxChildRelYExtent = yExtent }
		}
	}
	if hasFlowChildren {
		newW := maxChildRelXExtent + borderL + borderR; newH := maxChildRelYExtent + borderT + borderB
		if shouldAutoSizeW { el.RenderW = max(1, newW) }
		if shouldAutoSizeH { el.RenderH = max(1, newH) }
	}
}

func calculateAlignmentOffsets(alignment uint8, availableSpace, totalUsedSpace, childCount int, isReversed bool) (startOffset, spacing int) {
	unusedSpace := max(0, availableSpace-totalUsedSpace)
	startOffset = 0; spacing = 0
	switch alignment {
	case krb.LayoutAlignCenter:       startOffset = unusedSpace / 2
	case krb.LayoutAlignEnd:          startOffset = MuxInt(isReversed, 0, unusedSpace)
	case krb.LayoutAlignSpaceBetween: if childCount > 1 { spacing = unusedSpace / (childCount - 1) }; startOffset = 0
	default:                          startOffset = MuxInt(isReversed, unusedSpace, 0)
	}
	return startOffset, spacing
}

func calculateCrossAxisOffset(alignment uint8, crossAxisSize, childCrossSize int) int {
	switch alignment {
	case krb.LayoutAlignCenter: return (crossAxisSize - childCrossSize) / 2
	case krb.LayoutAlignEnd:    return crossAxisSize - childCrossSize
	default:                    return 0
	}
}

func clampOpposingBorders(borderA, borderB, totalSize int) (int, int) {
	if borderA+borderB >= totalSize { borderA = max(0, min(totalSize/2, borderA)); borderB = max(0, totalSize-borderA) }
	return borderA, borderB
}

func drawBorders(x, y, w, h, top, right, bottom, left int, color rl.Color) {
	if top > 0 { rl.DrawRectangle(int32(x), int32(y), int32(w), int32(top), color) }
	if bottom > 0 { rl.DrawRectangle(int32(x), int32(y+h-bottom), int32(w), int32(bottom), color) }
	sideY := y + top; sideH := max(0, h-top-bottom)
	if left > 0 { rl.DrawRectangle(int32(x), int32(sideY), int32(left), int32(sideH), color) }
	if right > 0 { rl.DrawRectangle(int32(x+w-right), int32(sideY), int32(right), int32(sideH), color) }
}

func calculateContentArea(x, y, w, h, top, right, bottom, left int) (cx, cy, cw, ch int) {
	cx = x + left; cy = y + top
	cw = max(0, w-left-right)
	ch = max(0, h-top-bottom)
	return cx, cy, cw, ch
}

// drawContent renders the specific content (text or image) for an element
// using the provided *effective* foreground color determined by the caller.

func (r *RaylibRenderer) drawContent(el *render.RenderElement, cx, cy, cw, ch int, scale float32, effectiveFgColor rl.Color) {

	// Draw Text if applicable
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		fontSize := int32(math.Max(1, math.Round(baseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		textHeightMeasured := fontSize
		textDrawX := cx
		textDrawY := cy + (ch-int(textHeightMeasured))/2

		switch el.TextAlignment {
		case krb.LayoutAlignCenter: textDrawX = cx + (cw-int(textWidthMeasured))/2
		case krb.LayoutAlignEnd: textDrawX = cx + cw - int(textWidthMeasured)
		}
		rl.DrawText(el.Text, int32(textDrawX), int32(textDrawY), fontSize, effectiveFgColor)
	}

	// Draw Image if applicable
	isImageElement := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton)
	if isImageElement && el.TextureLoaded {
		if rl.IsTextureReady(el.Texture) {
			texWidth := float32(el.Texture.Width)
			texHeight := float32(el.Texture.Height)
			sourceRec := rl.NewRectangle(0, 0, texWidth, texHeight)
			destRec := rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch))
			origin := rl.NewVector2(0, 0)
			tintColor := rl.White

			if destRec.Width > 0 && destRec.Height > 0 && sourceRec.Width > 0 && sourceRec.Height > 0 {
				rl.DrawTexturePro(el.Texture, sourceRec, destRec, origin, 0.0, tintColor)
			}
		}
	}
}


func (r *RaylibRenderer) RegisterEventHandler(name string, handler func()) {
	if name == "" || handler == nil {
		log.Printf("WARN RENDERER: Invalid event handler registration attempt (name: '%s', handler nil: %t)", name, handler == nil)
		return
	}
	if r.eventHandlerMap == nil { r.eventHandlerMap = make(map[string]func()) } // Defensive init
	if _, exists := r.eventHandlerMap[name]; exists { log.Printf("INFO: Overwriting standard event handler for callback name '%s'", name) }
	r.eventHandlerMap[name] = handler
	log.Printf("Registered standard event handler for callback name '%s'", name)
}

// RegisterCustomComponent: Uses instance map `r.customHandlers`.
func (r *RaylibRenderer) RegisterCustomComponent(identifier string, handler render.CustomComponentHandler) error {
	if identifier == "" || handler == nil {
		return fmt.Errorf("invalid custom component registration (identifier: '%s', handler nil: %t)", identifier, handler == nil)
	}
	if r.customHandlers == nil { r.customHandlers = make(map[string]render.CustomComponentHandler) } // Defensive init
	if _, exists := r.customHandlers[identifier]; exists { log.Printf("INFO: Overwriting custom component handler for identifier '%s'", identifier) }
	r.customHandlers[identifier] = handler
	log.Printf("Registered custom component handler for '%s'", identifier)
	return nil
}

func (r *RaylibRenderer) renderElementRecursiveWithCustomDraw(el *render.RenderElement, scale float32) {
	if el == nil || !el.IsVisible { return }

	skipStandardDraw := false
	var drawErr error

	// Check for Custom Draw Handler using component name
	componentIdentifier, foundName := getCustomPropertyValue(el, componentNameConventionKey, r.docRef)
	if foundName && componentIdentifier != "" {
		if handler, foundHandler := r.customHandlers[componentIdentifier]; foundHandler {
			if drawer, ok := handler.(interface { Draw(el *render.RenderElement, scale float32, rendererInstance render.Renderer) (bool, error) }); ok {
				skipStandardDraw, drawErr = drawer.Draw(el, scale, r)
				if drawErr != nil { log.Printf("ERROR: Custom Draw handler for '%s' [Elem %d] failed: %v", componentIdentifier, el.OriginalIndex, drawErr) }
			}
		}
	}

	// Perform Standard Drawing (if not skipped)
	if !skipStandardDraw {
		r.renderElementRecursive(el, scale) // <<< CALL THE METHOD VERSION
	}

	// Recursively Draw Children (using this function to allow custom child drawing)
	for _, child := range el.Children {
		r.renderElementRecursiveWithCustomDraw(child, scale)
	}
}

// ApplyCustomComponentLayoutAdjustments uses instance map `r.customHandlers` and identifies components via `_componentName`.
func (r *RaylibRenderer) ApplyCustomComponentLayoutAdjustments(elements []*render.RenderElement, doc *krb.Document) { // <<< Changed to method
    if doc == nil || len(r.customHandlers) == 0 { // Access instance map 'r.customHandlers'.
        return // Skip if no doc or no handlers registered.
    }

    for _, el := range elements {
        if el == nil { continue }

        // Explicit Component Identification using the convention key.
        // Assumes getCustomPropertyValue exists (likely in custom_component_registry.go or utils)
        componentIdentifier, found := getCustomPropertyValue(el, componentNameConventionKey, doc)

        if found && componentIdentifier != "" {
            // Look up handler in the instance map.
            handler, handlerFound := r.customHandlers[componentIdentifier] // Access instance map.

            if handlerFound {
                // Call the handler's layout adjustment method.
                err := handler.HandleLayoutAdjustment(el, doc)
                if err != nil {
                    log.Printf("ERROR: Custom layout handler for '%s' [Elem %d] failed: %v", componentIdentifier, el.OriginalIndex, err)
                }
            }
        }
    }
}