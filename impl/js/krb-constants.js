// KRB v0.2 Constants for JavaScript Implementation

export const KRB_SPEC_VERSION_MAJOR = 0;
export const KRB_SPEC_VERSION_MINOR = 2;

// Header Flags (Bitmask)
export const FLAG_HAS_STYLES = 1 << 0;
export const FLAG_HAS_ANIMATIONS = 1 << 1;
export const FLAG_HAS_RESOURCES = 1 << 2;
export const FLAG_COMPRESSED = 1 << 3; // Not implemented
export const FLAG_FIXED_POINT = 1 << 4;
export const FLAG_EXTENDED_COLOR = 1 << 5;
export const FLAG_HAS_APP = 1 << 6;
// Bits 7-15 Reserved

// Element Types
export const ELEM_TYPE_APP = 0x00;
export const ELEM_TYPE_CONTAINER = 0x01;
export const ELEM_TYPE_TEXT = 0x02;
export const ELEM_TYPE_IMAGE = 0x03;
export const ELEM_TYPE_CANVAS = 0x04;
export const ELEM_TYPE_BUTTON = 0x10;
export const ELEM_TYPE_INPUT = 0x11;
export const ELEM_TYPE_LIST = 0x20;
export const ELEM_TYPE_GRID = 0x21;
export const ELEM_TYPE_SCROLLABLE = 0x22;
export const ELEM_TYPE_VIDEO = 0x30;
// 0x31-0xFF Custom/Specialized

// Property IDs
export const PROP_ID_INVALID = 0x00;
export const PROP_ID_BG_COLOR = 0x01;
export const PROP_ID_FG_COLOR = 0x02;
export const PROP_ID_BORDER_COLOR = 0x03;
export const PROP_ID_BORDER_WIDTH = 0x04;
export const PROP_ID_BORDER_RADIUS = 0x05;
export const PROP_ID_PADDING = 0x06; // TODO: Implement rendering
export const PROP_ID_MARGIN = 0x07;  // TODO: Implement rendering
export const PROP_ID_TEXT_CONTENT = 0x08;
export const PROP_ID_FONT_SIZE = 0x09; // TODO: Implement rendering
export const PROP_ID_FONT_WEIGHT = 0x0A; // TODO: Implement rendering
export const PROP_ID_TEXT_ALIGNMENT = 0x0B;
export const PROP_ID_IMAGE_SOURCE = 0x0C;
export const PROP_ID_OPACITY = 0x0D; // TODO: Implement rendering
export const PROP_ID_ZINDEX = 0x0E; // TODO: Implement rendering
export const PROP_ID_VISIBILITY = 0x0F; // TODO: Implement rendering
export const PROP_ID_GAP = 0x10; // TODO: Implement rendering (Flexbox gap)
export const PROP_ID_MIN_WIDTH = 0x11; // TODO: Implement rendering
export const PROP_ID_MIN_HEIGHT = 0x12; // TODO: Implement rendering
export const PROP_ID_MAX_WIDTH = 0x13; // TODO: Implement rendering
export const PROP_ID_MAX_HEIGHT = 0x14; // TODO: Implement rendering
export const PROP_ID_ASPECT_RATIO = 0x15; // TODO: Implement rendering
export const PROP_ID_TRANSFORM = 0x16; // TODO: Implement rendering
export const PROP_ID_SHADOW = 0x17; // TODO: Implement rendering
export const PROP_ID_OVERFLOW = 0x18; // TODO: Implement rendering
export const PROP_ID_CUSTOM_PROP = 0x19; // TODO: Implement rendering (data-* attributes?)
export const PROP_ID_LAYOUT_FLAGS = 0x1A; // Special: Handled by compiler into Element Header Layout byte
export const PROP_ID_WINDOW_WIDTH = 0x20;
export const PROP_ID_WINDOW_HEIGHT = 0x21;
export const PROP_ID_WINDOW_TITLE = 0x22;
export const PROP_ID_RESIZABLE = 0x23;
export const PROP_ID_KEEP_ASPECT = 0x24; // TODO: Implement rendering
export const PROP_ID_SCALE_FACTOR = 0x25; // TODO: Maybe apply globally?
export const PROP_ID_ICON = 0x26;
export const PROP_ID_VERSION = 0x27; // TODO: Expose metadata
export const PROP_ID_AUTHOR = 0x28; // TODO: Expose metadata

// Value Types
export const VAL_TYPE_NONE = 0x00;
export const VAL_TYPE_BYTE = 0x01;
export const VAL_TYPE_SHORT = 0x02;
export const VAL_TYPE_COLOR = 0x03;
export const VAL_TYPE_STRING = 0x04; // Index into string table
export const VAL_TYPE_RESOURCE = 0x05; // Index into resource table
export const VAL_TYPE_PERCENTAGE = 0x06; // 8.8 Fixed Point
export const VAL_TYPE_RECT = 0x07; // 4 shorts (x, y, w, h)
export const VAL_TYPE_EDGEINSETS = 0x08; // 4 bytes (t, r, b, l)
export const VAL_TYPE_ENUM = 0x09; // Typically 1 byte
export const VAL_TYPE_VECTOR = 0x0A; // 2 shorts (x, y)
export const VAL_TYPE_CUSTOM = 0x0B;

// Event Types
export const EVENT_TYPE_NONE = 0x00;
export const EVENT_TYPE_CLICK = 0x01;
export const EVENT_TYPE_PRESS = 0x02; // Maps to mousedown/touchstart
export const EVENT_TYPE_RELEASE = 0x03; // Maps to mouseup/touchend
export const EVENT_TYPE_LONGPRESS = 0x04; // Requires JS logic
export const EVENT_TYPE_HOVER = 0x05; // Maps to mouseenter/mouseleave (or pointerenter/leave)
export const EVENT_TYPE_FOCUS = 0x06;
export const EVENT_TYPE_BLUR = 0x07;
export const EVENT_TYPE_CHANGE = 0x08; // Input value changed
export const EVENT_TYPE_SUBMIT = 0x09; // Form submission
export const EVENT_TYPE_CUSTOM = 0x0A;

// Layout Byte Bits/Masks (for Element Header 'layout' field)
export const LAYOUT_DIRECTION_MASK = 0x03; // 00:Row, 01:Col, 10:RowRev, 11:ColRev
export const LAYOUT_ALIGNMENT_MASK = 0x0C; // Shifted >> 2: 00:Start, 01:Center, 10:End, 11:SpaceBetween
export const LAYOUT_WRAP_BIT = (1 << 4);
export const LAYOUT_GROW_BIT = (1 << 5); // TODO: Implement rendering (flex-grow)
export const LAYOUT_ABSOLUTE_BIT = (1 << 6);
// Bit 7 Reserved

// Resource Types
export const RES_TYPE_NONE = 0x00;
export const RES_TYPE_IMAGE = 0x01;
export const RES_TYPE_FONT = 0x02;
export const RES_TYPE_SOUND = 0x03;
export const RES_TYPE_VIDEO = 0x04;
export const RES_TYPE_CUSTOM = 0x05;

// Resource Formats
export const RES_FORMAT_EXTERNAL = 0x00;
export const RES_FORMAT_INLINE = 0x01; // Not implemented in parser yet