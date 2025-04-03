// impl/js/krb-renderer.js
import * as C from './krb-constants.js';

// --- Global KRB Document Data (set by render function) ---
let KrbDoc = null;

// --- Event Handling Setup ---
function findCallback(callbackIdIndex) {
    if (!KrbDoc || !KrbDoc.strings || callbackIdIndex < 0 || callbackIdIndex >= KrbDoc.strings.length) {
        console.warn(`Invalid callback ID index: ${callbackIdIndex} (String table size: ${KrbDoc?.strings?.length})`);
        return () => console.warn(`Callback function for index ${callbackIdIndex} not found or index invalid.`);
    }
    const functionName = KrbDoc.strings[callbackIdIndex];
    if (!functionName) {
         return () => console.warn(`Callback function name for index ${callbackIdIndex} is null/empty.`);
    }

    // Try global scope first
    if (typeof window[functionName] === 'function') {
        return window[functionName];
    }

    // Try resolving dot notation path
    const parts = functionName.split('.');
    let func = window;
    try {
        for (const part of parts) {
            if (func === undefined || func === null || func[part] === undefined) { // Check func exists before indexing
                throw new Error(`Part "${part}" not found or parent object is undefined`);
            }
            func = func[part];
        }
        if (typeof func === 'function') {
            return func;
        } else {
             console.warn(`Global path "${functionName}" (index ${callbackIdIndex}) resolved, but is not a function (type: ${typeof func}).`);
        }
    } catch (e) {
         // console.warn(`Could not resolve function path "${functionName}": ${e.message}`);
         // Fall through to final warning
    }

    console.warn(`Global callback function or path "${functionName}" (index ${callbackIdIndex}) not found or not a function.`);
    return (event) => console.warn(`Callback function "${functionName}" (index ${callbackIdIndex}) not found. Event:`, event);
}

function mapKrbEventToDomEvent(krbEventType) {
    switch (krbEventType) {
        case C.EVENT_TYPE_CLICK: return 'click';
        case C.EVENT_TYPE_PRESS: return 'pointerdown';
        case C.EVENT_TYPE_RELEASE: return 'pointerup';
        case C.EVENT_TYPE_HOVER: return 'pointerenter'; // Separate leave listener added in attachEvents
        case C.EVENT_TYPE_FOCUS: return 'focus';
        case C.EVENT_TYPE_BLUR: return 'blur';
        case C.EVENT_TYPE_CHANGE: return 'change';
        case C.EVENT_TYPE_SUBMIT: return 'submit';
        // case C.EVENT_TYPE_LONGPRESS: // Needs custom JS logic
        // case C.EVENT_TYPE_CUSTOM: // Needs custom handling
        default:
            // console.warn(`Unsupported KRB event type: ${krbEventType}`); // Potentially noisy
            return null;
    }
}


// --- Style and Property Helpers ---

function parseColor(rawValueBytes, useExtendedColor) {
    // Check if rawValueBytes is a valid ArrayBuffer with some length
    if (!rawValueBytes || !(rawValueBytes instanceof ArrayBuffer) || rawValueBytes.byteLength === 0) {
         return 'inherit'; // Default or inherit
    }
    const view = new DataView(rawValueBytes);
    try {
        if (useExtendedColor) {
            // 4-byte RGBA (Check size explicitly)
            if (view.byteLength < 4) {
                 console.warn("Extended color data too short, expected 4 bytes got", view.byteLength);
                 return 'red'; // Error color
            }
            const r = view.getUint8(0);
            const g = view.getUint8(1);
            const b = view.getUint8(2);
            const a = view.getUint8(3) / 255.0;
            return `rgba(${r}, ${g}, ${b}, ${a.toFixed(3)})`;
        } else {
            // 1-byte palette index (Check size explicitly)
            if (view.byteLength < 1) {
                console.warn("Palette color data too short, expected 1 byte got", view.byteLength);
                return 'red'; // Error color
            }
            const index = view.getUint8(0);
            // TODO: Implement palette lookup
            console.warn(`Palette color index ${index} requested, but palette is not implemented.`);
            const shade = Math.min(255, index * 16); // Simple fallback
            return `rgb(${shade}, ${shade}, ${shade})`;
        }
    } catch (e) {
        console.error("Error parsing color:", e, rawValueBytes);
        return 'magenta'; // Indicate parsing error
    }
}

function parseFixedPointPercentage(rawValueU16) {
    if (typeof rawValueU16 !== 'number') return '0.0';
    // Value is 8.8 fixed point (0-255.99...) stored as u16
    return (rawValueU16 / 256.0).toFixed(4);
}

function parseEdgeInsets(rawValueBytes) {
     // Spec says "4 bytes/shorts". Assuming 4 bytes for common use like padding/margin.
     // Adjust if shorts (8 bytes) are needed for border-width etc.
     const expectedSize = 4;
     if (!rawValueBytes || !(rawValueBytes instanceof ArrayBuffer) || rawValueBytes.byteLength < expectedSize) {
         console.warn(`EdgeInsets data invalid or too short, expected ${expectedSize} bytes got ${rawValueBytes?.byteLength}`);
         return { top: 0, right: 0, bottom: 0, left: 0 };
     }
     const view = new DataView(rawValueBytes);
      try {
         // Assuming 4 * u8
         return {
             top: view.getUint8(0),
             right: view.getUint8(1),
             bottom: view.getUint8(2),
             left: view.getUint8(3),
         };
         // If using shorts (8 bytes):
         // return {
         //     top: view.getInt16(0, true), // Use Uint16 if always positive
         //     right: view.getInt16(2, true),
         //     bottom: view.getInt16(4, true),
         //     left: view.getInt16(6, true),
         // };
     } catch (e) {
         console.error("Error parsing edge insets:", e, rawValueBytes);
         return { top: 0, right: 0, bottom: 0, left: 0 };
     }
}

