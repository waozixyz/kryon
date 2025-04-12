// render/raylib/raylib_renderer.go
package raylib

import (
	"fmt"
	"encoding/binary"
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
	windowConfig.DefaultBg = rl.Black            // Set specific default background
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

		// Apply App's style properties to the default visual values
		if style, ok := findStyle(doc, appElement.Header.StyleID); ok {
			applyStylePropertiesToDefaults(style.Properties, doc, &windowConfig.DefaultBg, &defaultFg, &defaultBorder, &defaultBorderWidth, &defaultTextAlign)
		} else if appElement.Header.StyleID != 0 {
			log.Printf("Warn: App element has invalid StyleID %d", appElement.Header.StyleID)
		}

		// Apply App's direct KRB properties to the window configuration
		if len(doc.Properties) > 0 && len(doc.Properties[0]) > 0 {
			applyDirectPropertiesToConfig(doc.Properties[0], doc, &windowConfig)
		}

		// Update the renderer's scale factor based on final config
		r.scaleFactor = float32(math.Max(1.0, float64(windowConfig.ScaleFactor)))
		log.Printf("PrepareTree: Processed App. Config: %dx%d '%s' Scale:%.2f Resizable:%t", windowConfig.Width, windowConfig.Height, windowConfig.Title, r.scaleFactor, windowConfig.Resizable)
	} else {
		log.Println("PrepareTree: No App element found, using default window config.")
	}
	// Store the final window configuration
	r.config = windowConfig

	// --- Pass 2: Initialize All RenderElements ---
	// Apply defaults, styles, and direct standard properties to each element.
	for i := 0; i < int(doc.Header.ElementCount); i++ {
		currentEl := &r.elements[i]
		currentEl.Header = doc.Elements[i]
		currentEl.OriginalIndex = i

		// Apply Default visual properties
		currentEl.BgColor = windowConfig.DefaultBg
		currentEl.FgColor = defaultFg
		currentEl.BorderColor = defaultBorder
		currentEl.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth} // Uniform default border
		currentEl.TextAlignment = defaultTextAlign
		currentEl.IsVisible = defaultVisible

		// Set other basic properties derived from KRB header
		currentEl.IsInteractive = (currentEl.Header.Type == krb.ElemTypeButton || currentEl.Header.Type == krb.ElemTypeInput)
		currentEl.ResourceIndex = render.InvalidResourceIndex // Initialize resource index

		// Apply Style Properties (overrides defaults)
		style, styleOk := findStyle(doc, currentEl.Header.StyleID)
		if styleOk {
			applyStylePropertiesToElement(style.Properties, doc, currentEl) // Applies visuals and visibility from style
		} else if currentEl.Header.StyleID != 0 && i != 0 { // Don't warn again for App if already warned
			log.Printf("Warn: Elem %d (Type %d) has invalid StyleID %d", i, currentEl.Header.Type, currentEl.Header.StyleID)
		}

		// Apply Direct Standard Properties (overrides style and defaults)
		// This handles properties set directly on the element tag in KRY source.
		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			// Special handling for App visuals vs. config
			if i == 0 && hasAppElement {
				// Apply visual properties directly to the App element itself if needed
				applyDirectVisualPropertiesToAppElement(doc.Properties[0], doc, currentEl)
			} else if i != 0 || !hasAppElement {
				// Apply direct properties to non-App elements
				applyDirectPropertiesToElement(doc.Properties[i], doc, currentEl) // Applies visuals, visibility, text, image source etc.
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
	// Calculate layout based ONLY on standard KRB layout byte and properties.
	// This determines the initial positions and sizes before custom adjustments.
	log.Printf("DEBUG RenderFrame: == Starting Standard Layout Pass ==") // Added log
	for _, root := range roots {
		if root != nil { // Safety check
			PerformLayout(root, 0, 0, currentWidth, currentHeight, r.scaleFactor, r.docRef)
		}
	}
	log.Printf("DEBUG RenderFrame: == Standard Layout Pass Complete ==") // Added log


	// --- 2. Custom Component Layout Adjustment Pass ---
	// Call the dispatcher function (defined in custom_components.go).
	// This iterates through the laid-out elements and applies specific adjustments
	// based on custom KRB properties interpreted by registered handlers.
	log.Printf("DEBUG RenderFrame: == Starting Custom Adjustment Pass ==") // Added log
	// ===========================================================
	// >>>>>>>>>> THIS LINE IS NOW UNCOMMENTED <<<<<<<<<<<<<<<<<<<<
	//
	ApplyCustomComponentLayoutAdjustments(r.GetRenderTree(), r.docRef) // <<< Ensure this line is ACTIVE
	//
	// ===========================================================
	log.Printf("DEBUG RenderFrame: == Custom Adjustment Pass Complete ==") // Added log


	// --- 3. Draw Pass ---
	// Draw elements based on their *final* RenderX/Y/W/H, which might have
	// been modified by the custom adjustment pass.
	log.Printf("DEBUG RenderFrame: == Starting Draw Pass ==") // Added log
	for _, root := range roots {
		if root != nil { // Safety check
			renderElementRecursive(root, r.scaleFactor)
		}
	}
	log.Printf("DEBUG RenderFrame: == Draw Pass Complete ==") // Added log
}

