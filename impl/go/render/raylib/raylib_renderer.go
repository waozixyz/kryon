// render/raylib/raylib_renderer.go
package raylib

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	// "strings" // May not be needed directly here anymore if GetCustomPropertyValue moves to utils

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
)

const baseFontSize = 18.0
const componentNameConventionKey = "_componentName"
const childrenSlotIDName = "children_host" // Convention for KRY-usage children slot

type RaylibRenderer struct {
	config          render.WindowConfig
	elements        []render.RenderElement // Stores all elements, including expanded ones
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
		rl.SetWindowSize(config.Width, config.Height) // Enforce fixed size
	}

	rl.SetTargetFPS(60) // Or from config if specified

	if !rl.IsWindowReady() {
		return fmt.Errorf("RaylibRenderer Init: rl.InitWindow failed or window is not ready")
	}
	log.Println("RaylibRenderer Init: Raylib window is ready.")
	return nil
}

func (r *RaylibRenderer) Cleanup() {
	log.Println("RaylibRenderer Cleanup: Unloading textures...")
	unloadedCount := 0
	for resourceIdx, texture := range r.loadedTextures {
		if texture.ID > 0 { // Check if texture is valid before unloading
			rl.UnloadTexture(texture)
			unloadedCount++
		}
		delete(r.loadedTextures, resourceIdx) // Remove from map
	}
	log.Printf("RaylibRenderer Cleanup: Unloaded %d textures from cache.", unloadedCount)
	r.loadedTextures = make(map[uint8]rl.Texture2D) // Reinitialize map

	if rl.IsWindowReady() {
		log.Println("RaylibRenderer Cleanup: Closing Raylib window...")
		rl.CloseWindow()
	} else {
		log.Println("RaylibRenderer Cleanup: Raylib window was already closed or not initialized.")
	}
}

func (r *RaylibRenderer) ShouldClose() bool {
	// Check if window is initialized before checking if it should close
	return rl.IsWindowReady() && rl.WindowShouldClose()
}

func (r *RaylibRenderer) BeginFrame() {
	rl.BeginDrawing()
	rl.ClearBackground(r.config.DefaultBg)
}

func (r *RaylibRenderer) EndFrame() {
	rl.EndDrawing()
}

// GetRenderTree returns all processed render elements.
// This might be used for external inspection or debugging.
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

func (r *RaylibRenderer) PerformLayoutChildrenOfElement(
	parent *render.RenderElement,
	parentClientOriginX, parentClientOriginY,
	availableClientWidth, availableClientHeight float32,
) {
	// This method simply calls the internal PerformLayoutChildren.
	// It's exposed via the Renderer interface for custom handlers.
	r.PerformLayoutChildren(parent, parentClientOriginX, parentClientOriginY, availableClientWidth, availableClientHeight)
}

