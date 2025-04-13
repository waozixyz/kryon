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
	// Ensure this path matches your project structure
	"github.com/waozixyz/kryon/impl/go/render/raylib"
)

// --- Global state for handlers (simple approach) ---
var (
	appRenderer render.Renderer
	allElements []*render.RenderElement // Flat list from GetRenderTree()
	krbDocument *krb.Document         // Reference to parsed KRB for string/resource lookups
)

// --- Helper function to get element by ID name ---
func findElementByID(idName string) *render.RenderElement {
	if len(allElements) == 0 || krbDocument == nil {
		log.Printf("WARN findElementByID: State not ready (elements=%d, doc=%t)", len(allElements), krbDocument != nil)
		return nil
	}

	targetIDIndex := uint8(0)
	found := false
	for idx, str := range krbDocument.Strings {
		if str == idName {
			targetIDIndex = uint8(idx)
			found = true
			break
		}
	}
	if !found || targetIDIndex == 0 {
		log.Printf("WARN findElementByID: Element ID '%s' not found in string table.", idName)
		return nil
	}

	for _, el := range allElements {
		if el != nil && el.Header.ID == targetIDIndex {
			return el
		}
	}
	log.Printf("WARN findElementByID: Element with ID '%s' (Index %d) not found in render tree.", idName, targetIDIndex)
	return nil
}

// setActivePage updates the visibility of page containers.
func setActivePage(visiblePageID string) {
	log.Printf("ACTION: Setting active page to '%s'", visiblePageID)
	pageIDs := []string{"page_home", "page_search", "page_profile"} // List of page container IDs

	foundVisible := false
	for _, pageID := range pageIDs {
		pageElement := findElementByID(pageID)
		if pageElement != nil {
			isVisible := (pageID == visiblePageID)
			if isVisible != pageElement.IsVisible {
				pageElement.IsVisible = isVisible
				log.Printf("      Elem %d ('%s') visibility set to %t", pageElement.OriginalIndex, pageID, isVisible)
			}
			if isVisible { foundVisible = true }
		} else {
            log.Printf("WARN setActivePage: Could not find page element with ID '%s'", pageID)
        }
	}
	if !foundVisible && visiblePageID != "" {
		log.Printf("WARN setActivePage: Could not find or make visible page '%s'", visiblePageID)
	}
}

// --- Event Handler Functions ---
func showHomePage() {
	setActivePage("page_home")
}

func showSearchPage() {
	setActivePage("page_search")
}

func showProfilePage() {
	setActivePage("page_profile")
}

// --- Main Application Entry Point ---

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// --- Command Line Args ---
	krbFilePath := flag.String("file", "", "Path to the KRB file to render")
	flag.Parse()

	if *krbFilePath == "" {
		execName := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s -file <krb_file_path>\n", execName)
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
	krbDocument = doc
	log.Printf("Parsed KRB OK - Ver=%d.%d Elements=%d Styles=%d Strings=%d Resources=%d Flags=0x%04X",
		doc.VersionMajor, doc.VersionMinor, doc.Header.ElementCount, doc.Header.StyleCount, doc.Header.StringCount, doc.Header.ResourceCount, doc.Header.Flags)

	if doc.Header.ElementCount == 0 {
		log.Println("WARN: No elements found in KRB file. Exiting.")
		return
	}

	// --- Initialize Renderer ---
	renderer := raylib.NewRaylibRenderer()
	appRenderer = renderer

	// ================================================
	// ===>>> REGISTER CUSTOM COMPONENT HANDLERS <<<===
	// ================================================
	log.Println("Registering custom component handlers...")
	raylib.RegisterCustomComponent("TabBar", &raylib.TabBarHandler{})
	raylib.RegisterCustomComponent("MarkdownView", &raylib.MarkdownViewHandler{})
	// ================================================

	// ================================================
	// ===>>> REGISTER EVENT HANDLERS <<<===
	// ================================================
	log.Println("Registering event handlers...")
	renderer.RegisterEventHandler("showHomePage", showHomePage)
	renderer.RegisterEventHandler("showSearchPage", showSearchPage)
	renderer.RegisterEventHandler("showProfilePage", showProfilePage)
	// ================================================

	// --- Prepare Render Tree (Builds structure, gets window config) ---
	roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}
	allElements = renderer.GetRenderTree() // Get element list *after* PrepareTree

	// --- Initialize Window (using config derived from PrepareTree) ---
	err = renderer.Init(windowConfig)
	if err != nil {
		renderer.Cleanup()
		log.Fatalf("ERROR: Failed to initialize renderer: %v", err)
	}
	defer renderer.Cleanup()

	// ===============================================
	// ===>>> LOAD TEXTURES (AFTER Init) <<<===      // <<<--- ADDED THIS SECTION ---<<<
	// ===============================================
	err = renderer.LoadAllTextures()
	if err != nil {
		// Decide how to handle texture loading errors. Log warning or exit?
		log.Printf("WARNING: Failed to load all textures: %v", err)
		// For now, we continue, but images might be missing.
		// Consider adding: log.Fatalf("...") if textures are critical.
	}
	// ===============================================

	// --- Initial UI State Setup ---
	setActivePage("page_home")

	log.Println("Entering main loop...")

	// --- Main Loop ---
	for !renderer.ShouldClose() {
		renderer.PollEvents()
		renderer.BeginFrame()
		renderer.RenderFrame(roots)
		renderer.EndFrame()
	}

	log.Println("Exiting.")
}