# Kryon Project - Implementations & Examples

This repository contains multi-language implementations (readers, renderers) and examples for the Kryon binary UI format (`.krb`).

**Kryon is a binary UI format designed for:**
*   Universal compatibility (including 8-bit systems)
*   Compact representation
*   Performance-oriented parsing and rendering
*   Common UI features
*   Extensibility

**The Kryon Compiler (`kryc`), which translates human-readable `.kry` source files into the `.krb` binary format, resides in a separate repository:**
*   **Compiler Repository:** [https://github.com/waozixyz/kryc](https://github.com/waozixyz/kryc)

## Structure

*   `docs/`: Specifications (including the KRB format details below).
*   `examples/`: Sample `.kry` (source) and corresponding `.krb` (binary) files.
*   `implementations/`: Language-specific readers and renderers for the `.krb` format.

## Development Setup

If you are working with the `.kry` source files in the `examples` directory, this project uses VS Code with the following settings for a better development experience:
*   `.kry` files are treated as QML for syntax highlighting (see `.vscode/settings.json`).
*   Install the "QML" extension (`bbenoist.qml`) in VS Code.

## Language Implementation Status

This table shows the completion status of various language/renderer implementations against the examples found in the `examples/` directory.

**Legend:**
*   ✅: Fully working and implemented.
*   〰️: Partially working or implemented, but may have issues or missing features.
*   ❌: Not implemented or currently broken.

| Implementation | `hello_world` | `button` | `image` | `tab_bar` |
|----------------|:--------:|:-------------:|:-------:|:---------:|
| **C / Raylib** |    ✅    |       〰️      |    ✅   |     ❌    |
| **C / Term**   |    ✅    |       〰️      |    ❌   |     ❌    |
| **Go / Raylib**|    ✅    |       ✅      |    ❌   |     〰️    |
| **JS / Web**   |    〰️    |       〰️      |    〰️   |     ❌    |

## Build and Run

**Prerequisite:** You need to build the Kryon Compiler (`kryc`) from its [separate repository](https://github.com/waozixyz/kryc) first. Use the compiler to convert `.kry` source files (from `examples/`) into `.krb` binary files if they don't already exist or if you modify the source.

Example (using the compiler built from the `kryc` repository):
```bash
# Navigate to the kryc compiler directory (example)
cd ../kryc

# Compile a .kry file to .krb (outputting to the examples dir of *this* repo)
./kryc ../kryon/examples/hello_world.kry ../kryon/examples/hello_world.krb
