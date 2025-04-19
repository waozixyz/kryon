// render/render.go
package render

import (
	"github.com/waozixyz/kryon/impl/go/krb"
	rl "github.com/gen2brain/raylib-go/raylib" // Use rl alias for Raylib types
)

// Constants related to rendering or element limits.
const (
	MaxRenderElements    = 256 // Example limit, adjust if needed
	InvalidResourceIndex = 0xFF
	BaseFontSize         = 20 // Default font size
)

// EventCallbackInfo stores details about a standard KRB event handler.
type EventCallbackInfo struct {
	EventType   krb.EventType // The type of event (e.g., Click, Change).
	HandlerName string        // The string name referencing the Go function to call.
}

// RenderElement holds the state and properties needed to render a single UI element.
type RenderElement struct {
	Header        krb.ElementHeader // The original KRB element header.
	OriginalIndex int               // Index in the original flat KRB element list.
	Parent        *RenderElement    // Pointer to the parent element in the tree.
	Children      []*RenderElement  // Slice of pointers to child elements.

	// Resolved Visual Properties
	BgColor       rl.Color     // Background color (includes alpha).
	FgColor       rl.Color     // Foreground/Text color.
	BorderColor   rl.Color     // Border color.
	BorderWidths  [4]uint8     // Border widths: Top, Right, Bottom, Left.
	TextAlignment uint8        // Text alignment (0: Start, 1: Center, 2: End).
	Text          string       // Resolved text content for display.

	// Resource Information
	ResourceIndex uint8        // Index into KRB resource table (0-based). 0xFF if none.
	Texture       rl.Texture2D // Loaded texture (if applicable).
	TextureLoaded bool         // Flag indicating if the texture load succeeded.

	// Layout Calculation Results
	RenderX    int // Final calculated X position (absolute screen coordinates).
	RenderY    int // Final calculated Y position (absolute screen coordinates).
	RenderW    int // Final calculated width.
	RenderH    int // Final calculated height.
	IntrinsicW int // Calculated width before parent constraints/growth.
	IntrinsicH int // Calculated height before parent constraints/growth.

	// State
	IsVisible     bool // Whether the element should be rendered.
	IsInteractive bool // Whether the element responds to input (Button, Input).

	// Attached Handlers
	EventHandlers []EventCallbackInfo // List of standard KRB event handlers attached.
}

// WindowConfig holds application-level settings derived from the KRB App element.
type WindowConfig struct {
	Width       int      // Initial window width.
	Height      int      // Initial window height.
	Title       string   // Window title.
	Resizable   bool     // Whether the window can be resized.
	ScaleFactor float32  // UI scaling factor.
	DefaultBg   rl.Color // Default window background clear color.
}

// CustomComponentHandler defines the interface for implementing custom component logic
// (layout, drawing, event handling) within a specific renderer implementation.
type CustomComponentHandler interface {
	// HandleLayoutAdjustment allows modifying element layout after the standard pass.
	HandleLayoutAdjustment(el *RenderElement, doc *krb.Document) error

	// --- Optional Methods (Add signatures here if needed) ---
	// Prepare(el *RenderElement, doc *krb.Document) error
	// Draw(el *RenderElement, scale float32, rendererInstance Renderer) (skipStandardDraw bool, err error)
	// HandleEvent(el *RenderElement, eventType krb.EventType /* + details */) (handled bool, err error)
}

// Renderer defines the core interface that all Kryon rendering backends must implement.
type Renderer interface {
	// Init initializes the rendering backend (e.g., creates window).
	Init(config WindowConfig) error

	// PrepareTree processes the KRB document, builds the RenderElement tree, and loads resources.
	PrepareTree(doc *krb.Document, krbFilePath string) ([]*RenderElement, WindowConfig, error)

	// GetRenderTree returns the flat list of all processed RenderElements.
	GetRenderTree() []*RenderElement

	// RenderFrame draws the current UI state for one frame.
	RenderFrame(roots []*RenderElement)

	// Cleanup releases all resources used by the renderer.
	Cleanup()

	// ShouldClose checks if the renderer has received a signal to close.
	ShouldClose() bool

	// BeginFrame sets up drawing for the frame (e.g., clears background).
	BeginFrame()

	// EndFrame finalizes drawing for the frame (e.g., swaps buffers).
	EndFrame()

	// PollEvents processes user input and window events for the frame.
	PollEvents()

	// RegisterEventHandler registers a Go function for a standard KRB event callback name.
	RegisterEventHandler(name string, handler func())

	// RegisterCustomComponent registers a Go handler for a specific custom component identifier.
	RegisterCustomComponent(identifier string, handler CustomComponentHandler) error

	// LoadAllTextures explicitly triggers loading of textures (required after Init).
	LoadAllTextures() error
}

// DefaultWindowConfig provides sensible default values for the window configuration.
func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		Width:       800,
		Height:      600,
		Title:       "Kryon Application",
		Resizable:   false,
		ScaleFactor: 1.0,
		DefaultBg:   rl.Black, // Default to black background
	}
}