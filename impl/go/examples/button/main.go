//go:generate cp ../../../../examples/button.krb ./button.krb

package main

import (
	"bytes"
	_ "embed"
	"log"
	"os"

	"github.com/waozixyz/kryon/impl/go/krb"
	"github.com/waozixyz/kryon/impl/go/render/raylib"
)

//go:embed button.krb
var embeddedKrbData []byte

// --- Event Handler ---
func handleButtonClick() {
	log.Println("------------------------------------")
	log.Println(">>> Go Event Handler: Button Clicked! <<<")
	log.Println("------------------------------------")
}

// --- Main Application ---
func main() {
	log.SetOutput(os.Stdout)
	log.Println("INFO: Starting Go KRB Button Example (Embedded)")

	if len(embeddedKrbData) == 0 {
		log.Fatal("ERROR: Embedded KRB data is empty! Did you run 'go generate .' in this directory first?")
	}
	log.Printf("INFO: Using embedded KRB data (Size: %d bytes)", len(embeddedKrbData))

	// --- Create Reader from Embedded Data ---
	krbReader := bytes.NewReader(embeddedKrbData)

	// --- Parsing ---
	doc, err := krb.ReadDocument(krbReader)
	if err != nil {
		log.Fatalf("ERROR: Failed to parse embedded KRB data: %v", err)
	}
	log.Printf("INFO: Parsed embedded KRB OK - Elements=%d, Styles=%d, Strings=%d",
		doc.Header.ElementCount, doc.Header.StyleCount, doc.Header.StringCount)

	if doc.Header.ElementCount == 0 {
		log.Println("ERROR: No elements found in KRB data.")
		return
	}

	// --- Renderer Setup ---
	renderer := raylib.NewRaylibRenderer()

	// --- Register Event Handlers ---
	// The names MUST match the strings used in the KRB file's event definitions
	renderer.RegisterEventHandler("handleButtonClick", handleButtonClick)

	// --- Prepare Render Tree ---
	// Pass "." as the base path; PrepareTree uses this to resolve any *external*
    // resources referenced within the KRB file, relative to where the app is run.
	roots, windowConfig, err := renderer.PrepareTree(doc, ".")
	if err != nil {
		log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
	}
	if len(roots) == 0 && doc.Header.ElementCount > 0 {
        log.Fatal("ERROR: Render tree preparation resulted in no root elements.")
    }

	// --- Initialize Window ---
	err = renderer.Init(windowConfig)
	if err != nil {
		log.Fatalf("ERROR: Failed to initialize renderer window: %v", err)
	}

	// --- Main Loop ---
	log.Println("INFO: Entering main loop...")
	for !renderer.ShouldClose() {
		renderer.PollEvents() // Handles input and triggers callbacks

		renderer.BeginFrame()
		renderer.RenderFrame(roots) // Uses the prepared tree
		renderer.EndFrame()
	}

	// --- Cleanup ---
	log.Println("INFO: Closing window and cleaning up...")
	renderer.Cleanup()

	log.Println("Go button example finished.")
}
