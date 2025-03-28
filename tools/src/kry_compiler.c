#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>
#include <errno.h> // For error reporting
#include <stdbool.h> // For bool type

// --- Constants based on Specification ---
#define KRB_MAGIC "KRB1"
#define KRB_VERSION_MAJOR 1
#define KRB_VERSION_MINOR 0

// Header Flags
#define FLAG_HAS_STYLES     (1 << 0)
#define FLAG_HAS_ANIMATIONS (1 << 1)
#define FLAG_HAS_RESOURCES  (1 << 2)
#define FLAG_COMPRESSED     (1 << 3) // Not implemented
#define FLAG_FIXED_POINT    (1 << 4) // Indicate usage
#define FLAG_EXTENDED_COLOR (1 << 5) // Indicate usage (RGBA vs Palette)
#define FLAG_HAS_APP        (1 << 6)
// Bits 7-15 Reserved

// Element Types (Matching Specification)
#define ELEM_TYPE_APP         0x00
#define ELEM_TYPE_CONTAINER   0x01
#define ELEM_TYPE_TEXT        0x02
#define ELEM_TYPE_IMAGE       0x03
#define ELEM_TYPE_CANVAS      0x04
// 0x05-0x0F Reserved
#define ELEM_TYPE_BUTTON      0x10
#define ELEM_TYPE_INPUT       0x11
// 0x12-0x1F Reserved
#define ELEM_TYPE_LIST        0x20
#define ELEM_TYPE_GRID        0x21
#define ELEM_TYPE_SCROLLABLE  0x22
// 0x23-0x2F Reserved
// 0x30-0xFF Specialized/Custom

// Property IDs (Matching Specification)
#define PROP_ID_INVALID         0x00
#define PROP_ID_BG_COLOR        0x01
#define PROP_ID_FG_COLOR        0x02
#define PROP_ID_BORDER_COLOR    0x03
#define PROP_ID_BORDER_WIDTH    0x04 // Can be Byte or EdgeInsets
#define PROP_ID_BORDER_RADIUS   0x05
#define PROP_ID_PADDING         0x06 // Can be Short or EdgeInsets
#define PROP_ID_MARGIN          0x07 // Can be Short or EdgeInsets
#define PROP_ID_TEXT_CONTENT    0x08
#define PROP_ID_FONT_SIZE       0x09
#define PROP_ID_FONT_WEIGHT     0x0A
#define PROP_ID_TEXT_ALIGNMENT  0x0B
#define PROP_ID_IMAGE_SOURCE    0x0C
#define PROP_ID_OPACITY         0x0D
#define PROP_ID_ZINDEX          0x0E
#define PROP_ID_VISIBILITY      0x0F
#define PROP_ID_GAP             0x10
#define PROP_ID_MIN_WIDTH       0x11
#define PROP_ID_MIN_HEIGHT      0x12
#define PROP_ID_MAX_WIDTH       0x13
#define PROP_ID_MAX_HEIGHT      0x14
#define PROP_ID_ASPECT_RATIO    0x15
#define PROP_ID_TRANSFORM       0x16
#define PROP_ID_SHADOW          0x17
#define PROP_ID_OVERFLOW        0x18
#define PROP_ID_CUSTOM          0x19 // Uses string table ref for name
#define PROP_ID_LAYOUT_FLAGS    0x1A
// App Specific
#define PROP_ID_WINDOW_WIDTH    0x20
#define PROP_ID_WINDOW_HEIGHT   0x21
#define PROP_ID_WINDOW_TITLE    0x22
#define PROP_ID_RESIZABLE       0x23
#define PROP_ID_KEEP_ASPECT     0x24
#define PROP_ID_SCALE_FACTOR    0x25
#define PROP_ID_ICON            0x26 // Resource index (using string index for path for now)
#define PROP_ID_VERSION         0x27
#define PROP_ID_AUTHOR          0x28
// 0x29 - 0xFF Reserved

// Value Types (Matching Specification)
#define VAL_TYPE_NONE       0x00
#define VAL_TYPE_BYTE       0x01
#define VAL_TYPE_SHORT      0x02
#define VAL_TYPE_COLOR      0x03 // RGBA or palette index
#define VAL_TYPE_STRING     0x04 // Index to string table (1 byte)
#define VAL_TYPE_RESOURCE   0x05 // Index to resource table (1 byte)
#define VAL_TYPE_PERCENTAGE 0x06 // Fixed-point (e.g., 8.8) - size depends on flag
#define VAL_TYPE_RECT       0x07 // x,y,w,h (e.g., 4 shorts = 8 bytes)
#define VAL_TYPE_EDGEINSETS 0x08 // top,right,bottom,left (e.g., 4 bytes)
#define VAL_TYPE_ENUM       0x09 // Predefined options (1 byte usually)
#define VAL_TYPE_VECTOR     0x0A // x,y coords (e.g., 2 shorts = 4 bytes)
#define VAL_TYPE_CUSTOM     0x0B // Depends on context
// 0x0C - 0xFF Reserved

// Event Types (Matching Specification)
#define EVENT_TYPE_NONE     0x00
#define EVENT_TYPE_CLICK    0x01
#define EVENT_TYPE_PRESS    0x02
#define EVENT_TYPE_RELEASE  0x03
// ... add others as needed

// Layout Byte Bits (Matching Specification)
#define LAYOUT_DIRECTION_MASK 0x03 // Bits 0-1: 00=Row, 01=Col, 10=RowRev, 11=ColRev
#define LAYOUT_ALIGNMENT_MASK 0x0C // Bits 2-3: 00=Start, 01=Center, 10=End, 11=SpaceBetween
#define LAYOUT_WRAP_BIT       (1 << 4) // Bit 4: 0=NoWrap, 1=Wrap
#define LAYOUT_GROW_BIT       (1 << 5) // Bit 5: 0=Fixed, 1=Grow
#define LAYOUT_ABSOLUTE_BIT   (1 << 6) // Bit 6: 0=Flow, 1=Absolute
// Bit 7 Reserved

// Limits
#define MAX_ELEMENTS 256
#define MAX_STRINGS 256         // Limited by 1-byte index in properties
#define MAX_PROPERTIES 64       // Increased limit per element/style
#define MAX_STYLES 64           // Limit for defined styles
#define MAX_CHILDREN 128        // Limit for children per element
#define MAX_EVENTS 16           // Limit for events per element
#define MAX_ANIM_REFS 16        // Limit for animation refs per element
#define MAX_LINE_LENGTH 512     // Increased line buffer size
#define MAX_ANIMATIONS 64       // Placeholder
#define MAX_RESOURCES 64        // Placeholder

