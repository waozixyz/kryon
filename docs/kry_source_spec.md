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
*   **Whitespace:** Indentation and extra whitespace are generally ignored for parsing purposes but are strongly recommended for readability. Braces `{}` define blocks for elements, styles, and other language constructs.
*   **Case Sensitivity:** Keywords (`App`, `Container`, `style`, `Define`, etc.) are typically case-sensitive (convention: PascalCase for elements/definitions, camelCase or snake_case for properties). String values are case-sensitive.

A typical `.kry` file consists of:
*   Optional `@include` directives.
*   Optional `@variables` definitions.
*   Optional `style` definitions.
*   Optional `Define` blocks for custom components.
*   A single root `App` element definition (usually required for a runnable UI).

## 3.1. Variables (`@variables`)

A `@variables` block allows the definition of named constants that can be reused throughout the `.kry` file and included files. This is primarily for values like theme colors, standard spacing, font sizes, etc. Variables are resolved at compile time.

*   **Syntax:**
    ```kry
    @variables {
        variableName1: value
        another_variable: value
        # Colors
        theme_primary_color: "#007BFFFF"
        theme_text_color: "#333333FF"
        # Sizes
        standard_padding: 16
        button_height: 40
        # Strings
        default_placeholder: "Enter text..."
    }
    ```
*   **Scope:** Variables defined in a `@variables` block are globally available after their definition point within the current compilation unit (i.e., the main file and all textually included content). If multiple `@variables` blocks are encountered (e.g., through includes), their definitions are merged. If a variable name is redefined, the later definition takes precedence. The compiler should warn about redefinitions.
*   **Value Types:** Variable values can be any of the standard KRY property value types: Strings, Numbers (Integers, Floats), Hex Colors, Booleans. They cannot be Enums, Resource Paths, Style Names, or Callback Names directly as variable values (these are resolved differently by the compiler based on context).
*   **Usage:** To use a variable, prefix its name with a `$` (dollar sign) where a value is expected.
    ```kry
    style "my_button_style" {
        background_color: $theme_primary_color
        padding: $standard_padding
        height: $button_height
    }

    Text {
        text_color: $theme_text_color
        text: $default_placeholder
    }
    ```
*   **Resolution:** The compiler replaces variable usages (e.g., `$theme_primary_color`) with their literal defined values *before* further property parsing or type checking. This substitution is textual.
    *   If a variable `$varName` is used but not defined, the compiler **must** report an error.
    *   Recursive variable definitions (e.g., `varA: $varB`, `varB: $varA`) **must** be detected and reported as an error by the compiler during the variable resolution phase.
*   **KRB Mapping:** Variables are purely a compile-time construct. They do not exist in the `.krb` file. Their substituted literal values are processed as if they were written directly into the KRY source.

## 4. Core Elements

Standard UI building blocks. Elements are defined using `ElementName { ... }` or `ElementName { property1: value1; property2: value2; ... }` for single-line definitions with properties. These correspond directly to standard `ELEM_TYPE_*` values in the KRB specification.

*   **Syntax for Element Definition:**
    1.  **Block Form (Multi-line):**
        ```kry
        ElementName {
            # Properties on separate lines
            propertyName1: value1
            propertyName2: value2
            # Child elements
            ChildElement1 { ... }
            ChildElement2 { ... }
        }
        ```
    2.  **Single-Line Form (with Properties, No Children on Same Line):**
        For elements that primarily consist of properties and have no child elements *defined on the same line*, a single-line form is allowed. The closing brace `}` terminates the element's property definitions for that line.
        ```kry
        ElementName { propertyNameA: valueA; propertyNameB: valueB; propertyNameC: "value C" }
        ```
        *   Properties are separated by semicolons (`;`).
        *   The closing brace `}` terminates the property definitions for this element on this line.
        *   This form is typically used for leaf nodes. If children are intended, they must be defined within an explicit block.

    3.  **Single-Line Properties with Explicit Child Block:**
        An element can define properties on its declaration line, and if it has children, those children **must** be enclosed in a subsequent explicit block defined by `{}`.
        ```kry
        Container { layout: row; padding: 10; } { # Properties end with ';', then explicit block for children
            Text { text: "Item 1" }
            Text { text: "Item 2" }
        }

        # Example of single-line properties for an element that then contains children in a new block:
        Button { text: "Submit"; style: "primary" } {
            Image { source: "icons/submit.png"; width: 16; height: 16 }
        }
        ```
        *   The properties on the first line are parsed up to the closing brace `}` or the opening brace `{` of the child block.
        *   The child elements are then parsed within their own standard block structure.
        *   It is invalid to have children follow a single-line property definition without an explicit block structure for those children.
            ```kry
            # INVALID: Implicit child block after single-line properties
            # Container { layout: row; padding: 10 }
            #     Text { text: "Item 1" }
            ```

