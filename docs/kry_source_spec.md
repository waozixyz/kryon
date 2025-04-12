# Kryon Source Language Specification (.kry) v1.0

## 1. Introduction

The Kryon Source Language (`.kry`) is a human-readable, text-based language designed for defining user interfaces. It prioritizes simplicity and expressiveness, allowing developers to describe UI structure, styling, and basic interactions. `.kry` files are processed by a Kryon Compiler (e.g., `kryonc`) to produce the compact Kryon Binary Format (`.krb`) for deployment on target systems. The runtime environment then interprets the `.krb` file to render the UI and handle component-specific logic.

This document specifies version 1.0 of the `.kry` language.

## 2. Design Goals

*   **Readability:** Syntax should be clear and easy to understand.
*   **Expressiveness:** Allow definition of common UI patterns and layouts.
*   **Modularity:** Support code organization through includes and component definitions.
*   **Compiler Target:** Serve as the input for generating efficient `.krb` files.
*   **Runtime Interpretation:** Define structure and properties clearly enough for a separate runtime to interpret and render, including custom component behavior.

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

Standard UI building blocks. Elements are defined using `ElementName { ... }`. These correspond directly to standard `ELEM_TYPE_*` values in the KRB specification.

*   **`App`**: The root element defining application-level properties (window size, title, etc.). Must be the top-level element describing the runnable UI. Maps to `ELEM_TYPE_APP`.
*   **`Container`**: A generic element for grouping other elements and controlling layout. Maps to `ELEM_TYPE_CONTAINER`.
*   **`Text`**: Displays text content. Maps to `ELEM_TYPE_TEXT`.
*   **`Image`**: Displays an image resource. Maps to `ELEM_TYPE_IMAGE`.
*   **`Button`**: An interactive element that triggers an action on click. Maps to `ELEM_TYPE_BUTTON`.
*   **`Input`**: Allows user text input. Maps to `ELEM_TYPE_INPUT`.
*   *(Other elements like `Canvas`, `List`, `Grid`, `Scrollable`, `Video` can be defined, corresponding to standard `ELEM_TYPE_*` in KRB)*

## 5. Properties

Properties modify the appearance or behavior of an element. They are specified within the element's block as `propertyName: value`. These generally map to standard KRB properties or are handled as described in Section 8 (Component Definition).

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

*   **Standard Properties:** (Examples - Correspond to KRB `PROP_ID_*`)
    *   `id`: String identifier for referencing the element. Passed to KRB Element Header `ID` field (as string index).
    *   `pos_x`, `pos_y`, `width`, `height`: Basic geometry. Passed to KRB Element Header. Often influenced by runtime layout based on `layout` properties and custom component logic.
    *   `layout`: Layout mode hints for children (e.g., `row`, `column`, `center`, `grow`, `wrap`). Compiled into KRB `PROP_ID_LAYOUTFLAGS`, which the compiler *may* use to set the KRB Element Header `Layout` byte, but final layout arrangement often relies on runtime interpretation, especially within custom components.
    *   `style`: Name of a style block to apply. Passed to KRB Element Header `Style ID` field (as style index).
    *   `background_color`, `text_color`, `border_color`, `border_width`: Visual styling. Compiled into standard KRB properties.
    *   `text`: Text content for `Text` or `Button`. Compiled into standard KRB property (likely `PROP_ID_TEXTCONTENT`).
    *   `image_source`: Path for `Image`. Compiled into standard KRB property (likely `PROP_ID_IMAGESOURCE`).
    *   `onClick`, `onChange`, etc.: Event callbacks. Compiled into KRB Event entries.
    *   `visible`: Boolean controlling element visibility. Compiled into standard KRB property (likely `PROP_ID_VISIBILITY`).
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
*   **KRB Mapping:** Style inheritance is resolved entirely by the **compiler**. The final `.krb` file contains `Style Blocks` with the fully resolved set of *standard* properties for each style ID. The runtime does not need to know about the `extends` relationship. Styles define *standard* KRB properties.

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

Allows defining reusable custom UI components using standard elements as a base. This is a **source-level abstraction**. The compiler processes these definitions, but the **runtime interprets** the final behavior based on the compiled KRB structure and any associated custom properties.

