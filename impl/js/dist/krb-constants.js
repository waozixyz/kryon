// impl/js/krb-constants.js

// --- Version ---
export const KRB_SPEC_VERSION_MAJOR = 0;
export const KRB_SPEC_VERSION_MINOR = 3; // Updated v0.3

// --- Header Flags (Bitmask) ---
// No changes spec'd between v0.2 and v0.3
export const FLAG_HAS_STYLES = 1 << 0;
export const FLAG_HAS_ANIMATIONS = 1 << 1;
export const FLAG_HAS_RESOURCES = 1 << 2;
export const FLAG_COMPRESSED = 1 << 3; // Not specified
export const FLAG_FIXED_POINT = 1 << 4;
export const FLAG_EXTENDED_COLOR = 1 << 5;
export const FLAG_HAS_APP = 1 << 6;
// Bits 7-15 Reserved

// --- Element Types ---
// No value changes, but noting custom range
export const ELEM_TYPE_APP = 0x00;
export const ELEM_TYPE_CONTAINER = 0x01;
export const ELEM_TYPE_TEXT = 0x02;
export const ELEM_TYPE_IMAGE = 0x03;
export const ELEM_TYPE_CANVAS = 0x04;
// 0x05 - 0x0F Reserved
export const ELEM_TYPE_BUTTON = 0x10;
export const ELEM_TYPE_INPUT = 0x11;
// 0x12 - 0x1F Reserved
export const ELEM_TYPE_LIST = 0x20;
export const ELEM_TYPE_GRID = 0x21;
export const ELEM_TYPE_SCROLLABLE = 0x22;
// 0x23 - 0x2F Reserved
export const ELEM_TYPE_VIDEO = 0x30;
// 0x31 - 0xFF: Custom (Use Element ID String for identification)

// --- Property IDs ---
// Standard Visual/Layout Properties (0x01 - 0x18)
export const PROP_ID_INVALID = 0x00; // Added for completeness, though not in spec list
export const PROP_ID_BG_COLOR = 0x01;
export const PROP_ID_FG_COLOR = 0x02;
export const PROP_ID_BORDER_COLOR = 0x03;
export const PROP_ID_BORDER_WIDTH = 0x04;
export const PROP_ID_BORDER_RADIUS = 0x05;
export const PROP_ID_PADDING = 0x06;
export const PROP_ID_MARGIN = 0x07;
export const PROP_ID_TEXT_CONTENT = 0x08;
export const PROP_ID_FONT_SIZE = 0x09;
export const PROP_ID_FONT_WEIGHT = 0x0A;
export const PROP_ID_TEXT_ALIGNMENT = 0x0B;
export const PROP_ID_IMAGE_SOURCE = 0x0C;
export const PROP_ID_OPACITY = 0x0D;
export const PROP_ID_ZINDEX = 0x0E;
export const PROP_ID_VISIBILITY = 0x0F;
export const PROP_ID_GAP = 0x10;
export const PROP_ID_MIN_WIDTH = 0x11;
export const PROP_ID_MIN_HEIGHT = 0x12;
export const PROP_ID_MAX_WIDTH = 0x13;
export const PROP_ID_MAX_HEIGHT = 0x14;
export const PROP_ID_ASPECT_RATIO = 0x15;
export const PROP_ID_TRANSFORM = 0x16;
export const PROP_ID_SHADOW = 0x17;
export const PROP_ID_OVERFLOW = 0x18;

// Special Properties (0x19 - 0x1F)
export const PROP_ID_CUSTOM_DATA_BLOB = 0x19; // Clarified v0.3
export const PROP_ID_LAYOUT_FLAGS = 0x1A;     // Clarified v0.3 (Compiler source)
// 0x1B - 0x1F Reserved

// App-Specific Properties (0x20 - 0x28)
export const PROP_ID_WINDOW_WIDTH = 0x20;
export const PROP_ID_WINDOW_HEIGHT = 0x21;
export const PROP_ID_WINDOW_TITLE = 0x22;
export const PROP_ID_RESIZABLE = 0x23;
export const PROP_ID_KEEP_ASPECT = 0x24;
export const PROP_ID_SCALE_FACTOR = 0x25;
export const PROP_ID_ICON = 0x26;
export const PROP_ID_VERSION = 0x27;
export const PROP_ID_AUTHOR = 0x28;
// Others Reserved

