# Kryon Project - Implementations & Examples

## Introduction
Kryon is a system for defining user interfaces, inspired by the idea of having something like **HTML and CSS that works everywhere**, or like an **embeddable Flutter** for UI layout.

The goal is simple: describe your UI's structure and style **once** using Kryon's language. This description is then turned into a compact binary file.

This single binary file acts as a universal blueprint that different applications, running on various platforms (desktops, embedded systems, etc.) and using different technologies, can understand and use to display the same interface.

This repository contains multi-language implementations (readers, renderers) and examples for the Kryon binary UI format (.krb).

Kryon is a binary UI format designed for:
* Universal compatibility (including 8-bit systems)
* Compact representation
* Performance-oriented parsing and rendering
* Common UI features
* Extensibility

The Kryon Compiler (kryc), which translates human-readable .kry source files into the .krb binary format, resides in a separate repository:
* Compiler Repository: [https://github.com/waozixyz/kryc](https://github.com/waozixyz/kryc)

## Structure

* **docs/**: Contains the detailed specifications for the Kryon formats.
* **examples/**: Sample `.kry` (source) and corresponding `.krb` (binary) files, demonstrating various UI elements and features.
* **implementations/**: Language-specific readers and renderers for the `.krb` format.

## Documentation

Detailed specifications for the Kryon formats can be found in the `docs/` directory:

* [`kry_source_spec.md`](docs/kry_source_spec.md): Specification for the human-readable `.kry` source file format used to define UIs.
* [`krb_source_spec.md`](docs/krb_source_spec.md): Specification for the compiled `.krb` binary file format, detailing its structure and encoding.
* [`kryon_runtime_styling.md`](docs/kryon_runtime_styling.md): Defines default styling values, contextual property resolution (e.g., for borders), and property inheritance rules for Kryon Runtimes. Ensures consistent UI rendering from KRB files.


## Development Setup

If you are working with the `.kry` source files in the `examples` directory, this project uses VS Code with the following settings for a better development experience:

* `.kry` files are treated as QML for syntax highlighting (see `.vscode/settings.json`).
* Install the "QML" extension (`bbenoist.qml`) in VS Code.

## Language Implementation Status

These tables show the completion status of various language/renderer implementations against the examples and widgets found in the `examples/` directory.

**Legend:**
* ✅: Fully working and implemented with the latest spec.
* 〰️: Partially working or implemented, but may have issues or missing features.
* ❌: Not implemented.
* vX.X: Worked at this version.
* ~vX.X: Worked partially at this version.

### Examples Status

| Implementation | hello_world | button | image |
|----------------|:-----------:|:------:|:-----:|
| Go / Raylib    |     v0.4      |     v0.4     |  v0.4   |
| JS / Web       |     〰v0.3   |   〰️v0.3   |  〰️v0.3   |
| C / Raylib     |      ✅      |   ✅   |  ✅   |
| C / Term       |     v0.4     |   ~v0.4   |  ❌   |

### Widgets Status

| Implementation | tab_bar |
|----------------|:-------:|
| Go / Raylib    |    v0.4   |
| JS / Web       |    ❌   |
| C / Raylib     |    ~   |
| C / Term       |    ❌   |

## Build and Run

**Prerequisite:** You need to build the Kryon Compiler (kryc) from its [separate repository](https://github.com/waozixyz/kryc) first. Use the compiler to convert `.kry` source files (from `examples/`) into `.krb` binary files if they don't already exist or if you modify the source.

Example (using the compiler built from the `kryc` repository):

```bash
# Navigate to the kryc compiler directory (example path)
cd ../kryc

# Compile a .kry file to .krb (outputting to the examples dir of this repo)
# Example: compiling hello_world.kry
./kryc ../kryon/examples/hello_world.kry ../kryon/examples/hello_world.krb
```

## Running Examples with Implementations

Once you have built a specific language implementation (e.g., the Go/Raylib renderer), you can run it against the `.krb` files in the main `examples/` directory.

**1. Visual Rendering (General Examples):**

Most implementations will have a primary executable that takes a `.krb` file path as an argument. Running this will render the UI visually.

*   **Example (Go/Raylib):**
    ```bash
    # Navigate to the Go Raylib implementation directory
    cd implementations/go/cmd/kryon-raylib/
    
    # Build the renderer
    go build .
    
    # Run against an example .krb file
    ./kryon-raylib ../../../examples/button.krb 
    # Or for hello_world:
    # ./kryon-raylib ../../../examples/hello_world.krb
    ```
    When running examples directly from the `examples/` folder this way, the UI will be **visually rendered according to the KRB specification**. However, event handlers (`onClick`, etc.) defined in the `.kry` source as string names (e.g., `onClick: "handleButtonClick"`) **will not be connected to any actual Go functions by default.** This means interactive elements like buttons will appear correctly but will not perform actions when clicked.

**2. Testing Interactions (Implementation-Specific Examples):**

To test the full functionality of interactive elements, including event handling and custom component logic, implementations provide their own dedicated example programs. These programs explicitly register the necessary event handlers or custom component handlers.

*   **Location:** These are typically found within the specific implementation's directory structure. For example:
    *   `implementations/go/examples/button/` might contain a `main.go` that specifically registers a `handleButtonClick` function for the `button.krb` example.
    *   `implementations/go/examples/tabbar/` would similarly set up the `TabBarHandler` and any required event handlers for the tab bar example.

*   **Running these specific examples:**
    Follow the build and run instructions provided *within that implementation's example directory*. This usually involves building a separate small program that wires up the UI with the necessary runtime logic.

    *   **Example (Go/Raylib Button with Interaction):**
        ```bash
        # Navigate to the specific example directory within the Go implementation
        cd implementations/go/examples/button/
        
        # Build this specific example program
        go build .
        
        # Run the example (it will typically load its associated .krb file automatically)
        ./button 
        ```
    This ensures that when you click the button in `button.krb`, the `handleButtonClick` function defined in `implementations/go/examples/button/main.go` is actually executed.

**In summary:**
*   Use the main executable of an implementation (e.g., `kryon-raylib`) with `.krb` files from `examples/` for **quick visual testing and validation of KRB parsing/rendering**.
*   Use the dedicated example programs *within each implementation's `examples/` subdirectories* (e.g., `implementations/go/examples/button/`) to **test full interactivity and custom component behavior**.