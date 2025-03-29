#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>
#include <errno.h> // For error reporting
#include <stdbool.h> // For bool type

// --- Constants based on KRB v0.2 Specification ---
#define KRB_MAGIC "KRB1"
#define KRB_VERSION_MAJOR 0 // <-- Updated
#define KRB_VERSION_MINOR 2 // <-- Updated

// Header Flags (Unchanged)
#define FLAG_HAS_STYLES     (1 << 0)
#define FLAG_HAS_ANIMATIONS (1 << 1)
#define FLAG_HAS_RESOURCES  (1 << 2)
#define FLAG_COMPRESSED     (1 << 3) // Not implemented
#define FLAG_FIXED_POINT    (1 << 4) // Must be set if VAL_TYPE_PERCENTAGE used
#define FLAG_EXTENDED_COLOR (1 << 5) // Affects VAL_TYPE_COLOR
#define FLAG_HAS_APP        (1 << 6)
// Bits 7-15 Reserved

// Element Types (Unchanged)
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

// Property IDs (Unchanged logic, but note PROP_ID_ICON uses Resource Index now)
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
#define PROP_ID_IMAGE_SOURCE    0x0C // Uses Resource Index
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
#define PROP_ID_ICON            0x26 // Uses Resource Index <-- Updated usage
#define PROP_ID_VERSION         0x27
#define PROP_ID_AUTHOR          0x28
// 0x29 - 0xFF Reserved

// Value Types (Unchanged)
#define VAL_TYPE_NONE       0x00
#define VAL_TYPE_BYTE       0x01
#define VAL_TYPE_SHORT      0x02
#define VAL_TYPE_COLOR      0x03 // Interpretation depends on FLAG_EXTENDED_COLOR
#define VAL_TYPE_STRING     0x04 // Index (0-based) to string table (1 byte)
#define VAL_TYPE_RESOURCE   0x05 // Index (0-based) to resource table (1 byte)
#define VAL_TYPE_PERCENTAGE 0x06 // Requires FLAG_FIXED_POINT, 2 bytes 8.8
#define VAL_TYPE_RECT       0x07 // x,y,w,h (e.g., 4 shorts = 8 bytes)
#define VAL_TYPE_EDGEINSETS 0x08 // top,right,bottom,left (e.g., 4 bytes)
#define VAL_TYPE_ENUM       0x09 // Predefined options (1 byte usually)
#define VAL_TYPE_VECTOR     0x0A // x,y coords (e.g., 2 shorts = 4 bytes)
#define VAL_TYPE_CUSTOM     0x0B // Depends on context
// 0x0C - 0xFF Reserved

// Event Types (Unchanged)
#define EVENT_TYPE_NONE     0x00
#define EVENT_TYPE_CLICK    0x01
#define EVENT_TYPE_PRESS    0x02
#define EVENT_TYPE_RELEASE  0x03
// ... add others as needed

// Layout Byte Bits (Unchanged)
#define LAYOUT_DIRECTION_MASK 0x03 // Bits 0-1: 00=Row, 01=Col, 10=RowRev, 11=ColRev
#define LAYOUT_ALIGNMENT_MASK 0x0C // Bits 2-3: 00=Start, 01=Center, 10=End, 11=SpaceBetween
#define LAYOUT_WRAP_BIT       (1 << 4) // Bit 4: 0=NoWrap, 1=Wrap
#define LAYOUT_GROW_BIT       (1 << 5) // Bit 5: 0=Fixed, 1=Grow
#define LAYOUT_ABSOLUTE_BIT   (1 << 6) // Bit 6: 0=Flow, 1=Absolute
// Bit 7 Reserved

// Resource Types (Added)
#define RES_TYPE_NONE       0x00
#define RES_TYPE_IMAGE      0x01
#define RES_TYPE_FONT       0x02
#define RES_TYPE_SOUND      0x03
#define RES_TYPE_VIDEO      0x04
#define RES_TYPE_CUSTOM     0x05
// 0x06 - 0xFF Reserved

// Resource Formats (Added)
#define RES_FORMAT_EXTERNAL 0x00 // Data is string index to path/URL
#define RES_FORMAT_INLINE   0x01 // Data includes size + raw bytes (Not implemented here)

// Limits
#define MAX_ELEMENTS 256
#define MAX_STRINGS 256
#define MAX_PROPERTIES 64
#define MAX_STYLES 64
#define MAX_CHILDREN 128
#define MAX_EVENTS 16
#define MAX_ANIM_REFS 16
#define MAX_LINE_LENGTH 512
#define MAX_ANIMATIONS 64 // Placeholder
#define MAX_RESOURCES 64 // Updated limit

// --- Data Structures ---

typedef struct {
    uint8_t property_id;
    uint8_t value_type;
    uint8_t size;
    void* value;
} KrbProperty;

typedef struct {
    uint8_t event_type;
    uint8_t callback_id; // String table index (0-based)
} KrbEvent;

typedef struct Element {
    // Header Data
    uint8_t type;
    uint8_t id_string_index; // 0-based index
    uint16_t pos_x;
    uint16_t pos_y;
    uint16_t width;
    uint16_t height;
    uint8_t layout; // Effective layout
    uint8_t style_id; // 1-based index
    uint8_t property_count;
    uint8_t child_count;
    uint8_t event_count;
    uint8_t animation_count;

    // Compiler-Internal
    KrbProperty properties[MAX_PROPERTIES];
    KrbEvent events[MAX_EVENTS];
    struct Element* children[MAX_CHILDREN];
    int parent_index;
    int self_index;

    // Pass 2 Data
    uint32_t calculated_size;
    uint32_t absolute_offset;
} Element;