// Helper to convert ArrayBuffer to Hex String
function arrayBufferToHex(buffer) {
    if (!buffer || !(buffer instanceof ArrayBuffer)) return "";
    return [...new Uint8Array(buffer)]
        .map(b => b.toString(16).padStart(2, '0'))
        .join('');
}

/**
 * Applies styles and properties to a DOM element.
 * Direct properties override style properties.
 * @param {HTMLElement} domElement The target DOM element.
 * @param {object} elementData Parsed data for the KRB element. Includes header, properties, customProperties.
 */
function applyStyling(domElement, elementData) {
    const header = elementData.header;
    const directProperties = elementData.properties || [];
    const customProperties = elementData.customProperties || []; // Added v0.3
    const styleProperties = [];

    // 1. Get Style Properties (if applicable)
    if (header.styleId > 0 && KrbDoc.styles) {
        const style = KrbDoc.styles.find(s => s.id === header.styleId); // Style ID is 1-based
        if (style && style.properties) {
            style.properties.forEach(prop => styleProperties.push(prop));
        } else {
            console.warn(`Style ID ${header.styleId} not found or has no properties.`);
        }
    }

    // 2. Combine Standard Properties (Direct overrides Style)
    const combinedProps = new Map();
    styleProperties.forEach(prop => prop && combinedProps.set(prop.propertyId, prop)); // Add null check for prop
    directProperties.forEach(prop => prop && combinedProps.set(prop.propertyId, prop)); // Add null check for prop

    // 3. Apply Standard Properties as CSS
    const useExtendedColor = !!(KrbDoc.header.flags & C.FLAG_EXTENDED_COLOR);
    const useFixedPoint = !!(KrbDoc.header.flags & C.FLAG_FIXED_POINT);

    combinedProps.forEach(prop => {
        if (!prop) return; // Skip if prop was null

        // Check if value exists (parsed simple value) or rawValue exists
        if (prop.value === null && prop.rawValue === null && prop.size > 0) {
             // Log only if size was > 0 but value/rawValue are still null (parsing error?)
             console.warn(`Property ID 0x${prop.propertyId.toString(16)} (size ${prop.size}) has null value/rawValue.`);
             return;
        }
        if (prop.size === 0 && prop.propertyId !== C.PROP_ID_TEXT_CONTENT) {
             // Allow size 0 for text content reset? Otherwise skip size 0 props.
             return;
        }

        try {
            switch (prop.propertyId) {
                case C.PROP_ID_BG_COLOR:
                    if (prop.valueType === C.VAL_TYPE_COLOR && prop.rawValue) {
                        domElement.style.backgroundColor = parseColor(prop.rawValue, useExtendedColor);
                    }
                    break;
                case C.PROP_ID_FG_COLOR: // Text Color
                    if (prop.valueType === C.VAL_TYPE_COLOR && prop.rawValue) {
                        domElement.style.color = parseColor(prop.rawValue, useExtendedColor);
                    }
                    break;
                case C.PROP_ID_BORDER_COLOR:
                     if (prop.valueType === C.VAL_TYPE_COLOR && prop.rawValue) {
                        domElement.style.borderColor = parseColor(prop.rawValue, useExtendedColor);
                        // Ensure border style is set if color is applied but width might be 0 or default
                        if (!domElement.style.borderWidth || domElement.style.borderWidth === "0px") {
                            domElement.style.borderStyle = 'solid'; // Need a style for color alone
                        }
                     }
                    break;
                case C.PROP_ID_BORDER_WIDTH:
                    // Need a border-style for width to show. Set solid by default if width > 0.
                    let appliedWidth = false;
                    if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null && prop.value >= 0) {
                        domElement.style.borderWidth = `${prop.value}px`;
                        appliedWidth = prop.value > 0;
                    } else if (prop.valueType === C.VAL_TYPE_EDGEINSETS && prop.rawValue) {
                        // Assuming EdgeInsets for border-width uses 4 bytes (u8) for T,R,B,L widths
                        const edges = parseEdgeInsets(prop.rawValue);
                        domElement.style.borderTopWidth = `${edges.top}px`;
                        domElement.style.borderRightWidth = `${edges.right}px`;
                        domElement.style.borderBottomWidth = `${edges.bottom}px`;
                        domElement.style.borderLeftWidth = `${edges.left}px`;
                        appliedWidth = edges.top > 0 || edges.right > 0 || edges.bottom > 0 || edges.left > 0;
                    }
                    if (appliedWidth) {
                        domElement.style.borderStyle = 'solid'; // Apply style only if width > 0
                    }
                    break;
                 case C.PROP_ID_BORDER_RADIUS:
                     if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null && prop.value >= 0) {
                         domElement.style.borderRadius = `${prop.value}px`;
                     } // TODO: Support EdgeInsets for different corners? VAL_TYPE_EDGEINSETS?
                     break;
                 case C.PROP_ID_PADDING:
                      if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null && prop.value >= 0) {
                        domElement.style.padding = `${prop.value}px`;
                      } else if (prop.valueType === C.VAL_TYPE_EDGEINSETS && prop.rawValue) {
                        const edges = parseEdgeInsets(prop.rawValue); // Assumes 4 bytes
                        domElement.style.paddingTop = `${edges.top}px`;
                        domElement.style.paddingRight = `${edges.right}px`;
                        domElement.style.paddingBottom = `${edges.bottom}px`;
                        domElement.style.paddingLeft = `${edges.left}px`;
                      }
                     break;
                 case C.PROP_ID_MARGIN:
                      if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null) { // Can be negative
                        domElement.style.margin = `${prop.value}px`;
                      } else if (prop.valueType === C.VAL_TYPE_EDGEINSETS && prop.rawValue) {
                        const edges = parseEdgeInsets(prop.rawValue); // Assumes 4 bytes
                        domElement.style.marginTop = `${edges.top}px`; // Consider signed shorts if needed
                        domElement.style.marginRight = `${edges.right}px`;
                        domElement.style.marginBottom = `${edges.bottom}px`;
                        domElement.style.marginLeft = `${edges.left}px`;
                      }
                     break;
                case C.PROP_ID_TEXT_CONTENT:
                    // Handles size 0 correctly (sets empty string)
                    if (prop.valueType === C.VAL_TYPE_STRING && prop.value !== null) {
                         if (prop.value < KrbDoc.strings.length) {
                            domElement.textContent = KrbDoc.strings[prop.value] || "";
                         } else {
                             console.warn(`Text content string index ${prop.value} out of bounds (${KrbDoc.strings.length}). Element index ${domElement.dataset.krbIndex}.`);
                             domElement.textContent = "[Invalid StrIdx]";
                         }
                    } else if (prop.size === 0) {
                         domElement.textContent = ""; // Explicitly clear if size is 0
                    }
                    break;
                case C.PROP_ID_IMAGE_SOURCE:
                    if (domElement.tagName === 'IMG' && prop.valueType === C.VAL_TYPE_RESOURCE && prop.value !== null) {
                        setImageSource(domElement, prop.value); // Use helper
                    }
                     break;
                 case C.PROP_ID_FONT_SIZE:
                     if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null && prop.value > 0) {
                         domElement.style.fontSize = `${prop.value}px`;
                     }
                     // TODO: Handle VAL_TYPE_PERCENTAGE ? (Relative to parent font size) Needs parent computed style.
                     break;
                 case C.PROP_ID_FONT_WEIGHT:
                      if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                         // Spec doesn't define enum, assume 100-900 / 100
                         const weightMap = { 1: 100, 2: 200, 3: 300, 4: 400, 5: 500, 6: 600, 7: 700, 8: 800, 9: 900 };
                         domElement.style.fontWeight = weightMap[prop.value] || 'normal'; // Map enum or default
                      } else if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null) {
                         // Allow direct CSS numeric weight (100-900)
                         domElement.style.fontWeight = prop.value;
                      }
                     break;
                case C.PROP_ID_TEXT_ALIGNMENT: // Enum: 0=Left, 1=Center, 2=Right, 3=Justify?
                    if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                        // Align with common flexbox terminology slightly
                        switch (prop.value) {
                            case 0: domElement.style.textAlign = 'left'; break; // Start
                            case 1: domElement.style.textAlign = 'center'; break; // Center
                            case 2: domElement.style.textAlign = 'right'; break; // End
                            case 3: domElement.style.textAlign = 'justify'; break; // Justify
                            default: domElement.style.textAlign = 'left'; break; // Default
                        }
                    }
                    break;
                 case C.PROP_ID_OPACITY: // Use Percentage (8.8 fixed point -> 0.0 to 1.0)
                     if (prop.valueType === C.VAL_TYPE_PERCENTAGE && useFixedPoint && prop.value !== null) {
                         domElement.style.opacity = parseFixedPointPercentage(prop.value);
                     } else if (prop.valueType === C.VAL_TYPE_BYTE && prop.value !== null) { // Allow 0-255 byte? Convert to 0-1
                         domElement.style.opacity = (prop.value / 255.0).toFixed(3);
                     }
                     break;
                 case C.PROP_ID_ZINDEX:
                     if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null) {
                         domElement.style.zIndex = prop.value;
                     }
                     break;
                 case C.PROP_ID_VISIBILITY: // Enum: 0=visible, 1=hidden, 2=collapse?
                     if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                         if (prop.value === 1) { // Hidden
                              domElement.style.visibility = 'hidden';
                              domElement.style.display = ''; // Keep layout space
                         } else if (prop.value === 2) { // Collapse (like display: none)
                              domElement.style.visibility = 'hidden'; // Also hide
                              domElement.style.display = 'none'; // Remove from layout
                         } else { // Visible (default 0)
                              domElement.style.visibility = 'visible';
                              domElement.style.display = ''; // Ensure it's not display:none
                         }
                     }
                     break;
                case C.PROP_ID_CUSTOM_DATA_BLOB: // 0x19
                    console.log(`Element ${domElement.dataset.krbIndex} has PROP_ID_CUSTOM_DATA_BLOB (size ${prop.rawValue?.byteLength}), adding as data-custom-blob attribute (hex).`);
                    if (prop.rawValue) {
                         domElement.setAttribute('data-custom-blob', arrayBufferToHex(prop.rawValue));
                    }
                    break;
                 case C.PROP_ID_GAP: // Applied in applyLayout
                 case C.PROP_ID_LAYOUT_FLAGS: // Handled by compiler
                 // Ignore App specific props here, handled in renderKrb root
                 case C.PROP_ID_WINDOW_WIDTH:
                 case C.PROP_ID_WINDOW_HEIGHT:
                 case C.PROP_ID_WINDOW_TITLE:
                 case C.PROP_ID_RESIZABLE:
                 case C.PROP_ID_KEEP_ASPECT:
                 case C.PROP_ID_SCALE_FACTOR:
                 case C.PROP_ID_ICON:
                 case C.PROP_ID_VERSION:
                 case C.PROP_ID_AUTHOR:
                    break;
                // --- TODO: Add more standard property mappings ---
                case C.PROP_ID_MIN_WIDTH:
                     if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null && prop.value >= 0) {
                         domElement.style.minWidth = `${prop.value}px`;
                     }
                    break;
                case C.PROP_ID_MIN_HEIGHT:
                      if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null && prop.value >= 0) {
                         domElement.style.minHeight = `${prop.value}px`;
                     }
                    break;
                case C.PROP_ID_MAX_WIDTH:
                      if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null && prop.value >= 0) {
                         domElement.style.maxWidth = `${prop.value}px`;
                     }
                    break;
                case C.PROP_ID_MAX_HEIGHT:
                     if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null && prop.value >= 0) {
                         domElement.style.maxHeight = `${prop.value}px`;
                     }
                    break;
                 case C.PROP_ID_OVERFLOW: // Enum: 0=Visible, 1=Hidden, 2=Scroll, 3=Auto?
                     if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                         switch (prop.value) {
                             case 0: domElement.style.overflow = 'visible'; break;
                             case 1: domElement.style.overflow = 'hidden'; break;
                             case 2: domElement.style.overflow = 'scroll'; break;
                             case 3: domElement.style.overflow = 'auto'; break;
                             default: domElement.style.overflow = 'visible'; break;
                         }
                     }
                     break;
                 // case C.PROP_ID_ASPECT_RATIO: // Needs aspect-ratio CSS property
                 // case C.PROP_ID_TRANSFORM: // Complex, might need VAL_TYPE_MATRIX or string
                 // case C.PROP_ID_SHADOW: // Complex

                default:
                     console.warn(`Unhandled standard property ID 0x${prop.propertyId.toString(16)} on element index ${domElement.dataset.krbIndex}`);
                     break;
            }
        } catch (e) {
             console.error(`Error applying standard property ID 0x${prop.propertyId.toString(16)}:`, e, prop);
        }
    });

     // 4. Apply Element Header Width/Height (Use direct style for initial size)
     // Flex/Grid might override this based on grow/shrink/basis/alignment.
     // Use style.width/height directly. min/max props handle constraints.
     if (header.width > 0) domElement.style.width = `${header.width}px`;
     else domElement.style.width = ''; // Explicitly clear if 0? Or let CSS/content decide? Let CSS decide.
     if (header.height > 0) domElement.style.height = `${header.height}px`;
     else domElement.style.height = '';
     // Ensure block or inline-block display for width/height to take effect on spans/etc.
     // Do this *after* applying layout which sets display:flex/grid etc.
     const currentDisplay = window.getComputedStyle(domElement).display;
     if (currentDisplay === 'inline' && (header.width > 0 || header.height > 0)) {
        // This should rarely happen if layout sets display, but as a fallback.
        console.warn(`Element ${domElement.dataset.krbIndex} has width/height but computed display is inline. Setting inline-block.`);
        domElement.style.display = 'inline-block';
     }

     // 5. Apply Layout Flags (Positioning and Flex/Grid container settings)
     applyLayout(domElement, header.layout, combinedProps); // Pass combinedProps for Gap

     // 6. Apply Custom Properties as data-* attributes (Using v0.3 structure)
     if (customProperties.length > 0) {
         applyCustomPropertiesAsDataAttributes(domElement, customProperties, useFixedPoint);
     }

}


