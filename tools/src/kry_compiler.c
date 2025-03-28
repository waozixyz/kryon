#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>
#include <errno.h>
#include <stdbool.h>

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
    uint8_t event_count;    // Not implemented in parsing yet
    uint8_t animation_count;// Not implemented in parsing yet

    // Compiler-Internal Data
    KrbProperty properties[MAX_PROPERTIES];
    // TODO: Add Event structures
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

    uint8_t parsed_layout_byte;
    bool has_parsed_layout;  

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

// Safely adds a string to the global table, handles duplicates, trims. Returns index.
uint8_t add_string(const char* text) {
    if (!text) return 0; // Consider if index 0 should be reserved for "invalid"

    // 1. Trim leading/trailing whitespace
    const char *start = text;
    const char *end = text + strlen(text) - 1;
    while (start <= end && isspace((unsigned char)*start)) start++;
    while (end >= start && isspace((unsigned char)*end)) end--;

    // 2. Handle quotes (only if they are the very first and last non-whitespace chars)
    if (end >= start && *start == '"' && *end == '"') {
        start++;
        end--;
    }

    // 3. Calculate length and create a temporary buffer for the cleaned string
    size_t len = (end < start) ? 0 : (end - start + 1);
    char clean_text_buf[MAX_LINE_LENGTH]; // Use stack buffer for temporary cleaned string
    if (len >= sizeof(clean_text_buf)) {
         fprintf(stderr, "Error: Cleaned string too long: %zu chars\n", len);
         exit(EXIT_FAILURE);
    }
    strncpy(clean_text_buf, start, len);
    clean_text_buf[len] = '\0';

    // 4. Check for duplicates
    for (int i = 0; i < g_string_count; i++) {
        if (strcmp(g_strings[i].text, clean_text_buf) == 0) {
            return g_strings[i].index; // Return existing index
        }
    }

    // 5. Add new string
    if (g_string_count >= MAX_STRINGS) {
        fprintf(stderr, "Error: Maximum string count (%d) exceeded.\n", MAX_STRINGS);
        exit(EXIT_FAILURE);
    }

    g_strings[g_string_count].text = strdup(clean_text_buf); // Allocate permanent storage
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

    // Don't add the style name to the main string table here,
    // it's added when the style itself is parsed. Just search.
    // Trim the input name for searching consistency.
    const char *start = name;
    const char *end = name + strlen(name) - 1;
    while (start <= end && isspace((unsigned char)*start)) start++;
    while (end >= start && isspace((unsigned char)*end)) end--;
    if (end >= start && *start == '"' && *end == '"') {
        start++; end--;
    }
    size_t len = (end < start) ? 0 : (end - start + 1);
    char clean_name_buf[MAX_LINE_LENGTH];
     if (len >= sizeof(clean_name_buf)) {
         fprintf(stderr, "Error: Style name too long for search buffer: %zu chars\n", len);
         exit(EXIT_FAILURE);
    }
    strncpy(clean_name_buf, start, len);
    clean_name_buf[len] = '\0';

    for (int i = 0; i < g_style_count; i++) {
        // Compare against the string table entry pointed to by the style's name_index
        if (strcmp(g_strings[g_styles[i].name_index].text, clean_name_buf) == 0) {
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
                g_elements[i].properties[j].value = NULL; // Avoid double free
            }
        }
    }
    // Free style properties
    for (int i = 0; i < g_style_count; i++) {
        for (int j = 0; j < g_styles[i].property_count; j++) {
            if (g_styles[i].properties[j].value) {
                free(g_styles[i].properties[j].value);
                g_styles[i].properties[j].value = NULL; // Avoid double free
            }
        }
    }
    // Free strings
    for (int i = 0; i < g_string_count; i++) {
         if (g_strings[i].text) {
            free(g_strings[i].text);
            g_strings[i].text = NULL; // Avoid double free
        }
    }
    // TODO: Free animation data
    // TODO: Free resource data

    // Reset global counts/flags for potential reuse (though main exits)
    g_element_count = 0;
    g_string_count = 0;
    g_style_count = 0;
    g_animation_count = 0;
    g_resource_count = 0;
    g_has_app = 0;
    g_header_flags = 0;
}

// Helper to parse color strings like #RRGGBB or #RRGGBBAA -> outputs RGBA
int parse_color(const char* value_str, uint8_t color_out[4]) {
    color_out[3] = 255; // Default alpha
    if (sscanf(value_str, "#%2hhx%2hhx%2hhx%2hhx", &color_out[0], &color_out[1], &color_out[2], &color_out[3]) == 4) return 1;
    if (sscanf(value_str, "#%2hhx%2hhx%2hhx", &color_out[0], &color_out[1], &color_out[2]) == 3) return 1;
    // Allow common color names? Maybe later.
    // Allow rgb()/rgba() syntax? Maybe later.
    return 0; // Failed parse
}

