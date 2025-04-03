package raylib

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	// "os" // This import was unused - removed
	"path/filepath" // Use path/filepath for cross-platform paths

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"   // Corrected module path
	"github.com/waozixyz/kryon/impl/go/render" // Corrected module path
)

// RaylibRenderer implements the render.Renderer interface using raylib-go.
type RaylibRenderer struct {
	config        render.WindowConfig
	elements      []render.RenderElement // Flat list of all processed elements
	roots         []*render.RenderElement
	loadedTextures map[uint8]rl.Texture2D // Map resource index to texture
    krbFileDir    string               // Directory of the loaded KRB file for relative paths
    scaleFactor   float32              // Store effective scale factor
}

// NewRaylibRenderer creates a new Raylib renderer instance.
func NewRaylibRenderer() *RaylibRenderer {
	return &RaylibRenderer{
		loadedTextures: make(map[uint8]rl.Texture2D),
        scaleFactor: 1.0, // Default
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

// PrepareTree processes the KRB document and builds the render tree.
func (r *RaylibRenderer) PrepareTree(doc *krb.Document, krbFilePath string) ([]*render.RenderElement, render.WindowConfig, error) {
	if doc == nil || doc.Header.ElementCount == 0 {
		log.Println("PrepareTree: No elements in document.")
		return nil, r.config, nil // Return current config even if no elements
	}

	// Store KRB file directory for resource loading
	var err error
	r.krbFileDir, err = filepath.Abs(filepath.Dir(krbFilePath))
	if err != nil {
		return nil, r.config, fmt.Errorf("failed to get absolute path for KRB directory: %w", err)
	}
	log.Printf("PrepareTree: KRB Base Directory: %s", r.krbFileDir)


	r.elements = make([]render.RenderElement, doc.Header.ElementCount)
	r.roots = nil // Reset roots

	// Default colors/styles
	windowConfig := render.DefaultWindowConfig()
	windowConfig.DefaultBg = rl.Black // Reset defaults before App processing
	defaultFg := rl.RayWhite
	defaultBorder := rl.Gray
	defaultBorderWidth := uint8(0)
	defaultTextAlign := uint8(0) // Left

	// --- Pass 1: Process App Element (if exists) and Initialize RenderElements ---
	var appElement *render.RenderElement
	if (doc.Header.Flags&krb.FlagHasApp) != 0 && doc.Header.ElementCount > 0 && doc.Elements[0].Type == krb.ElemTypeApp {
		appElement = &r.elements[0]
		appElement.Header = doc.Elements[0]
		appElement.OriginalIndex = 0
		appElement.IsInteractive = false // App element usually isn't
        appElement.ResourceIndex = render.InvalidResourceIndex


		// Apply App Style (if any) - affects defaults
		style, ok := findStyle(doc, appElement.Header.StyleID)
		if ok {
			log.Printf("PrepareTree: Applying App Style %d", appElement.Header.StyleID)
			// Pass pointers to the defaults that the style might modify
			applyStyleProperties(style.Properties, doc, &windowConfig.DefaultBg, &defaultFg, &defaultBorder, &defaultBorderWidth, &defaultTextAlign)
		} else if appElement.Header.StyleID != 0 {
            log.Printf("Warning: App element references invalid Style ID %d", appElement.Header.StyleID)
        }
        // Initialize app element's own colors from defaults *before* direct props
		appElement.BgColor = windowConfig.DefaultBg
		appElement.FgColor = defaultFg
		appElement.BorderColor = defaultBorder
        appElement.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth}
        appElement.TextAlignment = defaultTextAlign

		// Apply App Direct Properties (override defaults and potentially style)
		if len(doc.Properties) > 0 && len(doc.Properties[0]) > 0 {
			log.Printf("PrepareTree: Applying App Direct Properties (Count=%d)", len(doc.Properties[0]))
			applyDirectProperties(doc.Properties[0], doc, appElement, &windowConfig) // Pass WindowConfig to modify
		}

		// Finalize App element render size (usually matches window)
		appElement.RenderW = windowConfig.Width
		appElement.RenderH = windowConfig.Height
		appElement.RenderX = 0
		appElement.RenderY = 0
        r.scaleFactor = windowConfig.ScaleFactor // Update renderer scale factor from App

		log.Printf("PrepareTree: Processed App. Window:%dx%d Title:'%s' Scale:%.2f Resizable:%t",
			windowConfig.Width, windowConfig.Height, windowConfig.Title, windowConfig.ScaleFactor, windowConfig.Resizable)

	} else {
		log.Println("PrepareTree: No App element found, using default window config.")
        // Use the initial default config
	}
    r.config = windowConfig // Update renderer's config


	// --- Pass 2: Initialize remaining RenderElements with styles and direct properties ---
	for i := 0; i < int(doc.Header.ElementCount); i++ {
		if appElement != nil && i == 0 {
			continue // Skip App element if already processed
		}
		currentEl := &r.elements[i]
		currentEl.Header = doc.Elements[i]
		currentEl.OriginalIndex = i
		currentEl.IsInteractive = (currentEl.Header.Type == krb.ElemTypeButton || currentEl.Header.Type == krb.ElemTypeInput)
		currentEl.ResourceIndex = render.InvalidResourceIndex // Default

		// Start with global defaults
		currentEl.BgColor = windowConfig.DefaultBg
		currentEl.FgColor = defaultFg
		currentEl.BorderColor = defaultBorder
		currentEl.BorderWidths = [4]uint8{defaultBorderWidth, defaultBorderWidth, defaultBorderWidth, defaultBorderWidth}
		currentEl.TextAlignment = defaultTextAlign


		// Apply Style
		style, ok := findStyle(doc, currentEl.Header.StyleID)
		if ok {
            // --- FIXED CALL --- Pass pointers using & ---
			applyStyleProperties(style.Properties, doc, &currentEl.BgColor, &currentEl.FgColor, &currentEl.BorderColor, &currentEl.BorderWidths[0], &currentEl.TextAlignment)
            // -----------------------------------------

            // Note: Style only sets a single border width value currently via BorderWidths[0] pointer.
            // We copy this value to other borders AFTER the call returns.
             for k:=1; k<4; k++ { currentEl.BorderWidths[k] = currentEl.BorderWidths[0]}
		} else if currentEl.Header.StyleID != 0 {
            log.Printf("Warning: Element %d references invalid Style ID %d", i, currentEl.Header.StyleID)
        }


		// Apply Direct Properties (override defaults and style)
		if len(doc.Properties) > i && len(doc.Properties[i]) > 0 {
			applyDirectProperties(doc.Properties[i], doc, currentEl, nil) // Pass nil config for non-app elements
		}

		// Resolve Text Content
		if currentEl.Header.Type == krb.ElemTypeText || currentEl.Header.Type == krb.ElemTypeButton {
			// Check for text content property (can be in direct or style)
			// Direct properties override style
			resolved := false
			if len(doc.Properties) > i {
				for _, prop := range doc.Properties[i] {
					if prop.ID == krb.PropIDTextContent && prop.ValueType == krb.ValTypeString && len(prop.Value) == 1 {
						idx := prop.Value[0]
						if int(idx) < len(doc.Strings) {
							currentEl.Text = doc.Strings[idx]
							resolved = true
							break
						}
					}
				}
			}
            // If not resolved by direct, check style
            if !resolved && ok { // 'ok' means style was found
                 for _, prop := range style.Properties {
                    if prop.ID == krb.PropIDTextContent && prop.ValueType == krb.ValTypeString && len(prop.Value) == 1 {
                        idx := prop.Value[0]
                        if int(idx) < len(doc.Strings) {
                            currentEl.Text = doc.Strings[idx]
                            resolved = true
                            break
                        }
                    }
                }
            }

		}
		// Resolve Image Source (similar logic: direct overrides style)
        if currentEl.Header.Type == krb.ElemTypeImage {
            resolved := false
            if len(doc.Properties) > i {
                for _, prop := range doc.Properties[i] {
                    if prop.ID == krb.PropIDImageSource && prop.ValueType == krb.ValTypeResource && len(prop.Value) == 1 {
                        currentEl.ResourceIndex = prop.Value[0]
                        resolved = true
                        break
                    }
                }
            }
            if !resolved && ok {
                for _, prop := range style.Properties {
                     if prop.ID == krb.PropIDImageSource && prop.ValueType == krb.ValTypeResource && len(prop.Value) == 1 {
                        currentEl.ResourceIndex = prop.Value[0]
                        resolved = true
                        break
                    }
                }
            }
        }


	}

	// --- Pass 3: Build Parent/Child Tree ---
	log.Println("PrepareTree: Building element tree...")
	// This logic assumes elements are roughly in DFS order, which KRB implies
    // Find element by offset might be more robust if order isn't guaranteed.
    // Using C logic ported (index based linking) assuming order:
    parentStack := make([]*render.RenderElement, 0, 10) // Initial capacity
	for i := 0; i < int(doc.Header.ElementCount); i++ {
        currentEl := &r.elements[i]

        // Pop stack until we find a parent with room for children
        for len(parentStack) > 0 {
            parent := parentStack[len(parentStack)-1]
            if len(parent.Children) >= int(parent.Header.ChildCount) {
                 parentStack = parentStack[:len(parentStack)-1] // Pop
            } else {
                break
            }
        }

        // Assign parent and add child
        if len(parentStack) > 0 {
             parent := parentStack[len(parentStack)-1]
             currentEl.Parent = parent
             parent.Children = append(parent.Children, currentEl)
        } else {
            // No parent on stack, this must be a root element
            r.roots = append(r.roots, currentEl)
        }


        // Push current element onto stack if it expects children
        if currentEl.Header.ChildCount > 0 {
             parentStack = append(parentStack, currentEl)
        }
	}
    log.Printf("PrepareTree: Finished building tree. Found %d root(s).", len(r.roots))
    if len(r.roots) == 0 && doc.Header.ElementCount > 0 {
        log.Println("ERROR: No root elements found, but elements exist!")
        // Depending on strictness, could return error here
    }


	// --- Pass 4: Load Textures ---
	log.Println("PrepareTree: Loading textures...")
	for i := range r.elements {
		el := &r.elements[i]
		if el.Header.Type == krb.ElemTypeImage && el.ResourceIndex != render.InvalidResourceIndex {
			if int(el.ResourceIndex) >= len(doc.Resources) {
				log.Printf("ERROR: Element %d has invalid resource index %d (max %d)", el.OriginalIndex, el.ResourceIndex, len(doc.Resources)-1)
				continue
			}
			res := doc.Resources[el.ResourceIndex]

			// Check if already loaded
			if _, exists := r.loadedTextures[el.ResourceIndex]; exists {
				el.Texture = r.loadedTextures[el.ResourceIndex]
				el.TextureLoaded = true
                log.Printf("  Using cached texture for Elem %d (Res %d)", el.OriginalIndex, el.ResourceIndex)
				continue
			}

			if res.Format == krb.ResFormatExternal {
				if int(res.NameIndex) >= len(doc.Strings) {
					log.Printf("ERROR: Resource %d (External) has invalid name index %d", el.ResourceIndex, res.NameIndex)
					continue
				}
				relativePath := doc.Strings[res.NameIndex]
				// Construct full path relative to KRB file directory
				fullPath := filepath.Join(r.krbFileDir, relativePath)

				log.Printf("  Loading texture Elem %d (Res %d): '%s' (Relative: '%s')", el.OriginalIndex, el.ResourceIndex, fullPath, relativePath)
				texture := rl.LoadTexture(fullPath)
				if rl.IsTextureReady(texture) {
					el.Texture = texture
					el.TextureLoaded = true
					r.loadedTextures[el.ResourceIndex] = texture // Cache it
					log.Printf("    -> OK (ID: %d, %dx%d)", texture.ID, texture.Width, texture.Height)
				} else {
					log.Printf("    -> FAILED loading texture: %s", fullPath)
					el.TextureLoaded = false // Explicitly set false on failure
				}
			} else if res.Format == krb.ResFormatInline {
                 log.Printf("  Loading inline texture Elem %d (Res %d) Size: %d", el.OriginalIndex, el.ResourceIndex, res.InlineDataSize)
                 if res.InlineData != nil && res.InlineDataSize > 0 {
                     // Raylib needs image format hint - assume PNG for now, could be passed in KRB later
                     // Or try LoadTextureFromImage(LoadImageFromMemory(...))
                     img := rl.LoadImageFromMemory(".png", res.InlineData, int32(len(res.InlineData))) // Assume PNG
                     if rl.IsImageReady(img) {
                         texture := rl.LoadTextureFromImage(img)
                         rl.UnloadImage(img) // Unload CPU image data after GPU texture created
                         if rl.IsTextureReady(texture) {
                             el.Texture = texture
                             el.TextureLoaded = true
                             r.loadedTextures[el.ResourceIndex] = texture // Cache it
                             log.Printf("    -> OK (ID: %d, %dx%d)", texture.ID, texture.Width, texture.Height)
                         } else {
                             log.Printf("    -> FAILED creating texture from inline image")
                             el.TextureLoaded = false
                         }
                     } else {
                         log.Printf("    -> FAILED loading inline image from memory")
                         el.TextureLoaded = false
                     }
                 } else {
                      log.Printf("    -> FAILED: Inline data is nil or zero size.")
                      el.TextureLoaded = false
                 }

			} else {
				log.Printf("WARN: Unknown resource format %d for image resource %d", res.Format, el.ResourceIndex)
			}
		}
	}
	log.Println("PrepareTree: Finished loading textures.")


	return r.roots, r.config, nil
}

// GetRenderTree returns the flat list of processed elements.
func (r *RaylibRenderer) GetRenderTree() []*render.RenderElement {
    // Return pointers to the elements in the slice
    pointers := make([]*render.RenderElement, len(r.elements))
    for i := range r.elements {
        pointers[i] = &r.elements[i]
    }
    return pointers
}


// RenderFrame draws the tree.
func (r *RaylibRenderer) RenderFrame(roots []*render.RenderElement) {
	// Recalculate layout if window resized - simple approach: recalculate every frame for now
    if rl.IsWindowResized() {
        r.config.Width = int(rl.GetScreenWidth())
        r.config.Height = int(rl.GetScreenHeight())
        // Update App element size if it's a root
        for _, root := range roots {
            if root.OriginalIndex == 0 && root.Header.Type == krb.ElemTypeApp {
                 root.RenderW = r.config.Width
                 root.RenderH = r.config.Height
                 break // Assume only one App root
            }
        }
        log.Printf("Window resized to %dx%d", r.config.Width, r.config.Height)
    }


    // Render all root elements
    for _, root := range roots {
        // Determine initial parent bounds (the window itself)
        parentContentX := 0
        parentContentY := 0
        parentContentW := r.config.Width
        parentContentH := r.config.Height
        // If the root is the App element, use its calculated size
        if root.OriginalIndex == 0 && root.Header.Type == krb.ElemTypeApp {
            parentContentW = root.RenderW
            parentContentH = root.RenderH
        }

        renderElementRecursive(root, parentContentX, parentContentY, parentContentW, parentContentH, r.scaleFactor)
    }
}

// renderElementRecursive is the core drawing function.
func renderElementRecursive(el *render.RenderElement, parentContentX, parentContentY, parentContentWidth, parentContentHeight int, scale float32) {
	if el == nil {
		return
	}

    scaled := func(val uint16) int { return int(math.Round(float64(val) * float64(scale))) }
    scaledU8 := func(val uint8) int { return int(math.Round(float64(val) * float64(scale))) }


	// --- Calculate Intrinsic Size (Natural size before layout constraints) ---
	intrinsicW := scaled(el.Header.Width)
	intrinsicH := scaled(el.Header.Height)

	if el.Header.Type == krb.ElemTypeText && el.Text != "" {
		fontSize := int32(math.Max(1, math.Round(render.BaseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		if el.Header.Width == 0 { intrinsicW = int(textWidthMeasured) + scaledU8(8) } // Add padding if width auto
		if el.Header.Height == 0 { intrinsicH = int(fontSize) + scaledU8(8) }         // Add padding if height auto
	} else if el.Header.Type == krb.ElemTypeImage && el.TextureLoaded {
		if el.Header.Width == 0 { intrinsicW = int(math.Round(float64(el.Texture.Width) * float64(scale))) }
		if el.Header.Height == 0 { intrinsicH = int(math.Round(float64(el.Texture.Height) * float64(scale))) }
	}

	// Clamp minimum size and ensure non-zero if specified
	if intrinsicW < 0 { intrinsicW = 0 }
	if intrinsicH < 0 { intrinsicH = 0 }
	if el.Header.Width > 0 && intrinsicW == 0 { intrinsicW = 1 }
	if el.Header.Height > 0 && intrinsicH == 0 { intrinsicH = 1 }

    // Store intrinsic size for layout pass
    el.IntrinsicW = intrinsicW
    el.IntrinsicH = intrinsicH


	// --- Determine Final Position & Size (Layout) ---
	// This part is complex. For flow layout, the parent calculates the child's position *before* calling recursively.
	// For absolute layout, we calculate it here relative to the parent's *content* area.

	finalX := 0
	finalY := 0
    finalW := intrinsicW // Start with intrinsic, might be overridden by layout
    finalH := intrinsicH

	isAbsolute := el.Header.LayoutAbsolute()
	hasPosition := el.Header.PosX != 0 || el.Header.PosY != 0

	if isAbsolute || hasPosition {
		// Absolute positioning relative to parent's content area
		finalX = parentContentX + scaled(el.Header.PosX)
		finalY = parentContentY + scaled(el.Header.PosY)
	} else if el.Parent != nil {
		// Flow layout: Position should have been calculated by the parent and stored in el.RenderX/Y
		finalX = el.RenderX
		finalY = el.RenderY
        // Parent might also assign width/height based on grow/stretch TBD
	} else {
		// Root element in flow layout (or App element) - defaults to parent content origin (0,0 for window)
		finalX = parentContentX + scaled(el.Header.PosX) // Use position even if not strictly 'absolute' for roots
		finalY = parentContentY + scaled(el.Header.PosY)
	}

	// Store final calculated render coordinates (might be adjusted by parent layout later if flow)
	el.RenderX = finalX
	el.RenderY = finalY
    // TODO: Implement Grow/Stretch - for now, just use intrinsic size
    el.RenderW = finalW
    el.RenderH = finalH


	// --- Apply Styling and Draw ---
	bgColor := el.BgColor
	fgColor := el.FgColor
	borderColor := el.BorderColor
    // Apply scaling to borders now
	topBW := scaledU8(el.BorderWidths[0])
	rightBW := scaledU8(el.BorderWidths[1])
	bottomBW := scaledU8(el.BorderWidths[2])
	leftBW := scaledU8(el.BorderWidths[3])

	// Clamp borders
	if el.RenderH > 0 && topBW+bottomBW >= el.RenderH { topBW = max(0, min(el.RenderH / 2, 1)); bottomBW = max(0, min(el.RenderH - topBW, 1)) }
    if el.RenderW > 0 && leftBW+rightBW >= el.RenderW { leftBW = max(0, min(el.RenderW / 2, 1)); rightBW = max(0, min(el.RenderW - leftBW, 1)) }

	// Draw Background
    drawBackground := true // Adjust if certain types shouldn't have bg (e.g., transparent container?)
	if drawBackground && el.RenderW > 0 && el.RenderH > 0 {
		rl.DrawRectangle(int32(el.RenderX), int32(el.RenderY), int32(el.RenderW), int32(el.RenderH), bgColor)
	}

	// Draw Borders
	if el.RenderW > 0 && el.RenderH > 0 {
		if topBW > 0 { rl.DrawRectangle(int32(el.RenderX), int32(el.RenderY), int32(el.RenderW), int32(topBW), borderColor) }
		if bottomBW > 0 { rl.DrawRectangle(int32(el.RenderX), int32(el.RenderY+el.RenderH-bottomBW), int32(el.RenderW), int32(bottomBW), borderColor) }
		sideBorderY := el.RenderY + topBW
		sideBorderHeight := el.RenderH - topBW - bottomBW
        if sideBorderHeight < 0 { sideBorderHeight = 0 }
		if leftBW > 0 { rl.DrawRectangle(int32(el.RenderX), int32(sideBorderY), int32(leftBW), int32(sideBorderHeight), borderColor) }
		if rightBW > 0 { rl.DrawRectangle(int32(el.RenderX+el.RenderW-rightBW), int32(sideBorderY), int32(rightBW), int32(sideBorderHeight), borderColor) }
	}

	// Calculate Content Area
	contentX := el.RenderX + leftBW
	contentY := el.RenderY + topBW
	contentWidth := el.RenderW - leftBW - rightBW
	contentHeight := el.RenderH - topBW - bottomBW
	if contentWidth < 0 { contentWidth = 0 }
	if contentHeight < 0 { contentHeight = 0 }

	// Draw Content (Text or Image) within Scissor Rectangle
	if contentWidth > 0 && contentHeight > 0 {
		rl.BeginScissorMode(int32(contentX), int32(contentY), int32(contentWidth), int32(contentHeight))

		// Draw Text
		if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
			fontSize := int32(math.Max(1, math.Round(render.BaseFontSize*float64(scale))))
			textWidthMeasured := rl.MeasureText(el.Text, fontSize)
			textDrawX := contentX
			if el.TextAlignment == 1 { // Center
				textDrawX = contentX + (contentWidth-int(textWidthMeasured))/2
			} else if el.TextAlignment == 2 { // End/Right
				textDrawX = contentX + contentWidth - int(textWidthMeasured)
			}
			// Vertical alignment (simple center)
			textDrawY := contentY + (contentHeight-int(fontSize))/2

            // Clamp draw position to be within content area (important with alignment)
            if textDrawX < contentX {
				textDrawX = contentX
			}
            if textDrawY < contentY {
				textDrawY = contentY
			}

			rl.DrawText(el.Text, int32(textDrawX), int32(textDrawY), fontSize, fgColor)
		} else if el.Header.Type == krb.ElemTypeImage && el.TextureLoaded {
			// Simple stretch draw for now
			sourceRec := rl.NewRectangle(0, 0, float32(el.Texture.Width), float32(el.Texture.Height))
			destRec := rl.NewRectangle(float32(contentX), float32(contentY), float32(contentWidth), float32(contentHeight))
			origin := rl.NewVector2(0, 0)
			rl.DrawTexturePro(el.Texture, sourceRec, destRec, origin, 0.0, rl.White) // Tint White = no tint
			// TODO: Add aspect ratio handling (letterboxing/pillarboxing) if desired
		} 

		rl.EndScissorMode()
	}


	// --- Layout and Render Children ---
	if len(el.Children) > 0 && contentWidth > 0 && contentHeight > 0 {
        layoutChildren(el, contentX, contentY, contentWidth, contentHeight, scale)
	}
}

// layoutChildren performs the layout calculation pass for flow layout children.
func layoutChildren(parent *render.RenderElement, contentX, contentY, contentWidth, contentHeight int, scale float32) {
    direction := parent.Header.LayoutDirection()
    alignment := parent.Header.LayoutAlignment()
    // wrap := parent.Header.LayoutWrap() // TODO: Implement Wrap

    currentFlowX := contentX
    currentFlowY := contentY
    totalFlowChildWidth := 0
    totalFlowChildHeight := 0
    flowChildCount := 0

    // Pass 1: Calculate total size of *flow* children (absolute ones are ignored here)
    for _, child := range parent.Children {
        if child.Header.LayoutAbsolute() || (child.Header.PosX != 0 || child.Header.PosY != 0) {
            continue // Skip absolute/positioned children
        }
        // Intrinsic size should have been calculated when the child was processed initially
        childW := child.IntrinsicW
        childH := child.IntrinsicH

        // TODO: Handle Grow/Stretch here - if child has Grow flag and parent is Row/Col,
        // it might take remaining space. This requires a more complex multi-pass layout.
        // For now, just use intrinsic size.

        if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse {
            totalFlowChildWidth += childW
        } else {
            totalFlowChildHeight += childH
        }
        flowChildCount++
    }

    // Pass 2: Calculate starting position based on alignment
    startX := contentX
    startY := contentY
    spacing := float32(0.0) // Space for SpaceBetween

    if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse { // Row Flow
        availableWidth := contentWidth
        extraWidth := availableWidth - totalFlowChildWidth
        if alignment == krb.LayoutAlignCenter {
            startX = contentX + extraWidth/2
        } else if alignment == krb.LayoutAlignEnd {
            startX = contentX + extraWidth
        } else if alignment == krb.LayoutAlignSpaceBetween && flowChildCount > 1 {
            spacing = float32(extraWidth) / float32(flowChildCount-1)
            // Start align for space-between, spacing is added *after* each element
        }
         // Clamp start position
        if startX < contentX { 
			startX = contentX
		}
        currentFlowX = startX
    } else { // Column Flow
        availableHeight := contentHeight
        extraHeight := availableHeight - totalFlowChildHeight
         if alignment == krb.LayoutAlignCenter {
            startY = contentY + extraHeight/2
        } else if alignment == krb.LayoutAlignEnd {
            startY = contentY + extraHeight
        } else if alignment == krb.LayoutAlignSpaceBetween && flowChildCount > 1 {
            spacing = float32(extraHeight) / float32(flowChildCount-1)
        }
        // Clamp start position
        if startY < contentY { startY = contentY }
        currentFlowY = startY
    }
    if spacing < 0 { spacing = 0 }


    // Pass 3: Position and Render children
    flowChildrenProcessed := 0
    for _, child := range parent.Children {
        isAbsolute := child.Header.LayoutAbsolute() || (child.Header.PosX != 0 || child.Header.PosY != 0)

        if isAbsolute {
            // Absolute children are positioned relative to parent content box
            renderElementRecursive(child, contentX, contentY, contentWidth, contentHeight, scale)
        } else {
            // Flow child
            childW := child.IntrinsicW // Use calculated intrinsic size
            childH := child.IntrinsicH // TODO: Adjust for Grow/Stretch later

            childFinalX := 0
            childFinalY := 0

            if direction == krb.LayoutDirRow || direction == krb.LayoutDirRowReverse { // Row Flow
                 childFinalX = currentFlowX
                 // Vertical alignment within the row (based on parent's alignment setting!)
                 // This needs careful thought - should children align to each other or the parent bounds?
                 // Simple approach: Align to parent content height for now.
                 if alignment == krb.LayoutAlignCenter {
                      childFinalY = contentY + (contentHeight - childH) / 2
                 } else if alignment == krb.LayoutAlignEnd {
                     childFinalY = contentY + contentHeight - childH
                 } else { // Start
                     childFinalY = contentY
                 }
                 // Clamp Y
                 if childFinalY < contentY {
					childFinalY = contentY
				}

                 child.RenderX = childFinalX
                 child.RenderY = childFinalY
                 child.RenderW = childW // TODO: Grow/Stretch
                 child.RenderH = childH

                 renderElementRecursive(child, contentX, contentY, contentWidth, contentHeight, scale)

                 // Advance flow position
                 currentFlowX += childW
                 if alignment == krb.LayoutAlignSpaceBetween && flowChildrenProcessed < flowChildCount - 1 {
                      currentFlowX += int(math.Round(float64(spacing)))
                 }
            } else { // Column Flow
                childFinalY = currentFlowY
                 // Horizontal alignment within the column
                 if alignment == krb.LayoutAlignCenter {
                      childFinalX = contentX + (contentWidth - childW) / 2
                 } else if alignment == krb.LayoutAlignEnd {
                     childFinalX = contentX + contentWidth - childW
                 } else { // Start
                     childFinalX = contentX
                 }
                 // Clamp X
                 if childFinalX < contentX { childFinalX = contentX }

                 child.RenderX = childFinalX
                 child.RenderY = childFinalY
                 child.RenderW = childW // TODO: Grow/Stretch
                 child.RenderH = childH

                 renderElementRecursive(child, contentX, contentY, contentWidth, contentHeight, scale)

                 // Advance flow position
                 currentFlowY += childH
                 if alignment == krb.LayoutAlignSpaceBetween && flowChildrenProcessed < flowChildCount - 1 {
                      currentFlowY += int(math.Round(float64(spacing)))
                 }
            }
            flowChildrenProcessed++
        }
    }

}


// Cleanup releases textures and closes the window.
func (r *RaylibRenderer) Cleanup() {
	log.Println("Raylib Cleanup: Unloading textures...")
	for idx, texture := range r.loadedTextures {
		log.Printf("  Unloading Res %d (TexID: %d)", idx, texture.ID)
		rl.UnloadTexture(texture)
	}
	r.loadedTextures = make(map[uint8]rl.Texture2D) // Clear map
    if rl.IsWindowReady() {
	    log.Println("Raylib Cleanup: Closing window...")
	    rl.CloseWindow()
    }
}

// ShouldClose checks Raylib's window close flag.
func (r *RaylibRenderer) ShouldClose() bool {
	return rl.WindowShouldClose()
}

// BeginFrame calls rl.BeginDrawing and clears background.
func (r *RaylibRenderer) BeginFrame() {
    rl.BeginDrawing()
    rl.ClearBackground(r.config.DefaultBg) // Use the config's default BG
}

// EndFrame calls rl.EndDrawing.
func (r *RaylibRenderer) EndFrame() {
    rl.EndDrawing()
}

// PollEvents handles basic Raylib event polling (like resize).
// More complex event handling (clicks, keys) would go here or be called from here.
func (r *RaylibRenderer) PollEvents() {
    // Raylib implicitly polls events during BeginDrawing/EndDrawing for basic things like WindowShouldClose()
    // If you need explicit mouse clicks, key presses etc., check them here:
    // mousePos := rl.GetMousePosition()
    // if rl.IsMouseButtonPressed(rl.MouseButtonLeft) { ... }

    // Handle hover cursor change (simple example)
     mousePos := rl.GetMousePosition()
     cursor := rl.MouseCursorDefault
     // Iterate backwards to check topmost element first
     for i := len(r.elements) - 1; i >= 0; i-- {
         el := &r.elements[i]
         if el.IsInteractive && el.RenderW > 0 && el.RenderH > 0 {
             bounds := rl.NewRectangle(float32(el.RenderX), float32(el.RenderY), float32(el.RenderW), float32(el.RenderH))
             if rl.CheckCollisionPointRec(mousePos, bounds) {
                 cursor = rl.MouseCursorPointingHand
                 break // Topmost interactive element found
             }
         }
     }
     rl.SetMouseCursor(cursor)
}


// --- Helper Functions ---

func findStyle(doc *krb.Document, styleID uint8) (*krb.Style, bool) {
	if styleID == 0 || int(styleID) > len(doc.Styles) {
		return nil, false
	}
	// StyleID is 1-based, slice index is 0-based
	return &doc.Styles[styleID-1], true
}

// applyStyleProperties applies properties from a style block to default values.
// It modifies the passed pointers.
func applyStyleProperties(props []krb.Property, doc *krb.Document, bg, fg, border *rl.Color, borderWidth *uint8, textAlign *uint8) {
	for _, prop := range props {
		switch prop.ID {
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { *bg = c }
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { *fg = c }
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { *border = c }
		case krb.PropIDBorderWidth:
            // Style currently applies single value to all borders via the first element's pointer
			if bw, ok := getByteValue(&prop); ok { *borderWidth = bw }
            // If EdgeInsets is needed for style border width, update this
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok { *textAlign = align }
		// Add other styleable properties here if needed (e.g., font size/weight -> requires font loading)
		}
	}
}

// applyDirectProperties applies direct element properties, overriding base/style values.
// If config is non-nil, it applies App-specific properties to the config.
func applyDirectProperties(props []krb.Property, doc *krb.Document, el *render.RenderElement, config *render.WindowConfig) {
	for _, prop := range props {
		switch prop.ID {
		// --- Visual Properties ---
		case krb.PropIDBgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BgColor = c }
		case krb.PropIDFgColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.FgColor = c }
		case krb.PropIDBorderColor:
			if c, ok := getColorValue(&prop, doc.Header.Flags); ok { el.BorderColor = c }
		case krb.PropIDBorderWidth:
			if bw, ok := getByteValue(&prop); ok {
                el.BorderWidths = [4]uint8{bw, bw, bw, bw} // Apply to all
            } else if edges, ok := getEdgeInsetsValue(&prop); ok {
                el.BorderWidths = edges // Apply specific edges
            }
		case krb.PropIDTextAlignment:
			if align, ok := getByteValue(&prop); ok { el.TextAlignment = align }
		case krb.PropIDTextContent:
             // Text is resolved during element initialization pass, not here generally
             // Could override here if needed, but less common than setting via initialization.
             if strIdx, ok := getByteValue(&prop); ok {
                if int(strIdx) < len(doc.Strings) {
                    el.Text = doc.Strings[strIdx]
                }
             }
        case krb.PropIDImageSource:
             if resIdx, ok := getByteValue(&prop); ok { el.ResourceIndex = resIdx }

		// --- App-Specific Properties (only if config is provided) ---
		case krb.PropIDWindowWidth:
			if config != nil { if w, ok := getShortValue(&prop); ok { config.Width = int(w); /* el.Header.Width = w // Don't modify original header */ } }
		case krb.PropIDWindowHeight:
			if config != nil { if h, ok := getShortValue(&prop); ok { config.Height = int(h); /* el.Header.Height = h */ } }
		case krb.PropIDWindowTitle:
			if config != nil { if s, ok := getStringValue(&prop, doc); ok { config.Title = s } }
		case krb.PropIDResizable:
			if config != nil { if r, ok := getByteValue(&prop); ok { config.Resizable = (r != 0) } }
		case krb.PropIDScaleFactor:
			if config != nil { if sf, ok := getFixedPointValue(&prop); ok { config.ScaleFactor = sf } }
		case krb.PropIDIcon:
             if config != nil {
                 // TODO: Handle App Icon loading if needed
                 // if resIdx, ok := getByteValue(&prop); ok { ... load resource ... rl.SetWindowIcon(...) }
             }
        case krb.PropIDVersion:
            // Could store this in config if needed
        case krb.PropIDAuthor:
             // Could store this in config if needed

		// Add other direct properties here (Opacity, ZIndex, etc. - require renderer support)
		}
	}
}


// --- Value Parsing Helpers ---

func getColorValue(prop *krb.Property, flags uint16) (rl.Color, bool) {
	if prop.ValueType != krb.ValTypeColor { return rl.Color{}, false }
	useExtended := (flags & krb.FlagExtendedColor) != 0
	if useExtended {
		if len(prop.Value) == 4 {
			return rl.NewColor(prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]), true
		}
	} else {
		if len(prop.Value) == 1 {
			// Palette index - requires palette definition, return Magenta for now
			// TODO: Implement palette lookup if needed
			log.Printf("Warning: Palette color index %d used, but palettes not implemented. Returning Magenta.", prop.Value[0])
			return rl.Magenta, true // Placeholder
		}
	}
	return rl.Color{}, false
}

