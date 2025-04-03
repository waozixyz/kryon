// render/raylib/raylib_renderer.go
package raylib

import (
	"encoding/binary"
	"os"
	"log"
	"math"
	"path/filepath" 
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

// RaylibRenderer implements the render.Renderer interface using raylib-go.
type RaylibRenderer struct {
	config         render.WindowConfig
	elements       []render.RenderElement // Flat list of all processed elements
	roots          []*render.RenderElement
	loadedTextures map[uint8]rl.Texture2D // Map resource index to texture
	krbFileDir     string               // Directory of the loaded KRB file for relative paths
	scaleFactor    float32              // Store effective scale factor
	docRef         *krb.Document        // Reference to the parsed document needed by helpers
	eventHandlerMap map[string]func()
}

// NewRaylibRenderer creates a new Raylib renderer instance.
func NewRaylibRenderer() *RaylibRenderer {
	return &RaylibRenderer{
		loadedTextures: make(map[uint8]rl.Texture2D),
		scaleFactor:    1.0, 
        eventHandlerMap: make(map[string]func()),
	}
}

// Init initializes the Raylib window.
func (r *RaylibRenderer) Init(config render.WindowConfig) error {
	r.config = config
	r.scaleFactor = config.ScaleFactor // Store from config
	log.Printf("Raylib Init: Window %dx%d Title: '%s' Scale: %.2f", config.Width, config.Height, config.Title, config.ScaleFactor)
	rl.InitWindow(int32(config.Width), int32(config.Height), config.Title)
	if config.Resizable {
		rl.SetWindowState(rl.FlagWindowResizable)
	}
	rl.SetTargetFPS(60)
	// SetExitKey(0) // Optional: disable ESC closing window if handling manually
	return nil
}
// render/raylib/raylib_renderer.go

// Replace the ENTIRE PrepareTree function with this:
func (r *RaylibRenderer) PrepareTree(doc *krb.Document, krbFilePath string) ([]*render.RenderElement, render.WindowConfig, error) {
	if doc == nil || doc.Header.ElementCount == 0 {
		log.Println("PrepareTree: No elements in document.")
		return nil, r.config, nil
	}
	r.docRef = doc

	// Determine base directory for resolving EXTERNAL resources.
	// If KRB is embedded, krbFilePath might be "." or empty. Use CWD.
	// If KRB is loaded from file, use that file's directory.
	if krbFilePath == "." || krbFilePath == "" {
        var err error
        r.krbFileDir, err = os.Getwd() // Use current working dir as base
        if err != nil {
             log.Printf("WARN PrepareTree: Could not get working directory for resource path: %v", err)
             r.krbFileDir = "." // Fallback
        }
    } else { // A potentially real path was provided
        var err error
	    // Try to get absolute directory of the provided path
        absDir, err := filepath.Abs(filepath.Dir(krbFilePath))
	    if err != nil {
		    log.Printf("WARN PrepareTree: Failed to get absolute path for KRB directory '%s': %v. Using relative.", krbFilePath, err)
            r.krbFileDir = filepath.Dir(krbFilePath) // Use relative as fallback
	    } else {
            r.krbFileDir = absDir
        }
    }
	log.Printf("PrepareTree: Base Directory for Resources: %s", r.krbFileDir)


	r.elements = make([]render.RenderElement, doc.Header.ElementCount)
	r.roots = nil // Reset roots

	// --- Initialize Defaults ---
	windowConfig := render.DefaultWindowConfig()
	windowConfig.DefaultBg = rl.Black
	defaultFg := rl.RayWhite
	defaultBorder := rl.Gray
	defaultBorderWidth := uint8(0)
	defaultTextAlign := uint8(0)

	// --- Pass 1: Process App Element and Initialize Defaults ---
	var appElement *render.RenderElement
	if (doc.Header.Flags&krb.FlagHasApp) != 0 && doc.Header.ElementCount > 0 && doc.Elements[0].Type == krb.ElemTypeApp {
		appElement = &r.elements[0]
		appElement.Header = doc.Elements[0]
		appElement.OriginalIndex = 0

		style, ok := findStyle(doc, appElement.Header.StyleID)
		if ok {
			applyStyleProperties(style.Properties, doc, &windowConfig.DefaultBg, &defaultFg, &defaultBorder, &defaultBorderWidth, &defaultTextAlign)
		} else if appElement.Header.StyleID != 0 { log.Printf("Warning: App element references invalid Style ID %d", appElement.Header.StyleID) }

		if len(doc.Properties) > 0 && len(doc.Properties[0]) > 0 {
			applyDirectPropertiesToConfig(doc.Properties[0], doc, &windowConfig)
			applyDirectPropertiesToElement(doc.Properties[0], doc, appElement)
		}

		appElement.BgColor = windowConfig.DefaultBg
		appElement.FgColor = defaultFg
		appElement.BorderColor = defaultBorder
		appElement.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth}
		appElement.TextAlignment = defaultTextAlign

		r.scaleFactor = windowConfig.ScaleFactor

		log.Printf("PrepareTree: Processed App. Window:%dx%d Title:'%s' Scale:%.2f Resizable:%t",
			windowConfig.Width, windowConfig.Height, windowConfig.Title, windowConfig.ScaleFactor, windowConfig.Resizable)

	} else { log.Println("PrepareTree: No App element found, using default window config.") }
	r.config = windowConfig

	// --- Pass 2: Initialize remaining RenderElements ---
	for i := 0; i < int(doc.Header.ElementCount); i++ {
		currentEl := &r.elements[i]

		if appElement != nil && i == 0 {
            if currentEl.Header.Type == 0 {
                currentEl.Header = appElement.Header
                currentEl.OriginalIndex = 0
            }
		} else {
			currentEl.Header = doc.Elements[i]
			currentEl.OriginalIndex = i
			currentEl.BgColor = windowConfig.DefaultBg
			currentEl.FgColor = defaultFg
			currentEl.BorderColor = defaultBorder
			currentEl.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth}
			currentEl.TextAlignment = defaultTextAlign
		}

		currentEl.IsInteractive = (currentEl.Header.Type == krb.ElemTypeButton || currentEl.Header.Type == krb.ElemTypeInput)
		currentEl.ResourceIndex = render.InvalidResourceIndex

		style, ok := findStyle(doc, currentEl.Header.StyleID)
		if ok {
			applyStylePropertiesToElement(style.Properties, doc, currentEl)
		} else if currentEl.Header.StyleID != 0 && !(appElement != nil && i == 0) { log.Printf("Warning: Element %d references invalid Style ID %d", i, currentEl.Header.StyleID) }

		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			applyDirectPropertiesToElement(doc.Properties[i], doc, currentEl)
		}

        // *** ADDED: Resolve Event Handlers ***
        currentEl.EventHandlers = nil // Ensure slice is empty initially
        if doc.Events != nil && currentEl.OriginalIndex < len(doc.Events) && doc.Events[currentEl.OriginalIndex] != nil {
            krbEvents := doc.Events[currentEl.OriginalIndex]
            for _, krbEvent := range krbEvents {
                if int(krbEvent.CallbackID) < len(doc.Strings) {
                    handlerName := doc.Strings[krbEvent.CallbackID]
                    // Assuming EventCallbackInfo is defined in the 'render' package
                    currentEl.EventHandlers = append(currentEl.EventHandlers, render.EventCallbackInfo{
                        EventType:   krbEvent.EventType,
                        HandlerName: handlerName,
                    })
                     log.Printf("DEBUG PREPARE: Elem %d assigned Event: Type=%d, Handler='%s'",
                         currentEl.OriginalIndex, krbEvent.EventType, handlerName)
                } else {
                     log.Printf("WARN PREPARE: Elem %d has invalid event callback string index %d",
                         currentEl.OriginalIndex, krbEvent.CallbackID)
                }
            }
        }
        // *** END: Resolve Event Handlers ***

		resolveElementText(doc, currentEl, style, ok)
		resolveElementImageSource(doc, currentEl, style, ok)

	} // End loop initializing elements

	// --- Pass 3: Build Parent/Child Tree ---
	log.Println("PrepareTree: Building element tree...")
	parentStack := make([]*render.RenderElement, 0, 10) // Using slice as stack
	for i := 0; i < int(doc.Header.ElementCount); i++ {
		currentEl := &r.elements[i]
		for len(parentStack) > 0 {
			parent := parentStack[len(parentStack)-1]
			// Check original header child count
			if len(parent.Children) >= int(parent.Header.ChildCount) {
				parentStack = parentStack[:len(parentStack)-1] // Pop
			} else {
				break
			}
		}
		if len(parentStack) > 0 {
			parent := parentStack[len(parentStack)-1]
			currentEl.Parent = parent
			parent.Children = append(parent.Children, currentEl) // Add child
		} else {
			r.roots = append(r.roots, currentEl) // Add root
		}
		if currentEl.Header.ChildCount > 0 {
			parentStack = append(parentStack, currentEl) // Push if expects children
		}
	}
	log.Printf("PrepareTree: Finished building tree. Found %d root(s).", len(r.roots))
	if len(r.roots) == 0 && doc.Header.ElementCount > 0 { log.Println("ERROR: No root elements found, but elements exist!") }


	// --- Pass 4: Load Textures ---
	log.Println("PrepareTree: Loading textures...")
	for i := range r.elements {
		el := &r.elements[i]
		if el.Header.Type == krb.ElemTypeImage && el.ResourceIndex != render.InvalidResourceIndex {
			if int(el.ResourceIndex) >= len(doc.Resources) { log.Printf("ERROR: Elem %d invalid resource index %d", el.OriginalIndex, el.ResourceIndex); continue }
			res := doc.Resources[el.ResourceIndex]
			if _, exists := r.loadedTextures[el.ResourceIndex]; exists { el.Texture = r.loadedTextures[el.ResourceIndex]; el.TextureLoaded = true; continue }

			if res.Format == krb.ResFormatExternal {
				if int(res.NameIndex) >= len(doc.Strings) { log.Printf("ERROR: Res %d invalid name index %d", el.ResourceIndex, res.NameIndex); continue }
				resourceName := doc.Strings[res.NameIndex]
				fullPath := filepath.Join(r.krbFileDir, resourceName) // Use resolved base path
				texture := rl.LoadTexture(fullPath)
				if rl.IsTextureReady(texture) { el.Texture = texture; el.TextureLoaded = true; r.loadedTextures[el.ResourceIndex] = texture; log.Printf("  Loaded texture Elem %d (Res %d): '%s' -> OK", el.OriginalIndex, el.ResourceIndex, fullPath) } else { log.Printf("  FAILED loading texture: %s", fullPath); el.TextureLoaded = false }
			} else if res.Format == krb.ResFormatInline {
                log.Printf("  Loading inline texture Elem %d (Res %d) Size: %d", el.OriginalIndex, el.ResourceIndex, res.InlineDataSize)
				if res.InlineData != nil && res.InlineDataSize > 0 {
					ext := ".png" // Default guess
                    if int(res.NameIndex) < len(doc.Strings) && doc.Strings[res.NameIndex] != "" {
                        ext = filepath.Ext(doc.Strings[res.NameIndex])
                    }
					img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData)))
					if rl.IsImageReady(img) {
						texture := rl.LoadTextureFromImage(img)
						rl.UnloadImage(img)
						if rl.IsTextureReady(texture) { el.Texture = texture; el.TextureLoaded = true; r.loadedTextures[el.ResourceIndex] = texture; log.Printf("    -> OK (ID: %d, %dx%d)", texture.ID, texture.Width, texture.Height)} else { log.Printf("    -> FAILED creating texture from inline image"); el.TextureLoaded = false }
					} else { log.Printf("    -> FAILED loading inline image from memory (format '%s')", ext); el.TextureLoaded = false }
				} else { log.Printf("    -> FAILED: Inline data is nil or zero size."); el.TextureLoaded = false }
			} else { log.Printf("WARN: Unknown resource format %d for image resource %d", res.Format, el.ResourceIndex) }
		}
	}
	log.Println("PrepareTree: Finished loading textures.")

	return r.roots, r.config, nil
}