typedef struct {
    char* text;
    size_t length;
    uint8_t index; // 0-based index
} StringEntry;

typedef struct {
    uint8_t id; // 1-based ID
    uint8_t name_index; // 0-based index
    KrbProperty properties[MAX_PROPERTIES];
    uint8_t property_count;
    uint32_t calculated_size;
} StyleEntry;

// Resource Entry Structure (Added)
typedef struct {
    uint8_t type;
    uint8_t name_index; // String index for name/path
    uint8_t format;
    uint8_t data_string_index; // For external: index into string table for path

    // Compiler-Internal
    uint8_t index; // 0-based index in resource table
    uint32_t calculated_size; // Size of this entry in the file
} ResourceEntry;

// TODO: Add Animation structure

// --- Global Compiler State ---
Element g_elements[MAX_ELEMENTS];
StringEntry g_strings[MAX_STRINGS];
StyleEntry g_styles[MAX_STYLES];
ResourceEntry g_resources[MAX_RESOURCES]; // Added
// TODO: Global animation array
int g_element_count = 0;
int g_string_count = 0;
int g_style_count = 0;
int g_animation_count = 0; // Placeholder
int g_resource_count = 0;  // Will be used
int g_has_app = 0;
uint16_t g_header_flags = 0;

// --- Utility Functions ---

void write_u8(FILE* file, uint8_t value) {
    if (fputc(value, file) == EOF) { perror("Error writing u8"); exit(EXIT_FAILURE); }
}

void write_u16(FILE* file, uint16_t value) {
    if (fputc(value & 0xFF, file) == EOF || fputc((value >> 8) & 0xFF, file) == EOF) {
        perror("Error writing u16"); exit(EXIT_FAILURE);
    }
}

void write_u32(FILE* file, uint32_t value) {
    if (fputc(value & 0xFF, file) == EOF || fputc((value >> 8) & 0xFF, file) == EOF ||
        fputc((value >> 16) & 0xFF, file) == EOF || fputc((value >> 24) & 0xFF, file) == EOF) {
        perror("Error writing u32"); exit(EXIT_FAILURE);
    }
}

// add_string - unchanged from previous version
uint8_t add_string(const char* text) {
    if (!text) return 0;
    const char *start = text;
    const char *end = text + strlen(text) - 1;
    while (start <= end && isspace((unsigned char)*start)) start++;
    while (end >= start && isspace((unsigned char)*end)) end--;
    if (end >= start && *start == '"' && *end == '"') { start++; end--; }
    size_t len = (end < start) ? 0 : (end - start + 1);
    char clean_text_buf[MAX_LINE_LENGTH];
    if (len >= sizeof(clean_text_buf)) { fprintf(stderr, "Error: Cleaned string too long: %zu chars\n", len); exit(EXIT_FAILURE); }
    strncpy(clean_text_buf, start, len); clean_text_buf[len] = '\0';

    for (int i = 0; i < g_string_count; i++) {
        if (g_strings[i].text && strcmp(g_strings[i].text, clean_text_buf) == 0) {
            return g_strings[i].index; // 0-based index
        }
    }
    if (g_string_count >= MAX_STRINGS) { fprintf(stderr, "Error: Max strings (%d).\n", MAX_STRINGS); exit(EXIT_FAILURE); }
    g_strings[g_string_count].text = strdup(clean_text_buf);
    if (!g_strings[g_string_count].text) { perror("strdup failed"); exit(EXIT_FAILURE); }
    g_strings[g_string_count].length = len;
    g_strings[g_string_count].index = g_string_count; // Assign 0-based index
    return g_string_count++; // Return assigned index, then increment
}


// Add Resource Function (Added/Updated)
uint8_t add_resource(uint8_t resource_type, const char* path_str) {
    if (!path_str) { fprintf(stderr, "Error: Null resource path.\n"); exit(EXIT_FAILURE); }

    uint8_t path_string_index = add_string(path_str);

    // Check duplicates (same type, same path string index for external)
    for (int i = 0; i < g_resource_count; i++) {
        if (g_resources[i].type == resource_type &&
            g_resources[i].format == RES_FORMAT_EXTERNAL && // Only check external for now
            g_resources[i].data_string_index == path_string_index) {
            return g_resources[i].index; // Return existing 0-based index
        }
    }

    if (g_resource_count >= MAX_RESOURCES) { fprintf(stderr, "Error: Max resources (%d).\n", MAX_RESOURCES); exit(EXIT_FAILURE); }

    ResourceEntry* res = &g_resources[g_resource_count];
    res->type = resource_type;
    res->name_index = path_string_index; // Use path as name for external
    res->format = RES_FORMAT_EXTERNAL;   // Assume external for now
    res->data_string_index = path_string_index; // Data is the path string index
    res->index = g_resource_count; // Assign 0-based index

    // Calculate size (External format = 4 bytes)
    res->calculated_size = 1 + 1 + 1 + 1; // Type + NameIdx + Format + DataIdx

    g_header_flags |= FLAG_HAS_RESOURCES; // Ensure flag is set

    return g_resource_count++; // Return assigned index, then increment
}