// --- Value Types ---
export const VAL_TYPE_NONE = 0x00;
export const VAL_TYPE_BYTE = 0x01;
export const VAL_TYPE_SHORT = 0x02;
export const VAL_TYPE_COLOR = 0x03; // Size depends on FLAG_EXTENDED_COLOR
export const VAL_TYPE_STRING = 0x04; // Index into string table (0-based)
export const VAL_TYPE_RESOURCE = 0x05; // Index into resource table (0-based)
export const VAL_TYPE_PERCENTAGE = 0x06; // 8.8 Fixed Point (u16)
export const VAL_TYPE_RECT = 0x07; // e.g., 4 shorts (x, y, w, h)
export const VAL_TYPE_EDGEINSETS = 0x08; // e.g., 4 bytes/shorts (t, r, b, l) - spec says bytes/shorts? Assuming bytes for padding/margin
export const VAL_TYPE_ENUM = 0x09; // Typically 1 byte
export const VAL_TYPE_VECTOR = 0x0A; // e.g., 2 shorts (x, y)
export const VAL_TYPE_CUSTOM = 0x0B; // Use with PROP_ID_CUSTOM_DATA_BLOB or custom props
// Others Reserved

// --- Event Types ---
export const EVENT_TYPE_NONE = 0x00; // Added for completeness
export const EVENT_TYPE_CLICK = 0x01;
export const EVENT_TYPE_PRESS = 0x02;
export const EVENT_TYPE_RELEASE = 0x03;
export const EVENT_TYPE_LONGPRESS = 0x04;
export const EVENT_TYPE_HOVER = 0x05;
export const EVENT_TYPE_FOCUS = 0x06;
export const EVENT_TYPE_BLUR = 0x07;
export const EVENT_TYPE_CHANGE = 0x08;
export const EVENT_TYPE_SUBMIT = 0x09;
export const EVENT_TYPE_CUSTOM = 0x0A; // Runtime defined
// Others Reserved

// --- Animation Trigger Types ---
export const TRIGGER_TYPE_AUTO = 0x00;
export const TRIGGER_TYPE_CLICK = 0x01;
export const TRIGGER_TYPE_HOVER = 0x02;
export const TRIGGER_TYPE_FOCUS = 0x03;
export const TRIGGER_TYPE_LOAD = 0x04;
export const TRIGGER_TYPE_CUSTOM = 0x05; // Runtime defined
// Others Reserved


// --- Layout Byte Bits/Masks (for Element Header 'layout' field) ---
// No changes spec'd between v0.2 and v0.3
export const LAYOUT_DIRECTION_MASK = 0x03; // Bits 0-1: 00:Row, 01:Col, 10:RowRev, 11:ColRev
export const LAYOUT_ALIGNMENT_MASK = 0x0C; // Bits 2-3: Shifted >> 2: 00:Start, 01:Center, 10:End, 11:SpaceBetween
export const LAYOUT_WRAP_BIT = (1 << 4);   // Bit 4: Wrap
export const LAYOUT_GROW_BIT = (1 << 5);   // Bit 5: Grow
export const LAYOUT_ABSOLUTE_BIT = (1 << 6); // Bit 6: Position (0=Flow, 1=Absolute)
// Bit 7 Reserved

// --- Resource Types ---
export const RES_TYPE_IMAGE = 0x01;
export const RES_TYPE_FONT = 0x02;
export const RES_TYPE_SOUND = 0x03;
export const RES_TYPE_VIDEO = 0x04;
export const RES_TYPE_CUSTOM = 0x05;
// Others Reserved

// --- Resource Formats ---
export const RES_FORMAT_EXTERNAL = 0x00; // Data = 1 byte string index (path)
export const RES_FORMAT_INLINE = 0x01; // Data = Size (u16) + Bytes