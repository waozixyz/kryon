# Kryon Go Button Example (Embedded KRB)

This example demonstrates a basic Kryon UI featuring a centered button, built using the Go implementation and the Raylib renderer.

## Features Demonstrated

*   Parsing a `.krb` (Kryon Binary) file.
*   Using `go:generate` to automatically copy the required `.krb` file from the main examples directory before building.
*   Using `go:embed` to bundle the `.krb` data directly into the Go executable.
*   Rendering basic elements (`App`, `Button`) with styles defined in the KRB file.
*   Implementing flow layout (`layout: center` on the `App` element) to center child elements.
*   Handling basic click events defined in the KRB file by mapping them to Go functions.
*   Using the `raylib-go` binding for rendering.

## Prerequisites

*   **Go:** Version 1.16 or later (for `go:embed`) is recommended.
*   **Raylib:** The Raylib C development library must be installed on your system, as `raylib-go` depends on it. Follow the installation instructions for your operating system on the [Raylib website](https://github.com/raysan5/raylib#installation).
*   **(Unix-like systems)** A `cp` command (used by `go:generate` in this example). For native Windows, you might need to adjust the `go:generate` directive in `main.go` to use `copy` or a Go script.

## How to Run

1.  **Navigate to this directory** in your terminal:
    ```bash
    cd path/to/your/project/kryon/impl/go/examples/button
    ```

2.  **Run `go generate`:** This command executes the directive in `main.go` to copy the necessary `button.krb` file from the main project examples into this directory.
    ```bash
    go generate .
    ```

3.  **Run the example:** Use `go run` to compile and execute the code.
    ```bash
    go run .
    ```
    Alternatively, build an executable first:
    ```bash
    go build .
    ./button # or .\button.exe on Windows
    ```

## Expected Behavior

*   A 600x400 window titled "Centered Button" should appear.
*   The window background should be dark grey (`#191919FF`).
*   A 1-pixel cyan border (`#00FFFFFF`) should be visible around the edge of the window.
*   A 150x50 button with a dark blue background (`#404080FF`) and yellow text (`#FFFF00FF`) should be centered horizontally and vertically within the window (inside the border).
*   The text "Click Me!" should appear centered on the button.
*   When you hover the mouse over the button, the cursor should change to a pointing hand.
*   When you click the button, a message like `>>> Go Event Handler: Button Clicked! <<<` should be printed to the terminal where you ran the command.
