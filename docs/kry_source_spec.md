# Kryon Source Language Specification (.kry) v1.1

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
    *   **Numbers (Integers and Floating-Point for Percentages):**
        *   Integers (e.g., `10`, `-5`) are used for pixel values, counts, etc.
        *   Floating-point literals (e.g., `0.5`, `1.0`) are primarily used for properties like `opacity` or others that map to KRB's `VAL_TYPE_PERCENTAGE`.
            *   When a KRY float literal `X` (e.g., `opacity: 0.5`) is compiled to a KRB property expecting `VAL_TYPE_PERCENTAGE`:
                *   It's converted to an 8.8 fixed-point value by `round(X * 256)`. So, `0.5` becomes `128` (0x0080), and `1.0` becomes `256` (0x0100).
                *   The `FLAG_FIXED_POINT` must be set in the KRB File Header if any such values are used.
    *   **Percentage Strings:** Strings ending with `%` (e.g. `"50%"`). Used for properties like `width`, `height`, `min_width`, `max_width`.
        *   When compiled to a KRB property expecting `VAL_TYPE_PERCENTAGE`:
            *   The numeric part `N` from `"N%"` is converted to an 8.8 fixed-point value by `round((N / 100.0) * 256)`. So, `"50%"` becomes `128` (0x0080).
            *   The `FLAG_FIXED_POINT` must be set in the KRB File Header if any such values are used.
    *   **Hex Colors:** `"#RRGGBBAA"` or `"#RGB"` (e.g., `"#FF0000FF"` for red).
    *   **Boolean:** `true`, `false`.
    *   **Enums:** Predefined keywords (e.g., `text_alignment: center`). For `Define`d component properties, see Section 8.
    *   **Resource Paths:** Strings referencing external files (`"images/logo.png"`).
    *   **Style Names:** Strings referencing a defined style (`"my_button_style"`). For `Define`d component properties, see Section 8.
    *   **Callback Names:** Strings referencing runtime functions (`"handleButtonClick"`).

*   **Standard Properties:** (Examples - Correspond to KRB `PROP_ID_*`)
    *   `id`: String identifier for referencing the element. Passed to KRB Element Header `ID` field (as string index).
    *   `pos_x`, `pos_y`: Integer coordinates. Passed to KRB Element Header.
    *   `width`, `height`: Integer (pixels) or Percentage String (`"50%"`). Defines size constraints. Maps to KRB `PROP_ID_MaxWidth`/`MaxHeight`. Final size often influenced by runtime layout.
    *   `min_width`, `min_height`, `max_width`, `max_height`: Integer (pixels) or Percentage String (`"50%"`). Defines size constraints. Maps to corresponding KRB properties.
    *   `layout`: Layout mode hints for children (e.g., `row`, `column`, `center`, `grow`, `wrap`, `absolute`). The compiler parses these hints to compute and set the 1-byte `Layout` field in the KRB Element Header. The corresponding `PROP_ID_LAYOUTFLAGS` (0x1A) identifier is generally not written as a separate property entry in the KRB file, as the layout information is directly encoded in the Element Header.
    *   `style`: Name of a style block to apply. Passed to KRB Element Header `Style ID` field (as style index).
    *   `background_color`, `text_color`, `border_color`: Hex Color String (`"#RRGGBBAA"`). Compiled into standard KRB properties.
    *   `border_width`: Integer. Compiled into `PROP_ID_BorderWidth`.
    *   `border_radius` Integer. Compiled into `PROP_ID_BorderRadius`.
    *   `opacity`: Float (0.0 to 1.0). Compiled into `PROP_ID_Opacity` (likely using `VAL_TYPE_PERCENTAGE`).
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

Allows defining reusable custom UI components using standard elements as a base. This is a **source-level abstraction**. The compiler processes these definitions. The resulting `.krb` file may contain these definitions in a `Component Definition Table` for runtime instantiation, or the compiler may expand component usages directly into the main UI tree. In either case, the **runtime interprets** the final behavior based on the compiled KRB structure (instantiated elements and their properties, including custom ones).

*   **Syntax:**
    ```kry
    Define ComponentName {
        # Optional: Declare properties the component accepts from usage tags.
        Properties {
            # propName: Type = DefaultValue # e.g., text_label: String = "Default"
            # e.g., count: Int = 0
            # e.g., is_enabled: Bool = true
            # e.g., item_style: StyleID = "default_item_style"
            # e.g., status: Enum(active, inactive, pending) = active
        }

        # Required: The root element structure using standard Kryon elements.
        # This defines the base KRB element(s) generated for this component's template.
        Container { # Or Button, Text, etc. - Must be a single root standard element.
            # ... (standard properties and child elements)
        }
    }
    ```