/** Helper to set Image source from Resource Index */
function setImageSource(domElement, resourceIndex) {
     if (resourceIndex < 0 || resourceIndex >= KrbDoc.resources.length) {
         console.warn(`Invalid resource index ${resourceIndex} for image source. Max index: ${KrbDoc.resources.length - 1}.`);
         domElement.src = ""; domElement.alt = "[Invalid ResIdx]";
         return;
     }

     const resource = KrbDoc.resources[resourceIndex];
     const altText = KrbDoc.strings[resource.nameIndex] || `Resource ${resourceIndex}`;
     domElement.alt = altText;

     if (resource.format === C.RES_FORMAT_EXTERNAL) {
         if (resource.dataStringIndex < 0 || resource.dataStringIndex >= KrbDoc.strings.length) {
              console.warn(`Cannot set image source: Resource ${resourceIndex} external path string index ${resource.dataStringIndex} is invalid.`);
              domElement.src = ""; domElement.alt += " (Invalid PathIdx)";
         } else {
             const imagePath = KrbDoc.strings[resource.dataStringIndex];
             // IMPORTANT: Assume path is relative TO THE EXECUTING HTML FILE (in dist/)
             // OR relative to the location of the *copied* KRB file (dist/examples/).
             // Let's assume relative to the HTML file for simplicity unless KRB files
             // expect paths relative to themselves. If KRB expects relative paths,
             // we need to construct the URL based on the KRB file location.
             // Example: If KRB is dist/examples/ui.krb and path is "img.png",
             //          URL should be "examples/img.png".
             // For now, treat path as relative to HTML:
             console.log(`Setting external image src: "${imagePath}" (relative to HTML)`);
             domElement.src = imagePath;
         }
     } else if (resource.format === C.RES_FORMAT_INLINE && resource.inlineData) {
         let mimeType = 'application/octet-stream';
         if (resource.type === C.RES_TYPE_IMAGE) {
              const nameStr = KrbDoc.strings[resource.nameIndex]?.toLowerCase();
              if (nameStr?.endsWith('.png')) mimeType = 'image/png';
              else if (nameStr?.endsWith('.jpg') || nameStr?.endsWith('.jpeg')) mimeType = 'image/jpeg';
              else if (nameStr?.endsWith('.gif')) mimeType = 'image/gif';
              else if (nameStr?.endsWith('.webp')) mimeType = 'image/webp';
              else if (nameStr?.endsWith('.svg') || nameStr?.endsWith('.svg+xml')) mimeType = 'image/svg+xml'; // Correct svg mime
              else mimeType = 'image/png'; // Default guess
         } // Add other RES_TYPEs like font/woff2 if needed

         try {
             const blob = new Blob([resource.inlineData], { type: mimeType });
             // Revoke previous blob URL if element already has one (simple cleanup)
             if (domElement.src && domElement.src.startsWith('blob:')) {
                  URL.revokeObjectURL(domElement.src);
             }
             domElement.src = URL.createObjectURL(blob);
         } catch (blobError) {
              console.error(`Error creating blob URL for inline resource ${resourceIndex}:`, blobError);
              domElement.src = ""; domElement.alt += " (Blob Error)";
         }
     } else {
         console.warn(`Cannot set image source: Resource ${resourceIndex} format (${resource.format}) not supported or data missing.`);
         domElement.src = ""; domElement.alt += " (Unsupported Format)";
     }
}


