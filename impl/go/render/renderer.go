package render

import (
	"github.com/waozixyz/kryon/impl/go/krb"
	rl "github.com/gen2brain/raylib-go/raylib" // Use rl alias
)

const (
	MaxRenderElements      = 256 // Mirror C define, adjust if needed
	InvalidResourceIndex = 0xFF
	BaseFontSize        = 20 // Mirror C define
)


// Event details stored per element
type EventCallbackInfo struct {
	EventType   krb.EventType // e.g., EVENT_TYPE_CLICK
	HandlerName string 
}

type RenderElement struct {
    Header        krb.ElementHeader
    OriginalIndex int
    Parent        *RenderElement
    Children      []*RenderElement

    // Visuals
    BgColor       rl.Color
    FgColor       rl.Color
    BorderColor   rl.Color
    BorderWidths  [4]uint8 // T, R, B, L
    TextAlignment uint8    // 0: Start, 1: Center, 2: End
    Text          string   // Resolved text content

    // Resources
    ResourceIndex uint8
    Texture       rl.Texture2D
    TextureLoaded bool

    // Layout Results
    RenderX int
    RenderY int
    RenderW int
    RenderH int
    IntrinsicW int
    IntrinsicH int

    // Interaction & Events
    IsInteractive bool
    EventHandlers []EventCallbackInfo
}

// WindowConfig holds application-level settings derived from the KRB App element.
type WindowConfig struct {
	Width       int
	Height      int
	Title       string
	Resizable   bool
	ScaleFactor float32
	DefaultBg   rl.Color
	// Icon info could be added here if needed
}

// Renderer defines the interface for a KRB rendering backend.
type Renderer interface {
	// Init initializes the rendering backend (e.g., window creation).
	Init(config WindowConfig) error

	// PrepareTree processes the parsed KRB document and builds the render tree.
	// It also loads necessary resources like textures.
	// Returns the root RenderElement(s) and potentially updated config.
	PrepareTree(doc *krb.Document, krbFilePath string) ([]*RenderElement, WindowConfig, error)

    // GetRenderTree returns the prepared render elements (useful for event handling).
    GetRenderTree() []*RenderElement

	// RenderFrame draws the current state of the render tree to the screen.
	RenderFrame(roots []*RenderElement)

	// Cleanup releases resources used by the renderer (e.g., textures, window).
	Cleanup()

    // ShouldClose checks if the renderer window should close (e.g., user action).
    ShouldClose() bool

    // BeginFrame performs setup before drawing (e.g., BeginDrawing in Raylib).
    BeginFrame()

    // EndFrame performs teardown after drawing (e.g., EndDrawing in Raylib).
    EndFrame()

    // PollEvents handles input events.
    PollEvents()
}

// Default Window Configuration
func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		Width:       800,
		Height:      600,
		Title:       "Kryon Renderer",
		Resizable:   false,
		ScaleFactor: 1.0,
		DefaultBg:   rl.Black,
	}
}