func (r *RaylibRenderer) PollEvents() {
	if !rl.IsWindowReady() {
		return
	}

	mousePos := rl.GetMousePosition()
	currentMouseCursor := rl.MouseCursorDefault // Default cursor
	isMouseButtonClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	clickHandledThisFrame := false

	// Iterate in reverse order (topmost elements first) through all elements
	// because r.elements is a flat list, and we need to check for hover/click
	// on the element visually on top.
	for i := len(r.elements) - 1; i >= 0; i-- {
		el := &r.elements[i] // Get pointer to the element

		if !el.IsVisible || el.RenderW <= 0 || el.RenderH <= 0 { // Skip non-visible or zero-size elements
			continue
		}

		elementBounds := rl.NewRectangle(el.RenderX, el.RenderY, el.RenderW, el.RenderH)
		isMouseHovering := rl.CheckCollisionPointRec(mousePos, elementBounds)

		// Handle hover state for cursor change (only for the topmost hovered element)
		if isMouseHovering {
			if el.IsInteractive { // Only change cursor if element is interactive
				currentMouseCursor = rl.MouseCursorPointingHand
			}
			// Process click event if it occurs on this interactive element
			if el.IsInteractive && isMouseButtonClicked && !clickHandledThisFrame {
				eventWasProcessedByCustomHandler := false
				componentID, isCustomInstance := GetCustomPropertyValue(el, componentNameConventionKey, r.docRef)

				if isCustomInstance && componentID != "" {
					if customHandler, handlerExists := r.customHandlers[componentID]; handlerExists {
						// Check if handler implements event handling
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
								clickHandledThisFrame = true // Ensure only one click is processed per frame
							}
						}
					}
				}

				// If not handled by custom handler, try standard KRB event handlers
				if !eventWasProcessedByCustomHandler && len(el.EventHandlers) > 0 {
					for _, eventInfo := range el.EventHandlers {
						if eventInfo.EventType == krb.EventTypeClick {
							goHandlerFunc, found := r.eventHandlerMap[eventInfo.HandlerName]
							if found {
								goHandlerFunc() // Execute the registered Go function
								clickHandledThisFrame = true
							} else {
								log.Printf("Warn PollEvents: Standard KRB click handler named '%s' (for %s) is not registered.",
									eventInfo.HandlerName, el.SourceElementName)
							}
							break // Assuming one click action per element
						}
					}
				}
			}
			// Once the topmost hovered element is found (and processed for click if any),
			// break the loop as no elements below it can be interacted with for this event pass.
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
		return fmt.Errorf("cannot load textures, Raylib window is not ready/initialized for GL operations")
	}

	log.Println("LoadAllTextures: Starting...")
	errCount := 0
	r.performTextureLoading(&errCount) // Uses r.docRef and r.elements internally
	log.Printf("LoadAllTextures: Complete. Encountered %d errors.", errCount)
	if errCount > 0 {
		return fmt.Errorf("encountered %d errors during texture loading", errCount)
	}
	return nil
}

