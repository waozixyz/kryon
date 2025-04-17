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

## Development Setup

If you are working with the `.kry` source files in the `examples` directory, this project uses VS Code with the following settings for a better development experience:

* `.kry` files are treated as QML for syntax highlighting (see `.vscode/settings.json`).
* Install the "QML" extension (`bbenoist.qml`) in VS Code.

## Language Implementation Status

These tables show the completion status of various language/renderer implementations against the examples and widgets found in the `examples/` directory.

**Legend:**
* ✅: Fully working and implemented.
* 〰️: Partially working or implemented, but may have issues or missing features.
* ❌: Not implemented or currently broken.

### Examples Status

| Implementation | hello_world | button | image |
|----------------|:-----------:|:------:|:-----:|
| C / Raylib     |     ✅      |   〰️   |  ✅   |
| C / Term       |     ✅      |   〰️   |  ❌   |
| Go / Raylib    |     ✅      |   ✅   |  ✅   |
| JS / Web       |     〰️      |   〰️   |  〰️   |

### Widgets Status

| Implementation | tab_bar |
|----------------|:-------:|
| C / Raylib     |    ❌   |
| C / Term       |    ❌   |
| Go / Raylib    |    〰️   |
| JS / Web       |    ❌   |

## Build and Run

**Prerequisite:** You need to build the Kryon Compiler (kryc) from its [separate repository](https://github.com/waozixyz/kryc) first. Use the compiler to convert `.kry` source files (from `examples/`) into `.krb` binary files if they don't already exist or if you modify the source.

Example (using the compiler built from the `kryc` repository):

```bash
# Navigate to the kryc compiler directory (example path)
cd ../kryc

# Compile a .kry file to .krb (outputting to the examples dir of this repo)
# Example: compiling hello_world.kry
./kryc ../kryon/examples/hello_world.kry ../kryon/examples/hello_world.krb