// PerformLayout calculates layout based on standard KRB rules (Layout byte, standard sizes).
// It does NOT interpret custom properties for layout.
func PerformLayout(el *render.RenderElement, parentContentX, parentContentY, parentContentW, parentContentH int, scale float32, doc *krb.Document) {
	if el == nil {
		log.Printf("DEBUG Layout: Skipping nil element.") // Added log
		return // Safety check
	}

	// --- Added Log: Element and Parent Bounds ---
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
	// --- End Added Log ---

	isRoot := (el.Parent == nil)
	scaled := func(v uint16) int { return int(math.Round(float64(v) * float64(scale))) }
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

	// --- 1. Determine Base Size ---
	baseW, baseH := calculateBaseSize(el, scale, doc)
	el.IntrinsicW = baseW
	el.IntrinsicH = baseH
	log.Printf("DEBUG Layout [%s]: Calculated Base Size: %dx%d (Intrinsic)", elIDStr, baseW, baseH) // Added log

	// Apply explicit size from KRB header
	explicitW, explicitH := false, false // Define flags HERE to capture header explicitness
	if el.Header.Width > 0 {
		baseW = scaled(el.Header.Width)
		explicitW = true // Set flag
		log.Printf("DEBUG Layout [%s]: Overriding Width with Header.Width %d -> %d", elIDStr, el.Header.Width, baseW) // Added log
	}
	if el.Header.Height > 0 {
		baseH = scaled(el.Header.Height)
		explicitH = true // Set flag
		log.Printf("DEBUG Layout [%s]: Overriding Height with Header.Height %d -> %d", elIDStr, el.Header.Height, baseH) // Added log
	}

	// --- 2. Determine Initial Render Size ---
	initialW, initialH := baseW, baseH
	isFlowElement := !el.Header.LayoutAbsolute()

	if isRoot {
		initialW, initialH = parentContentW, parentContentH
		log.Printf("DEBUG Layout [%s]: Is Root. Setting Initial Size to Parent Content Area: %dx%d", elIDStr, initialW, initialH) // Added log
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
		log.Printf("DEBUG Layout [%s]: Is Absolute. Initial Size remains Base Size: %dx%d", elIDStr, initialW, initialH) // Added log
	}

	el.RenderW = initialW
	el.RenderH = initialH // Store potentially clamped size

	// --- 3. Determine Initial Render Position ---
	el.RenderX = parentContentX
	el.RenderY = parentContentY
	initialX, initialY := el.RenderX, el.RenderY // Store for logging

	if isEffectivelyAbsolute(el) {
		offsetX, offsetY := scaled(el.Header.PosX), scaled(el.Header.PosY)
		if el.Parent != nil { el.RenderX = el.Parent.RenderX + offsetX; el.RenderY = el.Parent.RenderY + offsetY; log.Printf("DEBUG Layout [%s]: Is Absolute w/ Parent. Pos Offset (%d,%d) relative to Parent (%d,%d) -> (%d,%d)", elIDStr, offsetX, offsetY, el.Parent.RenderX, el.Parent.RenderY, el.RenderX, el.RenderY)
		} else if !isRoot { log.Printf("Warn: Absolute Elem %d has no parent, positioning relative to (0,0).", el.OriginalIndex); el.RenderX = offsetX; el.RenderY = offsetY; log.Printf("DEBUG Layout [%s]: Is Absolute w/o Parent. Pos Offset (%d,%d) relative to (0,0) -> (%d,%d)", elIDStr, offsetX, offsetY, el.RenderX, el.RenderY)
		} else { el.RenderX = offsetX; el.RenderY = offsetY; log.Printf("DEBUG Layout [%s]: Is Absolute Root. Pos Offset (%d,%d) relative to (0,0) -> (%d,%d)", elIDStr, offsetX, offsetY, el.RenderX, el.RenderY) }
	} else { log.Printf("DEBUG Layout [%s]: Is Flow. Initial Position set to Parent Content Origin: %d,%d (Final pos set later)", elIDStr, initialX, initialY) }


	// --- 4. Layout Children (Recursive Step) ---
	if el.Header.ChildCount > 0 && len(el.Children) > 0 {
		borderL := scaledU8(el.BorderWidths[3]); borderT := scaledU8(el.BorderWidths[0])
		borderR := scaledU8(el.BorderWidths[1]); borderB := scaledU8(el.BorderWidths[2])
		clientAbsX := el.RenderX + borderL
		clientAbsY := el.RenderY + borderT

		// Determine the available width and height FOR THE CHILDREN.
		potentialChildW := parentContentW
		potentialChildH := parentContentH
		if explicitW { // Use flag from Step 1
			potentialChildW = baseW
		}
		if explicitH { // Use flag from Step 1
			potentialChildH = baseH
		}

		clientWidth := max(0, potentialChildW - borderL - borderR)
		clientHeight := max(0, potentialChildH - borderT - borderB)

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

	// *** CORRECTION IS HERE: Define hasExplicitW/H combining header and style ***
	hasExplicitW := explicitW || hasExplicitW_StyleCheck // Use flag from Step 1
	hasExplicitH := explicitH || hasExplicitH_StyleCheck // Use flag from Step 1

	// Determine if auto-sizing should occur
	shouldAutoSizeW := !shouldGrow && !hasExplicitW
	shouldAutoSizeH := !shouldGrow && !hasExplicitH

	if (shouldAutoSizeW || shouldAutoSizeH) && !isRoot && isFlowElement {
		// Log detailed auto-size decision factors using the combined flags
		log.Printf("DEBUG Layout [%s]: Attempting Auto-Size (IsRoot:%t, IsFlow:%t | Grow:%t | ExplicitW:%t [Hdr:%t Stl:%t] | ExplicitH:%t [Hdr:%t Stl:%t] => AutoW:%t, AutoH:%t)",
			elIDStr, isRoot, isFlowElement, shouldGrow,
			hasExplicitW, explicitW, hasExplicitW_StyleCheck, // explicitW is flag from Step 1 header check
			hasExplicitH, explicitH, hasExplicitH_StyleCheck, // explicitH is flag from Step 1 header check
			shouldAutoSizeW, shouldAutoSizeH)

		borderL, borderT := scaledU8(el.BorderWidths[3]), scaledU8(el.BorderWidths[0])
		borderR, borderB := scaledU8(el.BorderWidths[1]), scaledU8(el.BorderWidths[2])
		clientAbsX := el.RenderX + borderL; clientAbsY := el.RenderY + borderT
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

func PerformLayoutChildren(parent *render.RenderElement, parentClientOriginX, parentClientOriginY, availableW, availableH int, scale float32, doc *krb.Document) {
	if parent == nil || len(parent.Children) == 0 { // Added parent nil check
		return
	}

	parentIDStr := fmt.Sprintf("Elem %d", parent.OriginalIndex) // Added for logging
	if doc != nil && parent.Header.ID != 0 && int(parent.Header.ID) < len(doc.Strings) {
		parentIDStr += fmt.Sprintf(" ID:'%s'", doc.Strings[parent.Header.ID])
	}

	flowChildren := make([]*render.RenderElement, 0, len(parent.Children))
	absoluteChildren := make([]*render.RenderElement, 0)
	for _, child := range parent.Children {
		if child == nil { continue } // Skip nil children if they somehow exist
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
		if growChildrenCount > 0 && spaceForGrowth > 0 {
			growSizePerChild = spaceForGrowth / growChildrenCount
			remainderForLastGrowChild = spaceForGrowth % growChildrenCount
			log.Printf("DEBUG LayoutChildren [%s]: Growth Calculated. Total:%d PerChild:%d Remainder:%d", parentIDStr, spaceForGrowth, growSizePerChild, remainderForLastGrowChild)
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
				if tempGrowCount == growChildrenCount-1 { growAmount += remainderForLastGrowChild }
				tempGrowCount++

				if mainAxisIsHorizontal { child.RenderW += growAmount; child.RenderH = crossAxisSize } else { child.RenderH += growAmount; child.RenderW = crossAxisSize }
				log.Printf("DEBUG LayoutChildren [%s Child %s]: Applied Growth %d. Size %dx%d -> %dx%d", parentIDStr, childIDStr, growAmount, preSizeW, preSizeH, child.RenderW, child.RenderH)
			} else {
				clampedCross := false
				if mainAxisIsHorizontal { if child.RenderH > crossAxisSize { child.RenderH = crossAxisSize; clampedCross = true }
				} else { if child.RenderW > crossAxisSize { child.RenderW = crossAxisSize; clampedCross = true } }
				if clampedCross { log.Printf("DEBUG LayoutChildren [%s Child %s]: Clamped Cross Axis. Size %dx%d -> %dx%d", parentIDStr, childIDStr, preSizeW, preSizeH, child.RenderW, child.RenderH) }
			}
			child.RenderW = max(1, child.RenderW); child.RenderH = max(1, child.RenderH)
			totalFinalFlowSizeMainAxis += MuxInt(mainAxisIsHorizontal, child.RenderW, child.RenderH)
		}
		log.Printf("DEBUG LayoutChildren [%s]: Pass 2 Done. TotalFinalFlowSize:%d", parentIDStr, totalFinalFlowSizeMainAxis)

		// --- Calculate Alignment Offsets ---
		startOffset, spacing := calculateAlignmentOffsets(alignment, mainAxisAvailableSpace, totalFinalFlowSizeMainAxis, len(flowChildren), isReversed)
		log.Printf("DEBUG LayoutChildren [%s]: Alignment Offsets Calculated. Start:%d Spacing:%d", parentIDStr, startOffset, spacing)

		// --- Pass 3: Position Flow Children ---
		currentMainAxisPos := startOffset
		indices := make([]int, len(flowChildren)); for i := range indices { indices[i] = i }
		if isReversed { ReverseSliceInt(indices) }

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

            // *** MODIFIED LOG MESSAGE ***
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
	// Absolutely positioned children are laid out relative to the parent's *origin* (not content area).
	// PerformLayout for absolute children already handles adding the parent's RenderX/Y.
	if len(absoluteChildren) > 0 {
		//scaledU8 := func(val uint8) int { return int(math.Round(float64(val) * float64(scale))) }
		// Note: For absolute positioning relative to parent CORNER, we might need parent.RenderX/Y, not parentClientOriginX/Y.
		// Let's assume PerformLayout correctly handles absolute positioning relative to parent's RenderX/Y.
		log.Printf("DEBUG LayoutChildren [%s]: Laying out %d Absolute Children relative to Parent Frame %d,%d", parentIDStr, len(absoluteChildren), parent.RenderX, parent.RenderY)
		for _, child := range absoluteChildren {
			// Pass parent's top-left corner (RenderX/Y) and full available space initially.
			// PerformLayout will use child.Header.PosX/Y relative to parent.RenderX/Y if absolute.
			PerformLayout(child, parent.RenderX, parent.RenderY, availableW, availableH, scale, doc)
		}
	}
	log.Printf("DEBUG LayoutChildren [%s]: == Child Layout Pass Complete ==", parentIDStr)
} // End PerformLayoutChildren

func renderElementRecursive(el *render.RenderElement, scale float32) {
	if el == nil {
		return
	}

	elIDStr := fmt.Sprintf("Elem %d", el.OriginalIndex) // Basic ID for logging

	if !el.IsVisible {
		// log.Printf("DEBUG Render [%s]: Skipping draw (IsVisible=false)", elIDStr) // Optional verbose log
		return // Skip drawing if element is explicitly hidden
	}


	renderX, renderY := el.RenderX, el.RenderY
	renderW, renderH := el.RenderW, el.RenderH

	// --- >>> ADD LOGGING HERE <<< ---
	log.Printf(">>> DRAW Check [%s]: Using Frame (%d,%d %dx%d) for drawing.", elIDStr, renderX, renderY, renderW, renderH)
	// --- >>> END LOGGING <<< ---


	// Only draw elements that have a positive size after layout
	if renderW > 0 && renderH > 0 {
		// log.Printf("DEBUG Render [%s]: Attempting Draw at %d,%d %dx%d (Type %d)", elIDStr, renderX, renderY, renderW, renderH, el.Header.Type) // Existing log

		scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

		// Get final visual properties
		bgColor := el.BgColor
		fgColor := el.FgColor
		borderColor := el.BorderColor
		topBW := scaledU8(el.BorderWidths[0]); rightBW := scaledU8(el.BorderWidths[1])
		bottomBW := scaledU8(el.BorderWidths[2]); leftBW := scaledU8(el.BorderWidths[3])
		topBW, bottomBW = clampOpposingBorders(topBW, bottomBW, renderH)
		leftBW, rightBW = clampOpposingBorders(leftBW, rightBW, renderW)

		// --- Draw Background ---
		if el.Header.Type != krb.ElemTypeText || bgColor.A > 0 {
			rl.DrawRectangle(int32(renderX), int32(renderY), int32(renderW), int32(renderH), bgColor)
		}

		// --- Draw Borders ---
		drawBorders(renderX, renderY, renderW, renderH, topBW, rightBW, bottomBW, leftBW, borderColor)

		// --- Calculate Content Area ---
		contentX, contentY, contentWidth, contentHeight := calculateContentArea(renderX, renderY, renderW, renderH, topBW, rightBW, bottomBW, leftBW)

		// --- Draw Content (Text/Image) ---
		if contentWidth > 0 && contentHeight > 0 {
			rl.BeginScissorMode(int32(contentX), int32(contentY), int32(contentWidth), int32(contentHeight))
			drawContent(el, contentX, contentY, contentWidth, contentHeight, scale, fgColor)
			rl.EndScissorMode()
		}
	} else {
		// log.Printf("DEBUG Render [%s]: Skipping draw (Zero/Negative Size: %dx%d)", elIDStr, renderW, renderH) // Optional log
	}

	// --- Recursively Draw Children ---
	for _, child := range el.Children {
		renderElementRecursive(child, scale)
	}
}

// --- Other Renderer interface methods ---

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
	cursor := rl.MouseCursorDefault          // Default cursor
	mouseClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	clickedElementFound := false // Ensure only the top-most element receives the click

	// Iterate through elements in reverse draw order (top-most first)
	// Assumes r.elements is populated in KRB order, which usually means parents before children
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
								// log.Printf("Debug: Executing click handler '%s' for Elem %d", eventInfo.HandlerName, el.OriginalIndex)
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
func buildElementTree(doc *krb.Document, elements []render.RenderElement, roots *[]*render.RenderElement, elementStartOffsets []uint32) error {
	if len(elements) != len(elementStartOffsets) {
		// Basic sanity check, should always match if populated correctly
		return fmt.Errorf("buildElementTree: mismatch between element count (%d) and start offsets count (%d)", len(elements), len(elementStartOffsets))
	}

	// --- Phase 1: Reset links and build offset lookup map ---
	offsetToIndex := make(map[uint32]int, len(elements))
	for i := range elements {
		elements[i].Parent = nil // Reset parent link
		elements[i].Children = nil // Reset children slice (nil is okay, append works)
		if i < len(elementStartOffsets) {
			offsetToIndex[elementStartOffsets[i]] = i // Map file offset to slice index
		} else {
			// This case should theoretically not happen if the length check passed
			log.Printf("Error: buildElementTree: Index %d out of bounds for elementStartOffsets (len %d)", i, len(elementStartOffsets))
			// Decide how critical this is. Maybe return an error or continue cautiously.
		}
	}
	log.Printf("Debug buildElementTree: Built offset map with %d entries.", len(offsetToIndex))


	// --- Phase 2: Link elements based on ChildRef offsets ---
	linkErrors := 0
	for parentIndex := 0; parentIndex < len(elements); parentIndex++ {
		parentEl := &elements[parentIndex]
		parentOffset := elementStartOffsets[parentIndex] // Get the starting offset of this parent's header in the file

		// Check if this parent has child references defined in the KRB
		// Ensure doc.ChildRefs is not nil and parentIndex is within its bounds
		if doc.ChildRefs != nil && parentIndex < len(doc.ChildRefs) && doc.ChildRefs[parentIndex] != nil {

			// Iterate through the KRB-defined child references for this parent
			for _, childRef := range doc.ChildRefs[parentIndex] {
				// Calculate the absolute file offset where the child's header should start
				// ChildOffset is relative to the *start* of the parent's header
				childHeaderOffset := parentOffset + uint32(childRef.ChildOffset)

				// Look up the slice index of the element that starts at this calculated offset
				childIndex, found := offsetToIndex[childHeaderOffset]

				// --- Validation ---
				if !found {
					log.Printf("Error: buildElementTree: Parent Elem %d (Offset %d) references child at offset %d, but no element found starting there. Skipping link.", parentIndex, parentOffset, childHeaderOffset)
					linkErrors++
					continue // Skip this broken reference
				}

				if childIndex == parentIndex {
					log.Printf("Error: buildElementTree: Parent Elem %d (Offset %d) references itself as a child (Offset %d). Skipping link.", parentIndex, parentOffset, childHeaderOffset)
					linkErrors++
					continue // Skip self-reference
				}

				if childIndex < 0 || childIndex >= len(elements) {
					// Should be caught by 'found' check, but good to be safe
					log.Printf("Error: buildElementTree: Invalid child index %d resolved for parent %d (Child Offset %d). Skipping link.", childIndex, parentIndex, childRef.ChildOffset)
					linkErrors++
					continue
				}

				// --- Perform Linking ---
				childEl := &elements[childIndex]

				// Check if child already has a parent (indicates non-tree structure or KRB issue)
				if childEl.Parent != nil {
					// Decide how to handle: log warning and overwrite, or treat as error.
					// Overwriting might hide KRB structure problems but allows proceeding.
					log.Printf("Warn: buildElementTree: Child Elem %d (Offset %d) already has Parent Elem %d. Overwriting with Parent Elem %d.", childIndex, childHeaderOffset, childEl.Parent.OriginalIndex, parentIndex)
					// Optionally remove from previous parent's children list? More complex.
				}

				childEl.Parent = parentEl                     // Set child's parent pointer
				parentEl.Children = append(parentEl.Children, childEl) // Add child to parent's children slice
                // log.Printf("Debug buildElementTree: Linked Parent %d (Offset %d) -> Child %d (Offset %d, RefOffset %d)", parentIndex, parentOffset, childIndex, childHeaderOffset, childRef.ChildOffset)

			} // End loop through child refs for this parent
		} // End check if parent has child refs
	} // End loop through all elements as potential parents

	if linkErrors > 0 {
		log.Printf("Warn: buildElementTree: Encountered %d linking errors due to invalid child references.", linkErrors)
	}

	// --- Phase 3: Identify root elements ---
	*roots = (*roots)[:0] // Clear the existing roots slice efficiently
	foundRoots := 0
	for i := range elements {
		if elements[i].Parent == nil {
			// This element has no parent after processing all links, it's a root.
			*roots = append(*roots, &elements[i])
			foundRoots++
		}
	}

	log.Printf("Debug buildElementTree: Linking complete. Found %d root elements.", foundRoots)

	// Optional sanity check: Does the number of children found match ChildCount?
	// Can be complex due to potential errors/overwrites.
	// for i := range elements {
	//     expected := int(elements[i].Header.ChildCount)
	//     actual := len(elements[i].Children)
	//     if expected != actual {
	//         log.Printf("Debug buildElementTree: Elem %d ChildCount mismatch: Expected %d, Found %d linked children.", i, expected, actual)
	//     }
	// }

	// Check if any roots were found if elements exist
    if len(elements) > 0 && foundRoots == 0 {
        log.Printf("Error: buildElementTree: No root elements identified after linking %d elements. Potential cycle or KRB structure issue.", len(elements))
        // Depending on requirements, might return an error here
		// return fmt.Errorf("no root elements found after linking")
    }

	return nil // Indicate success (or return error based on checks above)
}

// applyStylePropertiesToDefaults applies visual properties from an App element's style
// to the default values used for the window and other elements.
func applyStylePropertiesToDefaults(props []krb.Property, doc *krb.Document, defaultBg, defaultFg, defaultBorder *rl.Color, defaultBorderWidth *uint8, defaultTextAlign *uint8) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBg = c
			}
		case krb.PropIDFgColor: // Also affects default text color
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultFg = c
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBorder = c
			}
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok {
				*defaultBorderWidth = bw
			}
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				*defaultTextAlign = align
			}
			// Add other default properties here if needed (e.g., default font size from style)
		}
	}
}