// find_style_id_by_name - unchanged, returns 1-based ID or 0
uint8_t find_style_id_by_name(const char* name) {
    if (!name) return 0;
    const char *start = name; const char *end = name + strlen(name) - 1;
    while (start <= end && isspace((unsigned char)*start)) start++;
    while (end >= start && isspace((unsigned char)*end)) end--;
    if (end >= start && *start == '"' && *end == '"') { start++; end--; }
    size_t len = (end < start) ? 0 : (end - start + 1);
    char clean_name_buf[MAX_LINE_LENGTH];
    if (len >= sizeof(clean_name_buf)) { fprintf(stderr, "Error: Style name too long.\n"); exit(EXIT_FAILURE); }
    strncpy(clean_name_buf, start, len); clean_name_buf[len] = '\0';

    for (int i = 0; i < g_style_count; i++) {
        if (g_styles[i].name_index < g_string_count && g_strings[g_styles[i].name_index].text &&
            strcmp(g_strings[g_styles[i].name_index].text, clean_name_buf) == 0) {
            return g_styles[i].id; // Return 1-based ID
        }
    }
    return 0; // Not found
}

// cleanup_resources - unchanged
void cleanup_resources() {
    for (int i = 0; i < g_element_count; i++) {
        for (int j = 0; j < g_elements[i].property_count; j++) {
            if (g_elements[i].properties[j].value) { free(g_elements[i].properties[j].value); }
        }
    }
    for (int i = 0; i < g_style_count; i++) {
        for (int j = 0; j < g_styles[i].property_count; j++) {
            if (g_styles[i].properties[j].value) { free(g_styles[i].properties[j].value); }
        }
    }
    for (int i = 0; i < g_string_count; i++) {
         if (g_strings[i].text) { free(g_strings[i].text); }
    }
    // Reset counts - g_resources doesn't need freeing (statically allocated)
    g_element_count = 0; g_string_count = 0; g_style_count = 0;
    g_animation_count = 0; g_resource_count = 0; g_has_app = 0; g_header_flags = 0;
}

// parse_color - unchanged
int parse_color(const char* value_str, uint8_t color_out[4]) {
    color_out[0] = 0; color_out[1] = 0; color_out[2] = 0; color_out[3] = 255; // Default alpha
    if (!value_str) return 0;
    const char* p = value_str; while(isspace((unsigned char)*p)) p++;
    if (*p != '#') return 0; p++;
    size_t len = strlen(p); char* end = (char*)p + len - 1; while(end >= p && isspace((unsigned char)*end)) *end-- = '\0'; len = strlen(p);
    if (len == 8 && sscanf(p, "%2hhx%2hhx%2hhx%2hhx", &color_out[0], &color_out[1], &color_out[2], &color_out[3]) == 4) return 1;
    if (len == 6 && sscanf(p, "%2hhx%2hhx%2hhx", &color_out[0], &color_out[1], &color_out[2]) == 3) return 1; // Use default alpha
    return 0;
}

// add_property_to_list - unchanged
void add_property_to_list(KrbProperty* prop_array, uint8_t* prop_count, uint32_t* current_size,
                           uint8_t prop_id, uint8_t val_type, uint8_t size, const void* data) {
    if (!prop_array || !prop_count || !current_size) { fprintf(stderr, "FATAL: Null pointer in add_prop.\n"); exit(EXIT_FAILURE); }
    if (*prop_count >= MAX_PROPERTIES) { fprintf(stderr, "Error: Max props (%d).\n", MAX_PROPERTIES); exit(EXIT_FAILURE); }
    KrbProperty* p = &prop_array[*prop_count];
    p->property_id = prop_id; p->value_type = val_type; p->size = size;
    if (size > 0) {
        if (!data) { fprintf(stderr, "FATAL: Null data for prop %u size %u.\n", prop_id, size); exit(EXIT_FAILURE); }
        p->value = malloc(size);
        if (!p->value) { perror("malloc prop"); exit(EXIT_FAILURE); }
        memcpy(p->value, data, size);
    } else { p->value = NULL; }
    *current_size += 1 + 1 + 1 + size;
    (*prop_count)++;
}

