// cmd/kryon-raylib/main.go
package main

import (
    // Import the specific renderer implementation
    "github.com/waozixyz/kryon/impl/go/render/raylib"

    // Import the shared application logic
    // Adjust the path according to your module name in go.mod
    "github.com/waozixyz/kryon/impl/go/internal/app"
)

func main() {
    // Create an instance of the Raylib renderer
    renderer := raylib.NewRaylibRenderer()

    // Run the shared application logic with the Raylib renderer
    app.Run(renderer)
}