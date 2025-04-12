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
	"github.com/waozixyz/kryon/impl/go/krb"    // Adjust import path as needed
	"github.com/waozixyz/kryon/impl/go/render" // Adjust import path as needed
)

const baseFontSize = 18.0 // Base font size for text rendering

// RaylibRenderer implements the render.Renderer interface using raylib-go.
type RaylibRenderer struct {
	config          render.WindowConfig
	elements        []render.RenderElement // Flat list for easy iteration/lookup by OriginalIndex
	roots           []*render.RenderElement // Top-level elements in the hierarchy
	loadedTextures  map[uint8]rl.Texture2D // Cache loaded textures by resource index
	krbFileDir      string                 // Base directory for loading external resources
	scaleFactor     float32                // Effective UI scale factor
	docRef          *krb.Document          // Reference to the parsed KRB document
	eventHandlerMap map[string]func()      // Map callback names to Go functions
}

// NewRaylibRenderer creates a new Raylib renderer instance.
func NewRaylibRenderer() *RaylibRenderer {
	return &RaylibRenderer{
		loadedTextures:  make(map[uint8]rl.Texture2D),
		scaleFactor:     1.0,
		eventHandlerMap: make(map[string]func()),
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

	// --- Pass 4: Load Textures ---
	// Load image resources referenced by elements
	log.Println("PrepareTree: Loading textures...")
	loadTextures(r, doc)
	log.Println("PrepareTree: Texture loading complete.")

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
	log.Printf("DEBUG RenderFrame: == Starting Standard Layout Pass ==")
	for _, root := range roots {
		if root != nil { // Safety check
			PerformLayout(root, 0, 0, currentWidth, currentHeight, r.scaleFactor, r.docRef)
		}
	}
	log.Printf("DEBUG RenderFrame: == Standard Layout Pass Complete ==")

	// --- 2. Custom Component Layout Adjustment Pass ---
	log.Printf("DEBUG RenderFrame: == Starting Custom Adjustment Pass ==")
	ApplyCustomComponentLayoutAdjustments(r.GetRenderTree(), r.docRef) // <<< Ensure this line is ACTIVE
	log.Printf("DEBUG RenderFrame: == Custom Adjustment Pass Complete ==")

	// --- 3. Draw Pass ---
	log.Printf("DEBUG RenderFrame: == Starting Draw Pass ==")
	for _, root := range roots {
		if root != nil { // Safety check
			renderElementRecursive(root, r.scaleFactor)
		}
	}
	log.Printf("DEBUG RenderFrame: == Draw Pass Complete ==")
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
	parentIDStr := "nil"
	if el.Parent != nil {
		parentIDStr = fmt.Sprintf("Elem %d", el.Parent.OriginalIndex)
	}
	log.Printf("DEBUG Layout [%s]: Parent=%s | Available Parent Content Area: %d,%d %dx%d",
		elIDStr, parentIDStr, parentContentX, parentContentY, parentContentW, parentContentH)

	isRoot := (el.Parent == nil)
	scaled := func(v uint16) int { return int(math.Round(float64(v) * float64(scale))) }
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

	// --- 1. Determine Base Size ---
	baseW, baseH := calculateBaseSize(el, scale, doc)
	el.IntrinsicW = baseW
	el.IntrinsicH = baseH
	log.Printf("DEBUG Layout [%s]: Calculated Base Size: %dx%d (Intrinsic)", elIDStr, baseW, baseH)

	// Apply explicit size from KRB header
	explicitW, explicitH := false, false // Define flags HERE to capture header explicitness
	if el.Header.Width > 0 {
		baseW = scaled(el.Header.Width)
		explicitW = true // Set flag
		log.Printf("DEBUG Layout [%s]: Overriding Width with Header.Width %d -> %d", elIDStr, el.Header.Width, baseW)
	}
	if el.Header.Height > 0 {
		baseH = scaled(el.Header.Height)
		explicitH = true // Set flag
		log.Printf("DEBUG Layout [%s]: Overriding Height with Header.Height %d -> %d", elIDStr, el.Header.Height, baseH)
	}

	// --- 2. Determine Initial Render Size ---
	initialW, initialH := baseW, baseH
	isFlowElement := !el.Header.LayoutAbsolute()

	if isRoot {
		initialW, initialH = parentContentW, parentContentH
		log.Printf("DEBUG Layout [%s]: Is Root. Setting Initial Size to Parent Content Area: %dx%d", elIDStr, initialW, initialH)
	} else if isFlowElement {
		clampedW, clampedH := false, false
		// Clamp size to the available space from the parent *if* the element doesn't have an explicit size itself
		if !explicitW && initialW > parentContentW { // Use the flag from Step 1
			initialW = max(0, parentContentW)
			clampedW = true
		}
		if !explicitH && initialH > parentContentH { // Use the flag from Step 1
			initialH = max(0, parentContentH)
			clampedH = true
		}
		if clampedW || clampedH {
			log.Printf("DEBUG Layout [%s]: Is Flow. Maybe Clamped Initial Size by Parent Space: %dx%d (W Clamped: %t, H Clamped: %t)", elIDStr, initialW, initialH, clampedW, clampedH)
		}
	} else {
		log.Printf("DEBUG Layout [%s]: Is Absolute. Initial Size remains Base Size: %dx%d", elIDStr, initialW, initialH)
	}

	el.RenderW = initialW
	el.RenderH = initialH // Store potentially clamped size

	// --- 3. Determine Initial Render Position ---
	el.RenderX = parentContentX
	el.RenderY = parentContentY
	initialX, initialY := el.RenderX, el.RenderY // Store for logging

	if isEffectivelyAbsolute(el) {
		offsetX, offsetY := scaled(el.Header.PosX), scaled(el.Header.PosY)
		if el.Parent != nil {
			el.RenderX = el.Parent.RenderX + offsetX
			el.RenderY = el.Parent.RenderY + offsetY
			log.Printf("DEBUG Layout [%s]: Is Absolute w/ Parent. Pos Offset (%d,%d) relative to Parent (%d,%d) -> (%d,%d)", elIDStr, offsetX, offsetY, el.Parent.RenderX, el.Parent.RenderY, el.RenderX, el.RenderY)
		} else if !isRoot {
			log.Printf("Warn: Absolute Elem %d has no parent, positioning relative to (0,0).", el.OriginalIndex)
			el.RenderX = offsetX
			el.RenderY = offsetY
			log.Printf("DEBUG Layout [%s]: Is Absolute w/o Parent. Pos Offset (%d,%d) relative to (0,0) -> (%d,%d)", elIDStr, offsetX, offsetY, el.RenderX, el.RenderY)
		} else {
			el.RenderX = offsetX
			el.RenderY = offsetY
			log.Printf("DEBUG Layout [%s]: Is Absolute Root. Pos Offset (%d,%d) relative to (0,0) -> (%d,%d)", elIDStr, offsetX, offsetY, el.RenderX, el.RenderY)
		}
	} else {
		log.Printf("DEBUG Layout [%s]: Is Flow. Initial Position set to Parent Content Origin: %d,%d (Final pos set later)", elIDStr, initialX, initialY)
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

		log.Printf("DEBUG Layout [%s]: Determined Child Layout Area: Origin=%d,%d Size=%dx%d (Based on Potential %dx%d from Avail %dx%d/Explicit:%t,%t, Borders T%d R%d B%d L%d)",
			elIDStr, clientAbsX, clientAbsY, clientWidth, clientHeight,
			potentialChildW, potentialChildH, parentContentW, parentContentH, explicitW, explicitH, // Use flags from Step 1
			borderT, borderR, borderB, borderL)

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
	log.Printf("DEBUG Layout [%s]: == Standard Layout Pass Complete == Final Frame: %d,%d %dx%d",
		elIDStr, el.RenderX, el.RenderY, el.RenderW, el.RenderH)

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

		log.Printf("DEBUG LayoutChildren [%s]: Flow Layout Start. ParentLayoutByte:0x%02X (Dir:%d Align:%d Rev:%t Horiz:%t) | MainSpace:%d CrossSize:%d | %d flow children",
			parentIDStr, parent.Header.Layout, // Log the raw layout byte
			direction, alignment, isReversed, mainAxisIsHorizontal, mainAxisAvailableSpace, crossAxisSize, len(flowChildren))

		// --- Pass 1: Calculate Initial Sizes & Fixed Space ---
		totalFixedSizeMainAxis := 0
		growChildrenCount := 0
		for i, child := range flowChildren {
			log.Printf("DEBUG LayoutChildren [%s]: --- Pre-Layout Child %d/%d (Elem %d) ---", parentIDStr, i+1, len(flowChildren), child.OriginalIndex)
			// Call PerformLayout for the child. Note that PerformLayout itself logs the final frame *relative to its parent's content origin* initially.
			PerformLayout(child, parentClientOriginX, parentClientOriginY, availableW, availableH, scale, doc)

			if child.Header.LayoutGrow() {
				growChildrenCount++
			} else {
				fixedSize := MuxInt(mainAxisIsHorizontal, child.RenderW, child.RenderH)
				totalFixedSizeMainAxis += fixedSize
			}
		}
		log.Printf("DEBUG LayoutChildren [%s]: Pass 1 Done. FixedSpace:%d GrowCount:%d", parentIDStr, totalFixedSizeMainAxis, growChildrenCount)

		// --- Calculate & Distribute Growth Space ---
		spaceForGrowth := max(0, mainAxisAvailableSpace-totalFixedSizeMainAxis)
		growSizePerChild := 0
		remainderForLastGrowChild := 0
		// *** ADDED FLAG: Track if growth calculation applies to ALL available space ***
		allSpaceIsGrowth := (totalFixedSizeMainAxis == 0 && growChildrenCount > 0 && spaceForGrowth > 0)

		if growChildrenCount > 0 && spaceForGrowth > 0 {
			growSizePerChild = spaceForGrowth / growChildrenCount
			remainderForLastGrowChild = spaceForGrowth % growChildrenCount
			log.Printf("DEBUG LayoutChildren [%s]: Growth Calculated. Total:%d PerChild:%d Remainder:%d (AllSpaceIsGrowth: %t)", parentIDStr, spaceForGrowth, growSizePerChild, remainderForLastGrowChild, allSpaceIsGrowth) // Log the new flag
		}

		// --- Pass 2: Finalize Sizes ---
		totalFinalFlowSizeMainAxis := 0
		tempGrowCount := 0
		for _, child := range flowChildren {
			isGrowing := child.Header.LayoutGrow()
			childIDStr := fmt.Sprintf("Elem %d", child.OriginalIndex)
			preSizeW, preSizeH := child.RenderW, child.RenderH

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
					log.Printf("DEBUG LayoutChildren [%s Child %s]: SET Size (All Grow). Amount:%d Size %dx%d -> %dx%d", parentIDStr, childIDStr, growAmount, preSizeW, preSizeH, child.RenderW, child.RenderH)
				} else {
					// If only *some* space was for growth (mixed fixed/grow children), ADD the growth amount.
					if mainAxisIsHorizontal {
						child.RenderW += growAmount   // ADD width
						child.RenderH = crossAxisSize // Stretch height
					} else {
						child.RenderH += growAmount   // ADD height
						child.RenderW = crossAxisSize // Stretch width
					}
					log.Printf("DEBUG LayoutChildren [%s Child %s]: ADDED Growth (Mixed Grow). Amount:%d Size %dx%d -> %dx%d", parentIDStr, childIDStr, growAmount, preSizeW, preSizeH, child.RenderW, child.RenderH)
				}
				// --- >>> END CORRECTION <<< ---

			} else { // Not growing
				// Clamp cross axis size if needed (stretch not applicable)
				clampedCross := false
				if mainAxisIsHorizontal {
					if child.RenderH > crossAxisSize {
						child.RenderH = crossAxisSize
						clampedCross = true
					}
				} else {
					if child.RenderW > crossAxisSize {
						child.RenderW = crossAxisSize
						clampedCross = true
					}
				}
				if clampedCross {
					log.Printf("DEBUG LayoutChildren [%s Child %s]: Clamped Cross Axis. Size %dx%d -> %dx%d", parentIDStr, childIDStr, preSizeW, preSizeH, child.RenderW, child.RenderH)
				}
			}

			// Ensure minimum dimensions AFTER applying growth/clamping
			child.RenderW = max(1, child.RenderW)
			child.RenderH = max(1, child.RenderH)
			totalFinalFlowSizeMainAxis += MuxInt(mainAxisIsHorizontal, child.RenderW, child.RenderH)
		}
		log.Printf("DEBUG LayoutChildren [%s]: Pass 2 Done. TotalFinalFlowSize:%d", parentIDStr, totalFinalFlowSizeMainAxis)

		// --- Calculate Alignment Offsets ---
		startOffset, spacing := calculateAlignmentOffsets(alignment, mainAxisAvailableSpace, totalFinalFlowSizeMainAxis, len(flowChildren), isReversed)
		log.Printf("DEBUG LayoutChildren [%s]: Alignment Offsets Calculated. Start:%d Spacing:%d", parentIDStr, startOffset, spacing)

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
			childIDStr := fmt.Sprintf("Elem %d", child.OriginalIndex)
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

			// Log the FINAL absolute coordinates assigned to the child
			log.Printf("DEBUG LayoutChildren [%s Child %s]: ASSIGNED Final Frame: %d,%d %dx%d (From MainAxisPos:%d, CrossOffset:%d within Parent Area starting at %d,%d)",
				parentIDStr, childIDStr,
				child.RenderX, child.RenderY, child.RenderW, child.RenderH, // Use assigned values
				currentMainAxisPos, crossOffset, parentClientOriginX, parentClientOriginY)

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
		log.Printf("DEBUG LayoutChildren [%s]: Laying out %d Absolute Children relative to Parent Frame %d,%d", parentIDStr, len(absoluteChildren), parent.RenderX, parent.RenderY)
		for _, child := range absoluteChildren {
			// Pass parent's top-left corner (RenderX/Y) and full available space initially.
			// PerformLayout will use child.Header.PosX/Y relative to parent.RenderX/Y if absolute.
			PerformLayout(child, parent.RenderX, parent.RenderY, availableW, availableH, scale, doc)
		}
	}
	log.Printf("DEBUG LayoutChildren [%s]: == Child Layout Pass Complete ==", parentIDStr)
} // End PerformLayoutChildren


// renderElementRecursive draws an element and its children recursively based on final layout.
func renderElementRecursive(el *render.RenderElement, scale float32) {
	if el == nil {
		return // Safety check for nil element
	}

	// Basic element identifier for logging
	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex)

	// --- Check Visibility ---
	if !el.IsVisible {
		return // Skip drawing if element is explicitly hidden
	}

	// Get final calculated coordinates and dimensions from the RenderElement struct
	renderX, renderY := el.RenderX, el.RenderY
	renderW, renderH := el.RenderW, el.RenderH

	// Log the frame being used just before attempting to draw
	log.Printf(">>> DRAW Check [%s]: Using Frame (%d,%d %dx%d) for drawing.", elIDStr, renderX, renderY, renderW, renderH)

	// Only draw elements that have a positive size after layout
	if renderW > 0 && renderH > 0 {

		// --- Setup for Drawing ---
		scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }
		bgColor := el.BgColor
		fgColor := el.FgColor
		borderColor := el.BorderColor
		topBW := scaledU8(el.BorderWidths[0])
		rightBW := scaledU8(el.BorderWidths[1])
		bottomBW := scaledU8(el.BorderWidths[2])
		leftBW := scaledU8(el.BorderWidths[3])

		// Clamp borders to prevent overlap if element is too small
		topBW, bottomBW = clampOpposingBorders(topBW, bottomBW, renderH)
		leftBW, rightBW = clampOpposingBorders(leftBW, rightBW, renderW)

		// --- Draw Background (Check for Transparency) ---
		// We check Alpha channel. rl.Blank is {0, 0, 0, 0}.
		shouldDrawBackground := el.Header.Type != krb.ElemTypeText && bgColor.A > 0

		if shouldDrawBackground {
			rl.DrawRectangle(int32(renderX), int32(renderY), int32(renderW), int32(renderH), bgColor)
		} else {
			// Optional log: log.Printf("DEBUG Render [%s]: Skipping background draw (Type is Text or BgColor.A == 0).", elIDStr)
		}

		// --- Draw Borders ---
		drawBorders(renderX, renderY, renderW, renderH, topBW, rightBW, bottomBW, leftBW, borderColor)

		// --- Calculate Content Area (Inside Borders) ---
		contentX, contentY, contentWidth, contentHeight := calculateContentArea(renderX, renderY, renderW, renderH, topBW, rightBW, bottomBW, leftBW)

		// --- Draw Content (Text/Image) within the Content Area ---
		if contentWidth > 0 && contentHeight > 0 {
			// Use Scissor to clip content drawing to the calculated content area
			rl.BeginScissorMode(int32(contentX), int32(contentY), int32(contentWidth), int32(contentHeight))

			// Delegate to drawContent helper (which handles Text vs Image)
			// Pass fgColor as it's used for text color.
			drawContent(el, contentX, contentY, contentWidth, contentHeight, scale, fgColor)

			rl.EndScissorMode() // Stop clipping
		}

	} else { // Element size is zero or negative
		// Optional log: log.Printf("DEBUG Render [%s]: Skipping draw (Zero/Negative Size: %dx%d)", elIDStr, renderW, renderH)
	}

	// --- Recursively Draw Children ---
	// Children are drawn after the parent, appearing visually on top.
	for _, child := range el.Children {
		renderElementRecursive(child, scale) // Pass scale down
	}

} // End renderElementRecursive

// RegisterEventHandler stores a mapping from a callback name string to a Go function.
func (r *RaylibRenderer) RegisterEventHandler(name string, handler func()) {
	if name == "" || handler == nil {
		log.Printf("WARN RENDERER: Invalid event handler registration attempt for name '%s'", name)
		return
	}
	r.eventHandlerMap[name] = handler
	log.Printf("Registered event handler for callback name '%s'", name)
}

// Cleanup unloads resources and closes the Raylib window.
func (r *RaylibRenderer) Cleanup() {
	log.Println("Raylib Cleanup: Unloading textures...")
	for resIndex, texture := range r.loadedTextures {
		if rl.IsTextureReady(texture) {
			rl.UnloadTexture(texture)
		}
		delete(r.loadedTextures, resIndex) // Remove from map
	}
	r.loadedTextures = make(map[uint8]rl.Texture2D) // Clear map just in case

	if rl.IsWindowReady() {
		log.Println("Raylib Cleanup: Closing window...")
		rl.CloseWindow()
	}
}

// ShouldClose checks if the window close button or ESC key (if enabled) was pressed.
func (r *RaylibRenderer) ShouldClose() bool {
	// Also check IsWindowReady to prevent calls after CloseWindow
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
		return // Don't process events if window isn't ready
	}

	mousePos := rl.GetMousePosition()
	cursor := rl.MouseCursorDefault // Default cursor
	mouseClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	clickedElementFound := false // Ensure only the top-most element receives the click

	// Iterate through elements in reverse draw order (top-most first)
	for i := len(r.elements) - 1; i >= 0; i-- {
		el := &r.elements[i]

		// Check only visible, interactive elements with a positive rendered size
		if el.IsVisible && el.IsInteractive && el.RenderW > 0 && el.RenderH > 0 {
			// Define the element's bounding box
			bounds := rl.NewRectangle(float32(el.RenderX), float32(el.RenderY), float32(el.RenderW), float32(el.RenderH))

			// Check if mouse is within bounds
			if rl.CheckCollisionPointRec(mousePos, bounds) {
				cursor = rl.MouseCursorPointingHand // Change cursor to indicate interactable

				// Process click event if mouse was clicked and no element above handled it
				if mouseClicked && !clickedElementFound {
					clickedElementFound = true // Mark click as handled

					// Find and execute the registered click handler
					for _, eventInfo := range el.EventHandlers {
						if eventInfo.EventType == krb.EventTypeClick {
							handlerFunc, found := r.eventHandlerMap[eventInfo.HandlerName]
							if found {
								handlerFunc() // Execute the registered Go function
							} else {
								log.Printf("Warn: Click handler named '%s' (for Elem %d) not registered.", eventInfo.HandlerName, el.OriginalIndex)
							}
							break // Assume only one click handler per element for now
						}
					}
				}
				// Once the top-most element under the cursor is found, stop checking lower elements
				break
			}
		}
	}
	// Set the mouse cursor based on hover state
	rl.SetMouseCursor(cursor)

	// Handle other events like key presses, etc. here if needed
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
		if prop.ID == krb.PropIDBgColor {
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBg = c
			}
		}
	}
}

