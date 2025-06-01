// cmd/kryon-raylib/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render"
	"github.com/waozixyz/kryon/impl/go/render/raylib" // Your Raylib renderer
)

// --- Global state (optional, could be passed around) ---
var (
	appRenderer render.Renderer
	krbDocument *krb.Document 
	// allElements []*render.RenderElement // Less critical if not directly manipulating pages
)

// --- Example Event Handler Functions (Keep them simple or remove if not used by KRB) ---
func genericClickHandler() {
	log.Println("INFO: A KRB element was clicked (genericClickHandler).")
	// Add specific logic here if you have elements with onClick="genericClickHandler"
}

func anotherActionHandler() {
	log.Println("INFO: anotherActionHandler was called.")
}


func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile) // Added Lshortfile for easier debugging

	krbFilePath := flag.String("file", "", "Path to the KRB file to render")
	flag.Parse()

	if *krbFilePath == "" {
		execName := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s -file <krb_file_path>\n", execName)
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Loading KRB file: %s", *krbFilePath)

	file, err := os.Open(*krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Cannot open KRB file '%s': %v", *krbFilePath, err)
	}
	defer file.Close()

	doc, err := krb.ReadDocument(file)
	if err != nil {
		log.Fatalf("ERROR: Failed to parse KRB file '%s': %v", *krbFilePath, err)
	}
	krbDocument = doc // Store globally if needed by helpers (e.g., GetCustomPropertyValue in handlers)
	
	log.Printf("Parsed KRB OK - Ver=%d.%d Elements=%d Styles=%d CompDefs=%d Strings=%d Resources=%d Flags=0x%04X",
		doc.VersionMajor, doc.VersionMinor, doc.Header.ElementCount, 
		doc.Header.StyleCount, doc.Header.ComponentDefCount, // Added CompDefs
		doc.Header.StringCount, doc.Header.ResourceCount, doc.Header.Flags)

	if doc.Header.ElementCount == 0 {
		log.Println("WARN: No elements found in KRB file. Exiting.")
		return
	}

	renderer := raylib.NewRaylibRenderer()
	appRenderer = renderer // Store globally if handlers need it

	log.Println("Registering custom component handlers (if any)...")
	// Example: Register TabBar if your KRB uses it and you have the handler
	err = renderer.RegisterCustomComponent("TabBar", &raylib.TabBarHandler{})
	if err != nil {
		log.Printf("WARN: Failed to register TabBar handler: %v (This is OK if TabBar component is not used by the current KRB file)", err)
	}
	// Register other custom component handlers here as needed by your KRB files.
	// e.g., renderer.RegisterCustomComponent("MyCustomWidget", &myCustomWidgetHandler{})


	log.Println("Registering event handlers (if any)...")
	// Register any event handlers named in your KRB files.
	renderer.RegisterEventHandler("genericClick", genericClickHandler)
	renderer.RegisterEventHandler("anotherAction", anotherActionHandler)
	// Example for TabBar demo (if you were running that specific KRB)
	// renderer.RegisterEventHandler("showHomePage", showHomePage) 
	// renderer.RegisterEventHandler("showSearchPage", showSearchPage)
	// renderer.RegisterEventHandler("showProfilePage", showProfilePage)


	roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}
	// allElements = renderer.GetRenderTree() // Store if needed for direct manipulation

	err = renderer.Init(windowConfig)
	if err != nil {
		renderer.Cleanup()
		log.Fatalf("ERROR: Failed to initialize renderer: %v", err)
	}
	defer renderer.Cleanup()

	err = renderer.LoadAllTextures()
	if err != nil {
		log.Printf("WARNING: Failed to load all textures: %v. Proceeding might result in missing images.", err)
	}

	// No specific setActivePage call here; UI should render based on KRB structure.
	log.Println("Entering main loop...")

	for !renderer.ShouldClose() {
		renderer.PollEvents()

		// Update application state (if any dynamic updates needed per frame)

		renderer.BeginFrame()
		renderer.RenderFrame(roots)
		renderer.EndFrame()
	}

	log.Println("Exiting.")
}