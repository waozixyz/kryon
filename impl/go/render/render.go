// render/render.go
package render

import (
	"github.com/waozixyz/kryon/impl/go/krb"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	MaxRenderElements    = 1024
	InvalidResourceIndex = 0xFF
	BaseFontSize         = 18.0
)

type EventCallbackInfo struct {
	EventType   krb.EventType
	HandlerName string
}

type RenderElement struct {
	Header               krb.ElementHeader
	OriginalIndex        int
	Parent               *RenderElement
	Children             []*RenderElement
	BgColor              rl.Color
	FgColor              rl.Color
	BorderColor          rl.Color
	BorderWidths         [4]uint8
	Padding              [4]uint8
	TextAlignment        uint8
	Text                 string
	ResourceIndex        uint8
	Texture              rl.Texture2D
	TextureLoaded        bool
	RenderX              float32
	RenderY              float32
	RenderW              float32
	RenderH              float32
	IntrinsicW           int // No longer used in provided layout, but kept for potential future use
	IntrinsicH           int // No longer used in provided layout, but kept for potential future use
	IsVisible            bool
	IsInteractive        bool
	IsActive             bool
	ActiveStyleNameIndex uint8
	InactiveStyleNameIndex uint8
	EventHandlers        []EventCallbackInfo
	DocRef               *krb.Document
	SourceElementName    string
}

type WindowConfig struct {
	Width       int
	Height      int
	Title       string
	Resizable   bool
	ScaleFactor float32
	DefaultBg   rl.Color
}

// Renderer defines the core interface that all Kryon rendering backends must implement.
// (Keep existing Renderer interface definition from your file)
type Renderer interface {
	Init(config WindowConfig) error
	PrepareTree(doc *krb.Document, krbFilePath string) (roots []*RenderElement, config WindowConfig, err error)
	GetRenderTree() []*RenderElement
	RenderFrame(roots []*RenderElement)
	Cleanup()
	ShouldClose() bool
	BeginFrame()
	EndFrame()
	PollEvents()
	RegisterEventHandler(name string, handler func())
	RegisterCustomComponent(identifier string, handler CustomComponentHandler) error
	LoadAllTextures() error

	// Add method for custom handlers to trigger re-layout of children
	// This makes PerformLayoutChildren accessible in a controlled way.
	PerformLayoutChildrenOfElement(
		parent *RenderElement,
		parentClientOriginX, parentClientOriginY,
		availableClientWidth, availableClientHeight float32,
	)
}


// CustomDrawer interface allows a custom component to handle its own drawing.
type CustomDrawer interface {
	Draw(el *RenderElement, scale float32, rendererInstance Renderer) (skipStandardDraw bool, err error)
}

// CustomEventHandler interface allows a custom component to handle specific events.
type CustomEventHandler interface {
    HandleEvent(el *RenderElement, eventType krb.EventType, rendererInstance Renderer) (handled bool, err error)
}

// CustomComponentHandler defines an interface for Go code that provides specialized behavior.
type CustomComponentHandler interface {
	// HandleLayoutAdjustment allows final layout adjustments.
	// Pass the Renderer instance so it can call PerformLayoutChildren if needed.
	HandleLayoutAdjustment(el *RenderElement, doc *krb.Document, rendererInstance Renderer) error

	// Note: If a component also needs to be a CustomDrawer or CustomEventHandler,
	// it should implement those interfaces separately.
	// Example:
	// type MyHandler struct{}
	// func (h *MyHandler) HandleLayoutAdjustment(...) error { ... }
	// func (h *MyHandler) Draw(...) (bool, error) { ... } // Implements CustomDrawer
}


func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		Width:       800,
		Height:      600,
		Title:       "Kryon Application",
		Resizable:   true,
		ScaleFactor: 1.0,
		DefaultBg:   rl.NewColor(30, 30, 30, 255),
	}
}
