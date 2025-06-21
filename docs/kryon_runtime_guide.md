# **Kryon Runtime: Styling, Defaults, and Inheritance Specification v1.1**

## Change Log
*   **v1.1**: Added comprehensive script runtime integration supporting both embedded and external script loading. Implemented state-based property resolution for pseudo-selector styling support. Enhanced property resolution order to include script modifications and state property overlays. Added cursor property defaults and interactive state tracking. Extended error handling for missing external scripts with graceful degradation.
*   **v1.0**: Initial specification defining default styling values, property inheritance behavior, and rendering order for Kryon Runtime Environments. Established WindowConfig structure, contextual property resolution, and inheritance rules for consistent visual output across implementations.

## 1. Introduction

This document specifies the default values for styling properties, inheritance behavior, script execution, and interactive state management expected from a Kryon Runtime Environment when processing and rendering UI elements defined by the Kryon Binary Format (KRB). Adherence to these rules ensures consistent visual output, predictable behavior, and proper dynamic functionality across different Kryon runtime implementations.

These rules apply during the "RenderElement Preparation Phase," which occurs after an element's explicit style (from its `StyleID`) and direct KRB properties have been initially resolved, but before final layout and rendering.

## 2. Global and Application-Level Defaults

The Kryon Runtime utilizes a `WindowConfig` structure, which should include the following default styling values. These are typically initialized from `render.DefaultWindowConfig()` and then potentially overridden by properties from the `App` element's style or its direct KRB properties.

*   **`WindowConfig.DefaultBgColor`**:
    *   **Purpose**: The default background color used to clear the window each frame.
    *   **Default Value**: A sensible opaque color (e.g., Dark Gray `#1E1E1EFF` or `rl.NewColor(30, 30, 30, 255)`).
*   **`WindowConfig.DefaultFgColor`**:
    *   **Purpose**: The root default foreground/text color for the application. This is the starting point for text color inheritance.
    *   **Default Value**: A color contrasting with `DefaultBgColor` (e.g., White `#FFFFFFFF` or `rl.RayWhite`).
*   **`WindowConfig.DefaultBorderColor`**:
    *   **Purpose**: The default color used for element borders if a `border_width` is specified but no `border_color` is provided.
    *   **Default Value**: A neutral color (e.g., Mid-Gray `#808080FF` or `rl.Gray`).
*   **`WindowConfig.DefaultFontSize`**:
    *   **Purpose**: The root default font size for the application, used if no other font size is specified or inherited.
    *   **Default Value**: A readable base size (e.g., `18.0` pixels).
*   **`WindowConfig.DefaultFontFamily`**: (If font families are supported)
    *   **Purpose**: The root default font family.
    *   **Default Value**: System default sans-serif font.

The `App` element (`ELEM_TYPE_APP`) itself is also a `RenderElement`. Properties applied to it (via its style or direct KRB properties) can override these `WindowConfig` defaults and also style the main application "canvas."

## 3. Element Property Defaults and Contextual Resolution

For individual `RenderElement`s, the following default values and resolution logic **must** be applied if the property has not been explicitly set by its style or direct KRB properties.