// Helper to add a property and update the calculated size
void add_property_to_list(KrbProperty* prop_array, uint8_t* count, uint32_t* current_size, uint8_t prop_id, uint8_t val_type, uint8_t size, const void* data) {
    if (!prop_array || !count || !current_size || !data) {
        fprintf(stderr, "Internal Error: Null pointer passed to add_property_to_list.\n");
        exit(EXIT_FAILURE);
    }
    if (*count >= MAX_PROPERTIES) {
        fprintf(stderr, "Error: Maximum property count (%d) exceeded for element/style.\n", MAX_PROPERTIES);
        exit(EXIT_FAILURE);
    }
    KrbProperty* p = &prop_array[*count];
    p->property_id = prop_id;
    p->value_type = val_type;
    p->size = size;
    p->value = malloc(size);
    if (!p->value) {
        perror("Failed to allocate property value");
        exit(EXIT_FAILURE);
    }
    memcpy(p->value, data, size);

    *current_size += 1 + 1 + 1 + size; // PropID + Type + Size + ValueSize
    (*count)++;
}
// Convenience wrapper for string properties (Value is 1-byte index)
void add_string_property_to_list(KrbProperty* prop_array, uint8_t* count, uint32_t* current_size, uint8_t prop_id, const char* value_str) {
     uint8_t str_index = add_string(value_str);
     add_property_to_list(prop_array, count, current_size, prop_id, VAL_TYPE_STRING, 1, &str_index);
}
// Convenience wrapper for resource path properties (Value is 1-byte index to string path)
void add_resource_path_property_to_list(KrbProperty* prop_array, uint8_t* count, uint32_t* current_size, uint8_t prop_id, const char* path_str) {
     // Spec says Resource Type (0x05) uses index to resource table.
     // If we only support external resources for now, we store the path string index.
     // A more robust compiler might create a resource entry and store *that* index.
     // Let's store string index for now, assuming Format=0x00 (External).
     uint8_t str_index = add_string(path_str);
     // Using VAL_TYPE_STRING here to point to the path, NOT VAL_TYPE_RESOURCE yet.
     // A full resource implementation would differ. Prop ID is still PROP_ID_ICON etc.
     add_property_to_list(prop_array, count, current_size, prop_id, VAL_TYPE_STRING, 1, &str_index);
     // TODO: Or, implement Resource Table properly. If using Resource Table:
     // uint8_t res_index = add_resource(path_str, RES_TYPE_IMAGE, RES_FORMAT_EXTERNAL);
     // add_property_to_list(prop_array, count, current_size, prop_id, VAL_TYPE_RESOURCE, 1, &res_index);
     // g_header_flags |= FLAG_HAS_RESOURCES;
}
// --- Pass 1: Parsing and Size Calculation ---
int parse_and_calculate_sizes(FILE* in) {
    char line[MAX_LINE_LENGTH];
    Element* current_element = NULL;
    StyleEntry* current_style = NULL;
    int current_indent = -1; // Use -1 to indicate top level
    int element_indent_stack[MAX_ELEMENTS]; // Track indent level of parent elements
    int element_index_stack[MAX_ELEMENTS]; // Track indices of parent elements
    int element_stack_top = -1;
    int line_num = 0;

    g_header_flags = 0; // Reset flags for this compilation

    while (fgets(line, sizeof(line), in)) {
        line_num++;
        char* trimmed = line;
        int indent = 0;
        while (*trimmed == ' ' || *trimmed == '\t') {
            indent += (*trimmed == '\t' ? 4 : 1); // Simple indent count
            trimmed++;
        }
        char* end = trimmed + strlen(trimmed) - 1;
        while (end >= trimmed && isspace((unsigned char)*end)) *end-- = '\0';

        if (*trimmed == '\0' || *trimmed == '#') continue; // Skip empty/comment lines

        // --- End Block Logic ---
        if (*trimmed == '}') {
            if (current_element && element_stack_top >= 0 && indent == element_indent_stack[element_stack_top]) {
                 // Finalize element size
                current_element->calculated_size += current_element->event_count * 2;     // Event Ref Size
                current_element->calculated_size += current_element->animation_count * 2; // Anim Ref Size
                current_element->calculated_size += current_element->child_count * 2;     // Child Ref Size

                // Pop from element stack
                element_stack_top--;
                current_indent = (element_stack_top >= 0) ? element_indent_stack[element_stack_top] : -1;
                current_element = (element_stack_top >= 0) ? &g_elements[element_index_stack[element_stack_top]] : NULL;
                continue;
            } else if (current_style && indent == current_indent) {
                // End of style block
                current_style = NULL;
                current_indent = -1;
                continue;
            } else { // Handle mismatched '}'
                 fprintf(stderr, "Error line %d: Unexpected '}' or incorrect indentation. Current Indent: %d, Line Indent: %d, Stack Top: %d\n",
                         line_num, current_indent, indent, element_stack_top);
                 return 1;
             }
        }

        // --- Start New Block Logic ---
        if (strncmp(trimmed, "style ", 6) == 0 && strstr(trimmed, "{")) {
            if (current_element || current_style) {
                fprintf(stderr, "Error line %d: Cannot define style inside another block.\n", line_num); return 1;
            }
            char style_name[128];
            if (sscanf(trimmed, "style \"%127[^\"]\" {", style_name) == 1) {
                if (g_style_count >= MAX_STYLES) {
                     fprintf(stderr, "Error line %d: Maximum style count (%d) exceeded.\n", line_num, MAX_STYLES); return 1;
                 }
                // Setup current_style
                current_style = &g_styles[g_style_count];
                current_style->id = g_style_count + 1; // 1-based ID
                current_style->name_index = add_string(style_name);
                current_style->property_count = 0;
                current_style->calculated_size = 1 + 1 + 1; // ID + NameIndex + PropCount base size
                // Initialize layout fields
                current_style->parsed_layout_byte = 0;
                current_style->has_parsed_layout = false;

                g_style_count++;
                current_indent = indent;
                g_header_flags |= FLAG_HAS_STYLES;
            } else {
                 fprintf(stderr, "Error line %d: Invalid style definition: %s\n", line_num, trimmed); return 1;
             }
        }
        else if (isalpha((unsigned char)*trimmed) && strstr(trimmed, "{")) { // Element Start
             if (current_style) {
                 fprintf(stderr, "Error line %d: Cannot define element inside style block.\n", line_num); return 1;
             }
             if (g_element_count >= MAX_ELEMENTS) {
                 fprintf(stderr, "Error line %d: Maximum element count (%d) exceeded.\n", line_num, MAX_ELEMENTS); return 1;
             }

             Element* parent = (element_stack_top >= 0) ? &g_elements[element_index_stack[element_stack_top]] : NULL;
             current_element = &g_elements[g_element_count];

             // Initialize Element struct
             current_element->self_index = g_element_count;
             current_element->parent_index = (parent) ? parent->self_index : -1;
             memset(current_element->properties, 0, sizeof(current_element->properties));
             current_element->id_string_index = 0;
             current_element->pos_x = 0; current_element->pos_y = 0; current_element->width = 0; current_element->height = 0;
             current_element->layout = 0; // Default layout
             current_element->style_id = 0;
             current_element->property_count = 0; current_element->child_count = 0;
             current_element->event_count = 0; current_element->animation_count = 0;
             current_element->calculated_size = 16; // Base header size
             for(int i=0; i<MAX_CHILDREN; ++i) current_element->children[i] = NULL;

            // Determine Type
            if (strncmp(trimmed, "App {", 5) == 0) {
                current_element->type = 0x00;
                if (g_has_app++) { fprintf(stderr, "Error line %d: Only one App element allowed.\n", line_num); return 1; }
                if (parent) { fprintf(stderr, "Error line %d: App element cannot be a child.\n", line_num); return 1; }
                g_header_flags |= FLAG_HAS_APP;
            } else if (strncmp(trimmed, "Container {", 11) == 0) current_element->type = 0x01;
            else if (strncmp(trimmed, "Text {", 6) == 0) current_element->type = 0x02;
            else if (strncmp(trimmed, "Image {", 7) == 0) current_element->type = 0x03;
            else if (strncmp(trimmed, "Canvas {", 8) == 0) current_element->type = 0x04;
            else if (strncmp(trimmed, "Button {", 8) == 0) current_element->type = 0x10;
            else if (strncmp(trimmed, "Input {", 7) == 0) current_element->type = 0x11;
            else if (strncmp(trimmed, "List {", 6) == 0) current_element->type = 0x20;
            else if (strncmp(trimmed, "Grid {", 6) == 0) current_element->type = 0x21;
            else if (strncmp(trimmed, "Scrollable {", 12) == 0) current_element->type = 0x22;
             else { // Custom element handling (basic)
                 char custom_name[64];
                 if (sscanf(trimmed, "%63s {", custom_name) == 1) {
                     fprintf(stderr, "Warning line %d: Assuming '%s' is a custom element (Type 0x31+). Not fully implemented.\n", line_num, custom_name);
                     current_element->type = 0x31;
                     current_element->id_string_index = add_string(custom_name);
                 } else {
                     fprintf(stderr, "Error line %d: Unknown element type or invalid syntax: %s\n", line_num, trimmed); return 1;
                 }
             }

             // Link child to parent
             if (parent) {
                 if (parent->child_count >= MAX_CHILDREN) {
                     fprintf(stderr, "Error line %d: Maximum children (%d) exceeded for parent element index %d.\n", line_num, MAX_CHILDREN, parent->self_index); return 1;
                 }
                 parent->children[parent->child_count++] = current_element;
             }

             // Push onto element stack
             element_stack_top++;
             if(element_stack_top >= MAX_ELEMENTS) { fprintf(stderr, "Error line %d: Element nesting depth exceeds limit (%d).\n", line_num, MAX_ELEMENTS); return 1; }
             element_indent_stack[element_stack_top] = indent;
             element_index_stack[element_stack_top] = g_element_count; // Store index
             g_element_count++; // Increment global count
             current_indent = indent;
        }
        // --- Property Parsing Logic ---
        else if ((current_element && indent > current_indent) || (current_style && indent > current_indent)) {
            char key[64], value_str[MAX_LINE_LENGTH - 64];
            if (sscanf(trimmed, "%63[^:]:%[^\n]", key, value_str) == 2) {
                // Trim key and value start
                char* key_end = key + strlen(key) - 1; while (key_end >= key && isspace((unsigned char)*key_end)) *key_end-- = '\0';
                char* val_start = value_str; while (*val_start && isspace((unsigned char)*val_start)) val_start++;

                KrbProperty* target_props = current_element ? current_element->properties : current_style->properties;
                uint8_t* target_count = current_element ? &current_element->property_count : &current_style->property_count;
                uint32_t* target_size = current_element ? &current_element->calculated_size : &current_style->calculated_size;

                // --- Handle 'layout' Property FIRST ---
                 if (strcmp(key, "layout") == 0) {
                     uint8_t parsed_layout_byte = 0; // Calculate the byte based on val_start
                     // Parse Direction
                     if (strstr(val_start, "column_reverse")) parsed_layout_byte |= 0x03; else if (strstr(val_start, "row_reverse")) parsed_layout_byte |= 0x02; else if (strstr(val_start, "column")) parsed_layout_byte |= 0x01; else parsed_layout_byte |= 0x00;
                     // Parse Alignment
                     if (strstr(val_start, "space_between")) parsed_layout_byte |= (0x03 << 2); else if (strstr(val_start, "end")) parsed_layout_byte |= (0x02 << 2); else if (strstr(val_start, "center")) parsed_layout_byte |= (0x01 << 2); else parsed_layout_byte |= (0x00 << 2);
                     // Parse Flags
                     if (strstr(val_start, "wrap")) parsed_layout_byte |= LAYOUT_WRAP_BIT;
                     if (strstr(val_start, "grow")) parsed_layout_byte |= LAYOUT_GROW_BIT;
                     if (strstr(val_start, "absolute")) parsed_layout_byte |= LAYOUT_ABSOLUTE_BIT;

                     if (current_element) { // Direct layout on element
                         current_element->layout = parsed_layout_byte; // Set element's layout byte directly
                         printf("DEBUG line %d: Parsed direct layout '%s' (0x%02X) for element %d\n", line_num, val_start, parsed_layout_byte, current_element->self_index);
                     } else if (current_style) { // Layout definition within a style block
                         current_style->parsed_layout_byte = parsed_layout_byte;
                         current_style->has_parsed_layout = true;
                         printf("DEBUG line %d: Parsed layout '%s' (0x%02X) for style '%s'\n", line_num, val_start, parsed_layout_byte, g_strings[current_style->name_index].text);
                         // DO NOT add as a KrbProperty for the style block itself
                     }
                 }
                 // --- Handle Other Properties ---
                 else if (strcmp(key, "id") == 0 && current_element) { current_element->id_string_index = add_string(val_start); }
                 else if (strcmp(key, "pos_x") == 0 && current_element) { current_element->pos_x = atoi(val_start); }
                 else if (strcmp(key, "pos_y") == 0 && current_element) { current_element->pos_y = atoi(val_start); }
                 else if (strcmp(key, "width") == 0 && current_element) { current_element->width = atoi(val_start); }
                 else if (strcmp(key, "height") == 0 && current_element) { current_element->height = atoi(val_start); }
                 else if (strcmp(key, "style") == 0 && current_element) {
                     current_element->style_id = find_style_id_by_name(val_start);
                     if(current_element->style_id == 0) { fprintf(stderr, "Warning line %d: Style '%s' not found or defined yet.\n", line_num, val_start); }
                 }
                 else if (strcmp(key, "background_color") == 0) {
                     uint8_t color[4]; if (parse_color(val_start, color)) { add_property_to_list(target_props, target_count, target_size, PROP_ID_BG_COLOR, VAL_TYPE_COLOR, 4, color); g_header_flags |= FLAG_EXTENDED_COLOR; } else fprintf(stderr, "Warning line %d: Invalid background_color: %s\n", line_num, val_start);
                 }
                 else if (strcmp(key, "foreground_color") == 0 || strcmp(key, "text_color") == 0) {
                     uint8_t color[4]; if (parse_color(val_start, color)) { add_property_to_list(target_props, target_count, target_size, PROP_ID_FG_COLOR, VAL_TYPE_COLOR, 4, color); g_header_flags |= FLAG_EXTENDED_COLOR; } else fprintf(stderr, "Warning line %d: Invalid foreground/text_color: %s\n", line_num, val_start);
                 }
                 else if (strcmp(key, "border_color") == 0) {
                     uint8_t color[4]; if (parse_color(val_start, color)) { add_property_to_list(target_props, target_count, target_size, PROP_ID_BORDER_COLOR, VAL_TYPE_COLOR, 4, color); g_header_flags |= FLAG_EXTENDED_COLOR; } else fprintf(stderr, "Warning line %d: Invalid border_color: %s\n", line_num, val_start);
                 }
                 else if (strcmp(key, "border_width") == 0) {
                     uint8_t bw = atoi(val_start); add_property_to_list(target_props, target_count, target_size, PROP_ID_BORDER_WIDTH, VAL_TYPE_BYTE, 1, &bw);
                 }
                 else if (strcmp(key, "border_widths") == 0) {
                     uint8_t w[4]; if (sscanf(val_start, "%hhu %hhu %hhu %hhu", &w[0],&w[1],&w[2],&w[3]) == 4) add_property_to_list(target_props, target_count, target_size, PROP_ID_BORDER_WIDTH, VAL_TYPE_EDGEINSETS, 4, w); else fprintf(stderr, "Warning line %d: Invalid border_widths: %s\n", line_num, val_start);
                 }
                 else if (strcmp(key, "text") == 0 && current_element && current_element->type == 0x02) {
                     add_string_property_to_list(target_props, target_count, target_size, PROP_ID_TEXT_CONTENT, val_start);
                 }
                 else if (strcmp(key, "text_alignment") == 0 && ((current_element && current_element->type == 0x02) || current_style)) {
                     uint8_t align=0; if(strstr(val_start,"center"))align=1; else if(strstr(val_start,"right")||strstr(val_start,"end"))align=2; add_property_to_list(target_props, target_count, target_size, PROP_ID_TEXT_ALIGNMENT, VAL_TYPE_ENUM, 1, &align);
                 }
                 else if (strcmp(key, "source") == 0 && current_element && current_element->type == 0x03) {
                     add_resource_path_property_to_list(target_props, target_count, target_size, PROP_ID_IMAGE_SOURCE, val_start);
                 }
                 else if (current_element && current_element->type == 0x00) { // App-specific
                     if (strcmp(key, "window_width") == 0) { uint16_t v = atoi(val_start); add_property_to_list(target_props, target_count, target_size, PROP_ID_WINDOW_WIDTH, VAL_TYPE_SHORT, 2, &v); }
                     else if (strcmp(key, "window_height") == 0) { uint16_t v = atoi(val_start); add_property_to_list(target_props, target_count, target_size, PROP_ID_WINDOW_HEIGHT, VAL_TYPE_SHORT, 2, &v); }
                     else if (strcmp(key, "window_title") == 0) { add_string_property_to_list(target_props, target_count, target_size, PROP_ID_WINDOW_TITLE, val_start); }
                     else if (strcmp(key, "resizable") == 0) { uint8_t v = (strstr(val_start, "true") != NULL); add_property_to_list(target_props, target_count, target_size, PROP_ID_RESIZABLE, VAL_TYPE_BYTE, 1, &v); }
                     else if (strcmp(key, "keep_aspect") == 0) { uint8_t v = (strstr(val_start, "true") != NULL); add_property_to_list(target_props, target_count, target_size, PROP_ID_KEEP_ASPECT, VAL_TYPE_BYTE, 1, &v); }
                     else if (strcmp(key, "scale_factor") == 0) { float s = atof(val_start); uint16_t fp = (uint16_t)(s * 256.0f + 0.5f); add_property_to_list(target_props, target_count, target_size, PROP_ID_SCALE_FACTOR, VAL_TYPE_PERCENTAGE, 2, &fp); g_header_flags |= FLAG_FIXED_POINT; }
                     else if (strcmp(key, "icon") == 0) { add_resource_path_property_to_list(target_props, target_count, target_size, PROP_ID_ICON, val_start); }
                     else if (strcmp(key, "version") == 0) { add_string_property_to_list(target_props, target_count, target_size, PROP_ID_VERSION, val_start); }
                     else if (strcmp(key, "author") == 0) { add_string_property_to_list(target_props, target_count, target_size, PROP_ID_AUTHOR, val_start); }
                     else { fprintf(stderr, "Warning line %d: Unknown App property '%s'.\n", line_num, key); }
                 }
                 else { // Unhandled property
                    fprintf(stderr, "Warning line %d: Unhandled property '%s' for current context.\n", line_num, key);
                 }
                 // --- End Other Properties ---

            } else {
                 fprintf(stderr, "Error line %d: Invalid property syntax: %s\n", line_num, trimmed); return 1;
             }
        } else if (trimmed[0] != '\0') { // Non-empty line not matching expected patterns
             fprintf(stderr, "Error line %d: Unexpected syntax or indentation: %s\n", line_num, trimmed); return 1;
        }
    } // End while fgets

    // Check for unclosed blocks
    if (element_stack_top != -1 || current_style) {
        fprintf(stderr, "Error: Unclosed block%s at end of file.\n", current_style ? " (style)" : ""); return 1;
    }
    // Validate App element position
    if (g_has_app && g_element_count > 0 && g_elements[0].type != 0x00) {
        fprintf(stderr, "Internal Error: App element parsed but not first.\n"); return 1;
    }

    return 0; // Success
}

