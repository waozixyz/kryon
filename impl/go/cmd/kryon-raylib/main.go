// cmd/kryon-raylib/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
    
	"path/filepath"
    
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
	krbDocument *krb.Document // Keep reference to doc for ID lookups if needed
)

// Helper function to get element by ID name (more robust)
func findElementByID(idName string) *render.RenderElement {
	if appRenderer == nil || krbDocument == nil {
		return nil
	}
	targetIDIndex := uint8(0) // 0 is often reserved for "no ID"
	found := false
	for idx, str := range krbDocument.Strings {
		if str == idName {
			targetIDIndex = uint8(idx)
			found = true
			break
		}
	}
	if !found || targetIDIndex == 0 {
		log.Printf("WARN: Element ID '%s' not found in string table.", idName)
		return nil
	}

	elements := appRenderer.GetRenderTree() // Get the current tree elements
	for _, el := range elements {
		if el.Header.ID == targetIDIndex {
			return el // Return the first match
		}
	}
	log.Printf("WARN: Element with ID '%s' (Index %d) not found in render tree.", idName, targetIDIndex)
	return nil
}


func showHomePage() {
	log.Println("ACTION: Show Home Page")
	homePage := findElementByID("page_home")
	searchPage := findElementByID("page_search")
	profilePage := findElementByID("page_profile")

	if homePage != nil { homePage.IsVisible = true }
	if searchPage != nil { searchPage.IsVisible = false }
	if profilePage != nil { profilePage.IsVisible = false }

	// TODO: Update button styles (e.g., find buttons by ID, change style reference)
}

func showSearchPage() {
	log.Println("ACTION: Show Search Page")
	homePage := findElementByID("page_home")
	searchPage := findElementByID("page_search")
	profilePage := findElementByID("page_profile")

	if homePage != nil { homePage.IsVisible = false }
	if searchPage != nil { searchPage.IsVisible = true }
	if profilePage != nil { profilePage.IsVisible = false }

	// TODO: Update button styles
}

func showProfilePage() {
	log.Println("ACTION: Show Profile Page")
	homePage := findElementByID("page_home")
	searchPage := findElementByID("page_search")
	profilePage := findElementByID("page_profile")

	if homePage != nil { homePage.IsVisible = false }
	if searchPage != nil { searchPage.IsVisible = false }
	if profilePage != nil { profilePage.IsVisible = true }

	// TODO: Update button styles
}

// --- Main Application Entry Point ---

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Add file/line numbers to log

	// --- Command Line Args ---
	krbFilePath := flag.String("file", "", "Path to the KRB file to render")
	flag.Parse()

	if *krbFilePath == "" {
		// Use the actual executable name in usage message
		execName := filepath.Base(os.Args[0])
		fmt.Printf("Usage: %s -file <krb_file_path>\n", execName)
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
	krbDocument = doc // Store reference for handlers
	log.Printf("Parsed KRB OK - Ver=%d.%d Elements=%d Styles=%d Strings=%d Resources=%d Flags=0x%04X",
		doc.VersionMajor, doc.VersionMinor, doc.Header.ElementCount, doc.Header.StyleCount, doc.Header.StringCount, doc.Header.ResourceCount, doc.Header.Flags)

	if doc.Header.ElementCount == 0 {
		log.Println("WARN: No elements found in KRB file. Exiting.")
		return
	}

	// --- Initialize Renderer ---
	renderer := raylib.NewRaylibRenderer() // Instantiate the Raylib implementation
	appRenderer = renderer                 // Store global reference for handlers (simple approach)

	// ================================================
	// ===>>> REGISTER CUSTOM COMPONENT HANDLERS <<<===
	// ================================================
	// Register the handler for TabBar components. The identifier "TabBar"
	// must match the logic used in custom_components.go to identify TabBars.
	log.Println("Registering custom component handlers...") // Added log
	raylib.RegisterCustomComponent("TabBar", &raylib.TabBarHandler{})
	// Register handlers for other custom components here if needed:
	// raylib.RegisterCustomComponent("MyWidget", &raylib.MyWidgetHandler{})
	// ================================================


	// ================================================
	// ===>>> REGISTER EVENT HANDLERS <<<===
	// ================================================
	// Map the callback names used in the KRB file to the actual Go functions.
	log.Println("Registering event handlers...") // Added log
	renderer.RegisterEventHandler("showHomePage", showHomePage)
	renderer.RegisterEventHandler("showSearchPage", showSearchPage)
	renderer.RegisterEventHandler("showProfilePage", showProfilePage)
	// Register any other event handlers defined in your KRB files.
	// ================================================


	// --- Prepare Render Tree (Loads resources, processes App element) ---
	// This builds the initial element structure based on standard KRB data.
	roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}
	// Store the flat element list for event handlers to access/modify
	allElements = renderer.GetRenderTree() // Get the slice of pointers

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
		renderer.PollEvents() // Checks mouse clicks, window close, resize etc. -> Triggers registered handlers

		// --- Update Application State (Placeholder) ---
		// Visibility state is modified directly by event handlers in this simple example.

		// --- Drawing ---
		renderer.BeginFrame()       // BeginDrawing + ClearBackground
		renderer.RenderFrame(roots) // Standard Layout + Custom Adjustments + Drawing
		renderer.EndFrame()         // EndDrawing
	}

	log.Println("Exiting.")
}