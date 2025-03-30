import * as C from './krb-constants.js';

// --- Global KRB Document Data (set by render function) ---
let KrbDoc = null;

// --- Event Handling Setup ---
// Simple mechanism: Map callback ID (string index) to global function name
function findCallback(callbackIdIndex) {
    if (!KrbDoc || callbackIdIndex >= KrbDoc.strings.length) {
        console.warn(`Invalid callback ID index: ${callbackIdIndex}`);
        return () => console.warn(`Callback function for index ${callbackIdIndex} not found.`);
    }
    const functionName = KrbDoc.strings[callbackIdIndex];
    if (typeof window[functionName] === 'function') {
        return window[functionName];
    } else {
        console.warn(`Global callback function "${functionName}" (index ${callbackIdIndex}) not found or not a function.`);
        // Return a dummy function to avoid errors, but log the issue.
        return (event) => console.warn(`Callback function "${functionName}" (index ${callbackIdIndex}) not found. Event:`, event);
    }
}

// Map KRB event types to DOM event names
function mapKrbEventToDomEvent(krbEventType) {
    switch (krbEventType) {
        case C.EVENT_TYPE_CLICK: return 'click';
        case C.EVENT_TYPE_PRESS: return 'pointerdown'; // Use pointer events for broader compatibility
        case C.EVENT_TYPE_RELEASE: return 'pointerup';
        case C.EVENT_TYPE_HOVER: return 'pointerenter'; // Use enter/leave for hover start/end
        // TODO: Need separate handling for hover end (mouseleave/pointerleave) if KRB implies that
        case C.EVENT_TYPE_FOCUS: return 'focus';
        case C.EVENT_TYPE_BLUR: return 'blur';
        case C.EVENT_TYPE_CHANGE: return 'change';
        case C.EVENT_TYPE_SUBMIT: return 'submit';
        // case C.EVENT_TYPE_LONGPRESS: // Needs custom JS logic (setTimeout on pointerdown)
        // case C.EVENT_TYPE_CUSTOM: // Maybe use custom event names?
        default:
            console.warn(`Unsupported KRB event type: ${krbEventType}`);
            return null;
    }
}


// --- Style and Property Helpers ---

function parseColor(rawValueBytes, useExtendedColor) {
    if (!rawValueBytes) return 'inherit'; // Default or inherit?
    const view = new DataView(rawValueBytes);
    if (useExtendedColor) {
        // 4-byte RGBA
        if (view.byteLength < 4) return 'red'; // Error color
        const r = view.getUint8(0);
        const g = view.getUint8(1);
        const b = view.getUint8(2);
        const a = view.getUint8(3) / 255.0; // CSS alpha is 0-1
        return `rgba(${r}, ${g}, ${b}, ${a.toFixed(3)})`;
    } else {
        // 1-byte palette index - NEEDS PALETTE DEFINITION
        if (view.byteLength < 1) return 'red'; // Error color
        const index = view.getUint8(0);
        // TODO: Implement palette lookup
        console.warn(`Palette color index ${index} requested, but palette is not defined.`);
        // Fallback: generate a grayscale color based on index
        const shade = Math.min(255, index * 16); // Simple fallback
        return `rgb(${shade}, ${shade}, ${shade})`;
    }
}

function parseFixedPointPercentage(rawValue) {
    // rawValue is a u16 (8.8 fixed point)
    return (rawValue / 256.0).toFixed(4); // Keep some precision
}

function parseEdgeInsets(rawValueBytes) {
     if (!rawValueBytes || rawValueBytes.byteLength < 4) return { top: 0, right: 0, bottom: 0, left: 0 };
     const view = new DataView(rawValueBytes);
     return {
         top: view.getUint8(0),
         right: view.getUint8(1),
         bottom: view.getUint8(2),
         left: view.getUint8(3),
     };
}


/**
 * Applies styles and properties to a DOM element.
 * Direct properties override style properties.
 * @param {HTMLElement} domElement The target DOM element.
 * @param {object} elementData Parsed data for the KRB element.
 */
