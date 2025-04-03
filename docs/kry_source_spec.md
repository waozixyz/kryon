# Kryon Source Language Specification (.kry) v1.0

## 1. Introduction

The Kryon Source Language (`.kry`) is a human-readable, text-based language designed for defining user interfaces. It prioritizes simplicity and expressiveness, allowing developers to describe UI structure, styling, and basic interactions. `.kry` files are processed by a Kryon Compiler (e.g., `kryonc`) to produce the compact Kryon Binary Format (`.krb`) for deployment on target systems.

This document specifies version 1.0 of the `.kry` language.

## 2. Design Goals

*   **Readability:** Syntax should be clear and easy to understand.
*   **Expressiveness:** Allow definition of common UI patterns and layouts.
*   **Modularity:** Support code organization through includes and component definitions.
*   **Compiler Target:** Serve as the input for generating efficient `.krb` files.

## 3. File Structure and Syntax

*   **Encoding:** UTF-8.
*   **Comments:** Lines starting with `#` are ignored.
*   **Whitespace:** Indentation and extra whitespace are generally ignored, but recommended for readability. Braces `{}` define blocks.
*   **Case Sensitivity:** Keywords (`App`, `Container`, `style`, `Define`, etc.) are typically case-sensitive (convention: PascalCase for elements/definitions, camelCase or snake_case for properties). String values are case-sensitive.

A typical `.kry` file consists of:
*   Optional `@include` directives.
*   Optional `style` definitions.
*   Optional `Define` blocks for custom components.
*   A single root `App` element definition (usually required for a runnable UI).

## 4. Core Elements

Standard UI building blocks. Elements are defined using `ElementName { ... }`.

*   **`App`**: The root element defining application-level properties (window size, title, etc.). Must be the top-level element describing the runnable UI.
*   **`Container`**: A generic element for grouping other elements and controlling layout.
*   **`Text`**: Displays text content.
*   **`Image`**: Displays an image resource.
*   **`Button`**: An interactive element that triggers an action on click.
*   **`Input`**: Allows user text input.
*   *(Other elements like `Canvas`, `List`, `Grid`, `Scrollable`, `Video` can be defined)*

## 5. Properties

Properties modify the appearance or behavior of an element. They are specified within the element's block as `propertyName: value`.

*   **Syntax:** `propertyName: value`
*   **Values:**
    *   **Strings:** Enclosed in double quotes (`"Hello"`).
    *   **Numbers:** Integers (`10`, `-5`) or potentially floating-point (`1.5`).
    *   **Hex Colors:** `"#RRGGBBAA"` or `"#RGB"` (e.g., `"#FF0000FF"` for red).
    *   **Boolean:** `true`, `false`.
    *   **Enums:** Predefined keywords (e.g., `text_alignment: center`).
    *   **Resource Paths:** Strings referencing external files (`"images/logo.png"`).
    *   **Style Names:** Strings referencing a defined style (`"my_button_style"`).
    *   **Callback Names:** Strings referencing runtime functions (`"handleButtonClick"`).

*   **Standard Properties:** (Examples - See KRB spec for binary mapping)
    *   `id`: String identifier for referencing the element.
    *   `pos_x`, `pos_y`, `width`, `height`: Size and position.
    *   `layout`: Layout mode for children (e.g., `row`, `column`, `center`).
    *   `style`: Name of a style block to apply.
    *   `background_color`, `text_color`, `border_color`, `border_width`: Visual styling.
    *   `text`: Text content for `Text` or `Button`.
    *   `image_source`: Path for `Image`.
    *   `onClick`, `onChange`, etc.: Event callbacks.
    *   *(Many others corresponding to KRB `PROP_ID_*`)*

## 6. Styles (`style`)

Reusable blocks of properties that can be applied to elements. Styles enhance modularity and consistency.

*   **Syntax:**
    ```kry
    style "style_name" {
        # Optional: Inherit properties from a base style
        extends: "base_style_name" 

        # Properties defined in this block
        propertyName: value
        propertyName: value 
        # ... more properties
    }
    ```