// GetRenderTree returns the flat list of processed elements.
func (r *RaylibRenderer) GetRenderTree() []*render.RenderElement {
	pointers := make([]*render.RenderElement, len(r.elements))
	for i := range r.elements { pointers[i] = &r.elements[i] }
	return pointers
}

// RenderFrame performs layout calculation and then draws the tree.
func (r *RaylibRenderer) RenderFrame(roots []*render.RenderElement) {
	windowResized := rl.IsWindowResized()
	currentWidth := r.config.Width
	currentHeight := r.config.Height
	if windowResized {
		currentWidth = int(rl.GetScreenWidth())
		currentHeight = int(rl.GetScreenHeight())
		r.config.Width = currentWidth
		r.config.Height = currentHeight
		log.Printf("Window resized to %dx%d", currentWidth, currentHeight)
	}

	// --- Layout Pass ---
	for _, root := range roots {
		PerformLayout(root, 0, 0, currentWidth, currentHeight, r.scaleFactor, r.docRef) // Pass docRef down
	}

	// --- Draw Pass ---
	for _, root := range roots {
		renderElementRecursive(root, r.scaleFactor)
	}
}


// Keep the isEffectivelyAbsolute helper function:
func isEffectivelyAbsolute(el *render.RenderElement) bool {
	return el.Header.LayoutAbsolute() || el.Header.PosX != 0 || el.Header.PosY != 0
}