/**
 * Applies layout-related CSS based on the Layout byte and Gap property.
 * @param {HTMLElement} domElement
 * @param {number} layoutByte
 * @param {Map<number, object>} combinedProps - Map of standard properties applied to the element
 */
function applyLayout(domElement, layoutByte, combinedProps) {
     const isAbsolute = !!(layoutByte & C.LAYOUT_ABSOLUTE_BIT); // Bit 6

     // Reset potentially conflicting styles
     domElement.style.position = '';
     domElement.style.display = '';
     domElement.style.flexDirection = '';
     domElement.style.flexWrap = '';
     domElement.style.justifyContent = '';
     domElement.style.alignItems = '';
     domElement.style.gap = '';
     domElement.style.flexGrow = '';
     domElement.style.flexShrink = '';
     domElement.style.flexBasis = '';
     domElement.style.left = '';
     domElement.style.top = '';
     // Keep width/height set from header/props

     if (isAbsolute) {
         domElement.style.position = 'absolute';
         // posX/posY from header are applied relative to parent during recursion (see renderNextElementRecursive)
     } else {
         // --- Settings for when this element is a flex CONTAINER ---
         domElement.style.position = 'relative'; // Default for flow layout, allows z-index etc.
         domElement.style.display = 'flex'; // KRB primarily uses flex for layout

         const direction = layoutByte & C.LAYOUT_DIRECTION_MASK; // Bits 0-1
         switch (direction) {
             case 0x00: domElement.style.flexDirection = 'row'; break; // Row
             case 0x01: domElement.style.flexDirection = 'column'; break; // Column
             case 0x02: domElement.style.flexDirection = 'row-reverse'; break; // RowReverse
             case 0x03: domElement.style.flexDirection = 'column-reverse'; break; // ColumnReverse
         }

         const alignment = (layoutByte & C.LAYOUT_ALIGNMENT_MASK) >> 2; // Bits 2-3
         // CSS align-items (cross axis) / justify-content (main axis)
         let alignItemsVal = 'stretch'; // Default cross-axis is stretch
         let justifyContentVal = 'flex-start'; // Default main-axis is start

         switch (alignment) {
             case 0x00: // Start
                 justifyContentVal = 'flex-start';
                 // alignItemsVal default stretch is fine
                 break;
             case 0x01: // Center
                 justifyContentVal = 'center';
                 alignItemsVal = 'center'; // Center on both axes
                 break;
             case 0x02: // End
                 justifyContentVal = 'flex-end';
                 // alignItemsVal default stretch is fine, or maybe flex-end? Spec is ambiguous, stick to stretch.
                 break;
             case 0x03: // SpaceBetween
                 justifyContentVal = 'space-between';
                 // alignItemsVal default stretch is fine
                 break;
         }
         domElement.style.alignItems = alignItemsVal;
         domElement.style.justifyContent = justifyContentVal;

         domElement.style.flexWrap = (layoutByte & C.LAYOUT_WRAP_BIT) ? 'wrap' : 'nowrap'; // Bit 4

         // Apply Gap property if present on this element (for its children)
         if (combinedProps.has(C.PROP_ID_GAP)) {
             const gapProp = combinedProps.get(C.PROP_ID_GAP);
             if ((gapProp.valueType === C.VAL_TYPE_BYTE || gapProp.valueType === C.VAL_TYPE_SHORT) && gapProp.value !== null && gapProp.value >= 0) {
                domElement.style.gap = `${gapProp.value}px`;
             }
         }

         // --- Settings for when this element is a flex ITEM ---
         const grow = !!(layoutByte & C.LAYOUT_GROW_BIT); // Bit 5
         domElement.style.flexGrow = grow ? '1' : '0';
         // Let items shrink by default if needed, unless they have fixed size?
         domElement.style.flexShrink = '1'; // Allow shrinking
         // Setting basis to 'auto' respects width/height initially
         domElement.style.flexBasis = 'auto';
         // Consider if grow=1 should imply basis=0? flex: 1 1 0 vs flex: 1 1 auto
         // 'auto' is generally safer if width/height are set.
     }
}

/**
 * Applies Custom Properties from KRB as data-* attributes.
 * @param {HTMLElement} domElement
 * @param {Array} customProperties - Array of parsed custom property objects
 * @param {boolean} useFixedPoint - For formatting percentage values
 */
function applyCustomPropertiesAsDataAttributes(domElement, customProperties, useFixedPoint) {
     customProperties.forEach((prop, index) => {
         if (!prop) return; // Skip null props

         let keyString = `custom-prop-${index}`; // Default key if index is bad
         let rawKey = `[Invalid Idx ${prop.keyIndex}]`;
         if (prop.keyIndex >= 0 && prop.keyIndex < KrbDoc.strings.length) {
             rawKey = KrbDoc.strings[prop.keyIndex];
             if (rawKey) {
                 // Sanitize key for data-* attribute: lowercase, replace invalid chars with hyphen, trim hyphens
                 keyString = rawKey.toLowerCase()
                                   .replace(/[^a-z0-9_.:-]+/g, '-') // Allow alphanum, _, ., : , -
                                   .replace(/^-+|-+$/g, '');
                 if (!keyString) keyString = `custom-prop-${index}-${prop.keyIndex}`; // Handle edge case of only invalid chars
             } else {
                  rawKey = "[Empty String]";
                  keyString = `custom-prop-${index}-${prop.keyIndex}-empty`;
             }
         } else {
             console.warn(`Custom property key index ${prop.keyIndex} out of bounds (0-${KrbDoc.strings.length - 1}).`);
         }

         let valueString = `[Type 0x${prop.valueType?.toString(16)} Size ${prop.size}]`; // Default if no value parsed
         if (prop.value !== null) {
             // Use parsed value if available and simple type
             if (typeof prop.value === 'number' || typeof prop.value === 'string') {
                 valueString = String(prop.value);
             } else if (prop.value instanceof ArrayBuffer) {
                 // For raw buffers, use hex
                 valueString = "0x" + arrayBufferToHex(prop.value);
             }
             // Handle specific type formatting
             if (prop.valueType === C.VAL_TYPE_PERCENTAGE && useFixedPoint && typeof prop.value === 'number') {
                 valueString = parseFixedPointPercentage(prop.value); // Keep as number string "0.xxxx"
             } else if (prop.valueType === C.VAL_TYPE_COLOR && prop.rawValue) {
                  valueString = parseColor(prop.rawValue, !!(KrbDoc.header.flags & C.FLAG_EXTENDED_COLOR));
             }
         } else if (prop.rawValue) {
             // Fallback to hex if only raw value exists
             valueString = "0x" + arrayBufferToHex(prop.rawValue);
         } else if (prop.size === 0) {
              valueString = "[Size 0]"; // Indicate value is explicitly empty/default
         }

         try {
            // Set the attribute, potentially overwriting if key sanitized to same value
            domElement.setAttribute(`data-${keyString}`, valueString);
         } catch (attrError) {
             console.error(`Error setting custom attribute data-${keyString}="${valueString}" (Raw Key: "${rawKey}"):`, attrError);
         }
     });
}