*   **`App`**: The root element defining application-level properties (window size, title, etc.). Must be the top-level element describing the runnable UI. Maps to `ELEM_TYPE_APP`.
*   **`Container`**: A generic element for grouping other elements and controlling layout. Maps to `ELEM_TYPE_CONTAINER`.
*   **`Text`**: Displays text content. Maps to `ELEM_TYPE_TEXT`.
*   **`Image`**: Displays an image resource. Maps to `ELEM_TYPE_IMAGE`.
    *   *Example of single-line form (no children):*
        ```kry
        Image { source: "assets/icons/edit.png"; width: 24; height: 24 }
        Image { source: $icon_path; width: $icon_size; height: $icon_size }
        ```
*   **`Button`**: An interactive element that triggers an action on click. Maps to `ELEM_TYPE_BUTTON`.
*   **`Input`**: Allows user text input. Maps to `ELEM_TYPE_INPUT`.
*   *(Other elements like `Canvas`, `List`, `Grid`, `Scrollable`, `Video` can be defined, corresponding to standard `ELEM_TYPE_*` in KRB)*

## 5. Properties

Properties modify the appearance or behavior of an element. They are specified within the element's block or on the same line as the element declaration. These generally map to standard KRB properties or are handled as described in Section 8 (Component Definition).