// REPLACE THIS ENTIRE FUNCTION:
func PerformLayout(el *render.RenderElement, parentContentX, parentContentY, parentContentW, parentContentH int, scale float32, doc *krb.Document) {
	if el == nil {
		return
	}

	isRoot := (el.Parent == nil)
	scaled := func(val uint16) int { return int(math.Round(float64(val) * float64(scale))) }
	scaledU8 := func(val uint8) int { return int(math.Round(float64(val) * float64(scale))) }

	// --- 1. Calculate Intrinsic Size ---
	el.IntrinsicW = 0
	el.IntrinsicH = 0
	// (Intrinsic calculation logic remains the same as previous answer)
	intrinsicW := 0
	intrinsicH := 0
	style, styleOk := findStyle(doc, el.Header.StyleID)
	if styleOk {
		if el.Header.Width == 0 { if styleMaxWProp, propOk := getStylePropertyValue(style, krb.PropIDMaxWidth); propOk { if w, valOk := getShortValue(styleMaxWProp); valOk { intrinsicW = int(w) } } }
		if el.Header.Height == 0 { if styleMaxHProp, propOk := getStylePropertyValue(style, krb.PropIDMaxHeight); propOk { if h, valOk := getShortValue(styleMaxHProp); valOk { intrinsicH = int(h) } } }
	}
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		fontSize := int32(math.Max(1, math.Round(render.BaseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		hPadding := scaledU8(el.BorderWidths[1]) + scaledU8(el.BorderWidths[3])
		vPadding := scaledU8(el.BorderWidths[0]) + scaledU8(el.BorderWidths[2])
		if intrinsicW == 0 { intrinsicW = int(textWidthMeasured) + hPadding }
		if intrinsicH == 0 { intrinsicH = int(fontSize) + vPadding }
	} else if el.Header.Type == krb.ElemTypeImage && el.TextureLoaded {
		if intrinsicW == 0 { intrinsicW = int(el.Texture.Width) }
		if intrinsicH == 0 { intrinsicH = int(el.Texture.Height) }
	}
	el.IntrinsicW = int(math.Round(float64(intrinsicW) * float64(scale)))
	el.IntrinsicH = int(math.Round(float64(intrinsicH) * float64(scale)))


	// --- 2. Determine Render Size ---
	intrinsicW = el.IntrinsicW // Use the value calculated in Step 1
	intrinsicH = el.IntrinsicH // Use the value calculated in Step 1

	finalW := intrinsicW
	if el.Header.Width > 0 { finalW = scaled(el.Header.Width) }
	finalH := intrinsicH
	if el.Header.Height > 0 { finalH = scaled(el.Header.Height) }

	// Determine if this element *should* grow based on its own flag and context
	// Note: isEffectivelyAbsolute also checks PosX/Y != 0, which shouldn't prevent growth usually.
	// We primarily care about the LayoutAbsoluteBit for preventing flow/growth participation.
	isFlowElement := !el.Header.LayoutAbsolute()
	shouldGrow := el.Header.LayoutGrow() && isFlowElement

	parentDir := krb.LayoutDirRow // Default if no parent (e.g., root)
	if el.Parent != nil {
		parentDir = el.Parent.Header.LayoutDirection()
	}

	if isRoot {
		// Root element ALWAYS takes the full parent size (initial window size)
		finalW = parentContentW
		finalH = parentContentH
		log.Printf("DEBUG LAYOUT ROOT: Elem %d (Root) Size forced to Parent: %dx%d", el.OriginalIndex, finalW, finalH)
	} else { // --- Non-root element logic ---
		if shouldGrow {
			log.Printf("DEBUG LAYOUT GROW: Elem %d applying growth in ParentDir %d. Parent Avail: %dx%d", el.OriginalIndex, parentDir, parentContentW, parentContentH)
			// If growing, PREEMPTIVELY take available parent space along the parent's main axis
			// ONLY IF the dimension is auto-sized (0).
			if parentDir == krb.LayoutDirRow || parentDir == krb.LayoutDirRowReverse {
				if el.Header.Width == 0 { // Grow width if auto-width
					finalW = parentContentW
					log.Printf("  -> Growing Width to Parent: %d", finalW)
				}
				// Growing elements often fill the cross-axis too, if auto-sized
				if el.Header.Height == 0 {
					finalH = parentContentH
					log.Printf("  -> Growing Height (Cross-Axis) to Parent: %d", finalH)
				}

			} else { // Column or ColumnReverse
				if el.Header.Height == 0 { // Grow height if auto-height
					finalH = parentContentH
					log.Printf("  -> Growing Height to Parent: %d", finalH)
				}
				// Growing elements often fill the cross-axis too, if auto-sized
				if el.Header.Width == 0 {
					finalW = parentContentW
					log.Printf("  -> Growing Width (Cross-Axis) to Parent: %d", finalW)
				}
			}
			// Clamp negative sizes potentially introduced by border calc later? No, do clamping after final size.
		}

		// Clamp size to parent if NOT growing and part of the flow
		if !shouldGrow && isFlowElement {
			// Clamp calculated size (intrinsic or explicit) to parent's content area
			originalW := finalW
			originalH := finalH
			if finalW > parentContentW { finalW = parentContentW }
			if finalH > parentContentH { finalH = parentContentH }
			if finalW != originalW || finalH != originalH {
				log.Printf("DEBUG LAYOUT CLAMP: Elem %d (Non-Grow) clamped from %dx%d to %dx%d (Parent Avail: %dx%d)", el.OriginalIndex, originalW, originalH, finalW, finalH, parentContentW, parentContentH)
			}
		}
		// Absolute elements keep their calculated size (intrinsic or explicit), their position is handled later.
	} // --- End non-root element logic ---


	// Ensure minimum 1x1 size AFTER growth/clamping is applied
	// Important: Check *before* assigning to el.RenderW/H
	finalWBeforeMin := finalW
	finalHBeforeMin := finalH
	if finalW <= 0 { finalW = 1 }
	if finalH <= 0 { finalH = 1 }
	if finalW != finalWBeforeMin || finalH != finalHBeforeMin {
		log.Printf("DEBUG LAYOUT MINSIZE: Elem %d adjusted from %dx%d to %dx%d", el.OriginalIndex, finalWBeforeMin, finalHBeforeMin, finalW, finalH)
	}

	// Store the final calculated size for rendering and child layout
	el.RenderW = finalW
	el.RenderH = finalH

	// Log final decision for this element before laying out children
	log.Printf("DEBUG LAYOUT SIZE: Elem %d (Type %d) ParentAvail=(%dx%d) Intrinsic=(%d,%d) HeaderSize=(%d,%d) Grow=%t Final RenderSize=(%d,%d)",
		el.OriginalIndex, el.Header.Type, parentContentW, parentContentH, el.IntrinsicW, el.IntrinsicH, el.Header.Width, el.Header.Height, el.Header.LayoutGrow(), el.RenderW, el.RenderH)

		

	// --- 3. Determine Render Position ---
	finalX := 0
	finalY := 0
	if isEffectivelyAbsolute(el) {
		// Absolute positioning based on el.Header.PosX/Y
		finalX = parentContentX + scaled(el.Header.PosX)
		finalY = parentContentY + scaled(el.Header.PosY)
	} else if el.Parent != nil {
		// Flow element position determined by parent's PerformLayoutChildren.
        // Initialize based on parent origin; parent will overwrite.
		finalX = parentContentX
		finalY = parentContentY
	} else { // isRoot && !isEffectivelyAbsolute(el)
		// Root flow element: Position using alignment within window bounds.
        // NOTE: Root is now fixed size (window size), so alignment affects position.
		alignment := el.Header.LayoutAlignment()
		switch alignment {
		case krb.LayoutAlignCenter: // Centering a full-size root has no effect on position
			finalX = parentContentX
			finalY = parentContentY
		case krb.LayoutAlignEnd: // Aligning a full-size root also has no effect
			finalX = parentContentX
			finalY = parentContentY
		default: // Start alignment
			finalX = parentContentX
			finalY = parentContentY
		}
	}
	el.RenderX = finalX
	el.RenderY = finalY


    // --- 4. Layout Children ---
    if el.Header.ChildCount > 0 {
        // Calculate this element's client area (absolute coords and size)
        borderLeft := scaledU8(el.BorderWidths[3])
        borderTop := scaledU8(el.BorderWidths[0])
        borderRight := scaledU8(el.BorderWidths[1])
        borderBottom := scaledU8(el.BorderWidths[2])

        clientAbsX := el.RenderX + borderLeft
        clientAbsY := el.RenderY + borderTop
        // Use the FINAL RenderW/H calculated in Step 2
        clientWidth := el.RenderW - borderLeft - borderRight
        clientHeight := el.RenderH - borderTop - borderBottom

        if clientWidth < 0 { clientWidth = 0 }
        if clientHeight < 0 { clientHeight = 0 }

        // Call PerformLayoutChildren with the correct client area bounds
        PerformLayoutChildren(el, clientAbsX, clientAbsY, clientWidth, clientHeight, scale, doc)

        // --- 5. Update Auto-Sized Container Size - **SKIP FOR ROOT** ---
        // Resize this element ONLY if it's NOT the root, auto-sized, and part of the flow.
        isFlowParent := !isEffectivelyAbsolute(el)
        isAutoSizeW := el.Header.Width == 0
        isAutoSizeH := el.Header.Height == 0

        if !isRoot && isFlowParent && (isAutoSizeW || isAutoSizeH) {
            // (Auto-sizing logic for non-root containers remains the same as previous answer)
             hasFlowChildren := false
            for _, child := range el.Children { if !isEffectivelyAbsolute(child) { hasFlowChildren = true; break } }

            if hasFlowChildren {
                maxChildRelXExtent := 0
                maxChildRelYExtent := 0
                for _, child := range el.Children {
                    if !isEffectivelyAbsolute(child) {
                        relX := child.RenderX - clientAbsX
                        relY := child.RenderY - clientAbsY
                        xExtent := relX + child.RenderW
                        yExtent := relY + child.RenderH
                        if xExtent > maxChildRelXExtent { maxChildRelXExtent = xExtent }
                        if yExtent > maxChildRelYExtent { maxChildRelYExtent = yExtent }
                    }
                }
                newW := maxChildRelXExtent + borderLeft + borderRight
                newH := maxChildRelYExtent + borderTop + borderBottom

                if isAutoSizeW { el.RenderW = max(1, newW) }
                if isAutoSizeH { el.RenderH = max(1, newH) }
                // Repositioning non-root elements after resize is complex and often requires a second layout pass.
                // We omit it here for simplicity, assuming the initial layout provides enough space.
            }
        }
    } // End if ChildCount > 0

} // End PerformLayout

func PerformLayoutChildren(parent *render.RenderElement, parentClientOriginX, parentClientOriginY, availableW, availableH int, scale float32, doc *krb.Document) {
	if len(parent.Children) == 0 {
		return
	}

	flowChildren := make([]*render.RenderElement, 0, len(parent.Children))
	absoluteChildren := make([]*render.RenderElement, 0)
	for _, child := range parent.Children {
		// Use the helper function to sort based on effective positioning mode
		if isEffectivelyAbsolute(child) {
			absoluteChildren = append(absoluteChildren, child)
		} else {
			flowChildren = append(flowChildren, child)
		}
	}

	// --- Layout Flow Children ---
	if len(flowChildren) > 0 {
		direction := parent.Header.LayoutDirection()
		alignment := parent.Header.LayoutAlignment()

		totalFlowIntrinsicSize := 0
		totalGrowFactor := 0
		growChildren := make([]*render.RenderElement, 0)

		// Pass 1: Call PerformLayout for each flow child first.
		// This calculates their intrinsic/header-based size (RenderW/H)
		// constrained ONLY by the availableW/H of the parent's client area.
		// We pass the parent's client area as the 'parent content box' for the child.
		for _, child := range flowChildren {
			PerformLayout(child, parentClientOriginX, parentClientOriginY, availableW, availableH, scale, doc)
			// Now child.RenderW/H hold the size the child wants, clamped by available space.

			if child.Header.LayoutGrow() {
				totalGrowFactor++
				growChildren = append(growChildren, child)
			} else {
				if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse {
					totalFlowIntrinsicSize += child.RenderW // Use the calculated RenderW
				} else {
					totalFlowIntrinsicSize += child.RenderH // Use the calculated RenderH
				}
			}
		}

		// Determine extra space and distribute grow size
		availableSpace := 0
		crossAxisSize := 0
		if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse {
			availableSpace = availableW
			crossAxisSize = availableH // Height is cross-axis for Row
		} else {
			availableSpace = availableH
			crossAxisSize = availableW // Width is cross-axis for Column
		}

		extraSpace := availableSpace - totalFlowIntrinsicSize
		if extraSpace < 0 {
			extraSpace = 0
		}

		growSizePerChild := 0
		remainderForLast := 0
		if totalGrowFactor > 0 && extraSpace > 0 {
			growSizePerChild = extraSpace / totalGrowFactor
			remainderForLast = extraSpace % totalGrowFactor
		}

		// Recalculate RenderW/H for growing children and sum total final size
		totalFinalSize := totalFlowIntrinsicSize
		tempGrowCount := 0
		for _, child := range growChildren {
			growAmount := growSizePerChild
			tempGrowCount++
			if tempGrowCount == totalGrowFactor { growAmount += remainderForLast }

			if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse {
				child.RenderW += growAmount          // Add grow space to width
				child.RenderH = crossAxisSize        // Expand height to fill cross-axis
				if child.RenderW < 0 { child.RenderW = 0 }
				if child.RenderH < 0 { child.RenderH = 0 } // Clamp size
				totalFinalSize += child.RenderW - (child.RenderW - growAmount) // Add actual size used by grown child
			} else {
				child.RenderH += growAmount          // Add grow space to height
				child.RenderW = crossAxisSize        // Expand width to fill cross-axis
                if child.RenderH < 0 { child.RenderH = 0 }
                if child.RenderW < 0 { child.RenderW = 0 } // Clamp size
				totalFinalSize += child.RenderH - (child.RenderH - growAmount) // Add actual size used
			}
		}
        if totalFinalSize > availableSpace { totalFinalSize = availableSpace } // Clamp total size

		// Calculate alignment offsets
		startOffset := 0
		spacing := 0
		switch alignment {
		case krb.LayoutAlignCenter: startOffset = (availableSpace - totalFinalSize) / 2
		case krb.LayoutAlignEnd:    startOffset = availableSpace - totalFinalSize
		case krb.LayoutAlignSpaceBetween:
			if len(flowChildren) > 1 {
				remainingSpace := availableSpace - totalFinalSize
				if remainingSpace > 0 { spacing = remainingSpace / (len(flowChildren) - 1) }
			}
			startOffset = 0
		default: startOffset = 0 // Start alignment
		}
		if startOffset < 0 { startOffset = 0 }

		// Pass 2: Position Flow Children
		currentFlowPos := startOffset // Tracks position along main axis, relative to parent client origin
		for i, child := range flowChildren {
			childW := child.RenderW // Use final RenderW/H
			childH := child.RenderH

			childX := 0 // Final absolute screen X
			childY := 0 // Final absolute screen Y

			// Position based on main axis flow
			if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse {
				childX = parentClientOriginX + currentFlowPos
				childY = parentClientOriginY // Start cross-axis alignment at top
				currentFlowPos += childW     // Advance main axis position
			} else { // Column
				childX = parentClientOriginX // Start cross-axis alignment at left
				childY = parentClientOriginY + currentFlowPos
				currentFlowPos += childH     // Advance main axis position
			}

			// Apply cross-axis alignment within available cross-axis space
			parentCrossAlignment := alignment // Assuming parent alignment applies to cross-axis too
			if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse { // Cross is Y, space is availableH
				switch parentCrossAlignment {
				case krb.LayoutAlignCenter: childY = parentClientOriginY + (availableH - childH) / 2
				case krb.LayoutAlignEnd:    childY = parentClientOriginY + availableH - childH
				// default: childY is already parentClientOriginY (Start)
				}
			} else { // Cross is X, space is availableW
				switch parentCrossAlignment {
				case krb.LayoutAlignCenter: childX = parentClientOriginX + (availableW - childW) / 2
				case krb.LayoutAlignEnd:    childX = parentClientOriginX + availableW - childW
				// default: childX is already parentClientOriginX (Start)
				}
			}

			// Set the final absolute screen position for the child
			child.RenderX = childX
			child.RenderY = childY

			// Add spacing for SpaceBetween
			if alignment == krb.LayoutAlignSpaceBetween && i < len(flowChildren)-1 {
				currentFlowPos += spacing
			}
		}
	} // End if len(flowChildren) > 0

	// --- Layout Absolute Children ---
	// Call PerformLayout for them. Step 5 within their PerformLayout call
	// will use isEffectivelyAbsolute() and correctly position them using their
	// PosX/PosY relative to the parent's content box origin passed down.
	// We need to pass the parent's *original* content box origin here for correct absolute positioning.
	// Let's recalculate the parent's original content box origin.
    // parentClientOriginX = parent->RenderX + parent->BorderLeft
    // parentClientOriginY = parent->RenderY + parent->BorderTop
    // So, parentContentX = parentClientOriginX - parent->BorderLeft etc.
    // This requires knowing parent's borders... or rethinking the parameters.
    // Let's revert to passing the parent's original content box details down if needed.
    // Sticking with the simpler approach: PerformLayout handles the positioning logic based on isEffectivelyAbsolute.
	scaledU8 := func(val uint8) int { return int(math.Round(float64(val) * float64(scale))) }
    parentContentX := parentClientOriginX - scaledU8(parent.BorderWidths[3]) // Approx parent content X
    parentContentY := parentClientOriginY - scaledU8(parent.BorderWidths[0]) // Approx parent content Y

	for _, child := range absoluteChildren {
		// Pass the parent's content box origin and size as the reference area
		PerformLayout(child, parentContentX, parentContentY, availableW, availableH, scale, doc)
	}
}

func (r *RaylibRenderer) RegisterEventHandler(name string, handler func()) {
    if name == "" || handler == nil {
        log.Printf("WARN RENDERER: Attempted to register invalid event handler ('%s')", name)
        return
    }
    log.Printf("DEBUG RENDERER: Registering event handler '%s'", name)
    r.eventHandlerMap[name] = handler
}


func renderElementRecursive(el *render.RenderElement, scale float32) {
	if el == nil {
		return
	}

	// Use pre-calculated RenderX/Y/W/H
	renderX := el.RenderX
	renderY := el.RenderY
	renderW := el.RenderW
	renderH := el.RenderH

	if renderW <= 0 || renderH <= 0 {
		// Skip drawing zero-sized elements, but still recurse for potentially visible children (e.g., absolute positioned)
		// log.Printf("DEBUG DRAW SKIP: Elem %d (Type %d) - Zero render size (%dx%d)", el.OriginalIndex, el.Header.Type, renderW, renderH)
	} else {
		// --- Only draw the element itself if it has size ---
		scaledU8 := func(val uint8) int { return int(math.Round(float64(val) * float64(scale))) }

		bgColor := el.BgColor
		fgColor := el.FgColor
		borderColor := el.BorderColor
		topBW := scaledU8(el.BorderWidths[0])
		rightBW := scaledU8(el.BorderWidths[1])
		bottomBW := scaledU8(el.BorderWidths[2])
		leftBW := scaledU8(el.BorderWidths[3])

		// Clamp borders to prevent them from overlapping entirely
		if topBW+bottomBW >= renderH {
			// Allow at least 1 pixel if possible, otherwise 0
			topBW = max(0, min(renderH/2, topBW))
			bottomBW = max(0, renderH-topBW)
		}
		if leftBW+rightBW >= renderW {
			leftBW = max(0, min(renderW/2, leftBW))
			rightBW = max(0, renderW-leftBW)
		}

		// Draw Background *UNLESS* it's a Text Element
		// Text elements should be transparent by default, only drawing foreground text.
		// Containers, Buttons, Images etc. draw their background.
		if el.Header.Type != krb.ElemTypeText { // <<<--- FIX: Check element type
			rl.DrawRectangle(int32(renderX), int32(renderY), int32(renderW), int32(renderH), bgColor)
		}

		// Draw Borders (on top of background)
		if topBW > 0 {
			rl.DrawRectangle(int32(renderX), int32(renderY), int32(renderW), int32(topBW), borderColor)
		}
		if bottomBW > 0 {
			rl.DrawRectangle(int32(renderX), int32(renderY+renderH-bottomBW), int32(renderW), int32(bottomBW), borderColor)
		}
		// Calculate height for side borders correctly accounting for top/bottom borders
		sideBorderY := renderY + topBW
		sideBorderHeight := renderH - topBW - bottomBW
		if sideBorderHeight < 0 {
			sideBorderHeight = 0
		}
		if leftBW > 0 {
			rl.DrawRectangle(int32(renderX), int32(sideBorderY), int32(leftBW), int32(sideBorderHeight), borderColor)
		}
		if rightBW > 0 {
			rl.DrawRectangle(int32(renderX+renderW-rightBW), int32(sideBorderY), int32(rightBW), int32(sideBorderHeight), borderColor)
		}

		// Calculate Content Area (inside borders)
		contentX := renderX + leftBW
		contentY := renderY + topBW
		contentWidth := renderW - leftBW - rightBW
		contentHeight := renderH - topBW - bottomBW
		if contentWidth < 0 {
			contentWidth = 0
		}
		if contentHeight < 0 {
			contentHeight = 0
		}

		// Draw Content (Text or Image) within Scissor Rectangle
		if contentWidth > 0 && contentHeight > 0 {
			rl.BeginScissorMode(int32(contentX), int32(contentY), int32(contentWidth), int32(contentHeight))

			// Draw Text
			if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
				fontSize := int32(math.Max(1, math.Round(render.BaseFontSize*float64(scale))))
				textWidthMeasured := rl.MeasureText(el.Text, fontSize)
				textDrawX := contentX
				// Horizontal Alignment
				if el.TextAlignment == 1 { // Center
					textDrawX = contentX + (contentWidth-int(textWidthMeasured))/2
				} else if el.TextAlignment == 2 { // End
					textDrawX = contentX + contentWidth - int(textWidthMeasured)
				}
				// Vertical Alignment (Simple Center)
				textDrawY := contentY + (contentHeight-int(fontSize))/2

				// Clamp text position to be within content area (optional, prevents drawing outside bounds)
				// if textDrawX < contentX { textDrawX = contentX }
				// if textDrawY < contentY { textDrawY = contentY }
				// if textDrawX + int(textWidthMeasured) > contentX + contentWidth { /* Handle overflow? */ }
				// if textDrawY + int(fontSize) > contentY + contentHeight { /* Handle overflow? */ }

				rl.DrawText(el.Text, int32(textDrawX), int32(textDrawY), fontSize, fgColor)

			} else if el.Header.Type == krb.ElemTypeImage && el.TextureLoaded {
				sourceRec := rl.NewRectangle(0, 0, float32(el.Texture.Width), float32(el.Texture.Height))
				destRec := rl.NewRectangle(float32(contentX), float32(contentY), float32(contentWidth), float32(contentHeight))
				origin := rl.NewVector2(0, 0)
				// Use White tint to draw texture with its original colors
				rl.DrawTexturePro(el.Texture, sourceRec, destRec, origin, 0.0, rl.White)
			}

			rl.EndScissorMode()
		}
	} // --- End drawing the element itself ---

	// Recursively Draw Children (regardless of parent visibility/size)
	for _, child := range el.Children {
		renderElementRecursive(child, scale) // Pass scale down
	}
} // End renderElementRecursive
// --- Cleanup, ShouldClose, BeginFrame, EndFrame, PollEvents ---
func (r *RaylibRenderer) Cleanup() {
	log.Println("Raylib Cleanup: Unloading textures...")
	for idx, texture := range r.loadedTextures { log.Printf("  Unloading Res %d (TexID: %d)", idx, texture.ID); rl.UnloadTexture(texture) }
	r.loadedTextures = make(map[uint8]rl.Texture2D)
	if rl.IsWindowReady() { log.Println("Raylib Cleanup: Closing window..."); rl.CloseWindow() }
}
func (r *RaylibRenderer) ShouldClose() bool { return rl.WindowShouldClose() }
func (r *RaylibRenderer) BeginFrame() { rl.BeginDrawing(); rl.ClearBackground(r.config.DefaultBg) }
func (r *RaylibRenderer) EndFrame() { rl.EndDrawing() }
func (r *RaylibRenderer) PollEvents() {
	mousePos := rl.GetMousePosition()
	cursor := rl.MouseCursorDefault
	mouseClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
    clickedElementFound := false // Prevent multiple clicks per frame

	// Iterate top-down (reverse render order / Z-index approx)
	for i := len(r.elements) - 1; i >= 0; i-- {
		el := &r.elements[i]

		// Check only visible, interactive elements
		if el.IsInteractive && el.RenderW > 0 && el.RenderH > 0 {
			bounds := rl.NewRectangle(float32(el.RenderX), float32(el.RenderY), float32(el.RenderW), float32(el.RenderH))

			if rl.CheckCollisionPointRec(mousePos, bounds) {
				cursor = rl.MouseCursorPointingHand // Set cursor for hover

				// Check for click only if we haven't processed one yet this frame
				if mouseClicked && !clickedElementFound {
					log.Printf("DEBUG EVENT: Click detected within bounds of Elem %d", el.OriginalIndex)
                    clickedElementFound = true // Mark click as handled

					// Find and execute the CLICK handler for this element
                    // Iterate through handlers attached to this specific element
					for _, eventInfo := range el.EventHandlers {
						// Check if the handler is for a CLICK event
                        if eventInfo.EventType == krb.EventTypeClick {
                            // Look up the Go function associated with the handler name
							handlerFunc, found := r.eventHandlerMap[eventInfo.HandlerName]
							if found && handlerFunc != nil {
								log.Printf("INFO EVENT: Executing click handler '%s' for Elem %d", eventInfo.HandlerName, el.OriginalIndex)
								handlerFunc() // Execute the Go function
							} else {
								log.Printf("WARN EVENT: Click handler func not registered for name '%s' on Elem %d", eventInfo.HandlerName, el.OriginalIndex)
							}
                            // Assuming one click handler per element type for now
							break // Stop checking handlers for this element once click is handled
						}
					} // End loop through element's event handlers
				} // End if mouseClicked

                // Element found under cursor, don't check elements below it for hover/click
                break // Exit the loop through all elements
			} // end collision check
		} // end interactive check
	} // end element loop

	rl.SetMouseCursor(cursor) // Set the cursor based on hover state
}

func applyStyleProperties(props []krb.Property, doc *krb.Document,
	defaultBg, defaultFg, defaultBorder *rl.Color, // Pointers to default colors
	defaultBorderWidth *uint8, // Pointer to default border width
	defaultTextAlign *uint8) { // Pointer to default text align

	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBg = c // Modify the default background color
			}
		case krb.PropIDFgColor: // Use consistent naming with KRB spec
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultFg = c // Modify the default foreground/text color
			}
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok {
				*defaultBorder = c // Modify the default border color
			}
		case krb.PropIDBorderWidth:
			// Style applies a single value which becomes the default for all borders
			if bw, ok := getByteValue(&prop); ok {
				*defaultBorderWidth = bw // Modify the default border width
			}
			// Note: Styles don't typically set complex EdgeInsets for borders, just a single default width.
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok {
				*defaultTextAlign = align // Modify the default text alignment
			}
			// Add other potentially styleable *default* properties here
			// e.g., default font size could be set if applicable
		}
	}
}