func (r *RaylibRenderer) performTextureLoading(errorCounter *int) {
	if r.docRef == nil || r.elements == nil {
		log.Println("Error performTextureLoading: docRef or elements is nil.")
		if errorCounter != nil {
			*errorCounter++
		}
		return
	}

	for i := range r.elements {
		el := &r.elements[i] // Get pointer to the element in the slice
		needsTexture := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton) &&
			el.ResourceIndex != render.InvalidResourceIndex
		if !needsTexture {
			continue
		}

		resIndex := el.ResourceIndex
		if int(resIndex) >= len(r.docRef.Resources) {
			log.Printf("Error performTextureLoading: Elem %s (GlobalIdx %d) ResourceIndex %d out of bounds for doc.Resources (len %d)",
				el.SourceElementName, el.OriginalIndex, resIndex, len(r.docRef.Resources))
			if errorCounter != nil {
				*errorCounter++
			}
			el.TextureLoaded = false
			continue
		}
		res := r.docRef.Resources[resIndex]

		// Check cache first
		if loadedTex, exists := r.loadedTextures[resIndex]; exists {
			el.Texture = loadedTex
			el.TextureLoaded = (loadedTex.ID > 0) // Check if cached texture is valid
			if !el.TextureLoaded {
				log.Printf("Warn performTextureLoading: Cached texture for resource index %d was invalid. Re-attempting load.", resIndex)
				// Fall through to attempt loading again if cached one was bad.
				// To strictly prevent re-load, this block should `continue`.
				// For now, let's allow re-attempt by not continuing.
				// Or, better, remove the invalid entry from cache here.
				delete(r.loadedTextures, resIndex) // Remove invalid from cache
			} else {
				continue // Valid texture found in cache
			}
		}

		var texture rl.Texture2D
		loadedOk := false

		if res.Format == krb.ResFormatExternal {
			resourceName, nameOk := getStringValueByIdx(r.docRef, res.NameIndex)
			if !nameOk {
				log.Printf("Error performTextureLoading: Could not get resource name for external resource index: %d", res.NameIndex)
				if errorCounter != nil {
					*errorCounter++
				}
				el.TextureLoaded = false
				continue
			}

			fullPath := filepath.Join(r.krbFileDir, resourceName)
			if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
				log.Printf("Error performTextureLoading: External resource file not found: %s", fullPath)
				if errorCounter != nil {
					*errorCounter++
				}
				el.TextureLoaded = false
				continue
			}

			img := rl.LoadImage(fullPath)
			if img.Data == nil || img.Width == 0 || img.Height == 0 {
				log.Printf("Error performTextureLoading: Failed to load image data for external resource: %s", fullPath)
				if errorCounter != nil {
					*errorCounter++
				}
				rl.UnloadImage(img) // Important to unload even if data is nil
				el.TextureLoaded = false
				continue
			}

			texture = rl.LoadTextureFromImage(img) // Needs GL context (IsWindowReady should be true)
			rl.UnloadImage(img)                    // Unload image RAM copy after texture is in VRAM
			if texture.ID > 0 {
				loadedOk = true
			} else {
				log.Printf("Error performTextureLoading: Failed to create texture from image for %s", fullPath)
				if errorCounter != nil {
					*errorCounter++
				}
			}

		} else if res.Format == krb.ResFormatInline {
			if res.InlineData == nil || res.InlineDataSize == 0 {
				log.Printf("Error performTextureLoading: Inline resource data is nil or size 0 (name index: %d)", res.NameIndex)
				if errorCounter != nil {
					*errorCounter++
				}
				el.TextureLoaded = false
				continue
			}

			// Determine extension, assume png for now or get from krb.Resource if available
			ext := ".png" // TODO: This might need to be derived from resource metadata if not always PNG
			// Alternatively, raylib might auto-detect based on magic bytes for some formats.
			img := rl.LoadImageFromMemory(ext, res.InlineData, int32(len(res.InlineData)))
			if img.Data == nil || img.Width == 0 || img.Height == 0 {
				log.Printf("Error performTextureLoading: Failed to load image data from inline resource (name index: %d, size: %d)", res.NameIndex, res.InlineDataSize)
				if errorCounter != nil {
					*errorCounter++
				}
				rl.UnloadImage(img)
				el.TextureLoaded = false
				continue
			}

			texture = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
			if texture.ID > 0 {
				loadedOk = true
			} else {
				log.Printf("Error performTextureLoading: Failed to create texture from inline image data (name index %d)", res.NameIndex)
				if errorCounter != nil {
					*errorCounter++
				}
			}
		} else {
			log.Printf("Error performTextureLoading: Unknown resource format %d for resource (name index: %d)", res.Format, res.NameIndex)
			if errorCounter != nil {
				*errorCounter++
			}
		}

		if loadedOk {
			el.Texture = texture
			el.TextureLoaded = true
			r.loadedTextures[resIndex] = texture // Add to cache
		} else {
			el.TextureLoaded = false
			// r.loadedTextures[resIndex] = rl.Texture2D{} // Cache failure if needed, or just don't add
		}
	}
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
		// Enforce fixed size if window somehow changed despite not being resizable
		screenWidth := int(rl.GetScreenWidth())
		screenHeight := int(rl.GetScreenHeight())
		if currentWidth != screenWidth || currentHeight != screenHeight {
			rl.SetWindowSize(currentWidth, currentHeight)
		}
	}

	// Perform layout for all root elements and their descendants
	for _, root := range roots {
		if root != nil {
			r.PerformLayout(root, 0, 0, float32(currentWidth), float32(currentHeight))
		}
	}

	// Apply custom component layout adjustments after main layout pass
	r.ApplyCustomComponentLayoutAdjustments() // Operates on r.elements

	// Render all root elements and their descendants
	for _, root := range roots {
		if root != nil {
			r.renderElementRecursiveWithCustomDraw(root, r.scaleFactor)
		}
	}
}

