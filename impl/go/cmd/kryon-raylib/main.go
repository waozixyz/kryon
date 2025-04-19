// cmd/kryon-raylib/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	// KRB parser and core render interface/types
	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"

	// Specific Raylib renderer implementation and default handlers
	// Ensure this import path matches your project structure
	"github.com/waozixyz/kryon/impl/go/render/raylib"
)

// --- Global state for handlers (simple approach for this example) ---
// In larger apps, consider dependency injection or an app state struct.
var (
	appRenderer render.Renderer       // Holds the active renderer instance (using the interface type).
	allElements []*render.RenderElement // Flat list of all elements from GetRenderTree().
	krbDocument *krb.Document         // Reference to the parsed KRB for lookups.
)

// --- Helper function to find an element by its ID string ---
// Searches the string table for the ID, then iterates elements.

func findElementByID(idName string) *render.RenderElement {
	if len(allElements) == 0 || krbDocument == nil {
		log.Printf("WARN findElementByID: State not ready (elements=%d, doc=%t)", len(allElements), krbDocument != nil)
		return nil
	}

	targetIDIndex := uint8(0)
	found := false
	// Find the string table index for the given ID name.
	for idx, str := range krbDocument.Strings {
		if str == idName {
			targetIDIndex = uint8(idx)
			found = true
			break
		}
	}
	// Check if the ID name was found in the string table.
	if !found || targetIDIndex == 0 { // Index 0 is often reserved for empty string.
		log.Printf("WARN findElementByID: Element ID '%s' not found in string table.", idName)
		return nil
	}

	// Search the prepared render elements for one matching the ID index.
	for _, el := range allElements {
		// Use the correct field name 'ID' from the Header struct.
		if el != nil && el.Header.ID == targetIDIndex {
			return el
		}
	}

	log.Printf("WARN findElementByID: Element with ID '%s' (Index %d) not found in render tree.", idName, targetIDIndex)
	return nil
}

// setActivePage updates the visibility of specific container elements
// intended to act as application pages.
func setActivePage(visiblePageID string) {
	log.Printf("ACTION: Setting active page to '%s'", visiblePageID)
	// List of element IDs expected to be page containers in this example app.
	pageIDs := []string{"page_home", "page_search", "page_profile"}

	foundVisible := false
	for _, pageID := range pageIDs {
		pageElement := findElementByID(pageID)
		if pageElement != nil {
			// Set visibility based on whether the ID matches the target page.
			isVisible := (pageID == visiblePageID)
			if isVisible != pageElement.IsVisible {
				pageElement.IsVisible = isVisible
				log.Printf("      Elem %d ('%s') visibility set to %t", pageElement.OriginalIndex, pageID, isVisible)
			}
			if isVisible {
				foundVisible = true
			}
		} else {
			log.Printf("WARN setActivePage: Could not find page element with ID '%s'", pageID)
		}
	}
	if !foundVisible && visiblePageID != "" {
		log.Printf("WARN setActivePage: Could not find or make visible page '%s'", visiblePageID)
	}
}

// --- Event Handler Functions ---
// These are the actual Go functions called when KRB events reference their names.

func showHomePage() {
	setActivePage("page_home")
	// Add logic here to potentially update the active style of the "Home" tab button.
}

func showSearchPage() {
	setActivePage("page_search")
	// Add logic here to potentially update the active style of the "Search" tab button.
}

func showProfilePage() {
	setActivePage("page_profile")
	// Add logic here to potentially update the active style of the "Profile" tab button.
}

// --- Main Application Entry Point ---