// --- Helper Functions ---

// findStyle retrieves a style from the document by its 1-based ID.
func findStyle(doc *krb.Document, styleID uint8) (*krb.Style, bool) {
	if styleID == 0 || int(styleID) > len(doc.Styles) { return nil, false }
	return &doc.Styles[styleID-1], true
}

// getStylePropertyValue finds a specific property within a style struct.
func getStylePropertyValue(style *krb.Style, propID krb.PropertyID) (*krb.Property, bool) {
    if style == nil { return nil, false }
    for i := range style.Properties {
        if style.Properties[i].ID == propID {
            return &style.Properties[i], true
        }
    }
    return nil, false
}


// applyStylePropertiesToElement applies properties from a style to an element's fields.
func applyStylePropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BgColor = c }
		case krb.PropIDFgColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.FgColor = c }
		case krb.PropIDBorderColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BorderColor = c }
		case krb.PropIDBorderWidth: if bw, ok := getByteValue(&prop); ok { el.BorderWidths = [4]uint8{bw, bw, bw, bw} } else if edges, ok := getEdgeInsetsValue(&prop); ok { el.BorderWidths = edges } // Allow EdgeInsets for borders too
		case krb.PropIDTextAlignment: if align, ok := getByteValue(&prop); ok { el.TextAlignment = align }
		// Add other styleable properties (Padding, Margin, Font props if needed)
		}
	}
}