| Property (KRY/KRB)        | RenderElement Field(s)        | Default Value if Unset                                     | Contextual Default Logic                                                                                                                                                                                                   | Inheritable |
| :------------------------ | :---------------------------- | :--------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------- |
| `background_color`        | `BgColor`                     | Transparent (`rl.Blank` or RGBA `0,0,0,0`)                 | None.                                                                                                                                                                                                                      | No          |
| `text_color` / `fg_color` | `FgColor`                     | *Determined by Inheritance* (see Section 4)                | If inheritance results in no color (e.g., no ancestor specified one), defaults to `WindowConfig.DefaultFgColor`. For non-text-bearing elements, if unset, remains transparent/blank.                             | **Yes**     |
| `border_color`            | `BorderColor`                 | Transparent (`rl.Blank`)                                   | If any `BorderWidths[i] > 0` and `BorderColor` is transparent, `BorderColor` defaults to `WindowConfig.DefaultBorderColor`.                                                                                             | No          |
| `border_width`            | `BorderWidths` (all sides)    | `0` for all sides                                          | If `BorderColor` is set (and not transparent) and all `BorderWidths` are `0`, all `BorderWidths[i]` default to `1` (pixel, scaled at render time).                                                                         | No          |
| `border_radius`           | *(Renderer-specific)*         | `0`                                                        | None.                                                                                                                                                                                                                      | No          |
| `padding`                 | `Padding` (all sides)         | `0` for all sides                                          | None.                                                                                                                                                                                                                      | No          |
| `margin`                  | *(Renderer-specific)*         | `0` for all sides                                          | None.                                                                                                                                                                                                                      | No          |
| `text_alignment`          | `TextAlignment`               | `krb.LayoutAlignStart` (or equivalent numerical value)     | None beyond initial default.                                                                                                                                                                                               | **Yes**     |
| `font_size`               | *(Renderer-specific)*         | *Determined by Inheritance* (see Section 4)                | If inheritance results in no size, defaults to `WindowConfig.DefaultFontSize`.                                                                                                                                           | **Yes**     |
| `font_family`             | *(Renderer-specific)*         | *Determined by Inheritance* (see Section 4)                | If inheritance results in no family, defaults to `WindowConfig.DefaultFontFamily`.                                                                                                                                       | **Yes**     |
| `font_weight`             | *(Renderer-specific)*         | "Normal" / `krb.FontWeightNormal` (or equivalent)          | None beyond initial default.                                                                                                                                                                                               | **Yes**     |
| `cursor`                  | `Cursor`                      | `CursorDefault` (0)                                        | None.                                                                                                                                                                                                                      | No          |
| `opacity`                 | *(Renderer-specific)*         | `1.0` (fully opaque)                                       | None.                                                                                                                                                                                                                      | No          |
| `visibility`              | `IsVisible`                   | `true` (visible)                                           | While the `IsVisible` flag itself is not directly inherited, a parent's resolved state of being *not visible* will prevent the child from rendering, regardless of the child's own `IsVisible` flag.                     | No (effective visibility is cascaded) |
| `width`, `height`         | `RenderW`, `RenderH`          | Determined by layout engine (intrinsic, parent, grow, etc.)  | Default behavior is complex and part of the layout algorithm (e.g., content size, stretch if `LayoutGrowBit` is set). No simple default value applies before layout. After layout, if `0`, may receive minimums (see 3.1). | No          |
| `min_width`, `min_height` | *(Used by Layout Engine)*     | `0`                                                        | None.                                                                                                                                                                                                                      | No          |
| `max_width`, `max_height` | *(Used by Layout Engine)*     | "Infinity" / Unconstrained                                 | None.                                                                                                                                                                                                                      | No          |
| `layout` (for children)   | `Header.Layout`               | Default flow (e.g., `LayoutDirColumn`, `LayoutAlignStart`) | The `Layout` byte in `ElementHeader` dictates children layout. If a `Container` has no `layout` specified, it might default to column/start.                                                                          | No          |
| `gap`                     | *(Used by Layout Engine)*     | `0`                                                        | None.                                                                                                                                                                                                                      | No          |

**3.1. Minimum Visible Dimensions:**
*   After the layout pass, if an element has `RenderW > 0` but `RenderH == 0` (or vice-versa), and the element is intended to be visible (e.g., has a background, border, or is a known container type like `App` or `Container`), the runtime **should** assign a minimum sensible dimension to the zero-value axis (e.g., `1.0 * scaleFactor` or scaled `baseFontSize`). This prevents visually present elements from collapsing entirely.

## 4. Property Inheritance

The Kryon Runtime **must** implement property inheritance for designated inheritable properties. This allows styles to cascade down the element tree.

*   **4.1. Inheritance Process:**
    1.  **Initial Value Resolution:** For each `RenderElement`, its properties are first resolved based on:
        *   Highest Precedence: Direct KRB properties on the element.
        *   Next Precedence: Properties from the element's assigned style (via `StyleID`), with style extension/override rules already flattened by the KRY compiler.
        *   Next Precedence: Contextual defaults (as defined in Section 3).
    2.  **Inheritance Check:** If, after the above steps, a property designated as "Inheritable" (see table in Section 3) remains effectively "unset" (e.g., `FgColor` is transparent/blank, `FontSize` is 0 or a sentinel "not-set" value):
        *   The runtime **must** look to the element's computed value for that same property on its direct `Parent` `RenderElement`.
        *   The child element then inherits this computed value from its parent.
    3.  **Root of Inheritance:** This process continues up the tree. If the root `App` element is reached and an inheritable property is still "unset" on it, the value from the corresponding `WindowConfig` default (e.g., `WindowConfig.DefaultFgColor`, `WindowConfig.DefaultFontSize`) **must** be used.
    4.  **Stopping Inheritance:** If an element explicitly sets an inheritable property (even to a value like transparent for a color, or a specific font size), that explicit value is used for the element itself, and *this new computed value* becomes the value its own children will inherit for that property.

*   **4.2. Explicitly Inheritable Properties:**
    *   `text_color` (FgColor)
    *   `font_size`
    *   `font_family` (if supported)
    *   `font_weight`
    *   `text_alignment`

