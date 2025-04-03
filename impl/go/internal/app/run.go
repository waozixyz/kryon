// internal/app/run.go
package app

import (
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/waozixyz/kryon/impl/go/krb"
    "github.com/waozixyz/kryon/impl/go/render"

    // NOTE: NO direct import of specific renderers like raylib here!
)

// Run is the core application logic, independent of the specific renderer.
func Run(renderer render.Renderer) {
    log.SetFlags(log.LstdFlags | log.Lshortfile)

    // --- Command Line Args ---
    krbFilePath := flag.String("file", "", "Path to the KRB file to render")
    flag.Parse()

    if *krbFilePath == "" {
        // Consider making the executable name dynamic if needed
        fmt.Println("Usage: <executable_name> -file <krb_file_path>")
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
    log.Printf("Parsed KRB OK - Ver=%d.%d Elements=%d...", doc.VersionMajor, doc.VersionMinor, doc.Header.ElementCount) // Shortened log

    if doc.Header.ElementCount == 0 {
        log.Println("WARN: No elements found in KRB file. Exiting.")
        return
    }

    // --- Prepare Render Tree (using the passed-in renderer) ---
    roots, windowConfig, err := renderer.PrepareTree(doc, *krbFilePath)
    if err != nil {
        log.Fatalf("ERROR: Failed to prepare render tree: %v", err)
    }

    // --- Initialize Window (using the passed-in renderer) ---
    err = renderer.Init(windowConfig)
    if err != nil {
        renderer.Cleanup() // Attempt cleanup
        log.Fatalf("ERROR: Failed to initialize renderer: %v", err)
    }
    defer renderer.Cleanup()

    log.Println("Entering main loop...")

    // --- Main Loop (using the passed-in renderer) ---
    for !renderer.ShouldClose() {
        renderer.PollEvents()
        // Add your event handling logic here, potentially calling renderer methods
        // e.g., CheckClick(renderer, roots, ...)

        renderer.BeginFrame()
        renderer.RenderFrame(roots)
        renderer.EndFrame()
    }

    log.Println("Exiting.")
}