// ApplyCustomComponentLayoutAdjustments iterates through all elements to find custom components
// and allows them to make final layout adjustments.
func (r *RaylibRenderer) ApplyCustomComponentLayoutAdjustments() {
	if r.docRef == nil || len(r.customHandlers) == 0 || len(r.elements) == 0 {
		return
	}
	// Iterate over all elements, as custom components can be anywhere in the tree
	for i := range r.elements {
		el := &r.elements[i] // Get pointer to element
		if el == nil {
			continue
		}
		componentIdentifier, found := GetCustomPropertyValue(el, componentNameConventionKey, r.docRef)
		if found && componentIdentifier != "" {
			handler, handlerFound := r.customHandlers[componentIdentifier]
			if handlerFound {
				err := handler.HandleLayoutAdjustment(el, r.docRef, r) // Pass docRef if handler needs it
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
			if drawer, ok := handler.(render.CustomDrawer); ok { // Check against CustomDrawer interface
				skipStandardDraw, drawErr = drawer.Draw(el, scale, r) // Pass r as Renderer instance
				if drawErr != nil {
					log.Printf("ERROR renderElementRecursiveWithCustomDraw: Custom Draw handler for component '%s' [%s] failed: %v",
						componentIdentifier, el.SourceElementName, drawErr)
				}
			}
		}
	}

	if !skipStandardDraw {
		r.renderElementRecursive(el, scale) // Standard draw path
	} else {
		// If custom draw handles its own children, this loop might be skipped based on CustomDrawer's contract.
		// The current logic means: custom drawer draws the element, then its children are drawn recursively
		// using this same logic (they might also be custom or standard).
		// This assumes CustomDrawer.Draw() only draws the element itself, not its children.
		// If CustomDrawer is meant to handle its children, it should not recurse here.
		// Let's assume skipStandardDraw means the CustomDrawer has handled *everything* for this 'el',
		// including its children if it wants to. If it wants default child rendering, it should return `false` for skipStandardDraw
		// and potentially do its custom drawing within the standard recursive call.
		//
		// Original code's behavior: If skipStandardDraw is true, it then iterates children and calls
		// renderElementRecursiveWithCustomDraw on them. This means a custom component that skips standard draw
		// for itself can still have its children rendered through the normal pipeline (or further custom components). This seems reasonable.
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
		// Still process children, they might be absolutely positioned or have size independent of this non-drawn parent.
		for _, child := range el.Children {
			r.renderElementRecursive(child, scale) // Standard recursion for children
		}
		return
	}

	renderX, renderY := int32(renderXf), int32(renderYf)
	renderW, renderH := int32(renderWf), int32(renderHf)

	effectiveBgColor := el.BgColor
	effectiveFgColor := el.FgColor
	borderColor := el.BorderColor

	// Handle active/inactive styles for buttons (simplified: only color changes)
	if (el.Header.Type == krb.ElemTypeButton) && (el.ActiveStyleNameIndex != 0 || el.InactiveStyleNameIndex != 0) {
		targetStyleNameIndex := el.InactiveStyleNameIndex
		if el.IsActive { // This state needs to be set (e.g., during event polling or interaction)
			targetStyleNameIndex = el.ActiveStyleNameIndex
		}
		if r.docRef != nil && targetStyleNameIndex != 0 {
			targetStyleID := findStyleIDByNameIndex(r.docRef, targetStyleNameIndex)
			if targetStyleID != 0 {
				bg, fg, styleColorOk := getStyleColors(r.docRef, targetStyleID, r.docRef.Header.Flags)
				if styleColorOk { // Apply if colors were successfully retrieved
					effectiveBgColor = bg
					effectiveFgColor = fg
				}
			}
		}
	}

	// Draw Background (if not fully transparent)
	if effectiveBgColor.A > 0 {
		rl.DrawRectangle(renderX, renderY, renderW, renderH, effectiveBgColor)
	}

	// Draw Borders
	topBorder := scaledI32(el.BorderWidths[0], scale)
	rightBorder := scaledI32(el.BorderWidths[1], scale)
	bottomBorder := scaledI32(el.BorderWidths[2], scale)
	leftBorder := scaledI32(el.BorderWidths[3], scale)

/*
    if el.Header.Type == krb.ElemTypeApp {
        log.Printf("DEBUG AppBorderDraw: Name='%s', OrigIdx=%d", el.SourceElementName, el.OriginalIndex)
        log.Printf("  RenderRect: X:%.1f, Y:%.1f, W:%.1f, H:%.1f => renderW:%d, renderH:%d", renderXf, renderYf, renderWf, renderHf, renderW, renderH)
        log.Printf("  Borders (scaledI32): T:%d, R:%d, B:%d, L:%d", topBorder, rightBorder, bottomBorder, leftBorder)
        log.Printf("  BorderWidths (raw): T:%d, R:%d, B:%d, L:%d", el.BorderWidths[0], el.BorderWidths[1], el.BorderWidths[2], el.BorderWidths[3])
        log.Printf("  BorderColor: %v", borderColor)
    }
*/
	// Clamp borders if they exceed element size (to prevent overlap or negative content area)
	clampedTop, clampedBottom := clampOpposingBorders(int(topBorder), int(bottomBorder), int(renderH))
	clampedLeft, clampedRight := clampOpposingBorders(int(leftBorder), int(rightBorder), int(renderW))
	drawBorders(int(renderX), int(renderY), int(renderW), int(renderH),
		clampedTop, clampedRight, clampedBottom, clampedLeft, borderColor)

	// Calculate content area (inside borders and padding)
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
	contentWidth := maxI32(0, int32(contentWidth_f32))   // Ensure non-negative
	contentHeight := maxI32(0, int32(contentHeight_f32)) // Ensure non-negative

	// Draw Content (Text, Image) within the content area using Scissor mode
	if contentWidth > 0 && contentHeight > 0 {
		rl.BeginScissorMode(contentX, contentY, contentWidth, contentHeight)
		r.drawContent(el, int(contentX), int(contentY), int(contentWidth), int(contentHeight), scale, effectiveFgColor)
		rl.EndScissorMode()
	}

	// Recursively render children
	for _, child := range el.Children {
		r.renderElementRecursive(child, scale)
	}
}

func (r *RaylibRenderer) drawContent(el *render.RenderElement, cx, cy, cw, ch int, scale float32, effectiveFgColor rl.Color) {
	// Draw Text
	if (el.Header.Type == krb.ElemTypeText || el.Header.Type == krb.ElemTypeButton) && el.Text != "" {
		// TODO: Font size from properties if available, fallback to baseFontSize
		fontSize := int32(math.Max(1.0, math.Round(baseFontSize*float64(scale))))
		textWidthMeasured := rl.MeasureText(el.Text, fontSize)
		textHeightMeasured := fontSize // Assuming single line height

		textDrawX := int32(cx)
		textDrawY := int32(cy + (ch-int(textHeightMeasured))/2) // Vertically center

		switch el.TextAlignment {
		case krb.LayoutAlignCenter:
			textDrawX = int32(cx + (cw-int(textWidthMeasured))/2)
		case krb.LayoutAlignEnd:
			textDrawX = int32(cx + cw - int(textWidthMeasured))
		case krb.LayoutAlignStart: // Default
			// textDrawX = int32(cx) // Already set
		}
		rl.DrawText(el.Text, textDrawX, textDrawY, fontSize, effectiveFgColor)
	}

	// Draw Image
	isImageElement := (el.Header.Type == krb.ElemTypeImage || el.Header.Type == krb.ElemTypeButton)
	if isImageElement && el.TextureLoaded && el.Texture.ID > 0 {
		texWidth := float32(el.Texture.Width)
		texHeight := float32(el.Texture.Height)

		sourceRec := rl.NewRectangle(0, 0, texWidth, texHeight)
		destRec := rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch))

		if destRec.Width > 0 && destRec.Height > 0 && sourceRec.Width > 0 && sourceRec.Height > 0 {
			// TODO: Add image scaling mode (e.g., stretch, fit, fill) from KRB property if available
			rl.DrawTexturePro(el.Texture, sourceRec, destRec, rl.NewVector2(0, 0), 0.0, rl.White) // rl.White tint = no tint
		}
	}
}

// --- Drawing Helpers specific to this file ---

func drawBorders(x, y, w, h, top, right, bottom, left int, color rl.Color) {
	if color.A == 0 {
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
	// Effective Y and Height for side borders (inside top/bottom borders)
	sideY := y + top
	sideH := h - top - bottom
	if sideH > 0 { // Only draw side borders if there's positive height for them
		if left > 0 {
			rl.DrawRectangle(int32(x), int32(sideY), int32(left), int32(sideH), color)
		}
		if right > 0 {
			rl.DrawRectangle(int32(x+w-right), int32(sideY), int32(right), int32(sideH), color)
		}
	}
}

func (r *RaylibRenderer) GetKrbFileDir() string {
	return r.krbFileDir
}

// clampOpposingBorders ensures borders don't overlap or exceed the total size.
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
		// If sum of borders is greater than size, scale them down proportionally
		// to fit within totalSize.
		sum := float32(borderA + borderB)
		borderA = int(float32(borderA) / sum * float32(totalSize))
		borderB = totalSize - borderA // The rest goes to borderB to ensure sum is totalSize
	}
	return borderA, borderB
}