// --- Data Structures ---

typedef struct {
    uint8_t property_id;
    uint8_t value_type;
    uint8_t size;
    void* value;            // Pointer to dynamically allocated value data
} KrbProperty;

typedef struct {
    uint8_t event_type;
    uint8_t callback_id; // String table index
} KrbEvent;

typedef struct Element {
    // Header Data to be written
    uint8_t type;
    uint8_t id_string_index; // Index into string table for ID name (or 0 if none)
    uint16_t pos_x;
    uint16_t pos_y;
    uint16_t width;
    uint16_t height;
    uint8_t layout;
    uint8_t style_id;       // 1-based index into style array (0 = none)
    uint8_t property_count;
    uint8_t child_count;
    uint8_t event_count;    // Number of event references
    uint8_t animation_count;// Number of animation references

    // Compiler-Internal Data
    KrbProperty properties[MAX_PROPERTIES];
    KrbEvent events[MAX_EVENTS];
    // TODO: Add Animation reference structures
    struct Element* children[MAX_CHILDREN];
    int parent_index;       // Index in the global elements array, -1 for root/App
    int self_index;         // Index in the global elements array

    // Data for Pass 2
    uint32_t calculated_size; // Total size of this element's data block in the file
    uint32_t absolute_offset; // Absolute byte offset from start of file
} Element;

typedef struct {
    char* text;             // The actual string content (cleaned)
    size_t length;
    uint8_t index;          // Its index in the table (0 to MAX_STRINGS-1)
} StringEntry;

typedef struct {
    uint8_t id;             // 1-based ID for referencing
    uint8_t name_index;     // Index into string table for the style's name
    KrbProperty properties[MAX_PROPERTIES];
    uint8_t property_count;

    // Data for Pass 2
    uint32_t calculated_size; // Total size of this style's data block
} StyleEntry;

// TODO: Add Animation structure
// TODO: Add Resource structure

// --- Global Compiler State ---
Element g_elements[MAX_ELEMENTS];
StringEntry g_strings[MAX_STRINGS];
StyleEntry g_styles[MAX_STYLES];
// TODO: Global animation array
// TODO: Global resource array
int g_element_count = 0;
int g_string_count = 0;
int g_style_count = 0;
int g_animation_count = 0; // Placeholder
int g_resource_count = 0;  // Placeholder
int g_has_app = 0;
uint16_t g_header_flags = 0; // Combined flags for header

// --- Utility Functions ---

void write_u8(FILE* file, uint8_t value) {
    if (fputc(value, file) == EOF) {
        perror("Error writing u8"); exit(EXIT_FAILURE);
    }
}

void write_u16(FILE* file, uint16_t value) {
    if (fputc(value & 0xFF, file) == EOF || // Little-endian: low byte first
        fputc((value >> 8) & 0xFF, file) == EOF) {
        perror("Error writing u16"); exit(EXIT_FAILURE);
    }
}

void write_u32(FILE* file, uint32_t value) {
    if (fputc(value & 0xFF, file) == EOF ||
        fputc((value >> 8) & 0xFF, file) == EOF ||
        fputc((value >> 16) & 0xFF, file) == EOF ||
        fputc((value >> 24) & 0xFF, file) == EOF) {
        perror("Error writing u32"); exit(EXIT_FAILURE);
    }
}
uint8_t add_string(const char* text) {
    if (!text) return 0; // Index 0 could be reserved for "" or invalid

    // 1. Trim leading/trailing whitespace
    const char *start = text;
    const char *end = text + strlen(text) - 1;
    while (start <= end && isspace((unsigned char)*start)) start++;
    while (end >= start && isspace((unsigned char)*end)) end--;

    // 2. Handle quotes (only if they are the very first and last non-whitespace chars)
    int quoted = 0;
    if (end >= start && *start == '"' && *end == '"') {
        start++;
        end--;
        quoted = 1;
    }

    // 3. Calculate length and create a temporary buffer for the cleaned string
    size_t len = (end < start) ? 0 : (end - start + 1);
    char clean_text_buf[MAX_LINE_LENGTH];
    if (len >= sizeof(clean_text_buf)) {
        fprintf(stderr, "Error: Cleaned string too long: %zu chars\n", len);
        exit(EXIT_FAILURE);
    }
    strncpy(clean_text_buf, start, len);
    clean_text_buf[len] = '\0';

    // 4. Check for duplicates
    for (int i = 0; i < g_string_count; i++) {
        if (g_strings[i].text && strcmp(g_strings[i].text, clean_text_buf) == 0) {
            return g_strings[i].index; // Return existing index
        }
    }

    // 5. Add new string
    if (g_string_count >= MAX_STRINGS) {
        fprintf(stderr, "Error: Maximum string count (%d) exceeded.\n", MAX_STRINGS);
        exit(EXIT_FAILURE);
    }

    // Allocate memory for the string
    g_strings[g_string_count].text = strdup(clean_text_buf);
    if (!g_strings[g_string_count].text) {
        perror("Failed to duplicate cleaned string");
        exit(EXIT_FAILURE);
    }

    g_strings[g_string_count].length = len;
    g_strings[g_string_count].index = g_string_count;
    
    return g_string_count++;
}
// Finds style index by name (returns 0 if not found)
uint8_t find_style_id_by_name(const char* name) {
    if (!name) return 0;

    const char *start = name;
    const char *end = name + strlen(name) - 1;
    while (start <= end && isspace((unsigned char)*start)) start++;
    while (end >= start && isspace((unsigned char)*end)) end--;
    if (end >= start && *start == '"' && *end == '"') { start++; end--; }
    size_t len = (end < start) ? 0 : (end - start + 1);
    char clean_name_buf[MAX_LINE_LENGTH];
     if (len >= sizeof(clean_name_buf)) { fprintf(stderr, "Error: Style name too long for search buffer: %zu chars\n", len); exit(EXIT_FAILURE); }
    strncpy(clean_name_buf, start, len); clean_name_buf[len] = '\0';

    for (int i = 0; i < g_style_count; i++) {
        if (g_styles[i].name_index < g_string_count && // Bounds check
            g_strings[g_styles[i].name_index].text && // Ensure string text is valid
            strcmp(g_strings[g_styles[i].name_index].text, clean_name_buf) == 0) {
            return g_styles[i].id; // Return 1-based ID
        }
    }
    return 0; // Not found
}