// applyDirectPropertiesToElement applies direct properties to an element's fields.
func applyDirectPropertiesToElement(props []krb.Property, doc *krb.Document, el *render.RenderElement) {
    // This function primarily overrides visual aspects. Config props handled separately.
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BgColor = c }
		case krb.PropIDFgColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.FgColor = c }
		case krb.PropIDBorderColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BorderColor = c }
		case krb.PropIDBorderWidth: if bw, ok := getByteValue(&prop); ok { el.BorderWidths = [4]uint8{bw, bw, bw, bw} } else if edges, ok := getEdgeInsetsValue(&prop); ok { el.BorderWidths = edges }
		case krb.PropIDTextAlignment: if align, ok := getByteValue(&prop); ok { el.TextAlignment = align }
		case krb.PropIDTextContent: if strIdx, ok := getByteValue(&prop); ok { if int(strIdx) < len(doc.Strings) { el.Text = doc.Strings[strIdx] } } // Override text resolved earlier
		case krb.PropIDImageSource: if resIdx, ok := getByteValue(&prop); ok { el.ResourceIndex = resIdx } // Override resource resolved earlier
        // Ignore App-specific config properties here
        case krb.PropIDWindowWidth, krb.PropIDWindowHeight, krb.PropIDWindowTitle, krb.PropIDResizable, krb.PropIDScaleFactor, krb.PropIDIcon, krb.PropIDVersion, krb.PropIDAuthor:
            continue
        // Add other direct visual properties (Opacity, ZIndex, etc.)
		}
	}
}