// --- Recursive Rendering ---
let currentElementIndex = 0; // Global index tracker during recursive render

/**
 * Recursively renders KRB elements into DOM elements.
 * @param {HTMLElement} parentDomElement - The DOM element to append the new element to.
 * @returns {HTMLElement | null} The created DOM element, or null if error/end.
 */
function renderNextElementRecursive(parentDomElement) {
    if (!KrbDoc || currentElementIndex >= KrbDoc.elements.length) {
        return null; // No more elements or doc not loaded
    }

    const elementData = KrbDoc.elements[currentElementIndex];
    if (!elementData || !elementData.header) {
         console.error(`Invalid element data at index ${currentElementIndex}`);
         currentElementIndex++; // Skip invalid element
         return null;
    }

    const header = elementData.header;
    const currentIndex = currentElementIndex; // Capture index before incrementing
    currentElementIndex++; // Consume this element

    // 1. Create DOM Element based on Type
    let domElement;
    let defaultDisplayStyle = ''; // Track if we need to override default display later

    switch (header.type) {
        case C.ELEM_TYPE_CONTAINER:
        case C.ELEM_TYPE_APP: // Treat App like a root container
        case C.ELEM_TYPE_LIST: // Style as column flex container via Layout byte
        case C.ELEM_TYPE_GRID: // Style as grid via Layout logic? (applyLayout sets display:flex now)
        case C.ELEM_TYPE_SCROLLABLE: // Needs overflow CSS property
             domElement = document.createElement('div');
             // Layout byte will set display:flex usually. Override for grid/scroll:
             if (header.type === C.ELEM_TYPE_GRID) domElement.style.display = 'grid';
             if (header.type === C.ELEM_TYPE_SCROLLABLE) domElement.style.overflow = 'auto';
            break;
        case C.ELEM_TYPE_TEXT:
            domElement = document.createElement('span'); // Use span, let layout/styles control block behavior
            defaultDisplayStyle = 'inline'; // Default span display
            break;
        case C.ELEM_TYPE_IMAGE:
            domElement = document.createElement('img');
             domElement.style.display = 'block'; // Common reset for images
            break;
        case C.ELEM_TYPE_BUTTON:
            domElement = document.createElement('button');
            break;
        case C.ELEM_TYPE_INPUT:
            domElement = document.createElement('input');
            // TODO: Handle input types (text, number, password etc.) via standard/custom properties?
            break;
        case C.ELEM_TYPE_CANVAS:
            domElement = document.createElement('canvas');
            domElement.style.display = 'block'; // Usually needs block display
            break;
         case C.ELEM_TYPE_VIDEO:
            domElement = document.createElement('video');
            domElement.style.display = 'block';
            // TODO: Handle video source, controls etc. via properties
            break;
        default: // Handle Custom Element Types (0x31 - 0xFF)
            let idStrCustom = "[No ID]";
            let customTypeName = `Custom Type 0x${header.type.toString(16)}`;
            if (header.idIndex > 0 && header.idIndex < KrbDoc.strings.length) {
                idStrCustom = KrbDoc.strings[header.idIndex] || "[Empty ID]";
                // Convention: Use ID string as the custom type name if present
                customTypeName = idStrCustom;
            } else if (header.idIndex > 0) {
                idStrCustom = "[Invalid ID Index]";
            }
             console.warn(`Rendering custom element type 0x${header.type.toString(16)} (Name/ID: '${customTypeName}') as div.`);
            domElement = document.createElement('div');
             domElement.style.border = '1px dashed orange';
             domElement.style.padding = '4px';
             domElement.style.margin = '2px';
             domElement.textContent = customTypeName; // Display name/ID
             domElement.setAttribute('data-krb-custom-type-hex', `0x${header.type.toString(16)}`);
             domElement.setAttribute('data-krb-custom-type-name', customTypeName);
    }

     // Add common debug attributes
     domElement.setAttribute('data-krb-index', currentIndex);
     domElement.setAttribute('data-krb-type-hex', `0x${header.type.toString(16)}`);

    // 2. Set Element ID Attribute (if defined)
    setElementId(domElement, header.idIndex, currentIndex);

    // 3. Apply Styling and Properties (Standard and Custom data-*)
    applyStyling(domElement, elementData);

    // 4. Apply Absolute Position Offset (if applicable, check computed style)
    // Must happen *after* applyStyling sets position:absolute
     if (window.getComputedStyle(domElement).position === 'absolute') {
         domElement.style.left = `${header.posX}px`;
         domElement.style.top = `${header.posY}px`;
     }

    // 5. Attach Event Listeners
    attachEventListeners(domElement, elementData.events);

    // 6. Append to Parent DOM
    parentDomElement.appendChild(domElement);

    // 7. Render Children Recursively
    const expectedEndIndex = currentElementIndex + header.childCount;
    if (expectedEndIndex > KrbDoc.elements.length) {
         console.error(`Element ${currentIndex} declares ${header.childCount} children, but only ${KrbDoc.elements.length - currentElementIndex} elements remain in the list.`);
    }
    for (let i = 0; i < header.childCount; i++) {
        // Check if we've run out of elements prematurely
        if (currentElementIndex >= KrbDoc.elements.length) {
             console.error(`Reached end of elements list while rendering child ${i + 1} of ${header.childCount} for element ${currentIndex}.`);
             break;
        }
        renderNextElementRecursive(domElement); // Children are appended to the current element
    }

    return domElement;
}