func main() {
	// Configure logging format.
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// --- Command Line Argument Parsing ---
	krbFilePath := flag.String("file", "", "Path to the KRB file to render")
	flag.Parse()

	if *krbFilePath == "" {
		execName := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s -file <krb_file_path>\n", execName)
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Loading KRB file: %s", *krbFilePath)

	// --- Open and Parse the KRB File ---
	file, err := os.Open(*krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Cannot open KRB file '%s': %v", *krbFilePath, err)
	}
	defer file.Close()

	doc, err := krb.ReadDocument(file)
	if err != nil {
		log.Fatalf("ERROR: Failed to parse KRB file '%s': %v", *krbFilePath, err)
	}
	// Store document globally for helper functions (findElementByID).
	krbDocument = doc
	log.Printf("Parsed KRB OK - Ver=%d.%d Elements=%d Styles=%d Strings=%d Resources=%d Flags=0x%04X",
		doc.VersionMajor, doc.VersionMinor, doc.Header.ElementCount, doc.Header.StyleCount, doc.Header.StringCount, doc.Header.ResourceCount, doc.Header.Flags)

	if doc.Header.ElementCount == 0 {
		log.Println("WARN: No elements found in KRB file. Exiting.")
		return
	}

	// --- Initialize the Specific Renderer (Raylib) ---
	// Create an instance of the Raylib renderer implementation.
	renderer := raylib.NewRaylibRenderer()
	// Store instance globally for handler access (simple approach).
	appRenderer = renderer

	// ================================================
	// ===>>> REGISTER CUSTOM COMPONENT HANDLERS <<<===
	// ================================================
	// Register Go handlers for custom components defined in your .kry files.
	// The string identifier MUST match the `_componentName` convention used by the compiler.
	// Only register handlers for components actually used by this application.
	log.Println("Registering custom component handlers...")

	// Register the default TabBar handler provided by the raylib package.
	// Use the RegisterCustomComponent METHOD on the RENDERER INSTANCE.
	err = renderer.RegisterCustomComponent("TabBar", &raylib.TabBarHandler{})
	if err != nil {
		// Handle registration error (e.g., log warning, potentially exit).
		log.Printf("WARN: Failed to register TabBar handler: %v", err)
	}

	// Example: Registering another custom component (if used and handler exists).
	// Ensure raylib.MarkdownViewHandler is defined and exported if you uncomment this.
	// err = renderer.RegisterCustomComponent("MarkdownView", &raylib.MarkdownViewHandler{})
	// if err != nil {
	// 	log.Printf("WARN: Failed to register MarkdownView handler: %v", err)
	// }
	// ================================================

	// ================================================
	// ===>>> REGISTER STANDARD EVENT HANDLERS <<<===
	// ================================================
	// Map string names used in KRB `onClick` (etc.) properties to actual Go functions.
	// Use the RegisterEventHandler METHOD on the RENDERER INSTANCE.
	log.Println("Registering event handlers...")
	renderer.RegisterEventHandler("showHomePage", showHomePage)
	renderer.RegisterEventHandler("showSearchPage", showSearchPage)
	renderer.RegisterEventHandler("showProfilePage", showProfilePage)
	// Register other handlers needed by your KRB file here.
	// ================================================

	// --- Prepare Render Tree ---
	// This processes the KRB document, builds the element tree structure,
	// applies styles/properties, resolves resources, and gets window config.
	roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}
	// Get the flat list of elements *after* PrepareTree for access by handlers.
	allElements = renderer.GetRenderTree()

	// --- Initialize Renderer Window ---
	// Use the configuration potentially derived from the KRB App element.
	err = renderer.Init(windowConfig)
	if err != nil {
		renderer.Cleanup() // Attempt cleanup even if Init fails.
		log.Fatalf("ERROR: Failed to initialize renderer: %v", err)
	}
	// Ensure Cleanup is called when main function exits.
	defer renderer.Cleanup()

	// ===============================================
	// ===>>> LOAD TEXTURES (AFTER Init) <<<===
	// ===============================================
	// Textures require an active graphics context, so load after Init.
	err = renderer.LoadAllTextures()
	if err != nil {
		// Decide how critical texture loading errors are for your app.
		log.Printf("WARNING: Failed to load all textures: %v. Proceeding might result in missing images.", err)
		// Consider making this fatal if images are essential:
		// log.Fatalf("ERROR: Failed to load textures: %v", err)
	}
	// ===============================================

	// --- Initial UI State Setup ---
	// Set the initial visible page (example).
	setActivePage("page_home")

	log.Println("Entering main loop...")

	// --- Main Application Loop ---
	for !renderer.ShouldClose() { // Loop until the window close signal is received.
		// 1. Process Input & Events
		renderer.PollEvents()

		// 2. Update Application State (if needed outside of event handlers)
		// e.g., handle animations, background tasks, etc.

		// 3. Draw the UI
		renderer.BeginFrame()       // Prepare drawing surface (e.g., ClearBackground)
		renderer.RenderFrame(roots) // Perform layout and draw the element tree
		renderer.EndFrame()         // Finalize drawing (e.g., SwapBuffers)
	}

	log.Println("Exiting.")
}