void cleanup_resources() {
    // Free element properties
    for (int i = 0; i < g_element_count; i++) {
        for (int j = 0; j < g_elements[i].property_count; j++) {
            if (g_elements[i].properties[j].value) {
                free(g_elements[i].properties[j].value);
                g_elements[i].properties[j].value = NULL;
            }
        }
    }
    // Free style properties
    for (int i = 0; i < g_style_count; i++) {
        for (int j = 0; j < g_styles[i].property_count; j++) {
            if (g_styles[i].properties[j].value) {
                free(g_styles[i].properties[j].value);
                g_styles[i].properties[j].value = NULL;
            }
        }
    }
    // Free strings
    for (int i = 0; i < g_string_count; i++) {
         if (g_strings[i].text) {
            free(g_strings[i].text);
            g_strings[i].text = NULL;
        }
    }
    // Reset counts
    g_element_count = 0; g_string_count = 0; g_style_count = 0;
    g_animation_count = 0; g_resource_count = 0; g_has_app = 0; g_header_flags = 0;
}

// Helper to parse color strings like #RRGGBB or #RRGGBBAA -> outputs RGBA
int parse_color(const char* value_str, uint8_t color_out[4]) {
    color_out[0] = 0; color_out[1] = 0; color_out[2] = 0; color_out[3] = 255;
    if (!value_str) return 0;
    const char* p = value_str; while(isspace((unsigned char)*p)) p++;
    if (*p != '#') return 0; p++;
    int len = strlen(p); char* end = (char*)p + len -1; while(end >= p && isspace((unsigned char)*end)) *end-- = '\0'; len = strlen(p);
    if (len == 8 && sscanf(p, "%2hhx%2hhx%2hhx%2hhx", &color_out[0], &color_out[1], &color_out[2], &color_out[3]) == 4) return 1;
    if (len == 6 && sscanf(p, "%2hhx%2hhx%2hhx", &color_out[0], &color_out[1], &color_out[2]) == 3) return 1;
    return 0;
}
void add_property_to_list(KrbProperty* prop_array, uint8_t* prop_count, uint32_t* current_size, 
                           uint8_t prop_id, uint8_t val_type, uint8_t size, const void* data) {
    // Explicit NULL checks with detailed error reporting
    if (!prop_array) {
        fprintf(stderr, "FATAL: prop_array is NULL in add_property_to_list\n");
        fprintf(stderr, "Parameters: prop_id=%u, val_type=%u, size=%u\n", prop_id, val_type, size);
        exit(EXIT_FAILURE);
    }
    if (!prop_count) {
        fprintf(stderr, "FATAL: prop_count is NULL in add_property_to_list\n");
        fprintf(stderr, "Parameters: prop_id=%u, val_type=%u, size=%u\n", prop_id, val_type, size);
        exit(EXIT_FAILURE);
    }
    if (!current_size) {
        fprintf(stderr, "FATAL: current_size is NULL in add_property_to_list\n");
        fprintf(stderr, "Parameters: prop_id=%u, val_type=%u, size=%u\n", prop_id, val_type, size);
        exit(EXIT_FAILURE);
    }

    // Check maximum properties
    if (*prop_count >= MAX_PROPERTIES) {
        fprintf(stderr, "Error: Maximum property count (%d) exceeded.\n", MAX_PROPERTIES);
        exit(EXIT_FAILURE);
    }

    // Get pointer to current property
    KrbProperty* p = &prop_array[*prop_count];
    
    // Set property details
    p->property_id = prop_id;
    p->value_type = val_type;
    p->size = size;

    // Allocate and copy value if size > 0
    if (size > 0) {
        if (!data) {
            fprintf(stderr, "FATAL: Null data pointer passed to add_property_to_list with size > 0 (Prop ID: %u)\n", prop_id);
            exit(EXIT_FAILURE);
        }
        p->value = malloc(size);
        if (!p->value) {
            perror("Failed to allocate property value");
            exit(EXIT_FAILURE);
        }
        memcpy(p->value, data, size);
    } else {
        p->value = NULL;
    }

    // Update size and count
    *current_size += 1 + 1 + 1 + size; // PropID + Type + Size + ValueSize
    (*prop_count)++;
}

// Convenience wrapper for string properties (Value is 1-byte index)
void add_string_property_to_list(KrbProperty* prop_array, uint8_t* prop_count, uint32_t* current_size, 
                                  uint8_t prop_id, const char* value_str) {
    // Explicit NULL checks
    if (!prop_array || !prop_count || !current_size) {
        fprintf(stderr, "FATAL: Null pointer in add_string_property_to_list\n");
        fprintf(stderr, "prop_array: %p, prop_count: %p, current_size: %p\n", 
                (void*)prop_array, (void*)prop_count, (void*)current_size);
        exit(EXIT_FAILURE);
    }
    
    uint8_t str_index = add_string(value_str);
    add_property_to_list(prop_array, prop_count, current_size, prop_id, VAL_TYPE_STRING, 1, &str_index);
}