// --- Pass 1.5: Apply Style Layouts ---
void apply_style_layouts() {
    printf("Applying style layouts to elements...\n");
    for (int i = 0; i < g_element_count; i++) {
        Element* el = &g_elements[i];

        // Skip if element has no style ID
        if (el->style_id == 0 || el->style_id > g_style_count) {
            continue;
        }

        StyleEntry* style = &g_styles[el->style_id - 1]; // style_id is 1-based

        // Skip if the applied style didn't have a layout property parsed
        if (!style->has_parsed_layout) {
            continue;
        }

        uint8_t element_layout = el->layout; // Layout potentially set by direct property
        uint8_t style_layout = style->parsed_layout_byte;
        uint8_t final_layout = element_layout; // Start with element's direct setting

        printf("   Checking Element %d (Type 0x%02X, Style '%s') for layout merge...\n",
               i, el->type, g_strings[style->name_index].text);
        printf("      Element Direct Layout: 0x%02X\n", element_layout);
        printf("      Style Parsed Layout:   0x%02X\n", style_layout);

        // Define masks for individual layout aspects for clarity
        const uint8_t direction_mask = LAYOUT_DIRECTION_MASK; // 0x03
        const uint8_t alignment_mask = LAYOUT_ALIGNMENT_MASK; // 0x0C
        const uint8_t wrap_mask      = LAYOUT_WRAP_BIT;       // 0x10
        const uint8_t grow_mask      = LAYOUT_GROW_BIT;       // 0x20
        // Absolute bit (0x40) is usually *not* inherited/merged from style in this way

        // Merge Direction: Apply style's direction only if element's direction is default (Row, 00)
        if ((element_layout & direction_mask) == 0) {
            final_layout = (final_layout & ~direction_mask) | (style_layout & direction_mask);
        }
        // Merge Alignment: Apply style's alignment only if element's alignment is default (Start, 00)
        if ((element_layout & alignment_mask) == 0) {
            final_layout = (final_layout & ~alignment_mask) | (style_layout & alignment_mask);
        }
        // Merge Wrap: Apply style's wrap only if element's wrap is default (NoWrap, 0)
        if ((element_layout & wrap_mask) == 0) {
            final_layout |= (style_layout & wrap_mask); // Use OR to set the bit if style has it
        }
        // Merge Grow: Apply style's grow only if element's grow is default (Fixed, 0)
        if ((element_layout & grow_mask) == 0) {
            final_layout |= (style_layout & grow_mask); // Use OR to set the bit if style has it
        }

        // Update element's layout byte if changes were merged
        if (final_layout != element_layout) {
            printf("      Merged layout: Style(0x%02X) + Element(0x%02X) -> Final=0x%02X\n",
                   style_layout, element_layout, final_layout);
            el->layout = final_layout;
        } else {
             printf("      No layout changes merged (Element settings took precedence or were same).\n");
        }
    }
     printf("Finished applying style layouts.\n");
}

