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

type RenderElement struct {
	Header         krb.ElementHeader
	OriginalIndex  int
	Text           string
	BgColor        rl.Color
	FgColor        rl.Color
	BorderColor    rl.Color
	BorderWidths   [4]uint8 // [T, R, B, L]
	TextAlignment  uint8
	IsInteractive  bool
	ResourceIndex  uint8
	TextureLoaded  bool
	Texture        rl.Texture2D

	// Tree Structure
	Parent   *RenderElement
	Children []*RenderElement

	// --- Layout Calculation Fields ---
	// Calculated during Layout Pass (PerformLayout)
	RenderX    int
	RenderY    int
	RenderW    int // Final Width
	RenderH    int // Final Height
	IntrinsicW int // Natural width (content, children) before constraints/grow
	IntrinsicH int // Natural height (content, children) before constraints/grow
	// Add Padding/Margin fields if KRB/Styles support them
	// Padding [4]uint8 // Example
	// Margin [4]uint8 // Example
	// PositionHint string // If compiler/PrepareTree adds it later
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