// applyDirectPropertiesToConfig applies App-specific properties to the WindowConfig.
func applyDirectPropertiesToConfig(props []krb.Property, doc *krb.Document, config *render.WindowConfig) {
    if config == nil { return } // Safety check
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDWindowWidth: if w, ok := getShortValue(&prop); ok { config.Width = int(w) }
		case krb.PropIDWindowHeight: if h, ok := getShortValue(&prop); ok { config.Height = int(h) }
		case krb.PropIDWindowTitle: if s, ok := getStringValue(&prop, doc); ok { config.Title = s }
		case krb.PropIDResizable: if r, ok := getByteValue(&prop); ok { config.Resizable = (r != 0) }
		case krb.PropIDScaleFactor: if sf, ok := getFixedPointValue(&prop); ok { config.ScaleFactor = sf }
		case krb.PropIDBgColor: if c, ok := getColorValue(&prop, doc.Header.Flags); ok { config.DefaultBg = c } // Allow App BG override default
		// Icon, Version, Author currently ignored but could be added to config
		}
	}
}

// resolveElementText finds and sets the Text field for Text/Button elements.
func resolveElementText(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeText && el.Header.Type != krb.ElemTypeButton { return }
	resolvedText := ""; foundTextProp := false
	// Check Direct Props first
	if len(doc.Properties) > el.OriginalIndex {
		for _, prop := range doc.Properties[el.OriginalIndex] {
			if prop.ID == krb.PropIDTextContent { if s, ok := getStringValue(&prop, doc); ok { resolvedText = s; foundTextProp = true; break } }
		}
	}
	// Check Style if not found directly
	if !foundTextProp && styleOk {
		for _, prop := range style.Properties {
			if prop.ID == krb.PropIDTextContent { if s, ok := getStringValue(&prop, doc); ok { resolvedText = s; foundTextProp = true; break } }
		}
	}
	el.Text = resolvedText
}