*   **Syntax:**
    *   **Standard (Multi-line):** `propertyName: value` on its own line within an element's `{ ... }` block.
        ```kry
        Container {
            width: 100
            height: 50
        }
        ```
    *   **Single-Line (within Element Declaration):** `propertyName: value` pairs separated by semicolons (`;`) on the same line as the `ElementName { ... }`.
        ```kry
        Text { text: "Hello"; text_color: $theme_text_color; font_size: 16 }
        ```
        *   The last property on the line does not require a trailing semicolon before the closing brace `}` if the line ends the element definition.
        *   Whitespace around the colon `:` and semicolon `;` is flexible but recommended for readability.
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
    *   **Variables:** A variable reference (e.g., `$my_variable_name`) can be used as a value. The compiler first substitutes it with its defined literal value, which is then parsed according to the expected type for `propertyName`.
        ```kry
        Button { text: $button_label_confirm; height: $button_height; style: "primary_button" }
        ```

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
        # Optional: Inherit properties from one or more base styles
        extends: "base_style_name_single" 
        # OR
        extends: ["base_style_1", "base_style_2", ..., "base_style_N"]

        # Properties defined in this block
        propertyName: value
        propertyName: value
        # ... more properties
    }
    ```
*   **Inheritance (`extends`):**
    *   A style definition can optionally include an `extends` property as its **first** property (conventionally).
    *   The value for `extends` can be either:
        1.  A single **quoted string** representing one base style name (e.g., `extends: "base_style"`).
        2.  An **array of quoted strings** representing multiple base style names (e.g., `extends: ["base1", "base2"]`).
    *   Each `base_style_name` in the string or array must refer to another style defined elsewhere (or included).
    *   **Resolution for Multiple Base Styles:**
        *   The compiler will process the base styles in the order they are listed in the `extends` array.
        *   Properties from later base styles in the array will **override** properties from earlier base styles if there are conflicts. For example, in `extends: ["A", "B"]`, if both `A` and `B` define `color`, `B`'s `color` will be used as the inherited value.
    *   **Final Override:** Any properties defined directly within the current `style "style_name"` block will be applied last, **overriding** any properties with the same name inherited from any of the base styles.
    *   Inheritance can be chained (e.g., Style C extends Style B, which extends Style A).
    *   The compiler **must** detect and report errors for:
        *   Undefined `base_style_name`(s).
        *   Cyclic dependencies (e.g., Style A extends Style B, and Style B extends Style A, or more complex cycles involving multiple styles).
        *   Invalid syntax for the `extends` value (e.g., not a string or an array of strings).
*   **Usage:** Applied to an element using the `style: "style_name"` property. Properties defined directly on the element override those from the applied style (including any inherited properties).
*   **KRB Mapping:** Style inheritance is resolved entirely by the **compiler**. The final `.krb` file contains `Style Blocks` with the fully resolved set of *standard* properties for each style ID. The runtime does not need to know about the `extends` relationship. Styles define *standard* KRB properties.

*   **Example (Single Inheritance):**
    ```kry
    style "button_base" { /* ... */ }
    style "button_primary" { extends: "button_base"; background_color: blue; }
    ```

*   **Example (Multiple Inheritance):**
    ```kry
    style "typography_mixin" { font_size: 16; text_color: #333; }
    style "padding_mixin" { padding: 10; }
    style "border_mixin_red" { border_width: 1; border_color: red; }
    style "border_mixin_blue" { border_width: 2; border_color: blue; }

    # "border_mixin_blue" properties will override "border_mixin_red" for border properties.
    # "typography_mixin" properties will be included.
    # "padding_mixin" properties will be included.
    style "fancy_button" {
        extends: ["padding_mixin", "typography_mixin", "border_mixin_red", "border_mixin_blue"]
        background_color: #EEEEEE # Direct property, overrides any inherited background
        font_weight: bold        # Direct property
    }

    # Usage
    Button {
        style: "fancy_button"
        text: "Submit"
    }
    ```

## 7. File Inclusion (`@include`)

Textually includes the content of another `.kry` file. Processed by the compiler before main parsing.

*   **Syntax:** `@include "path/to/other_file.kry"`
*   **Use Cases:** Sharing styles, component definitions, or parts of the UI across files.

## 8. Component Definition (`Define`)

Allows defining reusable custom UI components. These definitions are compiled into a `Component Definition Table` in the `.krb` file, enabling the runtime to instantiate them.

*   **Syntax:**
    ```kry
    Define ComponentName {
        # Optional: Declare properties the component accepts from usage tags.
        Properties {
            # propName: Type = DefaultValue # e.g., text_label: String = "Default"
            # e.g., count: Int = 0
            # e.g., is_enabled: Bool = true
            # e.g., item_style: StyleID = "default_item_style" # Maps to standard StyleID of root
            # e.g., status: Enum(active, inactive, pending) = active # Becomes custom prop
        }

        # Required: The root element structure using standard Kryon elements.
        # This defines the base KRB element(s) generated for this component's template.
        # This template is stored in the KRB Component Definition Table.
        Container { # Or Button, Text, etc. - Must be a single root standard element.
            # ... (standard properties and child elements forming the template)
            # Properties here are defaults for the template.
            # Instance-specific 'id', 'style', etc. come from the usage tag.
            # Children defined here are part of the template.
            # Children provided in the usage tag are handled by the runtime (see "Handling Children" below).
        }
    }
    ```

*   **Properties Block Details:**
    The `Properties` block within a `Define` statement declares the properties that instances of this component can accept.
    *   **Supported Types:** `String`, `Int`, `Float`, `Bool`, `Color`, `StyleID`, `Enum(...)`, `Resource`.
    *   **Default Values:** Optional. If a default value is not provided, the property might be considered required by the runtime or have a runtime-defined default.
    *   **KRB Mapping of Declared Properties:**
        *   **Standard KRY Properties:** If a declared property in `Properties {}` has the same name as a standard KRY property applicable to the *root element of the component's template* (e.g., `id`, `width`, `height`, `style`, `pos_x`, `pos_y`, `layout`), the compiler will treat values for these from the usage tag as intending to set these standard aspects of the component *instance's placeholder element*. The runtime, upon instantiation, should then apply these to the *actual root element created from the template*.
            *   Example: `Define MyComponent { Properties { custom_style: StyleID = "default_component_root_style" } Container { ... } }`
                Usage: `<MyComponent custom_style="instance_specific_style">`
                The `custom_style` value from the KRY usage will be resolved to a StyleID by the compiler and potentially stored as a custom property on the placeholder. The runtime would apply this StyleID to the `Container` instantiated from `MyComponent`'s template.
            *   **Special Case: `style` and `id`**
                If a component usage includes `style: "some_style"` or `id: "some_id"`, these are always intended for the component instance itself. The `style` will be applied to the root element of the instantiated component. The `id` will be the identifier for the component instance. These are **not** treated as custom properties if they match standard KRY properties for elements.
        *   **Component-Specific Properties:** Any other declared properties (e.g., `orientation`, `position` for a `TabBar`, `label_text` for a custom button) are treated as component-specific. The compiler will encode these as **KRB Custom Properties** on the placeholder element representing the component instance. The runtime is responsible for interpreting these custom properties.

*   **Usage (Instantiation in KRY):**
    Use the defined component like a standard element. This KRY usage translates into a **placeholder KRB element** in the main UI tree.
    ```kry
    ComponentName {
        id: "my_instance_1"        # Standard property for the instance
        style: "instance_root_style" # Standard property for the instance's root
        width: "50%"               # Standard property for the instance
        # Declared properties from ComponentName.Properties block:
        text_label: "Click Me"     # Becomes a KRB Custom Property on placeholder
        is_enabled: false          # Becomes a KRB Custom Property on placeholder
        # ... other declared/standard properties ...

        # Optional: Children for this specific instance
        # These are NOT part of the component's Define template.
        # The runtime is responsible for placing these children within the
        # instantiated component, typically into a designated container
        # within the component's template (e.g., a child Container with id="content_area").
        Text { text: "Child passed to instance" }
    }
    ```

*   **Compiler Role:**
    1.  **Parsing `Define` Blocks:**
        *   Stores the definition: name, declared properties (name, type hint, default value string), and the root element template structure.
        *   Serializes this information into the `Component Definition Table` in the `.krb` file.
    2.  **Handling Component Usage (`<ComponentName>`):**
        *   Recognizes `ComponentName` as a defined component.
        *   Generates a **single placeholder KRB element** in the main UI tree.
            *   The `Type` of this placeholder element could be a generic `ELEM_TYPE_CONTAINER` or a specific `ELEM_TYPE_CUSTOM` if desired, but `ELEM_TYPE_CONTAINER` is often sufficient if the runtime uses the `_componentName` custom property for identification. The KRB spec (v0.4) suggests standard element types (like Container) combined with Custom Properties is often preferred. For this strategy, we'll assume the placeholder's `Type` matches the *root element type of the component's definition template*.
            *   The `ID` field in the placeholder's Element Header is set from the `id: "..."` in the KRY usage.
            *   Other standard KRY properties from the usage tag (`pos_x`, `pos_y`, `width`, `height`, `layout`, `style`) are applied to the *placeholder element's header/standard properties*. The runtime will then typically transfer/apply these to the actual root of the instantiated component.
        *   **`_componentName` Custom Property:** The compiler **must** add a KRB Custom Property with the key `_componentName` (or another agreed-upon convention) to the placeholder element. The value of this property will be the string table index of `ComponentName`. This allows the runtime to identify which component definition to use for instantiation.
        *   **Instance-Specific Properties:**
            *   For properties in the KRY usage tag that match a name in the `Define ComponentName { Properties {...} }` block (and are not standard KRY element properties like `id` or `style`), the compiler encodes them as **KRB Custom Properties** on the placeholder element.
            *   Values from the KRY usage tag override default values from the `Define` block.
        *   **Children in Usage:** Children defined within a component usage tag in KRY (e.g., the `Text` element in the `ComponentName` example above) are compiled into standard KRB child element blocks. These become children of the *placeholder KRB element*. The runtime is then responsible for taking these children and re-parenting them into an appropriate location within the instantiated component's structure. This often involves a convention (e.g., the component's template has a `Container { id: "slot" }` where instance children are placed).

*   **Runtime Role (Crucial for Instantiation Strategy):**
    1.  **Parsing KRB:** Reads the main UI tree and the `Component Definition Table`.
    2.  **Encountering a Placeholder Element:**
        *   Identifies it as a component instance placeholder, typically by checking for the `_componentName` custom property.
        *   Retrieves the component name string using the value of `_componentName`.
        *   Looks up the corresponding `ComponentDefinition` in its parsed `Component Definition Table`. If not found, this is an error (as seen in your logs).
    3.  **Instantiation:**
        *   Creates a new element subtree in its internal render tree based on the `Root Element Template` from the found `ComponentDefinition`.
        *   Applies standard properties (`ID`, `StyleID`, `PosX`, `PosY`, `Width`, `Height`, `Layout` byte) from the *placeholder KRB element* to the *root element of the newly instantiated subtree*.
        *   Processes KRB Custom Properties found on the *placeholder element*:
            *   These correspond to the component-specific properties declared in `Define.Properties` and set in the KRY usage tag.
            *   The runtime uses these custom properties to configure the behavior and appearance of the instantiated component and its internal elements.
    4.  **Handling Children from Usage:**
        *   If the placeholder KRB element has children, the runtime takes these children.
        *   It re-parents them into a designated "slot" or content area within the newly instantiated component's structure. This typically requires a convention (e.g., the component's template defines a `Container { id: "children_host" }` and the runtime looks for it). If no such slot is defined or found, the runtime might append them as direct children of the instantiated component's root, or it might be an error, depending on the component's design.
    5.  The placeholder element itself is effectively replaced by the instantiated component subtree in the runtime's final render tree.

*   **Relationship with KRB `Component Definition Table`:**
    *   The KRY `Define` block is the direct source for entries in the KRB `Component Definition Table`.
    *   The compiler ensures this table is populated.
    *   The runtime relies entirely on this table for instantiating components referenced in the main KRB element tree.
    ## 8. Component Definition (`Define`)

Allows defining reusable custom UI components. These definitions are compiled into a `Component Definition Table` in the `.krb` file, enabling the runtime to instantiate them.

*   **Syntax:**
    ```kry
    Define ComponentName {
        # Optional: Declare properties the component accepts from usage tags.
        Properties {
            # propName: Type = DefaultValue # e.g., text_label: String = "Default"
            # e.g., count: Int = 0
            # e.g., is_enabled: Bool = true
            # e.g., item_style: StyleID = "default_item_style" # Maps to standard StyleID of root
            # e.g., status: Enum(active, inactive, pending) = active # Becomes custom prop
        }

        # Required: The root element structure using standard Kryon elements.
        # This defines the base KRB element(s) generated for this component's template.
        # This template is stored in the KRB Component Definition Table.
        Container { # Or Button, Text, etc. - Must be a single root standard element.
            # ... (standard properties and child elements forming the template)
            # Properties here are defaults for the template.
            # Instance-specific 'id', 'style', etc. come from the usage tag.
            # Children defined here are part of the template.
            # Children provided in the usage tag are handled by the runtime (see "Handling Children" below).
        }
    }
    ```

*   **Properties Block Details:**
    The `Properties` block within a `Define` statement declares the properties that instances of this component can accept.
    *   **Supported Types:** `String`, `Int`, `Float`, `Bool`, `Color`, `StyleID`, `Enum(...)`, `Resource`.
    *   **Default Values:** Optional. If a default value is not provided, the property might be considered required by the runtime or have a runtime-defined default.
    *   **KRB Mapping of Declared Properties:**
        *   **Standard KRY Properties:** If a declared property in `Properties {}` has the same name as a standard KRY property applicable to the *root element of the component's template* (e.g., `id`, `width`, `height`, `style`, `pos_x`, `pos_y`, `layout`), the compiler will treat values for these from the usage tag as intending to set these standard aspects of the component *instance's placeholder element*. The runtime, upon instantiation, should then apply these to the *actual root element created from the template*.
            *   Example: `Define MyComponent { Properties { custom_style: StyleID = "default_component_root_style" } Container { ... } }`
                Usage: `<MyComponent custom_style="instance_specific_style">`
                The `custom_style` value from the KRY usage will be resolved to a StyleID by the compiler and potentially stored as a custom property on the placeholder. The runtime would apply this StyleID to the `Container` instantiated from `MyComponent`'s template.
            *   **Special Case: `style` and `id`**
                If a component usage includes `style: "some_style"` or `id: "some_id"`, these are always intended for the component instance itself. The `style` will be applied to the root element of the instantiated component. The `id` will be the identifier for the component instance. These are **not** treated as custom properties if they match standard KRY properties for elements.
        *   **Component-Specific Properties:** Any other declared properties (e.g., `orientation`, `position` for a `TabBar`, `label_text` for a custom button) are treated as component-specific. The compiler will encode these as **KRB Custom Properties** on the placeholder element representing the component instance. The runtime is responsible for interpreting these custom properties.

*   **Usage (Instantiation in KRY):**
    Use the defined component like a standard element. This KRY usage translates into a **placeholder KRB element** in the main UI tree.
    ```kry
    ComponentName {
        id: "my_instance_1"        # Standard property for the instance
        style: "instance_root_style" # Standard property for the instance's root
        width: "50%"               # Standard property for the instance
        # Declared properties from ComponentName.Properties block:
        text_label: "Click Me"     # Becomes a KRB Custom Property on placeholder
        is_enabled: false          # Becomes a KRB Custom Property on placeholder
        # ... other declared/standard properties ...

        # Optional: Children for this specific instance
        # These are NOT part of the component's Define template.
        # The runtime is responsible for placing these children within the
        # instantiated component, typically into a designated container
        # within the component's template (e.g., a child Container with id="content_area").
        Text { text: "Child passed to instance" }
    }
    ```

*   **Compiler Role:**
    1.  **Parsing `Define` Blocks:**
        *   Stores the definition: name, declared properties (name, type hint, default value string), and the root element template structure.
        *   Serializes this information into the `Component Definition Table` in the `.krb` file.
    2.  **Handling Component Usage (`<ComponentName>`):**
        *   Recognizes `ComponentName` as a defined component.
        *   Generates a **single placeholder KRB element** in the main UI tree.
            *   The `Type` of this placeholder element could be a generic `ELEM_TYPE_CONTAINER` or a specific `ELEM_TYPE_CUSTOM` if desired, but `ELEM_TYPE_CONTAINER` is often sufficient if the runtime uses the `_componentName` custom property for identification. The KRB spec (v0.4) suggests standard element types (like Container) combined with Custom Properties is often preferred. For this strategy, we'll assume the placeholder's `Type` matches the *root element type of the component's definition template*.
            *   The `ID` field in the placeholder's Element Header is set from the `id: "..."` in the KRY usage.
            *   Other standard KRY properties from the usage tag (`pos_x`, `pos_y`, `width`, `height`, `layout`, `style`) are applied to the *placeholder element's header/standard properties*. The runtime will then typically transfer/apply these to the actual root of the instantiated component.
        *   **`_componentName` Custom Property:** The compiler **must** add a KRB Custom Property with the key `_componentName` (or another agreed-upon convention) to the placeholder element. The value of this property will be the string table index of `ComponentName`. This allows the runtime to identify which component definition to use for instantiation.
        *   **Instance-Specific Properties:**
            *   For properties in the KRY usage tag that match a name in the `Define ComponentName { Properties {...} }` block (and are not standard KRY element properties like `id` or `style`), the compiler encodes them as **KRB Custom Properties** on the placeholder element.
            *   Values from the KRY usage tag override default values from the `Define` block.
        *   **Children in Usage:** Children defined within a component usage tag in KRY (e.g., the `Text` element in the `ComponentName` example above) are compiled into standard KRB child element blocks. These become children of the *placeholder KRB element*. The runtime is then responsible for taking these children and re-parenting them into an appropriate location within the instantiated component's structure. This often involves a convention (e.g., the component's template has a `Container { id: "slot" }` where instance children are placed).

*   **Runtime Role (Crucial for Instantiation Strategy):**
    1.  **Parsing KRB:** Reads the main UI tree and the `Component Definition Table`.
    2.  **Encountering a Placeholder Element:**
        *   Identifies it as a component instance placeholder, typically by checking for the `_componentName` custom property.
        *   Retrieves the component name string using the value of `_componentName`.
        *   Looks up the corresponding `ComponentDefinition` in its parsed `Component Definition Table`. If not found, this is an error (as seen in your logs).
    3.  **Instantiation:**
        *   Creates a new element subtree in its internal render tree based on the `Root Element Template` from the found `ComponentDefinition`.
        *   Applies standard properties (`ID`, `StyleID`, `PosX`, `PosY`, `Width`, `Height`, `Layout` byte) from the *placeholder KRB element* to the *root element of the newly instantiated subtree*.
        *   Processes KRB Custom Properties found on the *placeholder element*:
            *   These correspond to the component-specific properties declared in `Define.Properties` and set in the KRY usage tag.
            *   The runtime uses these custom properties to configure the behavior and appearance of the instantiated component and its internal elements.
    4.  **Handling Children from Usage:**
        *   If the placeholder KRB element has children, the runtime takes these children.
        *   It re-parents them into a designated "slot" or content area within the newly instantiated component's structure. This typically requires a convention (e.g., the component's template defines a `Container { id: "children_host" }` and the runtime looks for it). If no such slot is defined or found, the runtime might append them as direct children of the instantiated component's root, or it might be an error, depending on the component's design.
    5.  The placeholder element itself is effectively replaced by the instantiated component subtree in the runtime's final render tree.

*   **Relationship with KRB `Component Definition Table`:**
    *   The KRY `Define` block is the direct source for entries in the KRB `Component Definition Table`.
    *   The compiler ensures this table is populated.
    *   The runtime relies entirely on this table for instantiating components referenced in the main KRB element tree.
    
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