// Convenience wrapper for resource path properties (Value is 1-byte index to string path)
void add_resource_path_property_to_list(KrbProperty* prop_array, uint8_t* prop_count, uint32_t* current_size, 
                                         uint8_t prop_id, const char* path_str) {
    // Explicit NULL checks
    if (!prop_array || !prop_count || !current_size) {
        fprintf(stderr, "FATAL: Null pointer in add_resource_path_property_to_list\n");
        fprintf(stderr, "prop_array: %p, prop_count: %p, current_size: %p\n", 
                (void*)prop_array, (void*)prop_count, (void*)current_size);
        exit(EXIT_FAILURE);
    }
    
    uint8_t str_index = add_string(path_str);
    add_property_to_list(prop_array, prop_count, current_size, prop_id, VAL_TYPE_STRING, 1, &str_index);
}
// --- Pass 1: Parsing and Size Calculation ---
int parse_and_calculate_sizes(FILE* in) {
    char line[MAX_LINE_LENGTH];
    Element* current_element = NULL;
    StyleEntry* current_style = NULL;
    int current_indent = -1;
    int element_indent_stack[MAX_ELEMENTS];
    int element_index_stack[MAX_ELEMENTS];
    int element_stack_top = -1;
    int line_num = 0;
    g_header_flags = 0; // Reset flags at start of parse

    while (fgets(line, sizeof(line), in)) {
        line_num++;

        // Trim whitespace and comments
        char* trimmed = line; int indent = 0;
        while (*trimmed == ' ' || *trimmed == '\t') { indent += (*trimmed == '\t' ? 4 : 1); trimmed++; }
        char* end = trimmed + strlen(trimmed) - 1; while (end >= trimmed && isspace((unsigned char)*end)) *end-- = '\0';
        if (*trimmed == '\0' || *trimmed == '#') continue; // Skip empty or comment lines

        // --- End Block Logic ---
        if (*trimmed == '}') {
            if (current_element && element_stack_top >= 0 && indent == element_indent_stack[element_stack_top]) {
                // Finalize size calculation for element *after* properties and children are processed
                // Add size for variable-length sections following properties:
                current_element->calculated_size += current_element->event_count * 2; // Event Type(1) + Callback ID(1)
                // current_element->calculated_size += current_element->animation_count * 2; // Animation Index(1) + Trigger(1)
                current_element->calculated_size += current_element->child_count * 2; // Child Offset(2)

                // Pop element from stack
                element_stack_top--;
                current_indent = (element_stack_top >= 0) ? element_indent_stack[element_stack_top] : -1;
                current_element = (element_stack_top >= 0) ? &g_elements[element_index_stack[element_stack_top]] : NULL;
                continue; // Done processing the '}' line
            } else if (current_style && indent == current_indent) {
                // End of style block
                current_style = NULL;
                current_indent = -1;
                continue; // Done processing the '}' line
            } else {
                 // Mismatched '}' or indentation error
                 fprintf(stderr, "Error line %d: Mismatched '}' or indentation. Current Indent: %d, Expected: %d\n", line_num, indent, current_indent);
                 return 1; // Parsing failed
            }
        }

        // --- Start New Block Logic ---
        // Check for Style Definition: "style "name" {"
        if (strncmp(trimmed, "style ", 6) == 0 && strstr(trimmed, "{")) {
             if (current_element || current_style) { // Cannot nest blocks
                 fprintf(stderr, "Error line %d: Cannot define style inside another block.\n", line_num);
                 return 1;
             }
             char style_name[128];
             if (sscanf(trimmed, "style \"%127[^\"]\" {", style_name) == 1) { // Extract quoted name
                if (g_style_count >= MAX_STYLES) {
                    fprintf(stderr, "Error line %d: Maximum style count (%d) exceeded.\n", line_num, MAX_STYLES);
                    return 1;
                }
                // Initialize the new style entry
                current_style = &g_styles[g_style_count];
                memset(current_style, 0, sizeof(StyleEntry));
                current_style->id = g_style_count + 1; // 1-based ID
                current_style->name_index = add_string(style_name);
                // Base size: ID(1) + NameIndex(1) + PropCount(1)
                current_style->calculated_size = 1 + 1 + 1;
                g_style_count++; // Increment *after* using current count as index
                current_indent = indent; // Set expected indent for properties
                g_header_flags |= FLAG_HAS_STYLES; // Mark that styles are present
             } else {
                 fprintf(stderr, "Error line %d: Invalid style definition syntax (use quotes for name): %s\n", line_num, trimmed);
                 return 1;
             }
        }
        // Check for Element Definition: "ElementName {"
        else if (isalpha((unsigned char)*trimmed) && strstr(trimmed, "{")) {
             if (current_style) { // Cannot define element inside style
                 fprintf(stderr, "Error line %d: Cannot define element inside a style block.\n", line_num);
                 return 1;
             }
             if (g_element_count >= MAX_ELEMENTS) {
                 fprintf(stderr, "Error line %d: Maximum element count (%d) exceeded.\n", line_num, MAX_ELEMENTS);
                 return 1;
             }

             // Determine parent from stack
             Element* parent = (element_stack_top >= 0) ? &g_elements[element_index_stack[element_stack_top]] : NULL;
             // Initialize the new element entry
             current_element = &g_elements[g_element_count];
             memset(current_element, 0, sizeof(Element)); // CRITICAL: Zero out the struct, including header.layout
             current_element->self_index = g_element_count;
             current_element->parent_index = (parent) ? parent->self_index : -1;
             current_element->calculated_size = 16; // Base header size, properties/etc added later

             // Determine Element Type based on keyword
             if (strncmp(trimmed, "App {", 5) == 0) {
                 current_element->type = ELEM_TYPE_APP;
                 if (g_has_app || parent) { // Only one App allowed, must be root
                     fprintf(stderr, "Error line %d: Invalid App definition (must be single root).\n", line_num);
                     return 1;
                 }
                 g_has_app = 1; g_header_flags |= FLAG_HAS_APP;
             }
             else if (strncmp(trimmed, "Container {", 11) == 0) { current_element->type = ELEM_TYPE_CONTAINER; }
             else if (strncmp(trimmed, "Text {", 6) == 0)      { current_element->type = ELEM_TYPE_TEXT; }
             else if (strncmp(trimmed, "Image {", 7) == 0)     { current_element->type = ELEM_TYPE_IMAGE; }
             else if (strncmp(trimmed, "Canvas {", 8) == 0)    { current_element->type = ELEM_TYPE_CANVAS; }
             else if (strncmp(trimmed, "Button {", 8) == 0)    { current_element->type = ELEM_TYPE_BUTTON; }
             else if (strncmp(trimmed, "Input {", 7) == 0)     { current_element->type = ELEM_TYPE_INPUT; }
             else if (strncmp(trimmed, "List {", 6) == 0)      { current_element->type = ELEM_TYPE_LIST; }
             else if (strncmp(trimmed, "Grid {", 6) == 0)      { current_element->type = ELEM_TYPE_GRID; }
             else if (strncmp(trimmed, "Scrollable {", 12) == 0){ current_element->type = ELEM_TYPE_SCROLLABLE; }
             else { // Handle potential Custom Element
                 char custom_name[64];
                 if (sscanf(trimmed, "%63s {", custom_name) == 1) {
                     // Consider assigning a specific range or a generic custom type ID
                     current_element->type = 0x31; // Example: Start custom types here
                     current_element->id_string_index = add_string(custom_name); // Store its name
                     fprintf(stderr, "Info line %d: Detected custom element '%s' (Type 0x%02X).\n", line_num, custom_name, current_element->type);
                 } else {
                     fprintf(stderr, "Error line %d: Unrecognized element type or invalid syntax: %s\n", line_num, trimmed);
                     return 1;
                 }
             }

             // Link to parent if applicable
             if (parent) {
                 if (parent->child_count >= MAX_CHILDREN) {
                     fprintf(stderr, "Error line %d: Maximum children (%d) exceeded for parent element %d.\n", line_num, MAX_CHILDREN, parent->self_index);
                     return 1;
                 }
                 parent->children[parent->child_count++] = current_element;
             }

             // Push element onto stack
             element_stack_top++;
             if(element_stack_top >= MAX_ELEMENTS) {
                 fprintf(stderr, "Error line %d: Maximum nesting depth (%d) exceeded.\n", line_num, MAX_ELEMENTS);
                 return 1; // Avoid stack overflow
             }
             element_indent_stack[element_stack_top] = indent;
             element_index_stack[element_stack_top] = g_element_count;

             g_element_count++; // Increment *after* using current count as index
             current_indent = indent; // Set expected indent for properties/children
        }
        // --- Property/Event Parsing Logic ---
        // Check if the line is indented further than the current block start
        else if (indent > current_indent && (current_element != NULL || current_style != NULL)) {

            // *** Determine Context (Style or Element) and Target Pointers ***
            // These pointers will direct where the parsed property gets added.
            KrbProperty* target_props = NULL;
            uint8_t* target_prop_count = NULL;
            uint32_t* target_block_size = NULL;
            bool is_style_context = false;

            if (current_style != NULL) { // Currently parsing inside a style block
                target_props = current_style->properties;
                target_prop_count = &(current_style->property_count);  // FIXED: Use address of property_count
                target_block_size = &(current_style->calculated_size); // FIXED: Use address of calculated_size
                is_style_context = true;
                // Sanity check: element should be NULL when style is active
                if (current_element != NULL) {
                    fprintf(stderr, "FATAL INTERNAL Error line %d: Both current_style and current_element are non-NULL during property parsing.\n", line_num);
                    exit(EXIT_FAILURE); // Should not happen
                }
            } else if (current_element != NULL) { // Currently parsing inside an element block
                target_props = current_element->properties;
                target_prop_count = &(current_element->property_count); // FIXED: Use address of property_count
                target_block_size = &(current_element->calculated_size); // FIXED: Use address of calculated_size
                is_style_context = false;
            } else {
                // Should not be reached if logic above is correct
                fprintf(stderr, "FATAL INTERNAL Error line %d: Property parsing entered but no active context (style/element).\n", line_num);
                exit(EXIT_FAILURE);
            }

            // Basic null check on resolved target pointers
            if (target_props == NULL || target_prop_count == NULL || target_block_size == NULL) {
                fprintf(stderr, "FATAL INTERNAL Error line %d: Target pointers for property storage are NULL.\n", line_num);
                exit(EXIT_FAILURE); // Or return 1
            }
            // *** End Context Determination ***

            // --- Parse the 'key: value' line ---
            char key[64], value_str[MAX_LINE_LENGTH - 64];
            if (sscanf(trimmed, "%63[^:]:%[^\n]", key, value_str) == 2) {
                // Trim whitespace from key and value start
                char* key_end = key + strlen(key) - 1; while (key_end >= key && isspace((unsigned char)*key_end)) *key_end-- = '\0';
                char* val_start = value_str; while (*val_start && isspace((unsigned char)*val_start)) val_start++;

                // --- Process Property/Event based on Key ---
                // Properties/events are added to the 'target_*' context determined above.

                // == Element Header Fields (Only valid in Element context) ==
                 if (!is_style_context && strcmp(key, "id") == 0 ) { current_element->id_string_index = add_string(val_start); }
                 else if (!is_style_context && strcmp(key, "pos_x") == 0 ) { current_element->pos_x = (uint16_t)atoi(val_start); }
                 else if (!is_style_context && strcmp(key, "pos_y") == 0 ) { current_element->pos_y = (uint16_t)atoi(val_start); }
                 else if (!is_style_context && strcmp(key, "width") == 0 ) { current_element->width = (uint16_t)atoi(val_start); }
                 else if (!is_style_context && strcmp(key, "height") == 0 ) { current_element->height = (uint16_t)atoi(val_start); }
else if (!is_style_context && strcmp(key, "style") == 0 ) {
    current_element->style_id = find_style_id_by_name(val_start);
    if (current_element->style_id > 0) {
        int style_idx = current_element->style_id - 1;
        for (int j = 0; j < g_styles[style_idx].property_count; j++) {
            KrbProperty* prop = &g_styles[style_idx].properties[j];
            if (prop->property_id == PROP_ID_LAYOUT_FLAGS && prop->value_type == VAL_TYPE_BYTE && prop->size == 1) {
                current_element->layout = *(uint8_t*)prop->value; // Apply layout from style to header
                fprintf(stderr, "DEBUG: Applied layout 0x%02X from style %d to element %d\n",
                        current_element->layout, style_idx, current_element->self_index);
            }
        }
    }
}

                // == Layout Property (Valid in both Style and Element context) ==
                else if (strcmp(key, "layout") == 0) {
                    uint8_t b = 0; // The layout byte to calculate
                    const char* layout_str = val_start;
                    if (!layout_str) {
                        fprintf(stderr, "Error line %d: Null or empty layout string encountered.\n", line_num);
                        continue; // Skip this invalid property
                    }

                    // Parse Direction (Bits 0-1)
                    if (strstr(layout_str, "col_rev")) { b |= 3; } // 11 = ColumnReverse
                    else if (strstr(layout_str, "row_rev")) { b |= 2; } // 10 = RowReverse
                    else if (strstr(layout_str, "col"))     { b |= 1; } // 01 = Column
                    else { /* Default */                    b |= 0; } // 00 = Row

                    // Parse Alignment (Bits 2-3)
                    if (strstr(layout_str, "space_between")) { b |= (3 << 2); } // 11 = SpaceBetween
                    else if (strstr(layout_str, "end"))      { b |= (2 << 2); } // 10 = End
                    else if (strstr(layout_str, "center"))   { b |= (1 << 2); } // 01 = Center
                    else { /* Default */                       b |= (0 << 2); } // 00 = Start

                    // Parse Optional Flags
                    if (strstr(layout_str, "wrap"))     { b |= LAYOUT_WRAP_BIT; }     // Bit 4
                    if (strstr(layout_str, "grow"))     { b |= LAYOUT_GROW_BIT; }     // Bit 5
                    if (strstr(layout_str, "absolute")) { b |= LAYOUT_ABSOLUTE_BIT; } // Bit 6

                    // Add the calculated byte 'b' as a PROPERTY to the current context
                    add_property_to_list(target_props, target_prop_count, target_block_size,
                                         PROP_ID_LAYOUT_FLAGS, VAL_TYPE_BYTE, 1, &b);

                    // --- DEBUG PRINT ---
                    fprintf(stderr, "DEBUG Compiler L%d: Added PROP_ID_LAYOUT_FLAGS (0x%02X) with value 0x%02X to %s context.\n",
                            line_num, PROP_ID_LAYOUT_FLAGS, b, is_style_context ? "Style" : "Element");
                    // --- END DEBUG PRINT ---
                }

                // == Event Handling (Only valid in Element context) ==
                 else if (!is_style_context && strcmp(key, "onClick") == 0 ) {
                     if (current_element->event_count < MAX_EVENTS) {
                         uint8_t callback_index = add_string(val_start); // Get string index for callback name
                         current_element->events[current_element->event_count].event_type = EVENT_TYPE_CLICK;
                         current_element->events[current_element->event_count].callback_id = callback_index;
                         current_element->event_count++;
                         // Size for events is added at the end of the element block ('}')
                     } else {
                         fprintf(stderr,"Warning line %d: Maximum event count (%d) reached for element %d.\n", line_num, MAX_EVENTS, current_element->self_index);
                     }
                 }
                 // Add other event handlers (onPress, onHover, etc.) here...

                // == Visual Properties (Valid in both Style and Element context) ==
                 else if (strcmp(key, "background_color") == 0) {
                     uint8_t c[4]; // RGBA
                     if(parse_color(val_start, c)) {
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BG_COLOR, VAL_TYPE_COLOR, 4, c);
                         g_header_flags |= FLAG_EXTENDED_COLOR; // Mark that RGBA is used
                     } else {
                         fprintf(stderr,"Warning line %d: Invalid background_color format: '%s'. Expected #RRGGBB or #RRGGBBAA.\n", line_num, val_start);
                     }
                 }
                 else if (strcmp(key, "foreground_color") == 0 || strcmp(key, "text_color") == 0) {
                      uint8_t c[4];
                     if(parse_color(val_start, c)) {
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_FG_COLOR, VAL_TYPE_COLOR, 4, c);
                         g_header_flags |= FLAG_EXTENDED_COLOR;
                     } else {
                         fprintf(stderr,"Warning line %d: Invalid foreground_color/text_color format: '%s'.\n", line_num, val_start);
                     }
                 }
                 else if (strcmp(key, "border_color") == 0) {
                     uint8_t c[4];
                     if(parse_color(val_start, c)) {
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BORDER_COLOR, VAL_TYPE_COLOR, 4, c);
                         g_header_flags |= FLAG_EXTENDED_COLOR;
                     } else {
                         fprintf(stderr,"Warning line %d: Invalid border_color format: '%s'.\n", line_num, val_start);
                     }
                 }
                 else if (strcmp(key, "border_width") == 0) {
                     // Check if it contains spaces, indicating multiple values (EdgeInsets)
                     if(strchr(val_start,' ')) {
                         uint8_t widths[4]; // top, right, bottom, left
                         if(sscanf(val_start,"%hhu %hhu %hhu %hhu", &widths[0], &widths[1], &widths[2], &widths[3]) == 4) {
                             add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BORDER_WIDTH, VAL_TYPE_EDGEINSETS, 4, widths);
                         } else {
                             fprintf(stderr,"Warning line %d: Invalid border_width format for EdgeInsets (expected 4 numbers): '%s'.\n", line_num, val_start);
                         }
                     } else { // Single value (uniform width)
                         uint8_t width = (uint8_t)atoi(val_start);
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BORDER_WIDTH, VAL_TYPE_BYTE, 1, &width);
                     }
                 }
                 // Add other shared visual properties (padding, margin, border_radius, opacity, etc.)...

                // == Text Content (Only valid in specific Element types) ==
                 else if (!is_style_context && (current_element->type == ELEM_TYPE_TEXT || current_element->type == ELEM_TYPE_BUTTON || current_element->type == ELEM_TYPE_INPUT) && strcmp(key, "text") == 0) {
                     add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_TEXT_CONTENT, val_start);
                 }

                // == Text Styling Properties (Valid in specific Element types OR Style context) ==
                 else if ( (!is_style_context && (current_element->type == ELEM_TYPE_TEXT || current_element->type == ELEM_TYPE_BUTTON || current_element->type == ELEM_TYPE_INPUT)) || is_style_context ) {
                     if (strcmp(key, "text_alignment") == 0) {
                         uint8_t alignment_enum = 0; // Default: Start/Left
                         if(strstr(val_start,"cen"))      alignment_enum = 1; // Center
                         else if(strstr(val_start,"rig") || strstr(val_start,"end")) alignment_enum = 2; // End/Right
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_TEXT_ALIGNMENT, VAL_TYPE_ENUM, 1, &alignment_enum);
                     }
                     else if (strcmp(key, "font_size") == 0) {
                         uint16_t size = (uint16_t)atoi(val_start);
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_FONT_SIZE, VAL_TYPE_SHORT, 2, &size);
                     }
                     else if (strcmp(key, "font_weight") == 0) {
                         // Simple check for now, could parse numbers (100-900)
                         uint16_t weight = 400; // Default: Normal
                         if(strstr(val_start,"bold")) weight = 700; // Bold
                         // Could add checks for "light", "medium", etc.
                         add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_FONT_WEIGHT, VAL_TYPE_SHORT, 2, &weight);
                     }
                     else {
                         // If the key didn't match any text styling props, fall through
                         goto check_other_properties;
                     }
                 }
                 else { // Fallthrough point if not a text-style prop OR wrong element type
                 check_other_properties:

                    // == Image Source (Only valid for Image Element type) ==
                     if (!is_style_context && current_element->type == ELEM_TYPE_IMAGE && strcmp(key, "source") == 0) {
                         // Assuming source is a path string for now
                         add_resource_path_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_IMAGE_SOURCE, val_start);
                         // Need resource handling logic if FLAG_HAS_RESOURCES is intended
                     }

                    // == App Specific Properties (Only valid for App Element type) ==
                     else if (!is_style_context && current_element->type == ELEM_TYPE_APP) {
                         if (strcmp(key, "window_width") == 0) { uint16_t v=(uint16_t)atoi(val_start); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_WINDOW_WIDTH, VAL_TYPE_SHORT, 2, &v); }
                         else if (strcmp(key, "window_height") == 0) { uint16_t v=(uint16_t)atoi(val_start); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_WINDOW_HEIGHT, VAL_TYPE_SHORT, 2, &v); }
                         else if (strcmp(key, "window_title") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_WINDOW_TITLE, val_start); }
                         else if (strcmp(key, "resizable") == 0) { uint8_t v=(strstr(val_start,"true")!=NULL); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_RESIZABLE, VAL_TYPE_BYTE, 1, &v); }
                         else if (strcmp(key, "keep_aspect") == 0) { uint8_t v=(strstr(val_start,"true")!=NULL); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_KEEP_ASPECT, VAL_TYPE_BYTE, 1, &v); }
                         else if (strcmp(key, "scale_factor") == 0) {
                             float s=atof(val_start);
                             uint16_t fixed_point = (uint16_t)(s * 256.0f + 0.5f); // Convert float to 8.8 fixed-point
                             add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_SCALE_FACTOR, VAL_TYPE_PERCENTAGE, 2, &fixed_point);
                             g_header_flags |= FLAG_FIXED_POINT; // Mark that fixed-point is used
                          }
                         else if (strcmp(key, "icon") == 0) { add_resource_path_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_ICON, val_start); }
                         else if (strcmp(key, "version") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_VERSION, val_start); }
                         else if (strcmp(key, "author") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_AUTHOR, val_start); }
                         else {
                             fprintf(stderr, "Warning line %d: Unhandled App-specific property '%s'.\n", line_num, key);
                         }
                     }
                    // == Final Catch-all for Unhandled Properties ==
                     else {
                         fprintf(stderr, "Warning line %d: Unhandled property '%s' encountered in %s context.\n",
                                 line_num, key, is_style_context ? "Style" : "Element");
                     }
                 } // End main property chain `if/else if`

            } else { // sscanf failed to find "key: value" format
                 fprintf(stderr, "Error line %d: Invalid property syntax (expected 'key: value'): '%s'\n", line_num, trimmed);
                 return 1; // Syntax error
            }
        } else if (trimmed[0] != '\0') { // Line is not empty, not a comment, not start/end block, not property...
             // Indicates unexpected syntax or incorrect indentation level
             fprintf(stderr, "Error line %d: Unexpected syntax or incorrect indentation: '%s' (Indent %d vs Expected %d)\n",
                     line_num, trimmed, indent, current_indent);
             return 1; // Parsing failed
        }
    } // End while loop (reading lines)

    // --- End of File Checks ---
    if (element_stack_top != -1) {
        fprintf(stderr, "Error: Reached end of file with unclosed element block(s). Last element index on stack: %d\n", element_index_stack[element_stack_top]);
        return 1;
    }
    if (current_style) {
        fprintf(stderr, "Error: Reached end of file with unclosed style block.\n");
        return 1;
    }
    // Check if App element is correctly placed if present
    if (g_has_app && g_element_count > 0 && g_elements[0].type != ELEM_TYPE_APP) {
        fprintf(stderr, "Internal Compiler Error: App element was defined but is not the first element (Index 0).\n");
        return 1;
    }

    // If we reach here, parsing was successful
    return 0;
}