// resolveElementImageSource finds and sets the ResourceIndex field for Image elements.
func resolveElementImageSource(doc *krb.Document, el *render.RenderElement, style *krb.Style, styleOk bool) {
	if el.Header.Type != krb.ElemTypeImage { return }
	resolvedResIdx := uint8(render.InvalidResourceIndex); foundResProp := false
	// Check Direct Props first
	if len(doc.Properties) > el.OriginalIndex {
		for _, prop := range doc.Properties[el.OriginalIndex] {
			if prop.ID == krb.PropIDImageSource { if idx, ok := getByteValue(&prop); ok { resolvedResIdx = idx; foundResProp = true; break } }
		}
	}
	// Check Style if not found directly
	if !foundResProp && styleOk {
		for _, prop := range style.Properties {
			if prop.ID == krb.PropIDImageSource { if idx, ok := getByteValue(&prop); ok { resolvedResIdx = idx; foundResProp = true; break } }
		}
	}
	el.ResourceIndex = resolvedResIdx
}


// --- Value Parsing Helpers ---
// (Ensure these are complete and correct)

func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {
	if prop.ValueType != krb.ValTypeColor { return rl.Color{}, false }
	useExtended := (flags & krb.FlagExtendedColor) != 0
	if useExtended { if len(prop.Value) == 4 { return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true } } else { if len(prop.Value) == 1 { log.Printf("Warning: Palette color index %d used, palettes not implemented.", prop.Value[0]); return rl.Magenta, true } }
	return rl.Color{}, false
}
func getByteValue(prop *krb.Property) (uint8, bool) { if (prop.ValueType == krb.ValTypeByte || prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource || prop.ValueType == krb.ValTypeEnum) && len(prop.Value) == 1 { return prop.Value[0], true }; return 0, false }
func getShortValue(prop *krb.Property) (uint16, bool) { if prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 { return binary.LittleEndian.Uint16(prop.Value), true }; return 0, false }
func getStringValue(prop *krb.Property, doc *krb.Document) (string, bool) { if prop.ValueType == krb.ValTypeString && len(prop.Value) == 1 { idx := prop.Value[0]; if int(idx) < len(doc.Strings) { return doc.Strings[idx], true } }; return "", false }
func getFixedPointValue(prop *krb.Property) (float32, bool) { if prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 { val := binary.LittleEndian.Uint16(prop.Value); return float32(val) / 256.0, true }; return 0, false }
func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) { if prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 { return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true }; return [4]uint8{}, false }
func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }