// render/render.go
package render

import (
	"github.com/waozixyz/kryon/impl/go/krb" // Import for krb.ElementHeader, krb.EventType, etc.
	rl "github.com/gen2brain/raylib-go/raylib" // Use rl alias for Raylib types (like rl.Color, rl.Texture2D)
)

// Constants related to rendering or element limits.
const (
	// MaxRenderElements defines an example maximum number of renderable elements.
	// This can be adjusted based on application needs and performance considerations.
	MaxRenderElements = 1024 // Increased from 256 for potentially larger UIs.

	// InvalidResourceIndex is a sentinel value indicating no valid resource is associated.
	InvalidResourceIndex = 0xFF

	// BaseFontSize defines a default font size for text rendering if not otherwise specified.
	BaseFontSize = 18.0 // Adjusted from 20 for more common base.
)

// EventCallbackInfo stores details necessary for dispatching a standard KRB event
// to its corresponding Go handler function.
type EventCallbackInfo struct {
	EventType   krb.EventType // The type of event (e.g., Click, Change) defined in the KRB spec.
	HandlerName string        // The string name from KRB, used to look up the registered Go function.
}

// RenderElement holds all the state and resolved properties required by the renderer
// to lay out and draw a single UI element.
type RenderElement struct {
	// Core KRB Data
	Header        krb.ElementHeader // The original element header data from the KRB file.
	OriginalIndex int               // The element's 0-based index in the flat list from the KRB file.

	// Tree Structure
	Parent   *RenderElement   // Pointer to this element's parent in the UI hierarchy. Nil for root elements.
	Children []*RenderElement // Slice of pointers to this element's children.

	// Resolved Visual Properties (after styles and direct properties are applied)
	BgColor       rl.Color     // Final background color, including alpha.
	FgColor       rl.Color     // Final foreground (e.g., text) color, including alpha.
	BorderColor   rl.Color     // Final border color, including alpha.
	BorderWidths  [4]uint8     // Resolved border widths in pixels: [Top, Right, Bottom, Left].
	Padding       [4]uint8     // Resolved padding in pixels: [Top, Right, Bottom, Left].
	TextAlignment uint8        // Resolved text alignment (e.g., krb.LayoutAlignStart, krb.LayoutAlignCenter).
	Text          string       // Resolved text content for display.

	// Resource Information (for images, etc.)
	ResourceIndex uint8        // 0-based index into the KRB document's resource table. InvalidResourceIndex if none.
	Texture       rl.Texture2D // Loaded Raylib texture, if this element is an image or uses one.
	TextureLoaded bool         // Flag indicating if `Texture` holds a valid, loaded texture.

	// Layout Calculation Results
	// These are in absolute screen coordinates and dimensions after layout.
	// Using float32 for precision during layout, especially with scaling.
	RenderX    float32 // Final calculated X position for rendering.
	RenderY    float32 // Final calculated Y position for rendering.
	RenderW    float32 // Final calculated width for rendering.
	RenderH    float32 // Final calculated height for rendering.
	IntrinsicW int     // Calculated "natural" width based on content, before layout constraints.
	IntrinsicH int     // Calculated "natural" height based on content.

	// Element State
	IsVisible     bool // Whether the element (and its children) should be rendered.
	IsInteractive bool // True if the element type (e.g., Button, Input) typically interacts with user input.

	// State for dynamic styling or behavior (e.g., button pressed/active states).
	IsActive             bool  // General-purpose flag indicating an "active" state (e.g., selected tab, pressed button).
	ActiveStyleNameIndex uint8 // KRB string table index for the *name* of the style to apply when IsActive is true.
	InactiveStyleNameIndex uint8 // KRB string table index for the *name* of the style to apply when IsActive is false.

	// Attached Event Handlers
	EventHandlers []EventCallbackInfo // List of standard KRB event handlers resolved for this element.

	// --- Fields to support enhanced layout and debugging ---
	// DocRef provides access to the full KRB document (e.g., for string table lookups by helpers).
	DocRef *krb.Document
	// SourceElementName stores the original KRY name (e.g., component name or 'id' property) for easier debugging.
	SourceElementName string
}

// WindowConfig holds application-level settings, typically derived from the KRB App element,
// used to initialize and configure the main application window.
type WindowConfig struct {
	Width       int      // Initial window width in screen pixels.
	Height      int      // Initial window height in screen pixels.
	Title       string   // Text for the window title bar.
	Resizable   bool     // True if the user should be allowed to resize the window.
	ScaleFactor float32  // Global UI scaling factor (e.g., 1.0 for no scaling, 1.5 for 150%).
	DefaultBg   rl.Color // Default background color used to clear the window each frame.
}

// CustomComponentHandler defines an interface for Go code that provides specialized
// behavior (layout adjustments, custom drawing, event handling) for specific KRY components.
type CustomComponentHandler interface {
	// HandleLayoutAdjustment is called after the standard layout pass, allowing the handler
	// to make final adjustments to the element's (and its children's) layout.
	HandleLayoutAdjustment(el *RenderElement, doc *krb.Document) error

	// --- Optional methods for more advanced custom component behavior ---
	//
	// Prepare is called once when the RenderElement for a custom component instance
	// is first being prepared, allowing for one-time setup.
	// Prepare(el *RenderElement, doc *krb.Document) error
	//
	// Draw allows the handler to take over the drawing of the component entirely.
	// If it returns skipStandardDraw = true, the renderer's standard drawing for this element is skipped.
	// Draw(el *RenderElement, scale float32, rendererInstance Renderer) (skipStandardDraw bool, err error)
	//
	// HandleEvent allows the component to intercept or react to input events.
	// If it returns handled = true, further standard event processing for this event might be skipped.
	// HandleEvent(el *RenderElement, eventType krb.EventType /*, eventDetails ...*/) (handled bool, err error)
}

// Renderer defines the core interface that all Kryon rendering backends (e.g., Raylib, OpenGL)
// must implement. This allows the main application logic to be renderer-agnostic.
type Renderer interface {
	// Init initializes the rendering backend, including creating the application window.
	// It takes a WindowConfig, typically derived from the KRB App element.
	Init(config WindowConfig) error

	// PrepareTree processes the parsed KRB document, constructing a tree of RenderElements.
	// This involves resolving styles, properties, loading resources (or preparing to load),
	// and setting up the initial state of all UI elements.
	// It returns the root elements of the UI tree and the finalized WindowConfig.
	PrepareTree(doc *krb.Document, krbFilePath string) (roots []*RenderElement, config WindowConfig, err error)

	// GetRenderTree returns a flat slice containing pointers to all processed RenderElements.
	// This can be used by application logic or event systems to access any element.
	GetRenderTree() []*RenderElement

	// RenderFrame orchestrates the drawing of a single frame of the UI.
	// This typically involves performing layout calculations and then drawing elements.
	RenderFrame(roots []*RenderElement)

	// Cleanup releases all resources (textures, fonts, window context) used by the renderer.
	// This should be called when the application is shutting down.
	Cleanup()

	// ShouldClose checks if the rendering window has received a signal to close (e.g., user action).
	ShouldClose() bool

	// BeginFrame prepares the renderer for drawing a new frame (e.g., clears the screen).
	BeginFrame()

	// EndFrame finalizes the drawing for the current frame (e.g., swaps graphics buffers).
	EndFrame()

	// PollEvents processes pending user input and window events for the current frame.
	PollEvents()

	// RegisterEventHandler registers a Go function to be called when a KRB event with
	// the given name is triggered on an element.
	RegisterEventHandler(name string, handler func())

	// RegisterCustomComponent registers a Go CustomComponentHandler for a specific
	// component identifier (which should match the name used in KRY Define blocks).
	RegisterCustomComponent(identifier string, handler CustomComponentHandler) error

	// LoadAllTextures explicitly triggers the loading of all textures required by the UI elements.
	// This typically needs to be called after Init (when a graphics context is available)
	// and after PrepareTree (when all resource requirements are known).
	LoadAllTextures() error
}

// DefaultWindowConfig provides sensible default values for the WindowConfig struct.
// These are used if no App element is present in the KRB or if specific properties are missing.
func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		Width:       800,
		Height:      600,
		Title:       "Kryon Application",
		Resizable:   true, // Changed default to true for better UX.
		ScaleFactor: 1.0,
		DefaultBg:   rl.NewColor(30, 30, 30, 255), // Dark gray, similar to "app_base".
	}
}