*   **4.3. Non-Inheritable Properties by Default:**
    *   All properties not listed in 4.2 are generally not inheritable by default. This includes `background_color`, `border_*` properties, `padding`, dimensional properties (`width`, `height`), `layout` properties, and `cursor`.

## 5. Script Runtime Integration

When the runtime parses a `.krb` file that contains embedded scripts (`FLAG_HAS_SCRIPTS` is set), the following process is expected:

### 5.1. Script Loading

The runtime must handle both embedded and external scripts during KRB file loading:

1. **Parse Script Table** during KRB file loading
2. **For each script entry:**
   - If `Storage Format = 0x00` (Inline): Load script content directly from the script entry's code data
   - If `Storage Format = 0x01` (External): Load script content via Resource Table reference
3. **Initialize appropriate script engines** based on Language IDs
4. **Load and compile script code** using the determined content source
5. **Register entry point functions** in global namespace accessible to the UI event system

***
function LoadScripts(krbFile) {
    for each script in krbFile.Scripts {
        if (script.storageFormat == INLINE) {
            scriptContent = script.codeData
        } else if (script.storageFormat == EXTERNAL) {
            scriptContent = loadExternalScript(script.resourceIndex)
        }
        
        if (scriptContent != null) {
            engine = CreateEngine(script.languageId)
            engine.loadCode(scriptContent)
            
            for each entryPoint in script.entryPoints {
                functionName = getString(entryPoint.functionNameIndex)
                scriptManager.registerFunction(functionName, engine, functionName)
            }
        } else {
            handleMissingScript(script)
        }
    }
}
***

### 5.2. External Script Loading

When a script entry has `Storage Format = 0x01` (External), the runtime must load the script from an external resource:

1. **Resource Resolution:** Use the `Resource Index` field to look up the corresponding entry in the Resource Table.
2. **File Loading:** Based on the resource's format:
   - **External Resource**: Load script content from the file path specified in the resource's data
   - **Inline Resource**: Use script content embedded in the resource's data section
3. **Error Handling:** If external script loading fails:
   - Log appropriate warnings
   - Register no-op functions for the script's entry points to prevent runtime errors
   - Continue application execution with degraded functionality
4. **Caching:** Implementations should cache loaded external scripts to avoid repeated file system access.

### 5.3. Runtime API Implementation

The runtime must provide a standard API (typically through a `kryon` global object) accessible to all scripts:

***
kryon = {
    // Element manipulation
    getElementById: function(id) -> Element
    createElement: function(type) -> Element
    
    // Property access
    getProperty: function(elementId, propertyName) -> value
    setProperty: function(elementId, propertyName, value)
    
    // Event handling  
    addEventListener: function(elementId, eventType, callback)
    removeEventListener: function(elementId, eventType, callback)
    
    // State management
    setState: function(key, value)
    getState: function(key) -> value
    
    // Variable access (from KRY @variables)
    getVariable: function(name) -> value
    
    // Timers and scheduling
    setTimer: function(delay, callback) -> timerId
    clearTimer: function(timerId)
    setInterval: function(interval, callback) -> intervalId
    clearInterval: function(intervalId)
    
    // System integration
    showMessage: function(message)
    vibrate: function(duration)
    navigateTo: function(route)
}
***

### 5.4. Event Integration

When UI events occur (clicks, focus changes, etc.), the runtime:

1. Looks up the corresponding callback function name from the element's Event entries
2. Finds the registered script function with that name
3. Executes the function with appropriate parameters (element ID, event data)
4. Handles any script errors gracefully without crashing the UI

### 5.5. Error Handling

The runtime must implement robust error handling for script-related issues:

***
function handleMissingScript(scriptEntry) {
    logWarning("External script unavailable: " + getString(scriptEntry.nameIndex))
    
    // Register no-op functions for missing entry points
    for each entryPoint in scriptEntry.entryPoints {
        functionName = getString(entryPoint.functionNameIndex)
        scriptManager.registerNoOpFunction(functionName)
    }
}

function registerNoOpFunction(functionName) {
    globalFunctions[functionName] = function(...args) {
        logWarning("Called missing function: " + functionName)
        return null
    }
}
***

## 6. State-Based Property Resolution

When the runtime parses a `.krb` file that contains state-based properties (`FLAG_HAS_STATE_PROPERTIES` is set), the following process is expected:

### 6.1. State Tracking

The runtime must track interaction states for all applicable elements:

- **Hover**: Mouse cursor over element
- **Active**: Element being pressed/clicked
- **Focus**: Element has keyboard focus  
- **Disabled**: Element is non-interactive
- **Checked**: Element is in checked state (checkboxes, radio buttons)

### 6.2. Property Resolution Order

For each element, properties are resolved in the following order:

1. **Base Properties** (from style and direct element properties)
2. **Applicable State Property Sets** (overlaid when corresponding states are active)
3. **Final Computed Properties** (used for rendering)

***
function ResolveStateProperties(element) {
    // Start with base properties
    properties = element.baseProperties.copy()
    
    // Apply matching state property sets
    for each stateSet in element.statePropertySets {
        if (element.currentState & stateSet.stateFlags) {
            properties.overlay(stateSet.properties)
        }
    }
    
    element.computedProperties = properties
    return properties
}
***

### 6.3. State Change Handling

When element states change (e.g., mouse enter/leave, focus gain/loss), the runtime:

1. Updates the element's current state flags
2. Re-evaluates state property sets
3. Updates computed properties
4. Marks element for re-rendering if properties changed

***
function UpdateElementState(element, newState) {
    if (element.currentState != newState) {
        element.currentState = newState
        oldProperties = element.computedProperties.copy()
        newProperties = ResolveStateProperties(element)
        
        if (propertiesChanged(oldProperties, newProperties)) {
            MarkForRedraw(element)
        }
    }
}
***

### 6.4. State Property Format

State property sets follow the same format as standard properties but are grouped by state flags:

***
StatePropertySet {
    stateFlags: uint8     // Bit flags for applicable states
    propertyCount: uint8  // Number of properties in this set
    properties: []Property // Standard property entries
}

State Flags:
- Bit 0: STATE_HOVER
- Bit 1: STATE_ACTIVE  
- Bit 2: STATE_FOCUS
- Bit 3: STATE_DISABLED
- Bit 4: STATE_CHECKED
- Bit 5-7: Reserved
***

## 7. Order of Application Summary

For a given `RenderElement`, properties are conceptually determined in the following order:

1. **Basic Initialization:** Element created with fundamental type, ID, and structural links (parent/child array). Visual properties are at their most basic state (e.g., transparent colors, zero dimensions/spacing).

2. **Style Application:** Properties from the element's assigned `StyleID` (as resolved by the KRY compiler, including `extends`) are applied.

3. **Direct Property Application:** Direct KRB properties for the element are applied, overriding any values set by the style.

4. **State Property Resolution:** If `FLAG_HAS_STATE_PROPERTIES` is set and element has state property sets, apply matching state properties based on current element state.

5. **Script Property Modifications:** If scripts have modified element properties via the runtime API (`kryon.setProperty()`), apply those changes.

6. **Contextual Default Resolution:** Defaults based on inter-property dependencies are applied (e.g., default border width if only color is set, as per Section 3).

7. **Inheritance Resolution:** For inheritable properties still "unset," values are inherited from the parent chain or application-level defaults (as per Section 4).

8. **Layout Engine:** The layout engine computes `RenderX, RenderY, RenderW, RenderH` based on all resolved properties, parent constraints, and content. Minimum visible dimensions might be applied here (Section 3.1).

9. **Custom Component Adjustments:** `CustomComponentHandler.HandleLayoutAdjustment` may make final modifications to layout.

10. **Rendering:** The element is drawn using its finally resolved visual properties and layout geometry.

This order ensures a clear cascade and allows for explicit overrides at each stage while supporting dynamic behavior through scripts and interactive states.

## 8. Performance Considerations

### 8.1. Script Execution Overhead
- **Engine Selection:** Choose script engines appropriate for target platform constraints
- **Bytecode Caching:** Cache compiled script bytecode to avoid re-compilation
- **Function Call Optimization:** Minimize overhead in script-to-native function calls

### 8.2. State Property Optimization
- **State Batching:** Batch state changes to minimize re-rendering overhead
- **Property Diffing:** Only update properties that actually changed between states
- **Selective Recomputation:** Only recompute state properties for elements with state changes

### 8.3. Memory Management
- **Script Engine Pools:** Reuse script engines across multiple script blocks when possible
- **Property Cache:** Cache computed properties to avoid redundant calculations
- **Lazy Loading:** Load external scripts only when first needed

### 8.4. Error Recovery
- **Graceful Degradation:** Continue UI operation even when scripts fail or are missing
- **User Feedback:** Provide appropriate feedback for missing functionality without breaking the experience
- **Logging:** Comprehensive logging for debugging while avoiding performance impact in production

---

This order and the integrated script/state systems ensure a clear cascade while supporting rich dynamic behavior and maintaining performance characteristics suitable for resource-constrained environments.