// applyStylePropertiesToElement applies visual properties from a style
// directly onto a RenderElement instance, overriding defaults.
func applyStylePropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}
		case krb.PropIDFgColor: // Also text color
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.FgColor = c
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BorderColor = c
			}
		case krb.PropIDBorderWidth: // Style typically defines a uniform border width
			if bw, ok := getByteValue(&prop); ok {
				el.BorderWidths = [4]uint8{bw, bw, bw, bw}
			}
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				el.TextAlignment = align
			}
		case krb.PropIDVisibility: // Styles can affect visibility
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
			// Apply other styleable properties here (FontSize, etc.)
		}
	}
}

// applyDirectVisualPropertiesToAppElement applies visual properties set directly
// on the App element, overriding any defaults or style settings for the App's own visuals.
func applyDirectVisualPropertiesToAppElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor: // App's direct BG overrides default window BG derived earlier
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				el.BgColor = c
			}
		case krb.PropIDVisibility: // Can the App element itself be hidden?
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}
			// Add FgColor, BorderColor etc. if the App element should have direct visual overrides
		}
	}
}

// applyDirectPropertiesToElement applies standard KRB properties set directly on an element,
// overriding any values set by styles or defaults.
func applyDirectPropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		// Visual Properties
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
		case krb.PropIDBorderWidth: // Direct property could be single byte or EdgeInsets
			if bw, ok := getByteValue(&prop); ok { // Check if single byte first
				el.BorderWidths = [4]uint8{bw, bw, bw, bw}
			} else if edges, ok := getEdgeInsetsValue(&prop); ok { // Check if EdgeInsets
				el.BorderWidths = edges
			}
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				el.TextAlignment = align
			}
		case krb.PropIDVisibility:
			if vis, ok := getByteValue(&prop); ok {
				el.IsVisible = (vis != 0)
			}

		// Content Properties
		case krb.PropIDTextContent: // Resolve string index to text
			if strIdx, ok := getByteValue(&prop); ok {
				if doc != nil && int(strIdx) < len(doc.Strings) {
					el.Text = doc.Strings[strIdx]
				} else {
					log.Printf("Warn: Elem %d invalid text content string index %d", el.OriginalIndex, strIdx)
				}
			}
		case krb.PropIDImageSource: // Store resource index
			if resIdx, ok := getByteValue(&prop); ok {
				el.ResourceIndex = resIdx
			}

		// Ignore App config properties when applied to non-App elements
		case krb.PropIDWindowWidth, krb.PropIDWindowHeight, krb.PropIDWindowTitle,
			krb.PropIDResizable, krb.PropIDScaleFactor, krb.PropIDIcon,
			krb.PropIDVersion, krb.PropIDAuthor:
			continue // Skip these if not the App element

			// TODO: Handle other standard direct properties (Opacity, ZIndex, Padding, Margin, Font props etc.)
			// if RenderElement struct is extended to support them.
		}
	}
}