// --- Pass 2: Writing the KRB File ---
int write_krb_file(FILE* out) {
    // --- 1. Calculate Offsets ---
    uint32_t header_size = 38;
    uint32_t element_section_offset = header_size;
    uint32_t current_offset = element_section_offset;
    uint32_t style_section_offset = 0;
    uint32_t animation_section_offset = 0;
    uint32_t string_section_offset = 0;
    uint32_t resource_section_offset = 0;
    uint32_t total_size = 0;

    // Elements
    for (int i = 0; i < g_element_count; i++) { g_elements[i].absolute_offset = current_offset; if(g_elements[i].calculated_size < 16) { fprintf(stderr, "IntErr: Elem %d size %u<16\n",i,g_elements[i].calculated_size); return 1; } current_offset += g_elements[i].calculated_size; }
    // Styles
    if (g_style_count > 0) { style_section_offset = current_offset; g_header_flags |= FLAG_HAS_STYLES; for (int i = 0; i < g_style_count; i++) { if(g_styles[i].calculated_size < 3) { fprintf(stderr, "IntErr: Style %d size %u<3\n",i,g_styles[i].calculated_size); return 1; } current_offset += g_styles[i].calculated_size; } } else { style_section_offset = current_offset; g_header_flags &= ~FLAG_HAS_STYLES; }
    // Animations
    if (g_animation_count > 0) { animation_section_offset = current_offset; g_header_flags |= FLAG_HAS_ANIMATIONS; /* current_offset += ... */ } else { animation_section_offset = current_offset; g_header_flags &= ~FLAG_HAS_ANIMATIONS; }
    // Strings
    if (g_string_count > 0) { string_section_offset = current_offset; current_offset += 2; for (int i = 0; i < g_string_count; i++) { if (g_strings[i].length > 255) { fprintf(stderr, "Err: Str %d len %zu>255\n",i,g_strings[i].length); return 1; } current_offset += 1 + g_strings[i].length; } } else { string_section_offset = current_offset; }
    // Resources
    if (g_resource_count > 0) { resource_section_offset = current_offset; g_header_flags |= FLAG_HAS_RESOURCES; /* current_offset += ... */ } else { resource_section_offset = current_offset; g_header_flags &= ~FLAG_HAS_RESOURCES; }
    total_size = current_offset;

    // --- 2. Write Header ---
    rewind(out);
    fwrite(KRB_MAGIC, 1, 4, out); write_u16(out, (KRB_VERSION_MINOR << 8)|KRB_VERSION_MAJOR); write_u16(out, g_header_flags); write_u16(out, (uint16_t)g_element_count); write_u16(out, (uint16_t)g_style_count); write_u16(out, (uint16_t)g_animation_count); write_u16(out, (uint16_t)g_string_count); write_u16(out, (uint16_t)g_resource_count); write_u32(out, element_section_offset); write_u32(out, style_section_offset); write_u32(out, animation_section_offset); write_u32(out, string_section_offset); write_u32(out, total_size);
    if (ftell(out) != header_size) { fprintf(stderr, "IntErr: Header write size %ld!=%u\n", ftell(out), header_size); return 1; }

    // --- 3. Write Element Blocks ---
    if (fseek(out, element_section_offset, SEEK_SET) != 0) { perror("Seek fail Elem"); return 1; }
    for (int i = 0; i < g_element_count; i++) {
        Element* el = &g_elements[i]; uint32_t start = ftell(out); if (start != el->absolute_offset) { fprintf(stderr, "IntErr: Elem %d offset %u!=%u\n",i,start,el->absolute_offset); return 1; }
        write_u8(out, el->type); write_u8(out, el->id_string_index); write_u16(out, el->pos_x); write_u16(out, el->pos_y); write_u16(out, el->width); write_u16(out, el->height); write_u8(out, el->layout); write_u8(out, el->style_id); write_u8(out, el->property_count); write_u8(out, el->child_count); write_u8(out, el->event_count); write_u8(out, el->animation_count);
        for (int j=0; j<el->property_count; j++) { KrbProperty* p=&el->properties[j]; write_u8(out,p->property_id); write_u8(out,p->value_type); write_u8(out,p->size); if (p->size>0 && fwrite(p->value,1,p->size,out)!=p->size) { perror("Write Elem Prop"); return 1; } }
        for (int j=0; j<el->event_count; j++) { write_u8(out,el->events[j].event_type); write_u8(out,el->events[j].callback_id); }
        /* TODO: Write Anim Refs */
        for (int j=0; j<el->child_count; j++) { Element* c=el->children[j]; if(!c) { fprintf(stderr,"IntErr: Elem %d null child %d\n",i,j); return 1; } uint32_t off=c->absolute_offset-el->absolute_offset; if(off>0xFFFF){ fprintf(stderr,"Err: Elem %d child %d offset %u>16bit\n",i,j,off); return 1;} write_u16(out,(uint16_t)off); }
        if ((uint32_t)ftell(out)-start != el->calculated_size) { fprintf(stderr, "IntErr: Elem %d write size %u!=%u\n", i, (unsigned int)(ftell(out)-start), el->calculated_size); return 1; }
    }

    // --- 4. Write Style Blocks ---
    if (g_style_count > 0) {
         if (ftell(out) != style_section_offset) { fprintf(stderr, "IntErr: Style offset %ld!=%u\n",ftell(out),style_section_offset); return 1; }
        for (int i=0; i<g_style_count; i++) {
            StyleEntry* st=&g_styles[i]; uint32_t start=ftell(out);
            write_u8(out,st->id); write_u8(out,st->name_index); write_u8(out,st->property_count);
            for (int j=0; j<st->property_count; j++) { KrbProperty* p=&st->properties[j]; write_u8(out,p->property_id); write_u8(out,p->value_type); write_u8(out,p->size); if (p->size>0 && fwrite(p->value,1,p->size,out)!=p->size) { perror("Write Style Prop"); return 1; } }
            if ((uint32_t)ftell(out)-start != st->calculated_size) { fprintf(stderr, "IntErr: Style %d write size %u!=%u\n", i, (unsigned int)(ftell(out)-start), st->calculated_size); return 1; }
        }
    }

    // --- 5. Write Animation Table ---
    /* TODO */

    // --- 6. Write String Table ---
     if (g_string_count > 0) {
         if (ftell(out) != string_section_offset) { fprintf(stderr, "IntErr: String offset %ld!=%u\n",ftell(out),string_section_offset); return 1; }
        write_u16(out,(uint16_t)g_string_count);
        for (int i=0; i<g_string_count; i++) { StringEntry* s=&g_strings[i]; write_u8(out,(uint8_t)s->length); if (s->length>0 && fwrite(s->text,1,s->length,out)!=s->length) { perror("Write Str data"); return 1; } }
    }

    // --- 7. Write Resource Table ---
    /* TODO */

    // --- Final Size Check ---
    long final_pos = ftell(out); if (final_pos<0) { perror("Final ftell"); return 1; }
    if ((uint32_t)final_pos != total_size) { fprintf(stderr, "IntErr: Final size %ld!=%u\n", final_pos, total_size); return 1; }

    return 0; // Success
}