// add_string_property_to_list - unchanged
void add_string_property_to_list(KrbProperty* prop_array, uint8_t* prop_count, uint32_t* current_size,
                                  uint8_t prop_id, const char* value_str) {
    if (!prop_array || !prop_count || !current_size) { fprintf(stderr, "FATAL: Null pointer in add_string_prop.\n"); exit(EXIT_FAILURE); }
    uint8_t str_index = add_string(value_str); // Get 0-based index
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
    // Reset globals
    g_header_flags = 0; g_resource_count = 0; g_string_count = 0; g_style_count = 0; g_element_count = 0; g_animation_count = 0; g_has_app = 0;

    while (fgets(line, sizeof(line), in)) {
        line_num++;
        char* trimmed = line; int indent = 0;
        while (*trimmed == ' ' || *trimmed == '\t') { indent += (*trimmed == '\t' ? 4 : 1); trimmed++; }
        char* end = trimmed + strlen(trimmed) - 1; while (end >= trimmed && isspace((unsigned char)*end)) *end-- = '\0';
        if (*trimmed == '\0' || *trimmed == '#') continue;

        // End Block Logic
        if (*trimmed == '}') {
            if (current_element && element_stack_top >= 0 && indent == element_indent_stack[element_stack_top]) {
                current_element->calculated_size += current_element->event_count * 2; // Event(1)+Callback(1)
                // current_element->calculated_size += current_element->animation_count * 2; // AnimIdx(1)+Trigger(1)
                current_element->calculated_size += current_element->child_count * 2; // Child Offset(2)
                element_stack_top--;
                current_indent = (element_stack_top >= 0) ? element_indent_stack[element_stack_top] : -1;
                current_element = (element_stack_top >= 0) ? &g_elements[element_index_stack[element_stack_top]] : NULL;
                continue;
            } else if (current_style && indent == current_indent) {
                current_style = NULL; current_indent = -1; continue;
            } else { fprintf(stderr, "L%d: Mismatched '}'.\n", line_num); return 1; }
        }

        // Start New Block Logic
        if (strncmp(trimmed, "style ", 6) == 0 && strstr(trimmed, "{")) {
             if (current_element || current_style) { fprintf(stderr, "L%d: Cannot nest style.\n", line_num); return 1; }
             char style_name[128];
             if (sscanf(trimmed, "style \"%127[^\"]\" {", style_name) == 1) {
                if (g_style_count >= MAX_STYLES) { fprintf(stderr, "L%d: Max styles (%d).\n", line_num, MAX_STYLES); return 1; }
                current_style = &g_styles[g_style_count]; memset(current_style, 0, sizeof(StyleEntry));
                current_style->id = g_style_count + 1; // 1-based ID
                current_style->name_index = add_string(style_name); // 0-based index
                current_style->calculated_size = 1 + 1 + 1; // ID+NameIdx+PropCount
                g_style_count++; current_indent = indent; g_header_flags |= FLAG_HAS_STYLES;
             } else { fprintf(stderr, "L%d: Bad style syntax: %s\n", line_num, trimmed); return 1; }
        }
        else if (isalpha((unsigned char)*trimmed) && strstr(trimmed, "{")) {
             if (current_style) { fprintf(stderr, "L%d: Cannot define element in style.\n", line_num); return 1; }
             if (g_element_count >= MAX_ELEMENTS) { fprintf(stderr, "L%d: Max elements (%d).\n", line_num, MAX_ELEMENTS); return 1; }
             Element* parent = (element_stack_top >= 0) ? &g_elements[element_index_stack[element_stack_top]] : NULL;
             current_element = &g_elements[g_element_count]; memset(current_element, 0, sizeof(Element));
             current_element->self_index = g_element_count; current_element->parent_index = (parent) ? parent->self_index : -1;
             current_element->calculated_size = 16; // Base header size

             // Determine Type
             if (strncmp(trimmed, "App {", 5) == 0) { current_element->type = ELEM_TYPE_APP; if (g_has_app || parent) { fprintf(stderr, "L%d: Invalid App.\n", line_num); return 1; } g_has_app = 1; g_header_flags |= FLAG_HAS_APP; }
             else if (strncmp(trimmed, "Container {", 11) == 0) { current_element->type = ELEM_TYPE_CONTAINER; }
             else if (strncmp(trimmed, "Text {", 6) == 0)      { current_element->type = ELEM_TYPE_TEXT; }
             else if (strncmp(trimmed, "Image {", 7) == 0)     { current_element->type = ELEM_TYPE_IMAGE; }
             else if (strncmp(trimmed, "Canvas {", 8) == 0)    { current_element->type = ELEM_TYPE_CANVAS; }
             else if (strncmp(trimmed, "Button {", 8) == 0)    { current_element->type = ELEM_TYPE_BUTTON; }
             else if (strncmp(trimmed, "Input {", 7) == 0)     { current_element->type = ELEM_TYPE_INPUT; }
             else if (strncmp(trimmed, "List {", 6) == 0)      { current_element->type = ELEM_TYPE_LIST; }
             else if (strncmp(trimmed, "Grid {", 6) == 0)      { current_element->type = ELEM_TYPE_GRID; }
             else if (strncmp(trimmed, "Scrollable {", 12) == 0){ current_element->type = ELEM_TYPE_SCROLLABLE; }
             else { /* Custom Element */ char custom_name[64]; if (sscanf(trimmed, "%63s {", custom_name) == 1) { current_element->type = 0x31; current_element->id_string_index = add_string(custom_name); fprintf(stderr, "L%d: Info: Custom '%s'.\n", line_num, custom_name); } else { fprintf(stderr, "L%d: Bad element type: %s\n", line_num, trimmed); return 1; } }

             if (parent) { if (parent->child_count >= MAX_CHILDREN) { fprintf(stderr, "L%d: Max children.\n", line_num); return 1; } parent->children[parent->child_count++] = current_element; }
             element_stack_top++; if(element_stack_top >= MAX_ELEMENTS) { fprintf(stderr, "L%d: Max depth.\n", line_num); return 1; }
             element_indent_stack[element_stack_top] = indent; element_index_stack[element_stack_top] = g_element_count;
             g_element_count++; current_indent = indent;
        }
        // Property/Event Parsing
        else if (indent > current_indent && (current_element != NULL || current_style != NULL)) {
            KrbProperty* target_props = NULL; uint8_t* target_prop_count = NULL; uint32_t* target_block_size = NULL; bool is_style_context = false;
            if (current_style) { target_props = current_style->properties; target_prop_count = &(current_style->property_count); target_block_size = &(current_style->calculated_size); is_style_context = true; if (current_element) { fprintf(stderr, "FATAL: style/elem active.\n"); exit(EXIT_FAILURE); } }
            else if (current_element) { target_props = current_element->properties; target_prop_count = &(current_element->property_count); target_block_size = &(current_element->calculated_size); is_style_context = false; }
            else { fprintf(stderr, "FATAL: Prop parse no context.\n"); exit(EXIT_FAILURE); }
            if (!target_props || !target_prop_count || !target_block_size) { fprintf(stderr, "FATAL: Target ptrs null.\n"); exit(EXIT_FAILURE); }

            char key[64], value_str[MAX_LINE_LENGTH - 64];
            if (sscanf(trimmed, "%63[^:]:%[^\n]", key, value_str) == 2) {
                char* key_end = key + strlen(key) - 1; while (key_end >= key && isspace((unsigned char)*key_end)) *key_end-- = '\0';
                char* val_start = value_str; while (*val_start && isspace((unsigned char)*val_start)) val_start++;

                bool property_handled = false; // Flag to check if property was processed

                // == Element Header Fields ==
                 if (!is_style_context && strcmp(key, "id") == 0 ) { current_element->id_string_index = add_string(val_start); property_handled = true; }
                 else if (!is_style_context && strcmp(key, "pos_x") == 0 ) { current_element->pos_x = (uint16_t)atoi(val_start); property_handled = true; }
                 else if (!is_style_context && strcmp(key, "pos_y") == 0 ) { current_element->pos_y = (uint16_t)atoi(val_start); property_handled = true; }
                 else if (!is_style_context && strcmp(key, "width") == 0 ) { current_element->width = (uint16_t)atoi(val_start); property_handled = true; }
                 else if (!is_style_context && strcmp(key, "height") == 0 ) { current_element->height = (uint16_t)atoi(val_start); property_handled = true; }
                 else if (!is_style_context && strcmp(key, "style") == 0 ) { current_element->style_id = find_style_id_by_name(val_start); /* TODO: Apply layout from style? */ property_handled = true; }

                // == Layout Property ==
                else if (strcmp(key, "layout") == 0) {
                    uint8_t b = 0; const char* s = val_start; if (!s) { fprintf(stderr, "L%d: Empty layout.\n", line_num); continue; }
                    if (strstr(s, "col_rev")) b |= 3; else if (strstr(s, "row_rev")) b |= 2; else if (strstr(s, "col")) b |= 1; else b |= 0;
                    if (strstr(s, "space_between")) b |= (3 << 2); else if (strstr(s, "end")) b |= (2 << 2); else if (strstr(s, "center")) b |= (1 << 2); else b |= (0 << 2);
                    if (strstr(s, "wrap")) b |= LAYOUT_WRAP_BIT; if (strstr(s, "grow")) b |= LAYOUT_GROW_BIT; if (strstr(s, "absolute")) b |= LAYOUT_ABSOLUTE_BIT;
                    add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_LAYOUT_FLAGS, VAL_TYPE_BYTE, 1, &b);
                    if (!is_style_context) current_element->layout = b; // Update header byte
                    property_handled = true;
                }
                // == Event Handling ==
                 else if (!is_style_context && strcmp(key, "onClick") == 0 ) { if (current_element->event_count < MAX_EVENTS) { uint8_t cb_idx = add_string(val_start); current_element->events[current_element->event_count++] = (KrbEvent){.event_type=EVENT_TYPE_CLICK, .callback_id=cb_idx}; } else { fprintf(stderr,"L%d: Max events.\n",line_num); } property_handled = true; }
                 // ... other events ...

                // == Visual Properties ==
                 else if (strcmp(key, "background_color") == 0) { uint8_t c[4]; if(parse_color(val_start, c)) { add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BG_COLOR, VAL_TYPE_COLOR, 4, c); g_header_flags |= FLAG_EXTENDED_COLOR; } else { fprintf(stderr,"L%d: Bad bg color: %s\n", line_num, val_start); } property_handled = true; }
                 else if (strcmp(key, "foreground_color") == 0 || strcmp(key, "text_color") == 0) { uint8_t c[4]; if(parse_color(val_start, c)) { add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_FG_COLOR, VAL_TYPE_COLOR, 4, c); g_header_flags |= FLAG_EXTENDED_COLOR; } else { fprintf(stderr,"L%d: Bad fg color: %s\n", line_num, val_start); } property_handled = true; }
                 else if (strcmp(key, "border_color") == 0) { uint8_t c[4]; if(parse_color(val_start, c)) { add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BORDER_COLOR, VAL_TYPE_COLOR, 4, c); g_header_flags |= FLAG_EXTENDED_COLOR; } else { fprintf(stderr,"L%d: Bad border color: %s\n", line_num, val_start); } property_handled = true; }
                 else if (strcmp(key, "border_width") == 0) { if(strchr(val_start,' ')) { uint8_t w[4]; if(sscanf(val_start,"%hhu %hhu %hhu %hhu",&w[0],&w[1],&w[2],&w[3])==4) { add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BORDER_WIDTH, VAL_TYPE_EDGEINSETS, 4, w); } else { fprintf(stderr,"L%d: Bad border width edge: %s\n", line_num, val_start); } } else { uint8_t w=(uint8_t)atoi(val_start); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_BORDER_WIDTH, VAL_TYPE_BYTE, 1, &w); } property_handled = true; }
                 // ... other visual props ...

                // == Text Content ==
                 else if (!is_style_context && (current_element->type == ELEM_TYPE_TEXT || current_element->type == ELEM_TYPE_BUTTON || current_element->type == ELEM_TYPE_INPUT) && strcmp(key, "text") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_TEXT_CONTENT, val_start); property_handled = true; }

                // == Text Styling ==
                 else if ( (!is_style_context && (current_element->type == ELEM_TYPE_TEXT || current_element->type == ELEM_TYPE_BUTTON || current_element->type == ELEM_TYPE_INPUT)) || is_style_context ) {
                     if (strcmp(key, "text_alignment") == 0) { uint8_t align=0; if(strstr(val_start,"cen")) align=1; else if(strstr(val_start,"rig")||strstr(val_start,"end")) align=2; add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_TEXT_ALIGNMENT, VAL_TYPE_ENUM, 1, &align); property_handled = true; }
                     else if (strcmp(key, "font_size") == 0) { uint16_t sz=(uint16_t)atoi(val_start); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_FONT_SIZE, VAL_TYPE_SHORT, 2, &sz); property_handled = true; }
                     else if (strcmp(key, "font_weight") == 0) { uint16_t w=400; if(strstr(val_start,"bold")) w=700; add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_FONT_WEIGHT, VAL_TYPE_SHORT, 2, &w); property_handled = true; }
                 }

                 // == Resource Handling ==
                 // Image Source
                 if (!is_style_context && current_element->type == ELEM_TYPE_IMAGE && (strcmp(key, "image_source") == 0 || strcmp(key, "source") == 0) ) {
                     uint8_t resource_index = add_resource(RES_TYPE_IMAGE, val_start); // Get 0-based index
                     add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_IMAGE_SOURCE, VAL_TYPE_RESOURCE, 1, &resource_index);
                     property_handled = true;
                 }
                 // App Icon
                 else if (!is_style_context && current_element->type == ELEM_TYPE_APP && strcmp(key, "icon") == 0) {
                     uint8_t resource_index = add_resource(RES_TYPE_IMAGE, val_start); // Get 0-based index
                     add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_ICON, VAL_TYPE_RESOURCE, 1, &resource_index);
                     property_handled = true;
                 }

                 // == Other App Specific Properties ==
                 else if (!is_style_context && current_element->type == ELEM_TYPE_APP) {
                     if (strcmp(key, "window_width") == 0) { uint16_t v=(uint16_t)atoi(val_start); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_WINDOW_WIDTH, VAL_TYPE_SHORT, 2, &v); property_handled = true; }
                     else if (strcmp(key, "window_height") == 0) { uint16_t v=(uint16_t)atoi(val_start); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_WINDOW_HEIGHT, VAL_TYPE_SHORT, 2, &v); property_handled = true; }
                     else if (strcmp(key, "window_title") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_WINDOW_TITLE, val_start); property_handled = true; }
                     else if (strcmp(key, "resizable") == 0) { uint8_t v=(strstr(val_start,"true")!=NULL); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_RESIZABLE, VAL_TYPE_BYTE, 1, &v); property_handled = true; }
                     else if (strcmp(key, "keep_aspect") == 0) { uint8_t v=(strstr(val_start,"true")!=NULL); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_KEEP_ASPECT, VAL_TYPE_BYTE, 1, &v); property_handled = true; }
                     else if (strcmp(key, "scale_factor") == 0) { float s=atof(val_start); uint16_t fp=(uint16_t)(s*256.0f+0.5f); add_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_SCALE_FACTOR, VAL_TYPE_PERCENTAGE, 2, &fp); g_header_flags |= FLAG_FIXED_POINT; property_handled = true; }
                     // icon handled above
                     else if (strcmp(key, "version") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_VERSION, val_start); property_handled = true; }
                     else if (strcmp(key, "author") == 0) { add_string_property_to_list(target_props, target_prop_count, target_block_size, PROP_ID_AUTHOR, val_start); property_handled = true; }
                     else if (strcmp(key, "icon") != 0) { // Avoid duplicate warning for icon already handled
                         fprintf(stderr, "Warning L%d: Unhandled App property '%s'.\n", line_num, key);
                         property_handled = true; // Mark as handled to avoid generic warning
                     }
                 }

                 // == Final Catch-all for Unhandled ==
                 if (!property_handled) {
                     fprintf(stderr, "Warning L%d: Unhandled property '%s' in %s context.\n",
                             line_num, key, is_style_context ? "Style" : "Element");
                 }

            } else { fprintf(stderr, "L%d: Bad prop syntax: '%s'\n", line_num, trimmed); return 1; }
        } else if (trimmed[0] != '\0') { fprintf(stderr, "L%d: Bad syntax/indent: '%s'\n", line_num, trimmed); return 1; }
    } // End while

    // End of File Checks
    if (element_stack_top != -1) { fprintf(stderr, "Error: Unclosed element block.\n"); return 1; }
    if (current_style) { fprintf(stderr, "Error: Unclosed style block.\n"); return 1; }
    if (g_has_app && g_element_count > 0 && g_elements[0].type != ELEM_TYPE_APP) { fprintf(stderr, "Internal Error: App not index 0.\n"); return 1; }
    return 0; // Success
}