// --- Pass 2: Writing the KRB File ---
int write_krb_file(FILE* out) {
    // --- 1. Calculate Offsets ---
    uint32_t element_section_offset = 38; // Header size
    uint32_t current_offset = element_section_offset;
    uint32_t style_section_offset = 0;
    uint32_t animation_section_offset = 0;
    uint32_t string_section_offset = 0;
    uint32_t resource_section_offset = 0;
    uint32_t total_size = 0;

    // Calculate absolute offsets and total size of element blocks
    for (int i = 0; i < g_element_count; i++) {
        g_elements[i].absolute_offset = current_offset;
        current_offset += g_elements[i].calculated_size;
    }
    // current_offset now holds the end of the element section

    // Calculate style section offset and end
    if (g_style_count > 0) {
        style_section_offset = current_offset; // Starts after elements end
        for (int i = 0; i < g_style_count; i++) {
            current_offset += g_styles[i].calculated_size;
        }
        g_header_flags |= FLAG_HAS_STYLES; // Ensure flag is set if styles exist
    } else {
        style_section_offset = current_offset; // If no styles, starts where elements end
    }
    // current_offset now holds the end of the style section

    // TODO: Calculate animation section offset and end
    if (g_animation_count > 0) {
        animation_section_offset = current_offset; // Starts after styles end
        // Add sizes of animation entries...
        // current_offset += ... total animation size ...
        g_header_flags |= FLAG_HAS_ANIMATIONS; // Ensure flag is set
    } else {
        animation_section_offset = current_offset; // If no anims, starts where styles end
    }
    // current_offset now holds the end of the animation section

    // Calculate string section offset and end
    if (g_string_count > 0) {
        string_section_offset = current_offset; // Starts after animations end
        current_offset += 2; // String Count field
        for (int i = 0; i < g_string_count; i++) {
            current_offset += 1 + g_strings[i].length; // Length byte + UTF-8 bytes
        }
    } else {
         string_section_offset = current_offset; // If no strings, starts where anims end
    }
     // current_offset now holds the end of the string section

     // TODO: Calculate resource section offset and end
    if (g_resource_count > 0) {
        resource_section_offset = current_offset; // Starts after strings end
        // Add sizes of resource entries...
        // current_offset += ... total resource size ...
        g_header_flags |= FLAG_HAS_RESOURCES; // Ensure flag is set
    } else {
        resource_section_offset = current_offset; // If no resources, starts where strings end
    }
    // current_offset now holds the end of the resource section

    total_size = current_offset; // Final end is the total size

    // --- 2. Write Header ---
    rewind(out); // Go back to start to write header with calculated offsets
    fwrite(KRB_MAGIC, 1, 4, out);
    // Use version 1.0 as 0x0100 (Major 1, Minor 0) in little-endian -> write 0x00, 0x01
    write_u16(out, 0x0001); // Correctly writes 01 00 for LE 1.0
    write_u16(out, g_header_flags);
    write_u16(out, (uint16_t)g_element_count);
    write_u16(out, (uint16_t)g_style_count);
    write_u16(out, (uint16_t)g_animation_count); // Placeholder
    write_u16(out, (uint16_t)g_string_count);
    write_u16(out, (uint16_t)g_resource_count);   // Placeholder
    write_u32(out, element_section_offset);
    write_u32(out, style_section_offset);
    write_u32(out, animation_section_offset); // Placeholder
    write_u32(out, string_section_offset);
    // Spec V1.0 shows Total Size at offset 34, not Resource Offset
    write_u32(out, total_size);
    // Note: If Resource Offset is needed, the header spec needs adjustment/clarification.
    // Current writing matches provided spec v1.0 header layout.

    // Seek to start of element data
    if (fseek(out, element_section_offset, SEEK_SET) != 0) {
        perror("Error seeking to element section"); return 1;
    }

    // --- 3. Write Element Blocks ---
    if (ftell(out) != element_section_offset) {
         fprintf(stderr, "Internal Error: File pointer mismatch before writing elements (%ld != %u)\n", ftell(out), element_section_offset);
         return 1;
    }
    for (int i = 0; i < g_element_count; i++) {
        Element* el = &g_elements[i];
        uint32_t element_start_pos = ftell(out);
        if (element_start_pos != el->absolute_offset) {
            fprintf(stderr, "Internal Error: File pointer mismatch for element %d (%u != %u)\n", i, (unsigned int)element_start_pos, el->absolute_offset);
            return 1;
        }

        // Write Element Header
        write_u8(out, el->type);
        write_u8(out, el->id_string_index); // ID (String table index or 0)
        write_u16(out, el->pos_x);
        write_u16(out, el->pos_y);
        write_u16(out, el->width);
        write_u16(out, el->height);
        write_u8(out, el->layout);
        write_u8(out, el->style_id); // Style reference (1-based or 0)
        write_u8(out, el->property_count);
        write_u8(out, el->child_count);
        write_u8(out, el->event_count);
        write_u8(out, el->animation_count);

        // Write Properties
        for (int j = 0; j < el->property_count; j++) {
            KrbProperty* p = &el->properties[j];
            write_u8(out, p->property_id);
            write_u8(out, p->value_type);
            write_u8(out, p->size);
            if (fwrite(p->value, 1, p->size, out) != p->size) {
                 perror("Error writing property value"); return 1;
            }
        }

        // TODO: Write Event References
        // for (int j = 0; j < el->event_count; j++) { ... }

        // TODO: Write Animation References
        // for (int j = 0; j < el->animation_count; j++) { ... }

        // Write Child References (Relative Offsets)
        for (int j = 0; j < el->child_count; j++) {
            Element* child = el->children[j];
            if (!child) {
                 fprintf(stderr, "Internal Error: Child pointer is null for element %d, child %d\n", i, j);
                 return 1;
             }
            // Offset is from the start of *this* element's data block to the start of the *child's* data block
            uint32_t relative_offset_32 = child->absolute_offset - el->absolute_offset;
             if (relative_offset_32 > 0xFFFF) {
                 fprintf(stderr, "Error: Relative child offset exceeds 16 bits for element %d to child %d (%u).\n", i, child->self_index, relative_offset_32);
                 // This format might need 32-bit offsets if nesting gets very deep or elements very large.
                 // For now, truncate or error. Let's error.
                 return 1;
             }
            write_u16(out, (uint16_t)relative_offset_32);
        }

        // Sanity check size
        uint32_t element_end_pos = ftell(out);
        if (element_end_pos - element_start_pos != el->calculated_size) {
             fprintf(stderr, "Internal Error: Calculated size mismatch for element %d (%u != %u)\n",
                i, (unsigned int)(element_end_pos - element_start_pos), el->calculated_size);
             // This implies an error in Pass 1 size calculation.
             return 1;
        }
    }

    // --- 4. Write Style Blocks ---
    if (style_section_offset > 0 && g_style_count > 0) { // Check count too
         if (ftell(out) != style_section_offset) {
             fprintf(stderr, "Internal Error: File pointer mismatch before writing styles (%ld != %u)\n", ftell(out), style_section_offset);
             return 1;
        }
        for (int i = 0; i < g_style_count; i++) {
            StyleEntry* st = &g_styles[i];
            uint32_t style_start_pos = ftell(out);

            write_u8(out, st->id);         // Style ID (1-based)
            write_u8(out, st->name_index); // String table index for name
            write_u8(out, st->property_count);

            // Write Properties
            for (int j = 0; j < st->property_count; j++) {
                KrbProperty* p = &st->properties[j];
                write_u8(out, p->property_id);
                write_u8(out, p->value_type);
                write_u8(out, p->size);
                if (fwrite(p->value, 1, p->size, out) != p->size) {
                    perror("Error writing style property value"); return 1;
                }
            }
             // Sanity check size
            uint32_t style_end_pos = ftell(out);
            if (style_end_pos - style_start_pos != st->calculated_size) {
                fprintf(stderr, "Internal Error: Calculated size mismatch for style %d (%u != %u)\n",
                    i, (unsigned int)(style_end_pos - style_start_pos), st->calculated_size);
                return 1;
            }
        }
    }

    // --- 5. Write Animation Table ---
    // TODO: Implement Animation Table writing
    if (animation_section_offset > 0 && g_animation_count > 0) {
        if (ftell(out) != animation_section_offset) {
             fprintf(stderr, "Internal Error: File pointer mismatch before writing animations (%ld != %u)\n", ftell(out), animation_section_offset);
             return 1;
        }
        // Write animation data...
    }

    // --- 6. Write String Table ---
     if (string_section_offset > 0 && g_string_count > 0) { // Check count too
         if (ftell(out) != string_section_offset) {
             fprintf(stderr, "Internal Error: File pointer mismatch before writing strings (%ld != %u)\n", ftell(out), string_section_offset);
             return 1;
         }
        write_u16(out, (uint16_t)g_string_count);
        for (int i = 0; i < g_string_count; i++) {
            StringEntry* s = &g_strings[i];
            if (s->length > 255) {
                fprintf(stderr, "Error: String '%s' length (%zu) exceeds 255 bytes limit for length prefix.\n", s->text, s->length);
                return 1;
            }
            write_u8(out, (uint8_t)s->length);
            if (s->length > 0) {
                if (fwrite(s->text, 1, s->length, out) != s->length) {
                    perror("Error writing string data"); return 1;
                }
            }
        }
    }

    // --- 7. Write Resource Table ---
    // TODO: Implement Resource Table writing
    if (resource_section_offset > 0 && g_resource_count > 0) {
         if (ftell(out) != resource_section_offset) {
             fprintf(stderr, "Internal Error: File pointer mismatch before writing resources (%ld != %u)\n", ftell(out), resource_section_offset);
             return 1;
        }
        // Write resource data...
    }

    // --- Final Size Check ---
    long final_pos = ftell(out);
    if (final_pos < 0) { // Check for ftell error
        perror("Error getting final file position");
        return 1;
    }
    if ((uint32_t)final_pos != total_size) { // Cast final_pos to uint32_t for comparison
         fprintf(stderr, "Internal Error: Final file size mismatch (%ld != %u)\n", final_pos, total_size);
         // If this happens, an offset calculation was likely wrong.
         return 1;
    }

    return 0; // Success
}