*   **Syntax:**
    ```kry
    Define ComponentName {
        # Optional: Declare properties the component accepts from usage tags.
        # These properties guide the compiler and runtime.
        Properties {
            # propName: Type = DefaultValue # e.g., text_label: String = "Default"
            # isRequired: Bool             # e.g., data_source: String
            # specialProp: String = "default" # e.g., position: String = "bottom"
            # Supported Types: String, Int, Float, Bool, Color, StyleID, Enum(...)
            # (Compiler validates usage against these declarations)
        }

        # Required: The root element structure using standard Kryon elements.
        # This defines the base KRB element(s) generated for this component.
        Container { # Or Button, Text, etc. - Must be a single root standard element.
            # Standard properties from the usage tag (id, width, height, style)
            # are typically applied here by the compiler.

            # Children passed within the <ComponentName> usage tag are typically
            # inserted here by the compiler.

            # Properties declared in the 'Properties' block above might be:
            # 1. Used to set STANDARD properties on elements within this structure.
            # 2. Passed through as CUSTOM properties in the KRB for runtime handling.
        }
    }
    ```
*   **Usage:** Use the defined component like a standard element:
    ```kry
    ComponentName {
        id: "my_instance_1"
        text_label: "Click Me" # Provide values for declared properties
        position: "top"        # Provide values for declared properties
        width: 100             # Standard properties are applied too
        style: "some_style"

        # Children are placed inside based on the definition's structure
        Image { image_source: "icon.png" }
    }
    ```
*   **Compiler Role:** The Kryon compiler is responsible for:
    1.  **Parsing `Define` Blocks:** Storing the definition, including declared properties and the expansion structure.
    2.  **Expanding Usage:** When it encounters `<ComponentName>`, it generates the corresponding standard KRB element structure defined in the `Define` block (e.g., a `Container` in the example above).
    3.  **Validating & Merging Properties:**
        *   Checks if properties provided in the usage tag match the `Properties` declaration.
        *   Applies standard properties (`id`, `width`, `height`, `style`, etc.) from the usage tag to the root KRB element generated.
        *   For properties declared in the `Properties` block:
            *   If the property maps directly to a *standard* KRB property of the generated element(s) (e.g., `text_label` could set `PROP_ID_TEXTCONTENT` on an inner `Text` element), the compiler *may* perform this mapping.
            *   If the property represents component-specific logic or data (like `position` or `data_source`), the compiler typically **encodes it as a Custom Property** in the KRB (using the KRB v0.3+ Custom Properties section). It **does NOT** typically perform complex layout adjustments or interpretations based on these properties itself.
    4.  **Handling Children:** Inserts KRB representations of child elements from the usage tag into the appropriate location within the expanded KRB structure.
    5.  **Generating KRB:** The output `.krb` contains standard KRB elements, standard properties, events, children, and potentially **Custom Properties** carrying data for runtime interpretation.

*   **Runtime Role (Crucial):**
    *   The runtime parses the KRB file.
    *   When it encounters an element (e.g., a `Container` generated from a `<TabBar>` usage), it checks for associated **Custom Properties** (like `position`).
    *   The runtime contains the specific logic necessary to **interpret** these custom properties and implement the component's unique behavior (e.g., positioning the `TabBar` container at the bottom/top of its parent based on the `position` custom property value).