// --- Pass 2: Writing the KRB File ---
int write_krb_file(FILE* out) {
    // --- 1. Calculate Offsets ---
    const uint32_t header_size = 42; // Updated header size for v0.2
    uint32_t element_section_offset = header_size;
    uint32_t current_offset = element_section_offset;
    uint32_t style_section_offset = 0;
    uint32_t animation_section_offset = 0;
    uint32_t string_section_offset = 0;
    uint32_t resource_section_offset = 0;
    uint32_t total_size = 0;

    // Elements Size
    for (int i = 0; i < g_element_count; i++) {
        g_elements[i].absolute_offset = current_offset;
        if(g_elements[i].calculated_size < 16) { fprintf(stderr, "IntErr: Elem %d size %u<16\n",i,g_elements[i].calculated_size); return 1; }
        current_offset += g_elements[i].calculated_size;
    }

    // Styles Size
    style_section_offset = current_offset; // Offset is where elements end
    if (g_style_count > 0) {
        for (int i = 0; i < g_style_count; i++) {
            if(g_styles[i].calculated_size < 3) { fprintf(stderr, "IntErr: Style %d size %u<3\n",i,g_styles[i].calculated_size); return 1; }
            current_offset += g_styles[i].calculated_size;
        }
    } // Flag FLAG_HAS_STYLES already set during parsing if needed

    // Animations Size (Placeholder)
    animation_section_offset = current_offset; // Offset is where styles end
    if (g_animation_count > 0) {
        // current_offset += calculated_animation_section_size; // TODO
    } // Flag FLAG_HAS_ANIMATIONS already set during parsing if needed

    // Strings Size
    string_section_offset = current_offset; // Offset is where animations end
    if (g_string_count > 0) {
        uint32_t string_section_size = 2; // Count field
        for (int i = 0; i < g_string_count; i++) {
            if (g_strings[i].length > 255) { fprintf(stderr, "Err: Str %d len %zu>255\n",i,g_strings[i].length); return 1; }
            string_section_size += 1 + g_strings[i].length; // Len byte + data
        }
        current_offset += string_section_size;
    }

    // Resources Size
    resource_section_offset = current_offset; // Offset is where strings end
    if (g_resource_count > 0) {
        uint32_t resource_section_size = 2; // Count field
        for (int i = 0; i < g_resource_count; i++) {
            if (g_resources[i].calculated_size == 0) { fprintf(stderr, "IntErr: Res %d size 0\n", i); return 1; }
            resource_section_size += g_resources[i].calculated_size;
        }
        current_offset += resource_section_size;
    } // Flag FLAG_HAS_RESOURCES already set during parsing if needed

    total_size = current_offset; // Final total size

    // --- 2. Write Header (KRB v0.2) ---
    rewind(out);
    fwrite(KRB_MAGIC, 1, 4, out);                             // Offset 0
    write_u16(out, (KRB_VERSION_MINOR << 8)|KRB_VERSION_MAJOR); // Offset 4 (0x0002 for 0.2)
    write_u16(out, g_header_flags);                            // Offset 6
    write_u16(out, (uint16_t)g_element_count);                 // Offset 8
    write_u16(out, (uint16_t)g_style_count);                   // Offset 10
    write_u16(out, (uint16_t)g_animation_count);               // Offset 12
    write_u16(out, (uint16_t)g_string_count);                  // Offset 14
    write_u16(out, (uint16_t)g_resource_count);                // Offset 16
    write_u32(out, element_section_offset);                    // Offset 18
    write_u32(out, style_section_offset);                      // Offset 22
    write_u32(out, animation_section_offset);                  // Offset 26
    write_u32(out, string_section_offset);                     // Offset 30
    write_u32(out, resource_section_offset);                   // Offset 34 <-- Added for v0.2
    write_u32(out, total_size);                                // Offset 38 <-- Now holds total size

    // Check header size consistency
    if (ftell(out) != header_size) {
        fprintf(stderr, "IntErr: Header write size %ld != %u\n", ftell(out), header_size); return 1;
    }

    // --- 3. Write Element Blocks ---
    if (fseek(out, element_section_offset, SEEK_SET) != 0) { perror("Seek Elem"); return 1; }
    for (int i = 0; i < g_element_count; i++) {
        Element* el = &g_elements[i];
        long block_start = ftell(out);
        if ((uint32_t)block_start != el->absolute_offset) { fprintf(stderr, "IntErr: Elem %d offset %u!=%ld\n",i,el->absolute_offset, block_start); return 1; }
        write_u8(out, el->type); write_u8(out, el->id_string_index); write_u16(out, el->pos_x); write_u16(out, el->pos_y);
        write_u16(out, el->width); write_u16(out, el->height); write_u8(out, el->layout); write_u8(out, el->style_id);
        write_u8(out, el->property_count); write_u8(out, el->child_count); write_u8(out, el->event_count); write_u8(out, el->animation_count);
        // Properties
        for (int j=0; j<el->property_count; j++) { KrbProperty* p=&el->properties[j]; write_u8(out,p->property_id); write_u8(out,p->value_type); write_u8(out,p->size); if (p->size>0){ if (!p->value) {fprintf(stderr,"IntErr: E%d P%d null val\n",i,j); return 1;} if (fwrite(p->value,1,p->size,out)!=p->size) { perror("Write Elem Prop Val"); return 1; } } }
        // Events
        for (int j=0; j<el->event_count; j++) { write_u8(out,el->events[j].event_type); write_u8(out,el->events[j].callback_id); }
        // Anim Refs (TODO)
        // Child Offsets
        for (int j=0; j<el->child_count; j++) { Element* c=el->children[j]; if(!c) { fprintf(stderr,"IntErr: E%d null child %d\n",i,j); return 1; } uint32_t off=c->absolute_offset-el->absolute_offset; if(off>0xFFFF){ fprintf(stderr,"Err: E%d C%d offset %u>16b\n",i,j,off); return 1;} write_u16(out,(uint16_t)off); }
        // Size Check
        long block_end = ftell(out); if ((uint32_t)(block_end - block_start) != el->calculated_size) { fprintf(stderr, "IntErr: Elem %d write %lu != calc %u\n", i, block_end - block_start, el->calculated_size); return 1; }
    }

    // --- 4. Write Style Blocks ---
    if (g_style_count > 0) {
         if ((uint32_t)ftell(out) != style_section_offset) { fprintf(stderr, "IntErr: Style offset %ld!=%u\n",ftell(out),style_section_offset); return 1; }
        for (int i=0; i<g_style_count; i++) {
            StyleEntry* st=&g_styles[i]; long block_start=ftell(out);
            write_u8(out,st->id); write_u8(out,st->name_index); write_u8(out,st->property_count);
            for (int j=0; j<st->property_count; j++) { KrbProperty* p=&st->properties[j]; write_u8(out,p->property_id); write_u8(out,p->value_type); write_u8(out,p->size); if (p->size>0){ if (!p->value) {fprintf(stderr,"IntErr: S%d P%d null val\n",i,j); return 1;} if (fwrite(p->value,1,p->size,out)!=p->size) { perror("Write Style Prop Val"); return 1; } } }
            long block_end = ftell(out); if ((uint32_t)(block_end - block_start) != st->calculated_size) { fprintf(stderr, "IntErr: Style %d write %lu != calc %u\n", i, block_end - block_start, st->calculated_size); return 1; }
        }
    }

    // --- 5. Write Animation Table ---
    // TODO
    if (g_animation_count > 0) {
        if ((uint32_t)ftell(out) != animation_section_offset) { fprintf(stderr, "IntErr: Anim offset %ld!=%u\n",ftell(out),animation_section_offset); return 1; }
        // Write animation data here...
    }

    // --- 6. Write String Table ---
     if (g_string_count > 0) {
         if ((uint32_t)ftell(out) != string_section_offset) { fprintf(stderr, "IntErr: String offset %ld!=%u\n",ftell(out),string_section_offset); return 1; }
        write_u16(out,(uint16_t)g_string_count);
        for (int i=0; i<g_string_count; i++) {
            StringEntry* s=&g_strings[i];
            if (s->length > 255) { return 1;} // Should have been caught earlier
            write_u8(out,(uint8_t)s->length);
            if (s->length > 0) { if (!s->text) {fprintf(stderr,"IntErr: Str %d null\n",i); return 1;} if (fwrite(s->text,1,s->length,out)!=s->length) { perror("Write Str data"); return 1; } }
        }
    }

    // --- 7. Write Resource Table ---
    if (g_resource_count > 0) {
        if ((uint32_t)ftell(out) != resource_section_offset) { fprintf(stderr, "IntErr: Res offset %ld != calc %u\n", ftell(out), resource_section_offset); return 1; }
        write_u16(out, (uint16_t)g_resource_count); // Write count
        for (int i = 0; i < g_resource_count; i++) {
            ResourceEntry* res = &g_resources[i]; long entry_start = ftell(out);
            write_u8(out, res->type); write_u8(out, res->name_index); write_u8(out, res->format);
            if (res->format == RES_FORMAT_EXTERNAL) { write_u8(out, res->data_string_index); } // Write string index for path
            else if (res->format == RES_FORMAT_INLINE) { fprintf(stderr, "Error: Inline resources not implemented.\n"); return 1; }
            else { fprintf(stderr, "Error: Unknown res format %u.\n", res->format); return 1; }
            long entry_end = ftell(out); if ((uint32_t)(entry_end - entry_start) != res->calculated_size) { fprintf(stderr, "IntErr: Res %d write %lu != calc %u\n", i, entry_end - entry_start, res->calculated_size); return 1; }
        }
    }

    // --- Final Size Check ---
    long final_pos = ftell(out); if (final_pos<0) { perror("Final ftell"); return 1; }
    if ((uint32_t)final_pos != total_size) { fprintf(stderr, "IntErr: Final size %ld != calc total %u\n", final_pos, total_size); return 1; }

    return 0; // Success
}


