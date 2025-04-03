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

// RenderElement holds processed data for a single element ready for rendering.
type RenderElement struct {
	Header         krb.ElementHeader // Copy of original header
	OriginalIndex  int               // Index in the original krb.Document.Elements
	Text           string            // Resolved text content (if applicable)
	BgColor        rl.Color          // Resolved background color
	FgColor        rl.Color          // Resolved foreground/text color
	BorderColor    rl.Color          // Resolved border color
	BorderWidths   [4]uint8          // Resolved border widths [T, R, B, L] (scaled?)
	TextAlignment  uint8             // Resolved text alignment (0=L, 1=C, 2=R) - Use constants later?
	IsInteractive  bool              // E.g., Button, Input
	ResourceIndex  uint8             // Resolved resource index (0-based), or InvalidResourceIndex
	TextureLoaded  bool              // Flag if texture was successfully loaded
	Texture        rl.Texture2D      // Raylib texture (if applicable) - Specific to Raylib backend for now

	// Tree Structure
	Parent   *RenderElement
	Children []*RenderElement // Slice is more idiomatic Go

	// Runtime Layout Calculation Cache
	RenderX int // Final calculated X position on screen
	RenderY int // Final calculated Y position on screen
	RenderW int // Final calculated width on screen
	RenderH int // Final calculated height on screen
    IntrinsicW int // Calculated natural width before layout constraints
    IntrinsicH int // Calculated natural height before layout constraints
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