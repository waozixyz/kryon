package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render/raylib"

    rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Add file/line numbers to log

	// --- Command Line Args ---
	krbFilePath := flag.String("file", "", "Path to the KRB file to render")
	// Add other flags if needed (e.g., -debug)
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

	// --- Prepare Render Tree (Loads resources, processes App element) ---
	roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}

	// --- Initialize Window (using config derived from PrepareTree) ---
	err = renderer.Init(windowConfig)
	if err != nil {
        // Attempt cleanup even if init fails (might close partially opened window)
        renderer.Cleanup()
		log.Fatalf("ERROR: Failed to initialize renderer: %v", err)
	}
	defer renderer.Cleanup() // Ensure cleanup happens on exit

	log.Println("Entering main loop...")

	// --- Main Loop ---
	for !renderer.ShouldClose() {
        // Handle Input / Events
        renderer.PollEvents() // Includes updating mouse cursor, checking resize etc.
        // Check for specific events if needed (e.g., clicks)
        if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
             mousePos := rl.GetMousePosition()
             log.Printf("Debug: Mouse Clicked at %v", mousePos)
             // TODO: Implement hit testing: Iterate elements, check bounds, trigger callback?
        }


		// Drawing
		renderer.BeginFrame() // BeginDrawing + ClearBackground
		renderer.RenderFrame(roots) // Render the element tree
		renderer.EndFrame() // EndDrawing
	}

	log.Println("Exiting.")
}