/** Helper to set DOM Element ID safely */
function setElementId(domElement, idIndex, elementIndex) {
     if (idIndex > 0) { // ID 0 means no ID defined
        if (idIndex < KrbDoc.strings.length) {
            const idStr = KrbDoc.strings[idIndex];
            if (idStr) {
                 // Basic validation for HTML ID (starts with letter, contains letters, digits, hyphens, underscores, etc.)
                 // This is a simplified check.
                 if (/^[a-zA-Z][a-zA-Z0-9-_:.]*$/.test(idStr)) {
                     domElement.id = idStr;
                 } else {
                     console.warn(`Element index ${elementIndex}: Invalid HTML ID string "${idStr}" (from string index ${idIndex}). Using data-krb-id instead.`);
                      domElement.setAttribute('data-krb-id', idStr); // Store invalid ID safely
                 }
            } // else: ID string index resolved to null/empty, do nothing
        } else {
             console.warn(`Element index ${elementIndex}: ID string index ${idIndex} is out of bounds (${KrbDoc.strings.length}).`);
        }
    }
}

/** Helper to attach event listeners */
function attachEventListeners(domElement, events) {
     if (!events || events.length === 0) return;

     events.forEach(eventInfo => {
        const domEventName = mapKrbEventToDomEvent(eventInfo.eventType);
        if (domEventName) {
            const callback = findCallback(eventInfo.callbackIdIndex);
            domElement.addEventListener(domEventName, callback);

            // Add corresponding 'leave' event for hover
            if (eventInfo.eventType === C.EVENT_TYPE_HOVER) {
                 domElement.addEventListener('pointerleave', callback); // Use same callback, it checks event.type
            }
             // TODO: Implement LongPress logic (requires state tracking on pointerdown/up/leave)
        } else if (eventInfo.eventType === C.EVENT_TYPE_CUSTOM) {
             // Handle custom events - maybe use callback name as event name?
             const callbackName = KrbDoc.strings[eventInfo.callbackIdIndex];
             if (callbackName) {
                console.warn(`Attaching custom event listener "${callbackName}" is not fully implemented. Requires runtime dispatch.`);
                // Example: Add attribute for potential later use
                domElement.setAttribute(`data-krb-event-custom-${callbackName}`, eventInfo.callbackIdIndex);
             }
        }
    });
}