func getByteValue(prop *krb.Property) (uint8, bool) {
	// Allow Byte, Enum, and Index types represented by 1 byte
	if (prop.ValueType == krb.ValTypeByte || prop.ValueType == krb.ValTypeString || prop.ValueType == krb.ValTypeResource || prop.ValueType == krb.ValTypeEnum) && len(prop.Value) == 1 {
		return prop.Value[0], true
	}
	return 0, false
}

func getShortValue(prop *krb.Property) (uint16, bool) {
	if prop.ValueType == krb.ValTypeShort && len(prop.Value) == 2 {
		return binary.LittleEndian.Uint16(prop.Value), true
	}
    // Percentage is also uint16, but handle via getFixedPointValue
	return 0, false
}

func getStringValue(prop *krb.Property, doc *krb.Document) (string, bool) {
	if prop.ValueType == krb.ValTypeString && len(prop.Value) == 1 {
		idx := prop.Value[0]
		if int(idx) < len(doc.Strings) {
			return doc.Strings[idx], true
		}
	}
	return "", false
}

// Returns value / 256.0
func getFixedPointValue(prop *krb.Property) (float32, bool) {
    if prop.ValueType == krb.ValTypePercentage && len(prop.Value) == 2 {
        val := binary.LittleEndian.Uint16(prop.Value)
        return float32(val) / 256.0, true
    }
    return 0, false
}

// Returns [T, R, B, L] border widths
func getEdgeInsetsValue(prop *krb.Property) ([4]uint8, bool) {
    // Assuming 4 bytes for EdgeInsets (as in C example)
    if prop.ValueType == krb.ValTypeEdgeInsets && len(prop.Value) == 4 {
        return [4]uint8{prop.Value[0], prop.Value[1], prop.Value[2], prop.Value[3]}, true
    }
    // Could also support ValTypeShort with size 8 if needed
    return [4]uint8{}, false
}

// Go versions of min/max for integer types
func min(a, b int) int {
	if a < b { return a }
	return b
}
func max(a, b int) int {
    if a > b { return a }
    return b
}

// Removed unused import "os"
// Corrected the call to applyStyleProperties to pass pointers (&)