// applyDirectPropertiesToConfig applies properties from the App element
// directly to the WindowConfig struct, overriding defaults.
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
			if s, ok := getStringValue(&prop, doc); ok {
				config.Title = s
			}
		case krb.PropIDResizable:
			if r, ok := getByteValue(&prop); ok {
				config.Resizable = (r != 0)
			}
		case krb.PropIDScaleFactor:
			if sf, ok := getFixedPointValue(&prop); ok && sf > 0 {
				config.ScaleFactor = sf
			}
		case krb.PropIDBgColor: // App's direct BG color sets the default window background
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				config.DefaultBg = c
			}
			// Add Icon, KeepAspect etc. here if WindowConfig supports them
		}
	}
}

// resolveElementText determines the final text content for an element,
// checking direct properties first, then the applied style.
func resolveElementText(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	// Only applicable to elements that can display text
	if el.Header.Type != krb.ElemTypeText && el.Header.Type != krb.ElemTypeButton {
		return
	}

	resolvedText := ""
	foundTextProp := false

	// 1. Check direct properties on the element
	if len(doc.Properties) > el.OriginalIndex {
		for _, prop := range doc.Properties[el.OriginalIndex] {
			if prop.ID == krb.PropIDTextContent {
				if s, ok := getStringValue(&prop, doc); ok {
					resolvedText = s
					foundTextProp = true
					break // Found direct text, stop searching
				}
			}
		}
	}

	// 2. If no direct text, check the style
	if !foundTextProp && styleOk {
		if prop, ok := getStylePropertyValue(style, krb.PropIDTextContent); ok {
			if s, valOk := getStringValue(prop, doc); valOk {
				resolvedText = s
			}
		}
	}

	el.Text = resolvedText // Assign the final resolved text
}

// resolveElementImageSource determines the final resource index for an element's image,
// checking direct properties first, then the applied style.
func resolveElementImageSource(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	// Only applicable to elements that can display images
	if el.Header.Type != krb.ElemTypeImage && el.Header.Type != krb.ElemTypeButton {
		return
	}

	resolvedResIdx := uint8(render.InvalidResourceIndex) // Default to invalid
	foundResProp := false

	// 1. Check direct properties on the element
	if len(doc.Properties) > el.OriginalIndex {
		for _, prop := range doc.Properties[el.OriginalIndex] {
			if prop.ID == krb.PropIDImageSource {
				if idx, ok := getByteValue(&prop); ok { // ImageSource value is the resource index (byte)
					resolvedResIdx = idx
					foundResProp = true
					break // Found direct source, stop searching
				}
			}
		}
	}

	// 2. If no direct source, check the style
	if !foundResProp && styleOk {
		if prop, ok := getStylePropertyValue(style, krb.PropIDImageSource); ok {
			if idx, valOk := getByteValue(prop); valOk {
				resolvedResIdx = idx
			}
		}
	}

	el.ResourceIndex = resolvedResIdx // Assign the final resolved resource index
}

// resolveEventHandlers populates the EventHandlers slice by looking up
// callback names from the string table using indices from the KRB events section.
func resolveEventHandlers(doc *krb.Document, el *render.RenderElement) {
	el.EventHandlers = nil // Reset handlers for this element

	// Check if event data exists for this element index
	if doc.Events != nil && el.OriginalIndex < len(doc.Events) && doc.Events[el.OriginalIndex] != nil {
		krbEvents := doc.Events[el.OriginalIndex] // Get the list of KRB event entries

		// Allocate slice if events exist
		if len(krbEvents) > 0 {
			el.EventHandlers = make([]render.EventCallbackInfo, 0, len(krbEvents))
		}

		// Process each KRB event entry
		for _, krbEvent := range krbEvents {
			// Validate the callback string index
			if int(krbEvent.CallbackID) < len(doc.Strings) {
				handlerName := doc.Strings[krbEvent.CallbackID] // Look up the name
				// Append the resolved event info
				el.EventHandlers = append(el.EventHandlers, render.EventCallbackInfo{
					EventType:   krbEvent.EventType,
					HandlerName: handlerName,
				})
			} else {
				// Log warning if index is invalid
				log.Printf("Warn: Elem %d has invalid event callback string index %d", el.OriginalIndex, krbEvent.CallbackID)
			}
		}
	}
}

// loadTextures iterates through prepared elements and loads image resources
// into Raylib textures, caching them to avoid redundant loading.
func loadTextures(r *RaylibRenderer, doc *krb.Document) {
	for i := range r.elements {
		el := &r.elements[i]

		// Check if element needs a texture and has a valid resource index
		needsTexture := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) &&
			el.ResourceIndex != render.InvalidResourceIndex

		if !needsTexture {
			continue
		}

		resIndex := el.ResourceIndex

		// Validate resource index against the document's resource table
		if int(resIndex) >= len(doc.Resources) {
			log.Printf("Error: Elem %d has invalid resource index %d (max %d)", el.OriginalIndex, resIndex, len(doc.Resources)-1)
			continue
		}
		res := doc.Resources[resIndex] // Get the resource metadata

		// Check cache first
		if loadedTex, exists := r.loadedTextures[resIndex]; exists {
			el.Texture = loadedTex
			el.TextureLoaded = rl.IsTextureReady(loadedTex) // Re-check readiness just in case
			continue                                       // Use cached texture
		}

		// --- Load Texture ---
		var texture rl.Texture2D
		loadedOk := false

		// Load based on resource format
		if res.Format == krb.ResFormatExternal {
			// Validate name index
			if int(res.NameIndex) >= len(doc.Strings) {
				log.Printf("Error: External Resource %d has invalid name string index %d", resIndex, res.NameIndex)
				continue
			}
			resourceName := doc.Strings[res.NameIndex] // Get filename/path from string table
			fullPath := filepath.Join(r.krbFileDir, resourceName) // Construct full path

			// Check if file exists before trying to load
			if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
				log.Printf("Error: Texture file not found: %s (for Res %d)", fullPath, resIndex)
			} else {
				// Attempt to load texture from file
				texture = rl.LoadTexture(fullPath)
				if rl.IsTextureReady(texture) {
					loadedOk = true
					log.Printf("  Loaded external texture Res %d ('%s') -> ID:%d", resIndex, resourceName, texture.ID)
				} else {
					// Texture failed to load (unsupported format, corrupted file, etc.)
					log.Printf("Error: Failed LoadTexture for %s (for Res %d)", fullPath, resIndex)
					// rl.UnloadTexture(texture) // Should be safe even if not ready
				}
			}
		} else if res.Format == krb.ResFormatInline {
			if res.InlineData != nil && res.InlineDataSize > 0 {
				// Guess file extension for LoadImageFromMemory (default to png)
				ext := ".png"
				if int(res.NameIndex) < len(doc.Strings) {
					nameHint := doc.Strings[res.NameIndex] // Use name as hint
					if nameExt := filepath.Ext(nameHint); nameExt != "" {
						ext = nameExt // Use extension from name hint if available
					}
				}
				// Load image from memory buffer
				img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData)))
				if rl.IsImageReady(img) {
					// Convert loaded image to texture
					texture = rl.LoadTextureFromImage(img)
					rl.UnloadImage(img) // Unload image from CPU memory after GPU upload
					if rl.IsTextureReady(texture) {
						loadedOk = true
						log.Printf("  Loaded inline texture Res %d -> ID:%d (format hint: %s)", resIndex, texture.ID, ext)
					} else {
						log.Printf("Error: Failed LoadTextureFromImage for inline Res %d", resIndex)
					}
				} else {
					log.Printf("Error: Failed LoadImageFromMemory for inline Res %d (format hint: %s)", resIndex, ext)
				}
			} else {
				log.Printf("Error: Inline Resource %d has no data (Size: %d)", resIndex, res.InlineDataSize)
			}
		} else {
			// Unsupported resource format
			log.Printf("Warn: Unknown resource format %d for Resource %d", res.Format, resIndex)
		}

		// Store texture in element and cache if loaded successfully
		if loadedOk {
			el.Texture = texture
			el.TextureLoaded = true
			r.loadedTextures[resIndex] = texture // Add to cache
		} else {
			el.TextureLoaded = false
			// Optionally set a default "missing texture" texture here
		}
	}
}

// hasStyleSize checks if a style contains a specific size-related property (MaxWidth or MaxHeight).
func hasStyleSize(doc *krb.Document, el *render.RenderElement, propIDMax, propIDNormal krb.PropertyID) bool {
	if style, ok := findStyle(doc, el.Header.StyleID); ok {
		// Check only for MaxWidth/MaxHeight as standard style size indicators in KRB
		if _, maxOk := getStylePropertyValue(style, propIDMax); maxOk {
			return true
		}
		// PropIDNormal (Width/Height) is usually not standard in styles, only MaxWidth/MaxHeight
	}
	return false
}

// findStyle retrieves a style from the document by its 1-based ID.
func findStyle(doc *krb.Document, styleID uint8) (*krb.Style, bool) {
	if styleID == 0 || int(styleID) > len(doc.Styles) {
		return nil, false // Invalid ID (0 or out of bounds)
	}
	// Style array is 0-based, KRB StyleID is 1-based
	return &doc.Styles[styleID-1], true
}

// getStylePropertyValue finds a specific property within a style's resolved properties.
func getStylePropertyValue(style *krb.Style, propID krb.PropertyID) (*krb.Property, bool) {
	if style == nil {
		return nil, false
	}
	// Iterate through the properties resolved for this style
	for i := range style.Properties {
		if style.Properties[i].ID == propID {
			return &style.Properties[i], true // Found the property
		}
	}
	return nil, false // Property not found in this style
}

// --- Value Parsing Helper Functions ---

// getColorValue parses color data from a KRB property based on header flags.
func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {
	if prop == nil || prop.ValueType != krb.ValTypeColor {
		return rl.Color{}, false // Not a color property
	}
	useExtended := (flags & krb.FlagExtendedColor) != 0 // Check if RGBA format is used
	if useExtended {
		if len(prop.Value) == 4 { // Expect 4 bytes for RGBA
			return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true
		}
	} else {
		if len(prop.Value) == 1 { // Expect 1 byte for palette index
			// Palette support not implemented, return a placeholder color and log warning
			log.Printf("Warn: Palette color index %d used, but palettes are not implemented. Using Magenta.", prop.Value[0])
			return rl.Magenta, true // Placeholder
		}
	}
	// Data size mismatch for the expected color format
	log.Printf("Warn: Invalid color data size for prop ID %d. UseExtended=%t, Expected %d, Got %d", prop.ID, useExtended, MuxInt(useExtended, 4, 1), len(prop.Value))
	return rl.Color{}, false
}

// getByteValue extracts a single byte value (used for bools, enums, indices).
func getByteValue(prop *krb.Property) (uint8, bool) {
	// Check if property exists, has a compatible type (byte, index, enum), and has correct size
	if prop != nil &&
		(prop.ValueType == krb.ValTypeByte || prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource || prop.ValueType == krb.ValTypeEnum) &&
		len(prop.Value) == 1 {
		return prop.Value[0], true
	}
	return 0, false
}

// getShortValue extracts a uint16 value.
func getShortValue(prop *krb.Property) (uint16, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 {
		return binary.LittleEndian.Uint16(prop.Value), true
	}
	return 0, false
}

// getStringValue extracts a string by looking up the index from the property value.
func getStringValue(prop *krb.Property, doc *krb.Document) (string, bool) {
	// Check if it's a string index property
	if prop != nil && prop.ValueType == krb.ValTypeString && len(prop.Value) == 1 {
		idx := prop.Value[0]
		// Validate index against document's string table
		if doc != nil && int(idx) < len(doc.Strings) {
			return doc.Strings[idx], true // Return looked-up string
		} else {
			log.Printf("Warn: Invalid string index %d encountered for property ID %d.", idx, prop.ID)
		}
	}
	return "", false
}

// getFixedPointValue extracts an 8.8 fixed-point value and converts it to float32.
func getFixedPointValue(prop *krb.Property) (float32, bool) {
	if prop != nil && prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 {
		fixedVal := binary.LittleEndian.Uint16(prop.Value)
		// Convert 8.8 fixed point (uint16) to float32 by dividing by 2^8
		return float32(fixedVal) / 256.0, true
	}
	return 0, false
}

// getEdgeInsetsValue extracts four uint8 values (T, R, B, L).
func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) {
	if prop != nil && prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 {
		// Assumes order is Top, Right, Bottom, Left
		return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true
	}
	return [4]uint8{}, false
}

// --- Generic Math/Slice Helpers ---
func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }
func MuxInt(cond bool, a, b int) int { if cond { return a }; return b }
func ReverseSliceInt(s []int) { for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 { s[i], s[j] = s[j], s[i] } }

// --- Drawing Helper Functions ---

// isEffectivelyAbsolute checks if an element should use absolute positioning rules.
func isEffectivelyAbsolute(el *render.RenderElement) bool {
	if el == nil {
		return false
	}
	// Considered absolute if the absolute bit is set in layout byte OR if explicit X/Y coords are given in header
	return el.Header.LayoutAbsolute() || el.Header.PosX != 0 || el.Header.PosY != 0
}

// calculateBaseSize determines the initial ("natural" or preferred) size of an element
// before layout constraints (like growth or parent clamping) are applied.
// Precedence: KRB Header > Style (MaxWidth/MaxHeight) > Intrinsic Content.
func calculateBaseSize(el *render.RenderElement, scale float32, doc *krb.Document) (baseW int, baseH int) {
	scaled := func(v uint16) int { return int(math.Round(float64(v) * float64(scale))) }
	scaledU8 := func(v uint8) int { return int(math.Round(float64(v) * float64(scale))) }

	// 1. Explicit size from KRB Header (Highest priority)
	if el.Header.Width > 0 {
		baseW = scaled(el.Header.Width)
	}
	if el.Header.Height > 0 {
		baseH = scaled(el.Header.Height)
	}

	// 2. Size from Style (Use MaxWidth/MaxHeight if header didn't specify)
	// Only check style if a dimension is still unspecified (0)
	if baseW == 0 || baseH == 0 {
		if style, ok := findStyle(doc, el.Header.StyleID); ok {
			if baseW == 0 { // Check style for width only if not set by header
				// KRB convention is MaxWidth in styles for explicit size
				if prop, ok := getStylePropertyValue(style, krb.PropIDMaxWidth); ok {
					if w, valOk := getShortValue(prop); valOk {
						baseW = scaled(w)
					}
				}
			}
			if baseH == 0 { // Check style for height only if not set by header
				// KRB convention is MaxHeight in styles for explicit size
				if prop, ok := getStylePropertyValue(style, krb.PropIDMaxHeight); ok {
					if h, valOk := getShortValue(prop); valOk {
						baseH = scaled(h)
					}
				}
			}
		}
	}

	// 3. Intrinsic Content Size (Lowest priority)
	// Calculate size based on text or loaded image if size wasn't determined above.
	if baseW == 0 || baseH == 0 {
		intrinsicW, intrinsicH := 0, 0
		if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
			// Calculate size based on text rendering
			fontSize := int32(math.Max(1, math.Round(baseFontSize*float64(scale)))) // Use scaled base font size
			textWidthMeasured := rl.MeasureText(el.Text, fontSize)
			// Add border space to intrinsic size calculation
			hPadding := scaledU8(el.BorderWidths[1]) + scaledU8(el.BorderWidths[3]) // Right + Left borders
			vPadding := scaledU8(el.BorderWidths[0]) + scaledU8(el.BorderWidths[2]) // Top + Bottom borders
			intrinsicW = int(textWidthMeasured) + hPadding
			intrinsicH = int(fontSize) + vPadding // Approximate height based on font size + borders
		} else if el.Header.Type == krb.ElemTypeImage && el.TextureLoaded {
			// Use scaled size of the loaded texture
			intrinsicW = int(math.Round(float64(el.Texture.Width) * float64(scale)))
			intrinsicH = int(math.Round(float64(el.Texture.Height) * float64(scale)))
		}
		// Apply intrinsic size only if base size is still zero
		if baseW == 0 {
			baseW = intrinsicW
		}
		if baseH == 0 {
			baseH = intrinsicH
		}
	}
	return baseW, baseH
}

// fitToChildren adjusts the RenderW/H of an element (eligible for auto-sizing)
// to tightly contain its flow children.
func fitToChildren(el *render.RenderElement, clientAbsX, clientAbsY, borderL, borderT, borderR, borderB int, shouldAutoSizeW, shouldAutoSizeH bool) {
	maxChildRelXExtent := 0 // Farthest right edge relative to client origin
	maxChildRelYExtent := 0 // Farthest bottom edge relative to client origin
	hasFlowChildren := false

	for _, child := range el.Children {
		if !isEffectivelyAbsolute(child) { // Only consider flow children
			hasFlowChildren = true
			// Calculate child's position relative to the parent's client area
			relX := child.RenderX - clientAbsX
			relY := child.RenderY - clientAbsY
			// Determine the extent (position + size)
			xExtent := relX + child.RenderW
			yExtent := relY + child.RenderH
			// Update maximum extents
			if xExtent > maxChildRelXExtent {
				maxChildRelXExtent = xExtent
			}
			if yExtent > maxChildRelYExtent {
				maxChildRelYExtent = yExtent
			}
		}
	}

	if hasFlowChildren {
		// Calculate the required width/height including borders
		newW := maxChildRelXExtent + borderL + borderR
		newH := maxChildRelYExtent + borderT + borderB

		// Apply the new size ONLY if the dimension was marked for auto-sizing
		if shouldAutoSizeW {
			el.RenderW = max(1, newW) // Ensure minimum size
		}
		if shouldAutoSizeH {
			el.RenderH = max(1, newH) // Ensure minimum size
		}
	}
	// If no flow children, size remains as initially calculated
}

// calculateAlignmentOffsets determines the starting offset and spacing between elements
// for different alignment modes in flow layout.
func calculateAlignmentOffsets(alignment uint8, availableSpace, totalUsedSpace, childCount int, isReversed bool) (startOffset, spacing int) {
	unusedSpace := max(0, availableSpace-totalUsedSpace) // Space left over
	startOffset = 0
	spacing = 0

	switch alignment {
	case krb.LayoutAlignCenter: // Center the block of children
		startOffset = unusedSpace / 2
	case krb.LayoutAlignEnd: // Align children to the end
		startOffset = MuxInt(isReversed, 0, unusedSpace) // If reversed, start is 0, otherwise push to end
	case krb.LayoutAlignSpaceBetween: // Distribute space between children
		if childCount > 1 {
			spacing = unusedSpace / (childCount - 1)
		}
		startOffset = 0 // First child starts at the beginning
	default: // krb.LayoutAlignStart
		startOffset = MuxInt(isReversed, unusedSpace, 0) // If reversed, push to end, otherwise start at 0
	}
	return startOffset, spacing
}

// calculateCrossAxisOffset determines the offset needed to align a child
// along the cross axis (e.g., vertical alignment in a row, horizontal in a column).
func calculateCrossAxisOffset(alignment uint8, crossAxisSize, childCrossSize int) int {
	switch alignment {
	case krb.LayoutAlignCenter: // Center child in cross axis space
		return (crossAxisSize - childCrossSize) / 2
	case krb.LayoutAlignEnd: // Align child to the end of cross axis space
		return crossAxisSize - childCrossSize
	default: // krb.LayoutAlignStart or krb.LayoutAlignSpaceBetween (treat like Start for cross axis)
		return 0 // Align child to the start of cross axis space
	}
}

// clampOpposingBorders adjusts border sizes if they exceed the total available dimension.
func clampOpposingBorders(borderA, borderB, totalSize int) (int, int) {
	// If borders combined are larger than the element, shrink them proportionally
	if borderA+borderB >= totalSize {
		// Try to keep half, ensuring non-negative
		borderA = max(0, min(totalSize/2, borderA))
		borderB = max(0, totalSize-borderA) // Give remaining space to B
	}
	return borderA, borderB
}

// drawBorders draws the four borders of an element.
func drawBorders(x, y, w, h, top, right, bottom, left int, color rl.Color) {
	// Top border
	if top > 0 {
		rl.DrawRectangle(int32(x), int32(y), int32(w), int32(top), color)
	}
	// Bottom border
	if bottom > 0 {
		rl.DrawRectangle(int32(x), int32(y+h-bottom), int32(w), int32(bottom), color)
	}
	// Calculate vertical position and height for side borders (inside top/bottom)
	sideY := y + top
	sideH := max(0, h-top-bottom)
	// Left border
	if left > 0 {
		rl.DrawRectangle(int32(x), int32(sideY), int32(left), int32(sideH), color)
	}
	// Right border
	if right > 0 {
		rl.DrawRectangle(int32(x+w-right), int32(sideY), int32(right), int32(sideH), color)
	}
}

// calculateContentArea determines the rectangle inside the element's borders.
func calculateContentArea(x, y, w, h, top, right, bottom, left int) (cx, cy, cw, ch int) {
	cx = x + left // Content X starts after left border
	cy = y + top  // Content Y starts after top border
	cw = max(0, w-left-right) // Content width is total width minus side borders
	ch = max(0, h-top-bottom) // Content height is total height minus top/bottom borders
	return cx, cy, cw, ch
}

// drawContent draws the specific content (text or image) within the provided content bounds.
func drawContent(el *render.RenderElement, cx, cy, cw, ch int, scale float32, fgColor rl.Color) {
	// --- Draw Text ---
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		// Use scaled base font size (consider making font size a property)
		fontSize := int32(math.Max(1, math.Round(baseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)

		// Calculate text draw position based on alignment
		textDrawX := cx
		switch el.TextAlignment {
		case krb.LayoutAlignCenter: // Center
			textDrawX = cx + (cw-int(textWidthMeasured))/2
		case krb.LayoutAlignEnd: // End (Right)
			textDrawX = cx + cw - int(textWidthMeasured)
			// Default is Start (Left) which is cx
		}
		// Simple vertical centering
		textDrawY := cy + (ch-int(fontSize))/2

		// Draw the text clipped to the content area (clipping done by caller)
		rl.DrawText(el.Text, int32(textDrawX), int32(textDrawY), fontSize, fgColor)
	}

	// --- Draw Image ---
	// Draw if it's an Image/Button and the texture was successfully loaded
	if (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) && el.TextureLoaded {
		// Define source rectangle (full texture)
		sourceRec := rl.NewRectangle(0, 0, float32(el.Texture.Width), float32(el.Texture.Height))
		// Define destination rectangle (the content area)
		destRec := rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch))
		// Define origin for rotation/scaling (top-left)
		origin := rl.NewVector2(0, 0)
		// Draw the texture stretched/scaled into the destination rectangle
		rl.DrawTexturePro(el.Texture, sourceRec, destRec, origin, 0.0, rl.White) // Tint white (no tint)
	}
}