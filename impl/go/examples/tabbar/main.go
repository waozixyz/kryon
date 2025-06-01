// cmd/kryon-raylib/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/waozixyz/kryon/impl/go/krb"    // KRB parser
	"github.com/waozixyz/kryon/impl/go/render" // Renderer interface
	// Import the specific implementation (Raylib)
	"github.com/waozixyz/kryon/impl/go/render/raylib"
)

// --- Placeholder Event Handler Functions ---
// These functions will be called when the corresponding KRB onClick names are triggered.
// They need access to the application state (like the renderer/elements) to modify visibility.

var (
	// Keep a reference to the renderer/elements accessible to handlers
	// This is a simple way; consider dependency injection or a dedicated App state struct for larger apps.
	appRenderer render.Renderer
	allElements []*render.RenderElement
)

func showHomePage() {
	log.Println("ACTION: Show Home Page")
	if appRenderer == nil {
		return
	}
	// In a real app, you'd likely get elements by ID
	for _, el := range allElements {
		// Assuming specific IDs for page containers
		if el.Header.ID > 0 && el.Header.ID < uint8(len(appRenderer.GetRenderTree())) { // Basic bounds check
			//Need to get the string value of the ID first! Need KRB doc access or storing name on RenderElement
			// This logic needs refinement - better to store ID name on RenderElement or lookup via docRef
			if el.OriginalIndex == 2 { // Assuming page_home is element 2 (Fragile!)
				el.IsVisible = true
			} else if el.OriginalIndex == 4 || el.OriginalIndex == 6 { // page_search, page_profile
				el.IsVisible = false
			}
		}
		// Also update tab button styles if needed (more complex)
	}
}

func showSearchPage() {
	log.Println("ACTION: Show Search Page")
	if appRenderer == nil {
		return
	}
	for _, el := range allElements {
		if el.OriginalIndex == 4 {
			el.IsVisible = true
		} else if el.OriginalIndex == 2 || el.OriginalIndex == 6 {
			el.IsVisible = false
		}
	}
}

func showProfilePage() {
	log.Println("ACTION: Show Profile Page")
	if appRenderer == nil {
		return
	}
	for _, el := range allElements {
		if el.OriginalIndex == 6 {
			el.IsVisible = true
		} else if el.OriginalIndex == 2 || el.OriginalIndex == 4 {
			el.IsVisible = false
		}
	}
}

// --- Main Application Entry Point ---

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Add file/line numbers to log

	// --- Command Line Args ---
	krbFilePath := flag.String("file", "", "Path to the KRB file to render")
	flag.Parse()

	if *krbFilePath == "" {
		fmt.Println("Usage: kryon-go-renderer -file <krb_file_path>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Loading KRB file: %s", *krbFilePath)

	// --- Open and Parse KRB File ---
	file, err := os.Open(*krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Cannot open KRB file '%s': %v", *krbFilePath, err)
	}
	defer file.Close()

	doc, err := krb.ReadDocument(file)
	if err != nil {
		log.Fatalf("ERROR: Failed to parse KRB file '%s': %v", *krbFilePath, err)
	}
	log.Printf("Parsed KRB OK - Ver=%d.%d Elements=%d Styles=%d Strings=%d Resources=%d Flags=0x%04X",
		doc.VersionMajor, doc.VersionMinor, doc.Header.ElementCount, doc.Header.StyleCount, doc.Header.StringCount, doc.Header.ResourceCount, doc.Header.Flags)

	if doc.Header.ElementCount == 0 {
		log.Println("WARN: No elements found in KRB file. Exiting.")
		return
	}

	// --- Initialize Renderer ---
	renderer := raylib.NewRaylibRenderer() // Instantiate the Raylib implementation
	appRenderer = renderer                 // Store global reference for handlers (simple approach)

	// --- *** REGISTER CUSTOM COMPONENT HANDLERS *** ---
	// Register the handler for TabBar components. The identifier "TabBar"
	// must match the logic used in custom_components.go to identify TabBars.
	raylib.RegisterCustomComponent("TabBar", &raylib.TabBarHandler{})
	// Register handlers for other custom components here:
	// raylib.RegisterCustomComponent("DateTimePicker", &raylib.DateTimePickerHandler{})

	// --- *** REGISTER EVENT HANDLERS *** ---
	// Map the callback names used in the KRB file to the actual Go functions.
	renderer.RegisterEventHandler("showHomePage", showHomePage)
	renderer.RegisterEventHandler("showSearchPage", showSearchPage)
	renderer.RegisterEventHandler("showProfilePage", showProfilePage)
	// Register any other event handlers defined in your KRB files.

	// --- Prepare Render Tree (Loads resources, processes App element) ---
	// This builds the initial element structure based on standard KRB data.
	roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}
	// Store the flat element list for event handlers to access/modify
	allElements = renderer.GetRenderTree()

	// --- Initialize Window (using config derived from PrepareTree) ---
	err = renderer.Init(windowConfig)
	if err != nil {
		renderer.Cleanup() // Attempt cleanup even if init fails
		log.Fatalf("ERROR: Failed to initialize renderer: %v", err)
	}
	defer renderer.Cleanup() // Ensure cleanup happens on normal exit

	log.Println("Entering main loop...")

	// --- Main Loop ---
	for !renderer.ShouldClose() {
		// Handle Input / Window Events
		renderer.PollEvents() // Checks mouse clicks, window close, resize etc.

		// --- Update Application State (Placeholder) ---
		// This is where you might handle animations, state changes triggered
		// by events, etc. For now, visibility is handled directly in event handlers.

		// --- Drawing ---
		renderer.BeginFrame()       // BeginDrawing + ClearBackground
		renderer.RenderFrame(roots) // Standard Layout + Custom Adjustments + Drawing
		renderer.EndFrame()         // EndDrawing
	}

	log.Println("Exiting.")
}