// --- Main Function ---
int main(int argc, char* argv[]) {
    if (argc != 3) { fprintf(stderr, "Usage: %s <input.kry> <output.krb>\n", argv[0]); return 1; }
    const char* input_file = argv[1]; const char* output_file = argv[2];

    // Initialize Global Arrays
    memset(g_elements, 0, sizeof(g_elements));
    memset(g_styles, 0, sizeof(g_styles));
    memset(g_strings, 0, sizeof(g_strings));
    memset(g_resources, 0, sizeof(g_resources)); // Initialize resources

    FILE* in = fopen(input_file, "r"); if (!in) { fprintf(stderr, "Error opening input '%s': %s\n", input_file, strerror(errno)); return 1; }
    FILE* out = fopen(output_file, "wb+"); if (!out) { fprintf(stderr, "Error opening output '%s': %s\n", output_file, strerror(errno)); fclose(in); return 1; }

    printf("Compiling '%s' to '%s' (KRB v%d.%d)...\n", input_file, output_file, KRB_VERSION_MAJOR, KRB_VERSION_MINOR);

    printf("Pass 1: Parsing and calculating sizes...\n");
    if (parse_and_calculate_sizes(in) != 0) {
        fprintf(stderr, "Compilation failed during Pass 1.\n"); fclose(in); fclose(out); cleanup_resources(); remove(output_file); return 1;
    }
    printf("   Found %d elements, %d styles, %d strings, %d resources.\n", g_element_count, g_style_count, g_string_count, g_resource_count); // Added resource count

    printf("Pass 2: Writing binary file...\n");
    if (write_krb_file(out) != 0) {
        fprintf(stderr, "Compilation failed during Pass 2.\n"); fclose(in); fclose(out); cleanup_resources(); remove(output_file); return 1;
    }

    long final_size = ftell(out); if (final_size<0) { perror("Final size ftell"); final_size=0; }
    printf("Compilation successful. Output size: %ld bytes.\n", final_size);

    fclose(in); if (fflush(out)!=0) { perror("Flush output"); } fclose(out); cleanup_resources();
    return 0;
}