*   **Example (`TabBar`'s `position` Property):**
    *   Compiler sees `<TabBar { position: "bottom"; ... }>`, defined by `Define TabBar { Properties { position: String="bottom" } Container {...} }`.
    *   Compiler generates a KRB `Container` element.
    *   Compiler adds a **Custom Property** to this KRB `Container`: `{ key="position", value="bottom" }`.
    *   **Runtime** parses this `Container`, sees the `position="bottom"` custom property, and executes its internal logic to place this container visually at the bottom of its parent layout area. The compiler did not perform this placement.

## 9. Events

Event handlers are assigned via properties like `onClick`, `onChange`, etc. The value is a string naming a function expected to exist in the target runtime environment. The compiler maps these to KRB Event entries.

*   **Syntax:** `onClick: "functionNameInRuntime"`

## 10. Example

```kry
# examples/simple_layout.kry
@include "../widgets/basic_styles.kry" # Assume styles are defined here

App {
    window_width: 200
    window_height: 150
    window_title: "Layout App"
    style: "base_window_style" # From included file
    layout: column # Standard layout property for direct children of App

    Text {
        text: "Content Area"
        layout: grow center # Standard layout properties
        background_color: #444444FF
    }

    Button {
        id: "the_button"
        height: 40
        text: "A Button"
        style: "default_button_style" # From included file
        onClick: "handlePress"
    }
}

```
## 11. Standard Component Library (Widgets)

A standard library of common UI widgets (`TabBar`, `Card`, `Dialog`, etc.) is typically provided via `.kry` files using the `Define` mechanism (see Section 8). Developers `@include` these definition files and use the components in their application `.kry` source.

These components are defined using standard core elements (`Container`, `Button`, etc.) and properties within their `Define` blocks. The compiler processes these definitions and usages as described in Section 8. Component-specific behaviors and layout adjustments (like `TabBar` positioning relative to siblings) rely on the **runtime environment interpreting specific custom properties** that the compiler passes through into the `.krb` file.

*   **`TabBar`**: A component typically used for navigation, often placed at the top or bottom of a screen section.
    *   **Definition Source Example (`widgets/tab_bar.kry`):** This file would contain the `Define TabBar` block along with necessary base styles.
        ```kry
        # widgets/tab_bar.kry

        # --- Base Styles (for elements INSIDE the TabBar, applied via usage) ---
        style "tab_bar_style_base_row" { /* Standard properties for row layout */ }
        style "tab_bar_style_base_column" { /* Standard properties for column layout */ }
        style "tab_item_style_base" { /* Default style for Buttons inside */ }
        style "tab_item_style_active_base" { extends: "tab_item_style_base"; /* ... */ }

        # --- Widget Definition: TabBar ---
        Define TabBar {
            # Properties declared here guide compiler & runtime
            Properties {
                # Affects internal layout of children (e.g. Buttons).
                # Compiler might pass this as a custom prop, or use it to set
                # standard 'layout' prop on the generated Container. Runtime might also use it.
                orientation: String = "row"

                # **CRITICAL**: This property's value is intended for RUNTIME interpretation
                # to position the TabBar relative to its siblings.
                # Compiler passes this as a KRB Custom Property.
                position: String = "bottom" # e.g., "top", "bottom", "left", "right"

                # Optional style override for the root Container element.
                # Compiler uses this to set the standard 'style' property/StyleID.
                bar_style: StyleID = ""
            }

            # Expansion Root: Defines the base KRB structure.
            Container {
                # Compiler applies 'id', standard width/height/style etc. from usage tag here.
                # Compiler sets 'style' based on bar_style/style usage, potentially defaulting based on orientation.
                # Compiler inserts KRB children (e.g., Buttons from usage) here.

                # Compiler generates KRB Custom Properties for runtime handling:
                # - { key="position", value=<value_from_usage_or_default> }
                # - { key="orientation", value=<value_from_usage_or_default> } (Optional, if runtime needs it)
            }
        }
        ```
    *   **Common Usage:**
        ```kry
        @include "widgets/tab_bar.kry"

        App {
            # The runtime layout logic for App's children needs to consider
            # the 'position' custom property of the TabBar element.
            layout: column # Typical parent layout for top/bottom TabBar

            Container {
                 id: "main_content_area"
                 layout: grow # Make content fill space not taken by TabBar
                 # ... Page content elements ...
            }

            TabBar {
                id: "app_bottom_navigation"
                position: "bottom" # This value gets compiled into a KRB Custom Property.
                # style: "my_custom_overall_tabbar_style" # Overrides defaults

                Button { id: "tab_home"; style: "tab_item_style_active_base"; text: "Home"; /*...*/ }
                Button { id: "tab_search"; style: "tab_item_style_base"; text: "Search"; /*...*/ }
                Button { id: "tab_profile"; style: "tab_item_style_base"; text: "Profile"; /*...*/ }
            }
        }
        ```
    *   **KRB Result & Runtime Interpretation:**
        1.  The compiler encounters `<TabBar>`.
        2.  It generates a standard `Container` element in the KRB based on the `Define TabBar` structure.
        3.  It applies the `id` ("app_bottom_navigation") and any standard properties (`style`, etc.) from the usage tag to this KRB `Container`.
        4.  It generates KRB representations for the child `Button` elements and links them as children to the `Container`.
        5.  It adds **Custom Properties** to the KRB `Container` element, for example: `{ key="position", value="bottom" }` (using string table indices).
        6.  The **Runtime** parses the KRB file. When laying out the children of the `App` element, it encounters the `Container` with `id="app_bottom_navigation"`.
        7.  The runtime checks this `Container`'s Custom Properties. It finds `position="bottom"`.
        8.  The runtime's specific layout engine executes logic associated with the "position" custom property key. It places this container at the bottom of the available space within the `App` element, potentially adjusting the space available for siblings like "main_content_area". The rendering and exact placement logic reside entirely within the runtime.

*   **(Add definitions for other standard widgets like `Card`, `Dialog`, `Checkbox`, `Slider`, etc. following the same pattern):**
    *   Defined using `Define` with standard core elements.
    *   Declare expected properties in the `Properties` block.
    *   Compiler generates standard KRB elements.
    *   Compiler passes component-specific properties (those not mapping directly to standard KRB props) as **KRB Custom Properties**.
    *   **Runtime** contains the necessary code to find and **interpret** these Custom Properties to implement the widget's unique appearance, layout contribution, and behavior.