// applyStylePropertiesToElement applies style properties to an element.
func applyStylePropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BgColor = c }
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.FgColor = c }
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BorderColor = c }
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok { el.BorderWidths = [4]uint8{bw, bw, bw, bw} }
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok { el.TextAlignment = align }
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok { el.IsVisible = (vis != 0) }
		// Add other styleable properties here
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

// loadTextures loads image resources.
func loadTextures(r *RaylibRenderer, doc *krb.Document) {
	for i := range r.elements {
		el := &r.elements[i]
		needsTexture := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) &&
			el.ResourceIndex != render.InvalidResourceIndex
		if !needsTexture { continue }

		resIndex := el.ResourceIndex
		if int(resIndex) >= len(doc.Resources) { log.Printf("Error: Elem %d invalid res index %d", el.OriginalIndex, resIndex); continue }
		res := doc.Resources[resIndex]

		if loadedTex, exists := r.loadedTextures[resIndex]; exists {
			el.Texture = loadedTex
			el.TextureLoaded = rl.IsTextureReady(loadedTex)
			continue
		}

		var texture rl.Texture2D
		loadedOk := false
		if res.Format == krb.ResFormatExternal {
			if int(res.NameIndex) >= len(doc.Strings) { log.Printf("Error: Ext Res %d invalid name index %d", resIndex, res.NameIndex); continue }
			resourceName := doc.Strings[res.NameIndex]
			fullPath := filepath.Join(r.krbFileDir, resourceName)
			if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) { log.Printf("Error: Texture file not found: %s", fullPath) } else {
				texture = rl.LoadTexture(fullPath)
				if rl.IsTextureReady(texture) { loadedOk = true; log.Printf("  Loaded external texture Res %d ('%s') -> ID:%d", resIndex, resourceName, texture.ID) } else { log.Printf("Error: Failed LoadTexture for %s", fullPath) }
			}
		} else if res.Format == krb.ResFormatInline {
			// Inline handling (same as before)
			if res.InlineData != nil && res.InlineDataSize > 0 {
				ext := ".png"; if int(res.NameIndex) < len(doc.Strings) { nameHint := doc.Strings[res.NameIndex]; if nameExt := filepath.Ext(nameHint); nameExt != "" { ext = nameExt } }
				img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData)))
				if rl.IsImageReady(img) { texture = rl.LoadTextureFromImage(img); rl.UnloadImage(img); if rl.IsTextureReady(texture) { loadedOk = true; log.Printf("  Loaded inline texture Res %d -> ID:%d (hint: %s)", resIndex, texture.ID, ext) } else { log.Printf("Error: Failed LoadTextureFromImage inline Res %d", resIndex) } } else { log.Printf("Error: Failed LoadImageFromMemory inline Res %d (hint: %s)", resIndex, ext) }
			} else { log.Printf("Error: Inline Res %d has no data", resIndex) }
		} else { log.Printf("Warn: Unknown resource format %d for Res %d", res.Format, resIndex) }

		if loadedOk { el.Texture = texture; el.TextureLoaded = true; r.loadedTextures[resIndex] = texture } else { el.TextureLoaded = false }
	}
}

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

func drawContent(el *render.RenderElement, cx, cy, cw, ch int, scale float32, fgColor rl.Color) {
	// Draw Text
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		fontSize := int32(math.Max(1, math.Round(baseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		textHeightMeasured := fontSize // Approximation
		textDrawX := cx
		switch el.TextAlignment {
		case krb.LayoutAlignCenter: textDrawX = cx + (cw-int(textWidthMeasured))/2
		case krb.LayoutAlignEnd:    textDrawX = cx + cw - int(textWidthMeasured)
		}
		textDrawY := cy + (ch-int(textHeightMeasured))/2 // Always center vertically
		rl.DrawText(el.Text, int32(textDrawX), int32(textDrawY), fontSize, fgColor)
	}

	// Draw Image
	if (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) && el.TextureLoaded {
		sourceRec := rl.NewRectangle(0, 0, float32(el.Texture.Width), float32(el.Texture.Height))
		destRec := rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch))
		origin := rl.NewVector2(0, 0)
		rl.DrawTexturePro(el.Texture, sourceRec, destRec, origin, 0.0, rl.White)
	}
}