*   **Properties Block Details:**
    The `Properties` block within a `Define` statement declares the properties that instances of this component can accept.
    *   **Supported Types:** `String`, `Int`, `Float`, `Bool`, `Color`, `StyleID`, `Enum(...)`.
        *   **`StyleID` Type Note:** When a property is declared with type `StyleID` (e.g., `my_style_prop: StyleID = "default_button_style"`), its default value (the style name string) is stored in the KRB Component Definition Table using a `Value Type Hint` of `VAL_TYPE_STRING` (representing a string table index) and the corresponding string index as the `Default Value Data`. The compiler uses this property at instantiation time to look up the actual 1-based Style Block ID and apply it to the relevant element's header.
        *   **`Enum(...)` Type Note:** When a property is declared with an `Enum` type (e.g., `status: Enum(active, inactive, pending) = active`), its default value (the enum member string, e.g., "active") is stored in the KRB Component Definition Table using a `Value Type Hint` of `VAL_TYPE_STRING` (representing a string table index) and the corresponding string index as the `Default Value Data`. The list of valid enum members (e.g., "active", "inactive", "pending") is implicitly defined by the KRY source. During component instantiation, the provided value (as a string) would typically be validated by the compiler or runtime against these defined members.
    *   Default values are optional. If a default value is not provided, the property must be supplied when the component is used, unless the runtime has its own handling for missing optional properties.

*   **Usage:** Use the defined component like a standard element:
    ```kry
    ComponentName {
        id: "my_instance_1"
        text_label: "Click Me" # Assuming text_label was defined in Properties
        # ... other declared properties ...
    }
    ```