// --- Main Function ---
int main(int argc, char* argv[]) {
    if (argc != 3) { fprintf(stderr, "Usage: %s <input.kry> <output.krb>\n", argv[0]); return 1; }
    const char* input_file = argv[1]; const char* output_file = argv[2];

    // *** Initialize Global Arrays ***
    memset(g_elements, 0, sizeof(g_elements));
    memset(g_styles, 0, sizeof(g_styles));
    memset(g_strings, 0, sizeof(g_strings));
    // *** End Initialization ***

    FILE* in = fopen(input_file, "r"); if (!in) { fprintf(stderr, "Error opening input '%s': %s\n", input_file, strerror(errno)); return 1; }
    FILE* out = fopen(output_file, "wb+"); if (!out) { fprintf(stderr, "Error opening output '%s': %s\n", output_file, strerror(errno)); fclose(in); return 1; }

    printf("Compiling '%s' to '%s'...\n", input_file, output_file);

    printf("Pass 1: Parsing and calculating sizes...\n");
    if (parse_and_calculate_sizes(in) != 0) {
        fprintf(stderr, "Compilation failed during Pass 1.\n"); fclose(in); fclose(out); cleanup_resources(); remove(output_file); return 1;
    }
    printf("   Found %d elements, %d styles, %d strings.\n", g_element_count, g_style_count, g_string_count);

    printf("Pass 2: Writing binary file...\n");
    if (write_krb_file(out) != 0) {
        fprintf(stderr, "Compilation failed during Pass 2.\n"); fclose(in); fclose(out); cleanup_resources(); remove(output_file); return 1;
    }

    long final_size = ftell(out); if (final_size<0) { perror("Final size ftell"); final_size=0; }
    printf("Compilation successful. Output size: %ld bytes.\n", final_size);

    fclose(in); if (fflush(out)!=0) { perror("Flush output"); } fclose(out); cleanup_resources();
    return 0;
}