// --- Main Function ---
int main(int argc, char* argv[]) {
    if (argc != 3) {
        fprintf(stderr, "Usage: %s <input.kry> <output.krb>\n", argv[0]);
        return 1;
    }

    const char* input_file = argv[1];
    const char* output_file = argv[2];

    FILE* in = fopen(input_file, "r");
    if (!in) {
        fprintf(stderr, "Error: Could not open input file '%s': %s\n", input_file, strerror(errno));
        return 1;
    }

    FILE* out = fopen(output_file, "wb+");
    if (!out) {
        fprintf(stderr, "Error: Could not open output file '%s': %s\n", output_file, strerror(errno));
        fclose(in);
        return 1;
    }

    printf("Compiling '%s' to '%s'...\n", input_file, output_file);

    // Pass 1: Parse and Calculate Sizes
    printf("Pass 1: Parsing and calculating sizes...\n");
    if (parse_and_calculate_sizes(in) != 0) {
        fprintf(stderr, "Compilation failed during Pass 1.\n");
        fclose(in); fclose(out); cleanup_resources(); remove(output_file);
        return 1;
    }
    printf("   Found %d elements, %d styles, %d strings.\n", g_element_count, g_style_count, g_string_count);

    // --- ADDED: Pass 1.5: Apply Style Layouts ---
    apply_style_layouts();

    // Pass 2: Write Binary File
    printf("Pass 2: Writing binary file...\n");
    if (write_krb_file(out) != 0) {
        fprintf(stderr, "Compilation failed during Pass 2.\n");
        fclose(in); fclose(out); cleanup_resources(); remove(output_file);
        return 1;
    }

    // Final reporting
    long final_size = ftell(out);
     if (final_size < 0) { perror("Error getting final file size after writing"); }
     else { printf("Compilation successful. Output size: %ld bytes.\n", final_size); }

    // Cleanup
    fclose(in);
    if (fflush(out) != 0) { perror("Error flushing output file"); }
    fclose(out);
    cleanup_resources();

    return 0;
}
