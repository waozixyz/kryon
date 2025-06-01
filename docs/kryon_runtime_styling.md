# **Kryon Runtime: Styling, Defaults, and Inheritance Specification v1.0**

## 1. Introduction

This document specifies the default values for styling properties and the inheritance behavior expected from a Kryon Runtime Environment when processing and rendering UI elements defined by the Kryon Binary Format (KRB). Adherence to these rules ensures consistent visual output and predictable behavior across different Kryon runtime implementations.

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
    *   All properties not listed in 4.2 are generally not inheritable by default. This includes `background_color`, `border_*` properties, `padding`, dimensional properties (`width`, `height`), and `layout` properties.

## 5. Order of Application Summary

For a given `RenderElement`, properties are conceptually determined in the following order:

1.  **Basic Initialization:** Element created with fundamental type, ID, and structural links (parent/child array). Visual properties are at their most basic state (e.g., transparent colors, zero dimensions/spacing).
2.  **Style Application:** Properties from the element's assigned `StyleID` (as resolved by the KRY compiler, including `extends`) are applied.
3.  **Direct Property Application:** Direct KRB properties for the element are applied, overriding any values set by the style.
4.  **Contextual Default Resolution:** Defaults based on inter-property dependencies are applied (e.g., default border width if only color is set, as per Section 3).
5.  **Inheritance Resolution:** For inheritable properties still "unset," values are inherited from the parent chain or application-level defaults (as per Section 4).
6.  **Layout Engine:** The layout engine computes `RenderX, RenderY, RenderW, RenderH` based on all resolved properties, parent constraints, and content. Minimum visible dimensions might be applied here (Section 3.1).
7.  **Custom Component Adjustments:** `CustomComponentHandler.HandleLayoutAdjustment` may make final modifications to layout.
8.  **Rendering:** The element is drawn using its finally resolved visual properties and layout geometry.

This order ensures a clear cascade and allows for explicit overrides at each stage.

---