// --- Public Renderer Function ---

/**
 * Renders a parsed KRB document into the target DOM element.
 * @param {object} parsedDocument The document object from KrbParser.
 * @param {HTMLElement} targetContainer The DOM element to render into.
 */
export function renderKrb(parsedDocument, targetContainer) {
    // Validate input
    if (!parsedDocument || !parsedDocument.header || !parsedDocument.elements) {
        console.error("Invalid or incomplete KRB document provided to renderKrb.");
        if (targetContainer) targetContainer.innerHTML = '<p style="color: red;">Invalid KRB data.</p>';
        return;
    }
     if (!targetContainer || !(targetContainer instanceof HTMLElement)) {
         console.error("Invalid target container provided to renderKrb.");
         return;
     }

    KrbDoc = parsedDocument; // Make doc globally accessible for helpers during this render pass
    targetContainer.innerHTML = ''; // Clear previous content
    // Ensure target container can contain positioned elements correctly
    if (window.getComputedStyle(targetContainer).position === 'static') {
         targetContainer.style.position = 'relative';
    }

    // --- Handle App Element Properties (Optional Root Styling) ---
    let startingElementIndex = 0;
    document.title = "Kryon App"; // Default title

    if ((KrbDoc.header.flags & C.FLAG_HAS_APP) && KrbDoc.elements.length > 0) {
         const firstElement = KrbDoc.elements[0];
         if (firstElement?.header?.type === C.ELEM_TYPE_APP) {
            startingElementIndex = 0; // Render the App element itself and its children
            console.log("App element found (index 0). Rendering hierarchy.");

            // Extract App properties to potentially style the root container or page
            const combinedProps = new Map();
            // Get style props first
            if (firstElement.header.styleId > 0 && KrbDoc.styles) {
                const style = KrbDoc.styles.find(s => s.id === firstElement.header.styleId);
                style?.properties?.forEach(prop => prop && combinedProps.set(prop.propertyId, prop));
            }
            // Direct props override style props
            firstElement.properties?.forEach(prop => prop && combinedProps.set(prop.propertyId, prop));

            // Apply specific App props
            if (combinedProps.has(C.PROP_ID_WINDOW_TITLE)) {
                 const titleProp = combinedProps.get(C.PROP_ID_WINDOW_TITLE);
                 if (titleProp.valueType === C.VAL_TYPE_STRING && titleProp.value !== null && titleProp.value < KrbDoc.strings.length) {
                     document.title = KrbDoc.strings[titleProp.value] || document.title;
                 }
             }
            if (combinedProps.has(C.PROP_ID_BG_COLOR)) { // Apply App BG to target container?
                 const bgProp = combinedProps.get(C.PROP_ID_BG_COLOR);
                 const useExt = !!(KrbDoc.header.flags & C.FLAG_EXTENDED_COLOR);
                  if (bgProp.rawValue) targetContainer.style.backgroundColor = parseColor(bgProp.rawValue, useExt);
             }
             // Apply Window Width/Height to target container?
             if (combinedProps.has(C.PROP_ID_WINDOW_WIDTH)) {
                  const widthProp = combinedProps.get(C.PROP_ID_WINDOW_WIDTH);
                  if ((widthProp.valueType === C.VAL_TYPE_SHORT || widthProp.valueType === C.VAL_TYPE_BYTE) && widthProp.value > 0) {
                      targetContainer.style.width = `${widthProp.value}px`;
                      targetContainer.style.maxWidth = `${widthProp.value}px`; // Prevent flex growing beyond?
                  }
             }
             if (combinedProps.has(C.PROP_ID_WINDOW_HEIGHT)) {
                  const heightProp = combinedProps.get(C.PROP_ID_WINDOW_HEIGHT);
                   if ((heightProp.valueType === C.VAL_TYPE_SHORT || heightProp.valueType === C.VAL_TYPE_BYTE) && heightProp.value > 0) {
                      targetContainer.style.height = `${heightProp.value}px`;
                  }
             }
         } else {
             console.warn("FLAG_HAS_APP is set, but the first element is not ELEM_TYPE_APP. Rendering all elements as potential roots.");
             startingElementIndex = 0; // Still start from 0
         }
    } else {
         console.log("No <App> element flagged. Rendering all elements as potential roots.");
         startingElementIndex = 0;
    }


    console.log("Starting KRB Rendering...");
    currentElementIndex = startingElementIndex; // Initialize global index

    // Render all top-level elements from the starting index
    // The recursive function handles advancing the index and rendering children.
    while(currentElementIndex < KrbDoc.elements.length) {
        // This loop ensures multiple root-level elements are rendered into the targetContainer.
        renderNextElementRecursive(targetContainer);
    }

     // Cleanup global reference
     KrbDoc = null;
     console.log("KRB Rendering Finished.");
}