*   **Inheritance (`extends`):**
    *   A style definition can optionally include **one** `extends: "base_style_name"` property as its **first** property (conventionally).
    *   The `base_style_name` must refer to another style defined elsewhere (or included).
    *   The compiler will first copy all properties from the `base_style_name`.
    *   Then, any properties defined directly within the current `style "style_name"` block will be applied, **overriding** any properties with the same name inherited from the base style.
    *   Inheritance can be chained (e.g., Style C extends Style B, which extends Style A).
    *   The compiler **must** detect and report errors for:
        *   Undefined `base_style_name`.
        *   Cyclic dependencies (e.g., Style A extends Style B, and Style B extends Style A).
*   **Usage:** Applied to an element using the `style: "style_name"` property. Properties defined directly on the element override those from the applied style (including any inherited properties).
*   **KRB Mapping:** Style inheritance is resolved entirely by the **compiler**. The final `.krb` file contains `Style Blocks` with the fully resolved set of properties for each style ID. The runtime does not need to know about the `extends` relationship.

*   **Example:**
    ```kry
    # Base button style
    style "button_base" {
        background_color: #555555FF
        text_color: #EEEEEEFF
        border_width: 1
        border_color: #333333FF
    }

    # Primary button inherits from base and overrides colors
    style "button_primary" {
        extends: "button_base" # Inherit properties first
        background_color: #007BFFFF # Override base
        text_color: #FFFFFFFF     # Override base
        border_color: #0056B3FF # Override base
    }

    # Usage
    Button {
        style: "button_primary" 
        text: "Submit"
    } 
    ```

## 7. File Inclusion (`@include`)

Textually includes the content of another `.kry` file. Processed by the compiler before main parsing.

*   **Syntax:** `@include "path/to/other_file.kry"`
*   **Use Cases:** Sharing styles, component definitions, or parts of the UI across files.

## 8. Component Definition (`Define`)

Allows defining reusable custom UI components using standard elements. This is a **source-level abstraction** primarily handled by the **compiler**.

*   **Syntax:**
    ```kry
    Define ComponentName {
        # Optional: Declare properties the component accepts
        Properties {
            propName: Type = DefaultValue # e.g., orientation: String = "row"
            isRequired: Bool             # e.g., label: String 
            # Supported Types: String, Int, Float, Bool, Color, StyleID, Enum(...)
        }

        # Required: The structure using standard Kryon elements
        Container { 
            id: "${ComponentName}_root" # Example unique ID generation
            # ... structure ...
            # Compiler logic uses declared properties here
        }
    }
    ```
*   **Usage:** Use the defined component like a standard element:
    ```kry
    ComponentName {
        id: "my_instance"
        label: "Click Me" # Provide values for declared properties
        orientation: "column"
        # Standard properties like width, height, style can also be applied
        width: 100

        # Children can be placed inside if the definition supports it (implicitly or via <slot>)
        Text { text: "Child content" }
    }
    ```
*   **Compiler Role:** The compiler is expected to:
    *   Parse `Define` blocks and store definitions.
    *   Expand component usage (`<ComponentName>`) into its defined standard element structure.
    *   Validate and merge properties from the usage according to the `Properties` declaration.
    *   Handle special declared properties (like `position`) to potentially modify the layout *outside* the component (e.g., parent layout).
    *   **Generate standard KRB v0.3 elements and properties**, resolving all component abstractions. The runtime typically does not need to know about the original source components.

## 9. Events

Event handlers are assigned via properties like `onClick`, `onChange`, etc. The value is a string naming a function expected to exist in the target runtime environment.

*   **Syntax:** `onClick: "functionNameInRuntime"`

## 10. Example

```kry
# examples/simple_button.kry
@include "../widgets/basic_styles.kry" # Assume styles are defined here

App {
    window_width: 200
    window_height: 100
    window_title: "Button App"
    style: "base_window_style" # From included file
    layout: center

    Button {
        id: "the_button"
        width: 100
        height: 40
        text: "Press"
        style: "default_button_style" # From included file
        onClick: "handlePress"
    }
}
```