function applyStyling(domElement, elementData) {
    const header = elementData.header;
    const directProperties = elementData.properties;
    const styleProperties = [];
    const inheritedStyles = {}; // Track styles for children

    // 1. Get Style Properties (if applicable)
    if (header.styleId > 0 && KrbDoc.styles) {
        // Style ID is 1-based, array is 0-based
        const style = KrbDoc.styles.find(s => s.id === header.styleId);
        if (style && style.properties) {
            style.properties.forEach(prop => styleProperties.push(prop));
        } else {
            console.warn(`Style ID ${header.styleId} not found or has no properties.`);
        }
    }

    // 2. Combine Properties (Direct overrides Style)
    const combinedProps = new Map(); // Use a Map to handle overrides easily
    styleProperties.forEach(prop => combinedProps.set(prop.propertyId, prop));
    directProperties.forEach(prop => combinedProps.set(prop.propertyId, prop));

    // 3. Apply Properties as CSS
    const useExtendedColor = !!(KrbDoc.header.flags & C.FLAG_EXTENDED_COLOR);
    const useFixedPoint = !!(KrbDoc.header.flags & C.FLAG_FIXED_POINT);

    combinedProps.forEach(prop => {
        if (!prop.value && prop.propertyId !== C.PROP_ID_TEXT_CONTENT && prop.propertyId !== C.PROP_ID_IMAGE_SOURCE) { // Allow null value for text/image source reset?
             // console.warn(`Property ID 0x${prop.propertyId.toString(16)} has no value.`);
             // return;
        }

        try {
            switch (prop.propertyId) {
                case C.PROP_ID_BG_COLOR:
                    if (prop.valueType === C.VAL_TYPE_COLOR) {
                        domElement.style.backgroundColor = parseColor(prop.rawValue, useExtendedColor);
                        inheritedStyles.backgroundColor = domElement.style.backgroundColor; // Track for inheritance? Maybe not needed with CSS.
                    }
                    break;
                case C.PROP_ID_FG_COLOR:
                    if (prop.valueType === C.VAL_TYPE_COLOR) {
                        domElement.style.color = parseColor(prop.rawValue, useExtendedColor);
                        inheritedStyles.color = domElement.style.color;
                    }
                    break;
                case C.PROP_ID_BORDER_COLOR:
                     if (prop.valueType === C.VAL_TYPE_COLOR) {
                        domElement.style.borderColor = parseColor(prop.rawValue, useExtendedColor);
                     }
                    break;
                case C.PROP_ID_BORDER_WIDTH:
                    domElement.style.borderStyle = 'solid'; // Need a style for width to show
                    if (prop.valueType === C.VAL_TYPE_BYTE && prop.value !== null) {
                        domElement.style.borderWidth = `${prop.value}px`;
                    } else if (prop.valueType === C.VAL_TYPE_EDGEINSETS && prop.rawValue) {
                        const edges = parseEdgeInsets(prop.rawValue);
                        domElement.style.borderTopWidth = `${edges.top}px`;
                        domElement.style.borderRightWidth = `${edges.right}px`;
                        domElement.style.borderBottomWidth = `${edges.bottom}px`;
                        domElement.style.borderLeftWidth = `${edges.left}px`;
                    }
                    break;
                 case C.PROP_ID_BORDER_RADIUS: // TODO: Support different corner radii if needed
                     if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null) {
                         domElement.style.borderRadius = `${prop.value}px`;
                     }
                     break;
                 case C.PROP_ID_PADDING:
                      if (prop.valueType === C.VAL_TYPE_BYTE && prop.value !== null) {
                        domElement.style.padding = `${prop.value}px`;
                      } else if (prop.valueType === C.VAL_TYPE_EDGEINSETS && prop.rawValue) {
                        const edges = parseEdgeInsets(prop.rawValue);
                        domElement.style.paddingTop = `${edges.top}px`;
                        domElement.style.paddingRight = `${edges.right}px`;
                        domElement.style.paddingBottom = `${edges.bottom}px`;
                        domElement.style.paddingLeft = `${edges.left}px`;
                      }
                     break;
                 case C.PROP_ID_MARGIN:
                      if (prop.valueType === C.VAL_TYPE_BYTE && prop.value !== null) {
                        domElement.style.margin = `${prop.value}px`;
                      } else if (prop.valueType === C.VAL_TYPE_EDGEINSETS && prop.rawValue) {
                        const edges = parseEdgeInsets(prop.rawValue);
                        domElement.style.marginTop = `${edges.top}px`;
                        domElement.style.marginRight = `${edges.right}px`;
                        domElement.style.marginBottom = `${edges.bottom}px`;
                        domElement.style.marginLeft = `${edges.left}px`;
                      }
                     break;
                case C.PROP_ID_TEXT_CONTENT:
                    if (prop.valueType === C.VAL_TYPE_STRING && prop.value !== null && prop.value < KrbDoc.strings.length) {
                        domElement.textContent = KrbDoc.strings[prop.value] || "";
                    } else {
                        domElement.textContent = ""; // Clear if index invalid
                    }
                    break;
                case C.PROP_ID_IMAGE_SOURCE:
                    if (domElement.tagName === 'IMG' && prop.valueType === C.VAL_TYPE_RESOURCE && prop.value !== null) {
                        const resourceIndex = prop.value;
                        if (resourceIndex < KrbDoc.resources.length) {
                            const resource = KrbDoc.resources[resourceIndex];
                            if (resource.format === C.RES_FORMAT_EXTERNAL && resource.dataStringIndex < KrbDoc.strings.length) {
                                const imagePath = KrbDoc.strings[resource.dataStringIndex];
                                domElement.src = imagePath; // Assumes path is relative to HTML or absolute
                                domElement.alt = KrbDoc.strings[resource.nameIndex] || imagePath; // Use name as alt text
                            } else {
                                console.warn(`Cannot set image source: Resource ${resourceIndex} is not external or path index is invalid.`);
                                domElement.src = ""; // Clear src on error
                            }
                        } else {
                            console.warn(`Invalid resource index ${resourceIndex} for image source.`);
                             domElement.src = ""; // Clear src on error
                        }
                    }
                     break;
                 case C.PROP_ID_FONT_SIZE:
                     if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null) {
                         domElement.style.fontSize = `${prop.value}px`;
                     }
                     // TODO: Handle VAL_TYPE_PERCENTAGE ? (Relative to parent font size)
                     break;
                 case C.PROP_ID_FONT_WEIGHT: // Simplified: Assume 1 byte enum: 0=normal, 1=bold?
                      if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                         domElement.style.fontWeight = prop.value === 1 ? 'bold' : 'normal'; // Example mapping
                      }
                     break;
                case C.PROP_ID_TEXT_ALIGNMENT:
                    if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                        switch (prop.value) {
                            case 0: domElement.style.textAlign = 'left'; break;
                            case 1: domElement.style.textAlign = 'center'; break;
                            case 2: domElement.style.textAlign = 'right'; break;
                            default: domElement.style.textAlign = 'left'; break;
                        }
                    }
                    break;
                 case C.PROP_ID_OPACITY:
                     if (prop.valueType === C.VAL_TYPE_PERCENTAGE && useFixedPoint && prop.value !== null) {
                         domElement.style.opacity = parseFixedPointPercentage(prop.value);
                     } else if (prop.valueType === C.VAL_TYPE_BYTE && prop.value !== null) { // Allow 0-255 byte?
                         domElement.style.opacity = (prop.value / 255.0).toFixed(3);
                     }
                     break;
                 case C.PROP_ID_ZINDEX:
                     if (prop.valueType === C.VAL_TYPE_SHORT && prop.value !== null) {
                         domElement.style.zIndex = prop.value;
                     }
                     break;
                 case C.PROP_ID_VISIBILITY: // Assume 1 byte enum: 0=visible, 1=hidden, 2=collapse?
                     if (prop.valueType === C.VAL_TYPE_ENUM && prop.value !== null) {
                         if (prop.value === 1 || prop.value === 2) domElement.style.visibility = 'hidden';
                         // Note: 'collapse' behaviour is complex, often 'hidden' is sufficient fallback
                         else domElement.style.visibility = 'visible';
                     }
                     break;
                 case C.PROP_ID_GAP:
                     if ((prop.valueType === C.VAL_TYPE_BYTE || prop.valueType === C.VAL_TYPE_SHORT) && prop.value !== null) {
                         // Applied to parent flex/grid container, handled in layout section
                     }
                     break;
                // Add more property mappings here (Min/MaxWidth, Overflow, Transform etc.)
                // App specific properties are handled at the root level usually
            }
        } catch (e) {
             console.error(`Error applying property ID 0x${prop.propertyId.toString(16)}:`, e);
        }
    });

     // 4. Apply Element Header Width/Height (if specified)
     // Use min-width/height to allow content expansion unless overflow is handled
     if (header.width > 0) domElement.style.minWidth = `${header.width}px`;
     if (header.height > 0) domElement.style.minHeight = `${header.height}px`;
     // Override width/height explicitly if needed (e.g., for images/canvas fixed size)
     if (header.type === C.ELEM_TYPE_IMAGE || header.type === C.ELEM_TYPE_CANVAS) {
         if (header.width > 0) domElement.style.width = `${header.width}px`;
         if (header.height > 0) domElement.style.height = `${header.height}px`;
     }

     // 5. Apply Layout Flags (Positioning and Flex/Grid container settings)
     applyLayout(domElement, header.layout, combinedProps);

    return inheritedStyles;
}

/**
 * Applies layout-related CSS based on the Layout byte and Gap property.
 * @param {HTMLElement} domElement
 * @param {number} layoutByte
 * @param {Map<number, object>} combinedProps - Map of properties applied to the element
 */
function applyLayout(domElement, layoutByte, combinedProps) {
     const isAbsolute = !!(layoutByte & C.LAYOUT_ABSOLUTE_BIT);

     if (isAbsolute) {
         domElement.style.position = 'absolute';
         // posX/posY from header are applied relative to parent during recursion
     } else {
         // Default to flow layout (may become flex item)
         domElement.style.position = 'relative'; // Or static? Relative allows z-index.

         // --- Settings for when this element is a flex CONTAINER ---
         // These properties control the children of this domElement
         domElement.style.display = 'flex'; // Assume flex layout for children by default if not absolute

         const direction = layoutByte & C.LAYOUT_DIRECTION_MASK;
         switch (direction) {
             case 0x00: domElement.style.flexDirection = 'row'; break;
             case 0x01: domElement.style.flexDirection = 'column'; break;
             case 0x02: domElement.style.flexDirection = 'row-reverse'; break;
             case 0x03: domElement.style.flexDirection = 'column-reverse'; break;
         }

         const alignment = (layoutByte & C.LAYOUT_ALIGNMENT_MASK) >> 2;
         // Note: CSS align-items/justify-content meaning depends on flex-direction
         const isRow = direction === 0x00 || direction === 0x02;
         const mainAxisAlignment = isRow ? 'justifyContent' : 'alignItems';
         const crossAxisAlignment = isRow ? 'alignItems' : 'justifyContent';

         switch (alignment) {
             case 0x00: // Start
                 domElement.style[mainAxisAlignment] = 'flex-start';
                 domElement.style[crossAxisAlignment] = 'stretch'; // Default cross-axis stretch
                 break;
             case 0x01: // Center
                 domElement.style[mainAxisAlignment] = 'center';
                 domElement.style[crossAxisAlignment] = 'center'; // Usually center on both axes
                 break;
             case 0x02: // End
                 domElement.style[mainAxisAlignment] = 'flex-end';
                 domElement.style[crossAxisAlignment] = 'stretch'; // Or center/flex-end? Defaulting to stretch.
                 break;
             case 0x03: // SpaceBetween
                 domElement.style[mainAxisAlignment] = 'space-between';
                 domElement.style[crossAxisAlignment] = 'stretch'; // Default cross-axis stretch
                 break;
         }
         // Allow cross-axis alignment to be overridden if needed (e.g., via a specific property)


         domElement.style.flexWrap = (layoutByte & C.LAYOUT_WRAP_BIT) ? 'wrap' : 'nowrap';

         // Apply Gap property if present
         if (combinedProps.has(C.PROP_ID_GAP)) {
             const gapProp = combinedProps.get(C.PROP_ID_GAP);
             if ((gapProp.valueType === C.VAL_TYPE_BYTE || gapProp.valueType === C.VAL_TYPE_SHORT) && gapProp.value !== null) {
                domElement.style.gap = `${gapProp.value}px`;
             }
         }
     }

     // --- Settings for when this element is a flex ITEM ---
     const grow = !!(layoutByte & C.LAYOUT_GROW_BIT);
     if (!isAbsolute) { // Grow only applies in flow layout
        domElement.style.flexGrow = grow ? '1' : '0';
        domElement.style.flexShrink = grow ? '1' : '0'; // Allow shrinking if growing? Default to 0 if fixed size expected.
        // TODO: Basis? Maybe set flex: grow 0 auto; or flex: 0 0 width;
     }
}

// --- Recursive Rendering ---

// Keep track of the current index in the flat elements array during recursion
let currentElementIndex = 0;

/**
 * Recursively renders KRB elements into DOM elements.
 * Assumes elements are stored sequentially and children follow parents.
 * @param {HTMLElement} parentDomElement - The DOM element to append the new element to.
 * @returns {HTMLElement | null} The created DOM element, or null if error/end.
 */
function renderNextElementRecursive(parentDomElement) {
    if (currentElementIndex >= KrbDoc.elements.length) {
        return null; // No more elements
    }

    const elementData = KrbDoc.elements[currentElementIndex];
    const header = elementData.header;
    const currentIndex = currentElementIndex; // Capture index before incrementing for children
    currentElementIndex++; // Consume this element

    // 1. Create DOM Element
    let domElement;
    switch (header.type) {
        case C.ELEM_TYPE_CONTAINER:
        case C.ELEM_TYPE_APP: // Treat App mostly like a container for content
        case C.ELEM_TYPE_LIST: // Basic list = column container
        case C.ELEM_TYPE_GRID: // Needs display: grid
        case C.ELEM_TYPE_SCROLLABLE: // Needs overflow CSS
             domElement = document.createElement('div');
             if (header.type === C.ELEM_TYPE_GRID) domElement.style.display = 'grid'; // TODO: Grid template props
             if (header.type === C.ELEM_TYPE_SCROLLABLE) domElement.style.overflow = 'auto'; // Basic scroll
            break;
        case C.ELEM_TYPE_TEXT:
            domElement = document.createElement('span'); // Use span for inline-block potential
            domElement.style.display = 'inline-block'; // Allow width/height/padding
            break;
        case C.ELEM_TYPE_IMAGE:
            domElement = document.createElement('img');
             domElement.style.display = 'block'; // Prevent extra space below img
            break;
        case C.ELEM_TYPE_BUTTON:
            domElement = document.createElement('button');
            break;
        case C.ELEM_TYPE_INPUT:
            domElement = document.createElement('input');
            // TODO: Handle input types (text, number, password) via properties
            break;
        case C.ELEM_TYPE_CANVAS:
            domElement = document.createElement('canvas');
            break;
         case C.ELEM_TYPE_VIDEO:
            domElement = document.createElement('video');
            // TODO: Handle video source, controls etc. via properties
            break;
        default:
            console.warn(`Unsupported element type: 0x${header.type.toString(16)} at index ${currentIndex}. Rendering as div.`);
            domElement = document.createElement('div');
             domElement.style.border = '1px dashed red'; // Indicate unknown type
            domElement.textContent = `Unsupported Type ${header.type}`;
    }

     // Add a debug attribute
     domElement.setAttribute('data-krb-index', currentIndex);
     domElement.setAttribute('data-krb-type', `0x${header.type.toString(16)}`);

    // 2. Set ID
    if (header.idIndex > 0 && header.idIndex < KrbDoc.strings.length) {
        const idStr = KrbDoc.strings[header.idIndex];
        if (idStr) {
             domElement.id = idStr;
        } else {
             console.warn(`Element ${currentIndex}: ID string index ${header.idIndex} resolved to null/empty.`);
        }
    } else if (header.idIndex > 0) {
         console.warn(`Element ${currentIndex}: ID string index ${header.idIndex} is out of bounds (${KrbDoc.strings.length}).`);
    }

    // 3. Apply Styling and Properties
    applyStyling(domElement, elementData);

     // 4. Apply Absolute Position Offset (if applicable)
     if (header.layout & C.LAYOUT_ABSOLUTE_BIT) {
         // Position relative to parent's padding box
         domElement.style.left = `${header.posX}px`;
         domElement.style.top = `${header.posY}px`;
     }
     // For flow layout, position is handled by flexbox on parent

    // 5. Attach Event Listeners
    elementData.events.forEach(eventInfo => {
        const domEventName = mapKrbEventToDomEvent(eventInfo.eventType);
        if (domEventName) {
            const callback = findCallback(eventInfo.callbackIdIndex);
            domElement.addEventListener(domEventName, callback);
            // Special case: Add pointerleave for hover end if needed
            if (eventInfo.eventType === C.EVENT_TYPE_HOVER) {
                 // TODO: Decide if a separate KRB event for hover end is needed
                 // or if the same callback handles both enter/leave via event properties.
                 // For now, just attaching 'pointerenter'. Add 'pointerleave' if required.
                 // domElement.addEventListener('pointerleave', callback);
            }
             // TODO: Implement LongPress logic
        }
    });

    // 6. Append to Parent
    parentDomElement.appendChild(domElement);

    // 7. Render Children
    // Assumes children immediately follow parent in the flat elements array
    for (let i = 0; i < header.childCount; i++) {
        renderNextElementRecursive(domElement); // Children are appended to the current element
    }

    return domElement;
}

// --- Public Renderer Function ---

/**
 * Renders a parsed KRB document into the target DOM element.
 * @param {object} parsedDocument The document object from KrbParser.
 * @param {HTMLElement} targetContainer The DOM element to render into.
 */
export function renderKrb(parsedDocument, targetContainer) {
    if (!parsedDocument || !targetContainer) {
        console.error("Invalid arguments for renderKrb.");
        return;
    }
    KrbDoc = parsedDocument; // Make doc globally accessible for helpers
    targetContainer.innerHTML = ''; // Clear previous content

    // --- Handle App Element Properties ---
    let appElementData = null;
    let windowTitle = "Kryon App"; // Default title
    // TODO: Handle window size, resizable etc. (Maybe apply to body/html or container?)

    if ((KrbDoc.header.flags & C.FLAG_HAS_APP) && KrbDoc.elements.length > 0 && KrbDoc.elements[0].header.type === C.ELEM_TYPE_APP) {
        appElementData = KrbDoc.elements[0];

        // Extract relevant App properties (like window title)
         const combinedProps = new Map();
         // Apply App Style props first
         if (appElementData.header.styleId > 0 && KrbDoc.styles) {
            const style = KrbDoc.styles.find(s => s.id === appElementData.header.styleId);
            if (style && style.properties) style.properties.forEach(prop => combinedProps.set(prop.propertyId, prop));
         }
         // Apply App Direct props (override)
         appElementData.properties.forEach(prop => combinedProps.set(prop.propertyId, prop));

         if (combinedProps.has(C.PROP_ID_WINDOW_TITLE)) {
             const titleProp = combinedProps.get(C.PROP_ID_WINDOW_TITLE);
             if (titleProp.valueType === C.VAL_TYPE_STRING && titleProp.value !== null && titleProp.value < KrbDoc.strings.length) {
                 windowTitle = KrbDoc.strings[titleProp.value] || windowTitle;
             }
         }
         // TODO: Handle window width/height, scale factor etc. Affect targetContainer size?

        document.title = windowTitle; // Set the browser window title

        // The App element itself acts as the root container for its children
        // It won't be rendered directly if it's just a logical root.
        // However, we *will* render it if it has visual styles/children defined.
        // If FLAG_HAS_APP is set, we start rendering from index 0.
        currentElementIndex = 0;

    } else {
         // No App element, treat all elements as roots
         console.warn("No <App> element found or FLAG_HAS_APP not set. Rendering all elements as roots.");
         currentElementIndex = 0; // Start from the beginning anyway
    }


    console.log("Starting KRB Rendering...");
    // Render all top-level elements (roots)
    // The recursive function handles advancing the index
    while(currentElementIndex < KrbDoc.elements.length) {
         // Check if the current element's parent should have been a previous element
         // This basic check helps if elements aren't strictly hierarchical in the flat list
         // More robust would be explicit parent tracking if needed.
         // For now, assume top-level elements are encountered until consumed by children.
        renderNextElementRecursive(targetContainer);
    }

     // Cleanup global reference
     KrbDoc = null;
     console.log("KRB Rendering Finished.");
}