*   **Compiler Role:**
    1.  **Parsing `Define` Blocks:** Storing the definition, including declared properties (their names, types, default values) and the expansion structure (the "template").
    2.  **Populating KRB Component Definition Table (Optional/Strategy-Dependent):**
        *   The compiler **may** serialize the parsed `Define ComponentName` block (its declared properties, default values, and root element template) into the `Component Definition Table` within the `.krb` file. This makes the component definition available for runtime instantiation or linking by other `.krb` files.
        *   This is especially relevant when compiling `.kry` files intended as component libraries.
    3.  **Handling Component Usage (`<ComponentName>`) - Two Primary Strategies:**
        *   **A) Inline Expansion:** The compiler might directly expand the usage of `<ComponentName>` into the corresponding standard KRB element structure (defined in its `Define` block) within the main UI element tree of the `.krb` file. This is similar to a macro expansion.
        *   **B) Instantiation Reference (Conceptual):** If the component's definition is in the `Component Definition Table`, the usage in `.kry` could conceptually translate to a placeholder or a special element type in the `.krb`'s main UI tree that instructs the runtime to instantiate that component from the table. *(The exact KRB mechanism for this reference needs to be defined if not simple inline expansion – e.g., a specific ELEM_TYPE or a custom property pointing to the definition name).* **For KRB v0.4, the primary documented mechanism for `Define` is still compiler-side expansion into standard elements, possibly with custom properties. The Component Definition Table serves as a library of these templates.**
    4.  **Validating & Merging Properties (during expansion or for instance creation):**
        *   Checks if properties provided in the usage tag match the `Properties` declaration (name and type compatibility).
        *   Applies standard properties (`id`, `pos_x`, `width`, `style`, etc.) from the usage tag to the root KRB element generated/instantiated from the template.
        *   For properties declared in the `Define`'s `Properties` block:
            *   If the property maps directly to a *standard* KRB property of the generated element(s) (e.g., a `Define`d `bar_style: StyleID` property setting the `Style ID` in the KRB Element Header), the compiler performs this mapping.
            *   If the property represents component-specific logic or data (e.g., `TabBar`'s `position`), the compiler typically **encodes it as a Custom Property** in the KRB for runtime handling. Values from the usage tag take precedence over default values from the `Define` block.
    5.  **Handling Children:** Inserts KRB representations of child elements from the usage tag into the appropriate location within the expanded/instantiated KRB structure (often as children of the template's root element).
    6.  **Generating KRB:** The output `.krb` contains standard KRB elements, standard properties, events, children, potentially Custom Properties, and potentially a `Component Definition Table`.

*   **Runtime Role (Crucial):**
    *   The runtime parses the KRB file.
    *   **If the `.krb` contains a `Component Definition Table`:** The runtime might use these definitions to:
        *   Dynamically instantiate components not present in the initial UI tree (e.g., via code).
        *   Resolve references from the main UI tree that point to these definitions (if using an instantiation reference strategy).
    *   When it encounters an element (e.g., a `Container` generated from a `<TabBar>` usage, or an instance of a `TabBar` from the definition table), it checks for associated **Custom Properties** (like `position`).
    *   The runtime contains the specific logic necessary to **interpret** these custom properties and implement the component's unique behavior.

*   **Example (`TabBar`'s `position` Property - Compiler/Runtime Interaction):**
    *   Compiler sees `<TabBar { position: "bottom"; ... }>`, where `TabBar` is defined via `Define TabBar { Properties { position: String; ... } Container {...} }`.
    *   **Scenario 1 (Inline Expansion):** Compiler generates a KRB `Container` element directly in the UI tree based on the `TabBar`'s `Define`d `Container` template. It adds a **Custom Property** to this KRB `Container`: `{ key_index_for_ "position", value_type_string_index, value_size_1, string_index_for_"bottom" }`.
    *   **Scenario 2 (Using Definition Table - advanced):** The `Define TabBar` is in the KRB's `Component Definition Table`. The main UI tree might have a reference. When instantiated, the runtime would apply the `position: "bottom"` from the usage as a custom property to the instantiated `Container`.
    *   **Runtime** (in both scenarios) parses this `Container`, sees the `position="bottom"` custom property, and executes its internal logic to place this container accordingly.

*   **Relationship with KRB `Component Definition Table`:**
    *   The KRY `Define` block is the source for entries in the KRB `Component Definition Table`.
    *   A `.kry` file might consist entirely of `Define` blocks, acting as a component library. When compiled, this would primarily populate the `Component Definition Table` in the `.krb` file.
    *   An application `.kry` file might use components defined locally or included from such libraries.
    *   The primary expectation for KRB v0.4 is that `Define`d components in an application `.kry` are expanded by the compiler into standard elements in the main UI tree, potentially with custom properties. The `Component Definition Table` serves as a library of these templates, allowing them to be stored and potentially reused (e.g., by a runtime for dynamic instantiation via code, or by other `.kry` files at compile time).

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

These components are defined using standard core elements (`Container`, `Button`, etc.) and properties within their `Define` blocks. The compiler processes these definitions and usages as described in Section 8. Component-specific behaviors and layout adjustments (like `TabBar` positioning relative to siblings) often rely on the **runtime environment interpreting specific custom properties** that the compiler passes through into the `.krb` file.

### `TabBar`

A `TabBar` component is typically used for navigation, often placed at the top or bottom of a screen section.

*   **Declared Properties (within `Define TabBar { Properties { ... } }`):**
    *   `orientation: String = "row"`
        *   **Purpose:** Influences how the `TabBar` itself stretches when positioned (e.g., a "row" orientation `TabBar` will stretch its width when `position` is "top" or "bottom").
        *   **KRB Mapping:** Passed as a KRB Custom Property for runtime interpretation by custom layout handlers.
        *   **Note:** The internal layout of items *within* the `TabBar` (e.g., arranging buttons in a row or column) is typically controlled by the `layout` property of the style applied via the `bar_style` property (or a default style determined by the compiler/runtime based on `orientation`).
    *   `position: String = "bottom"`
        *   **Purpose:** Intended for runtime interpretation to position the `TabBar` relative to its siblings (e.g., "top", "bottom", "left", "right") and potentially adjust those siblings.
        *   **KRB Mapping:** Passed as a KRB Custom Property. This is critical for runtime layout engines.
    *   `bar_style: StyleID = ""`
        *   **Purpose:** Allows specifying a KRY style name to be applied to the root `Container` element of the `TabBar`. This controls the visual appearance (background, etc.) and internal layout (e.g., `layout: row` or `layout: column`) of the `TabBar`'s content. If empty, a default style might be applied by the compiler or runtime, potentially based on the `orientation`.
        *   **KRB Mapping:** The compiler resolves the style name to a Style Block ID and sets the `Style ID` field in the KRB Element Header of the `TabBar`'s root `Container`. It does not become a KRB Custom Property.

*   **Definition Source Example (`widgets/tab_bar.kry`):**
    This file would contain the `Define TabBar` block along with necessary base styles for the bar itself and its items.

    ```kry
    # widgets/tab_bar.kry

    # --- Base Styles for the TabBar's appearance and internal item layout ---
    style "tab_bar_style_base_row" {
        # e.g., background_color, height, layout: row center ...
    }
    style "tab_bar_style_base_column" {
        # e.g., background_color, width, layout: column center ...
    }

    # --- Base Styles for items (Buttons) within the TabBar ---
    style "tab_item_style_base" {
        # e.g., background_color, text_alignment, layout: grow ...
    }
    style "tab_item_style_active_base" {
        extends: "tab_item_style_base"
        # e.g., overrides for background_color, text_color ...
    }

    # --- Widget Definition: TabBar ---
    Define TabBar {
        Properties {
            orientation: String = "row"
            position: String = "bottom"
            bar_style: StyleID = "" // If empty, compiler/runtime might select a default like "tab_bar_style_base_row"
        }

        # Expansion Root: Defines the base KRB structure.
        Container {
            # The compiler will:
            # 1. Apply 'id' and other standard properties from the <TabBar> usage tag here.
            # 2. Determine the 'style' for this Container:
            #    - If 'bar_style' is provided in usage, use that.
            #    - Else if 'bar_style' has a default in 'Properties', use that.
            #    - Else, potentially pick a default style (e.g., "tab_bar_style_base_row"
            #      if orientation is "row"). This logic resides in the compiler/runtime.
            # 3. Insert KRB representations of child elements (e.g., Buttons from the
            #    <TabBar> usage tag) inside this Container.
            # 4. Generate KRB Custom Properties for 'position' and 'orientation'
            #    based on values from the usage tag or defaults from 'Properties'.
        }
    }
    ```

*   **Common Usage:**
    ```kry
    @include "widgets/tab_bar.kry" // Assuming TabBar definition and styles are here

    App {
        window_title: "TabBar Example"
        layout: column // App arranges its direct children (content area, TabBar) vertically

        Container {
             id: "main_content_area"
             layout: grow // Content area fills space not taken by TabBar
             # ... Page content elements ...
        }

        TabBar {
            id: "app_bottom_navigation"
            orientation: "row"      // Passed as KRB Custom Property
            position: "bottom"    // Passed as KRB Custom Property
            bar_style: "tab_bar_style_base_row" // Sets the Style ID on the TabBar's root Container

            Button { id: "tab_home"; style: "tab_item_style_active_base"; text: "Home"; /*...*/ }
            Button { id: "tab_search"; style: "tab_item_style_base"; text: "Search"; /*...*/ }
            // ... more Button children ...
        }
    }
    ```

*   **KRB Result & Runtime Interpretation (Simplified):**
    1.  The compiler processes `<TabBar id="app_bottom_navigation" ...>`.
    2.  Based on `Define TabBar`, it generates a KRB `Container` element.
    3.  Standard properties from usage (`id`, `bar_style` which becomes `Style ID` in header) are applied to this KRB `Container`.
    4.  `orientation="row"` and `position="bottom"` are encoded as **KRB Custom Properties** on this `Container`.
    5.  Child `Button` elements are compiled and linked as children to this `Container`.
    6.  The **Runtime**:
        *   Parses the KRB `Container` (representing the `TabBar`).
        *   Its standard layout engine initially places it according to the `App`'s `layout: column`.
        *   A custom component handler (like your `TabBarHandler`) for elements with `id="app_bottom_navigation"` (or identified by a specific custom property or element type if you used one) then finds the `position` and `orientation` custom properties.
        *   It uses these custom properties to adjust the `TabBar`'s final frame (X, Y, Width, Height) relative to its parent, potentially stretching it (e.g., width-wise if `orientation="row"` and `position="bottom"`).
        *   It may also adjust sibling elements (like `main_content_area`) to make space.
        *   The `TabBar`'s root `Container` then lays out its own children (the `Button`s) according to its *own* `Layout` byte (derived from `bar_style`'s `layout` property, e.g., `row center`).

**(Add definitions for other standard widgets like `Card`, `Dialog`, `Checkbox`, `Slider`, etc. following a similar pattern of explaining declared properties, KRB mapping, and runtime expectations for custom properties.)**