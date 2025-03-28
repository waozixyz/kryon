#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h> // Include for bool type
#include <errno.h>
#include <math.h>   // Include for roundf
#include "raylib.h" // Include Raylib first
#include "krb.h"    // Include KRB definitions (Element types, Prop IDs, etc.)

// --- Basic Definitions ---
// MAX_ELEMENTS is already potentially defined via krb.h, no need to repeat if guarded
#define DEFAULT_WINDOW_WIDTH 800
#define DEFAULT_WINDOW_HEIGHT 600
#define DEFAULT_SCALE_FACTOR 1.0f
#define BASE_FONT_SIZE 20

// Guessed Style Property ID for Layout (If needed specifically for renderer heuristic)
#define STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS 0x07 // Verify if used/needed here

// --- Renderer Structure ---
typedef struct RenderElement {
    KrbElementHeader header;      // Copy of the header
    char* text;                   // Resolved text string
    Color bg_color;               // Resolved background color
    Color fg_color;               // Resolved foreground/text color
    Color border_color;           // Resolved border color
    uint8_t border_widths[4];     // Resolved border widths [T, R, B, L]
    uint8_t text_alignment;       // Resolved text alignment (0=L, 1=C, 2=R)
    struct RenderElement* parent; // Pointer to parent RenderElement
    struct RenderElement* children[MAX_ELEMENTS]; // Array of pointers to children
    int child_count;              // Actual number of children linked

    // --- Additions for Interaction ---
    int render_x;                 // Final calculated X position on screen
    int render_y;                 // Final calculated Y position on screen
    int render_w;                 // Final calculated width on screen
    int render_h;                 // Final calculated height on screen
    bool is_interactive;          // Flag indicating if this element responds to hover/click
    // --------------------------------

} RenderElement;

// --- Helper Functions ---
uint16_t read_u16(const void* data) {
    if (!data) return 0;
    const unsigned char* p = (const unsigned char*)data;
    return (uint16_t)(p[0] | (p[1] << 8)); // Little-endian
}

// --- Rendering Function ---
void render_element(RenderElement* el, int parent_content_x, int parent_content_y, int parent_content_width, int parent_content_height, float scale_factor, FILE* debug_file) {
    if (!el) return;

    // --- 1. Calculate Element Intrinsic Size ---
    int intrinsic_w = (int)(el->header.width * scale_factor);
    int intrinsic_h = (int)(el->header.height * scale_factor);

    if (el->header.type == ELEM_TYPE_TEXT && el->text) { // Use defined type from krb.h
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        if (font_size < 1) font_size = 1;
        int text_width_measured = (el->text[0] != '\0') ? MeasureText(el->text, font_size) : 0;
        if (el->header.width == 0) intrinsic_w = text_width_measured + (int)(8 * scale_factor);
        if (el->header.height == 0) intrinsic_h = font_size + (int)(8 * scale_factor);
    }
    if (intrinsic_w < 0) intrinsic_w = 0;
    if (intrinsic_h < 0) intrinsic_h = 0;
    if (el->header.width > 0 && intrinsic_w == 0) intrinsic_w = 1;
    if (el->header.height > 0 && intrinsic_h == 0) intrinsic_h = 1;


    // --- 2. Determine Element Position and Final Size ---
    int final_x, final_y;
    int final_w = intrinsic_w;
    int final_h = intrinsic_h;
    bool has_pos = (el->header.pos_x != 0 || el->header.pos_y != 0);

    if (has_pos) {
        final_x = parent_content_x + (int)(el->header.pos_x * scale_factor);
        final_y = parent_content_y + (int)(el->header.pos_y * scale_factor);
    } else {
        final_x = parent_content_x;
        final_y = parent_content_y;
    }

    // --- Store Calculated Bounds ---
    el->render_x = final_x;
    el->render_y = final_y;
    el->render_w = final_w;
    el->render_h = final_h;

    // --- 4. Get Resolved Styles ---
    Color bg_color = el->bg_color;
    Color fg_color = el->fg_color;
    Color border_color = el->border_color;
    int top_bw = (int)(el->border_widths[0] * scale_factor);
    int right_bw = (int)(el->border_widths[1] * scale_factor);
    int bottom_bw = (int)(el->border_widths[2] * scale_factor);
    int left_bw = (int)(el->border_widths[3] * scale_factor);
    if (top_bw + bottom_bw >= el->render_h && el->render_h > 0) { top_bw = el->render_h > 1 ? 1 : el->render_h; bottom_bw = 0; }
    if (left_bw + right_bw >= el->render_w && el->render_w > 0) { left_bw = el->render_w > 1 ? 1 : el->render_w; right_bw = 0; }

    // --- 5. Debug Logging ---
    fprintf(debug_file, "DEBUG RENDER: Render Elem %p: Type=0x%02X @(%d,%d) FinalSize=%dx%d Borders=[%d,%d,%d,%d] Text='%s' Align=%d Layout=0x%02X Interact=%d\n",
            (void*)el, el->header.type, el->render_x, el->render_y, el->render_w, el->render_h,
            top_bw, right_bw, bottom_bw, left_bw,
            el->text ? el->text : "NULL", el->text_alignment, el->header.layout, el->is_interactive);

    // --- 6. Drawing Background & Borders ---
    bool draw_background = true;
    if (el->header.type == ELEM_TYPE_TEXT && top_bw == 0 && right_bw == 0 && bottom_bw == 0 && left_bw == 0) {
        draw_background = false;
    }
    if (draw_background && el->render_w > 0 && el->render_h > 0) {
        DrawRectangle(el->render_x, el->render_y, el->render_w, el->render_h, bg_color);
    }
    if (el->render_w > 0 && el->render_h > 0) {
        if (top_bw > 0) DrawRectangle(el->render_x, el->render_y, el->render_w, top_bw, border_color);
        if (bottom_bw > 0) DrawRectangle(el->render_x, el->render_y + el->render_h - bottom_bw, el->render_w, bottom_bw, border_color);
        int side_border_y = el->render_y + top_bw;
        int side_border_height = el->render_h - top_bw - bottom_bw;
        if (side_border_height < 0) side_border_height = 0;
        if (left_bw > 0) DrawRectangle(el->render_x, side_border_y, left_bw, side_border_height, border_color);
        if (right_bw > 0) DrawRectangle(el->render_x + el->render_w - right_bw, side_border_y, right_bw, side_border_height, border_color);
    }

    // --- 7. Calculate Content Area ---
    int content_x = el->render_x + left_bw;
    int content_y = el->render_y + top_bw;
    int content_width = el->render_w - left_bw - right_bw;
    int content_height = el->render_h - top_bw - bottom_bw;
    if (content_width < 0) content_width = 0;
    if (content_height < 0) content_height = 0;

    // --- 8. Draw Content (Text) ---
    if (el->text && el->text[0] != '\0' && content_width > 0 && content_height > 0) {
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        if (font_size < 1) font_size = 1;
        int text_width_measured = MeasureText(el->text, font_size);

        int text_draw_x = content_x;
        if (el->text_alignment == 1) { // Center
            text_draw_x = content_x + (content_width - text_width_measured) / 2;
        } else if (el->text_alignment == 2) { // Right
            text_draw_x = content_x + content_width - text_width_measured;
        }
        int text_draw_y = content_y + (content_height - font_size) / 2; // Vertical center

        if (text_draw_x < content_x) text_draw_x = content_x;
        if (text_draw_y < content_y) text_draw_y = content_y;

        BeginScissorMode(content_x, content_y, content_width, content_height);
        DrawText(el->text, text_draw_x, text_draw_y, font_size, fg_color);
        EndScissorMode();
    }

    // --- 9. Layout and Render Children ---
    if (el->child_count > 0 && content_width > 0 && content_height > 0) {
        uint8_t direction = el->header.layout & LAYOUT_DIRECTION_MASK;
        uint8_t alignment = (el->header.layout & LAYOUT_ALIGNMENT_MASK) >> 2;

        int total_child_width_scaled = 0;
        int total_child_height_scaled = 0;
        int flow_child_count = 0;
        int child_sizes[MAX_ELEMENTS][2];

        for (int i = 0; i < el->child_count; i++) {
             RenderElement* child = el->children[i]; if (!child) continue;
             int child_w = (int)(child->header.width * scale_factor);
             int child_h = (int)(child->header.height * scale_factor);
             if (child->header.type == ELEM_TYPE_TEXT && child->text) {
                 int fs = (int)(BASE_FONT_SIZE * scale_factor); if(fs<1)fs=1; int tw = (child->text[0]!='\0') ? MeasureText(child->text, fs):0;
                 if (child->header.width == 0) child_w = tw + (int)(8 * scale_factor);
                 if (child->header.height == 0) child_h = fs + (int)(8 * scale_factor);
             }
             if (child_w < 0) child_w = 0; if (child_h < 0) child_h = 0;
             if (child->header.width > 0 && child_w == 0) child_w = 1; if (child->header.height > 0 && child_h == 0) child_h = 1;
             child_sizes[i][0] = child_w; child_sizes[i][1] = child_h;
             bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);
             if (!child_has_pos) {
                 if (direction == 0x00 || direction == 0x02) total_child_width_scaled += child_w; else total_child_height_scaled += child_h;
                 flow_child_count++;
             }
        }

        int current_x = content_x;
        int current_y = content_y;
        if (direction == 0x00 || direction == 0x02) { // Row flow
            if (alignment == 0x01) current_x = content_x + (content_width - total_child_width_scaled) / 2; // Center H
            else if (alignment == 0x02) current_x = content_x + content_width - total_child_width_scaled; // End H
            if (current_x < content_x) current_x = content_x;
        } else { // Column flow
             if (alignment == 0x01) current_y = content_y + (content_height - total_child_height_scaled) / 2; // Center V
             else if (alignment == 0x02) current_y = content_y + content_height - total_child_height_scaled; // End V
             if (current_y < content_y) current_y = content_y;
        }
        float space_between = 0;
        if (alignment == 0x03 && flow_child_count > 1) { // SpaceBetween
            if (direction == 0x00 || direction == 0x02) space_between = (float)(content_width - total_child_width_scaled) / (flow_child_count - 1);
            else space_between = (float)(content_height - total_child_height_scaled) / (flow_child_count - 1);
            if (space_between < 0) space_between = 0;
        }

        int flow_children_processed = 0;
        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i]; if (!child) continue;
            int child_w = child_sizes[i][0];
            int child_h = child_sizes[i][1];
            int child_render_origin_x, child_render_origin_y;
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);

            if (child_has_pos) { // Absolute position relative to parent content area
                child_render_origin_x = content_x;
                child_render_origin_y = content_y;
            } else { // Flow position
                child_render_origin_x = current_x;
                child_render_origin_y = current_y;
                if (direction == 0x00 || direction == 0x02) { // Row -> Align V
                    if (alignment == 0x01) child_render_origin_y = content_y + (content_height - child_h) / 2;
                    else if (alignment == 0x02) child_render_origin_y = content_y + content_height - child_h;
                    else child_render_origin_y = content_y;
                } else { // Column -> Align H
                    if (alignment == 0x01) child_render_origin_x = content_x + (content_width - child_w) / 2;
                    else if (alignment == 0x02) child_render_origin_x = content_x + content_width - child_w;
                    else child_render_origin_x = content_x;
                }
                if(child_render_origin_x < content_x) child_render_origin_x = content_x;
                if(child_render_origin_y < content_y) child_render_origin_y = content_y;
                if (direction == 0x00 || direction == 0x02) { current_x += child_w; if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) current_x += (int)roundf(space_between); }
                else { current_y += child_h; if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) current_y += (int)roundf(space_between); }
                flow_children_processed++;
            }
            render_element(child, child_render_origin_x, child_render_origin_y, content_width, content_height, scale_factor, debug_file);
        }
    }
}

// --- Main Application Logic ---
int main(int argc, char* argv[]) {
    // --- Setup ---
    if (argc != 2) { printf("Usage: %s <krb_file>\n", argv[0]); return 1; }
    FILE* debug_file = fopen("krb_render_debug.log", "w");
    if (!debug_file) { debug_file = stderr; }
    fprintf(debug_file, "INFO: Opening KRB file: %s\n", argv[1]);
    FILE* file = fopen(argv[1], "rb");
    if (!file) { fprintf(debug_file, "ERROR: Could not open file '%s': %s\n", argv[1], strerror(errno)); if (debug_file != stderr) fclose(debug_file); return 1; }

    // --- Parsing ---
    KrbDocument doc = {0};
    fprintf(debug_file, "INFO: Reading KRB document...\n");
    if (!krb_read_document(file, &doc)) {
        fprintf(debug_file, "ERROR: Failed to parse KRB document\n");
        fclose(file); krb_free_document(&doc); if (debug_file != stderr) fclose(debug_file); return 1;
    }
    fclose(file); file = NULL;
    fprintf(debug_file, "INFO: Parsed KRB OK - Elements=%u, Styles=%u, Strings=%u, Flags=0x%04X\n",
            doc.header.element_count, doc.header.style_count, doc.header.string_count, doc.header.flags);
    if (doc.header.element_count == 0) { fprintf(debug_file, "WARN: No elements found.\n"); if (debug_file != stderr) fclose(debug_file); return 0; }

    // --- Prepare Render Elements ---
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) { fprintf(debug_file, "ERROR: Failed to allocate memory for render elements.\n"); krb_free_document(&doc); if (debug_file != stderr) fclose(debug_file); return 1; }

    // --- Process App & Defaults ---
    Color default_bg = BLACK, default_fg = RAYWHITE, default_border = GRAY;
    int window_width = DEFAULT_WINDOW_WIDTH, window_height = DEFAULT_WINDOW_HEIGHT;
    float scale_factor = DEFAULT_SCALE_FACTOR;
    char* window_title = NULL; bool resizable = false;
    RenderElement* app_element = NULL;

    if ((doc.header.flags & FLAG_HAS_APP) && doc.header.element_count > 0 && doc.elements[0].type == ELEM_TYPE_APP) {
        app_element = &elements[0];
        app_element->header = doc.elements[0];
        app_element->text = NULL; app_element->parent = NULL; app_element->child_count = 0;
        for(int k=0; k<MAX_ELEMENTS; ++k) app_element->children[k] = NULL;
        app_element->render_x = 0; app_element->render_y = 0; app_element->render_w = window_width; app_element->render_h = window_height;
        app_element->is_interactive = false;
        fprintf(debug_file, "INFO: Processing App Element (Index 0)\n");

        if (app_element->header.style_id > 0 && app_element->header.style_id <= doc.header.style_count) {
            int style_idx = app_element->header.style_id - 1;
            if (doc.styles && style_idx >= 0 && style_idx < doc.header.style_count) {
                KrbStyle* app_style = &doc.styles[style_idx];
                for (int j = 0; j < app_style->property_count; j++) {
                    KrbProperty* prop = &app_style->properties[j]; if (!prop || !prop->value) continue;
                    if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; default_bg = (Color){c[0], c[1], c[2], c[3]}; }
                    else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; default_fg = (Color){c[0], c[1], c[2], c[3]}; }
                    else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; default_border = (Color){c[0], c[1], c[2], c[3]}; }
                }
            } else { fprintf(debug_file, "WARN: App Style ID %d is invalid.\n", app_element->header.style_id); }
        }
        app_element->bg_color = default_bg; app_element->fg_color = default_fg;
        app_element->border_color = default_border; memset(app_element->border_widths, 0, 4);

        if (doc.properties && doc.properties[0]) {
            for (int j = 0; j < app_element->header.property_count; j++) {
                KrbProperty* prop = &doc.properties[0][j]; if (!prop || !prop->value) continue;
                if (prop->property_id == PROP_ID_WINDOW_WIDTH && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { uint16_t w = read_u16(prop->value); window_width = app_element->header.width = w; app_element->render_w = w;}
                else if (prop->property_id == PROP_ID_WINDOW_HEIGHT && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { uint16_t h = read_u16(prop->value); window_height = app_element->header.height = h; app_element->render_h = h;}
                else if (prop->property_id == PROP_ID_WINDOW_TITLE && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(window_title); window_title = strdup(doc.strings[idx]); } else { fprintf(debug_file, "WARN: App window title string index %d invalid.\n", idx); } }
                else if (prop->property_id == PROP_ID_RESIZABLE && prop->value_type == VAL_TYPE_BYTE && prop->size == 1) { resizable = *(uint8_t*)prop->value; }
                else if (prop->property_id == PROP_ID_SCALE_FACTOR && prop->value_type == VAL_TYPE_PERCENTAGE && prop->size == 2) { uint16_t sf = read_u16(prop->value); scale_factor = sf / 256.0f; }
                else if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; app_element->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; app_element->fg_color = (Color){c[0], c[1], c[2], c[3]}; }
            }
        }
    } else { fprintf(debug_file, "WARN: No App element found (or flag not set). Using default settings.\n"); }

    // --- Populate & Process Remaining RenderElements ---
    for (int i = 0; i < doc.header.element_count; i++) {
        if (app_element && i == 0) continue;

        RenderElement* current_render_el = &elements[i];
        current_render_el->header = doc.elements[i];

        // Init with defaults AND NEW FIELDS
        current_render_el->text = NULL;
        current_render_el->bg_color = default_bg; current_render_el->fg_color = default_fg;
        current_render_el->border_color = default_border; memset(current_render_el->border_widths, 0, 4);
        current_render_el->text_alignment = 0; current_render_el->parent = NULL;
        current_render_el->child_count = 0; for(int k=0; k<MAX_ELEMENTS; ++k) current_render_el->children[k] = NULL;
        current_render_el->render_x = 0; current_render_el->render_y = 0;
        current_render_el->render_w = 0; current_render_el->render_h = 0;
        current_render_el->is_interactive = false; // Default

        // *** Determine Interactivity based on Element Type ***
        // This check uses ELEM_TYPE_BUTTON from krb.h
        if (current_render_el->header.type == ELEM_TYPE_BUTTON) {
            current_render_el->is_interactive = true;
            fprintf(debug_file, "DEBUG: Element %d marked interactive based on type 0x%02X.\n", i, current_render_el->header.type);
        }

        fprintf(debug_file, "INFO: Processing Element %d: type=0x%02X\n", i, current_render_el->header.type);

        // Apply Style FIRST
        if (current_render_el->header.style_id > 0 && current_render_el->header.style_id <= doc.header.style_count) {
            int style_idx = current_render_el->header.style_id - 1;
            if (doc.styles && style_idx >= 0 && style_idx < doc.header.style_count) {
                KrbStyle* style = &doc.styles[style_idx];
                for (int j = 0; j < style->property_count; j++) {
                    KrbProperty* prop = &style->properties[j]; if (!prop || !prop->value) continue;
                    if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                    else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->fg_color = (Color){c[0], c[1], c[2], c[3]}; }
                    else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->border_color = (Color){c[0], c[1], c[2], c[3]}; }
                    else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if (prop->value_type == VAL_TYPE_BYTE && prop->size == 1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size == 4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); }
                    else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size == 1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                    else if (prop->property_id == STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS && prop->value_type == VAL_TYPE_ENUM && prop->size == 1 && prop->value) { uint8_t sa=*(uint8_t*)prop->value; if(sa<=3){ uint8_t ol=current_render_el->header.layout; current_render_el->header.layout &= ~LAYOUT_ALIGNMENT_MASK; current_render_el->header.layout |= (sa << 2); fprintf(debug_file, "WARN: Applied Layout Align %d from Style (0x%02X) to Elem %d. Layout 0x%02X -> 0x%02X\n", sa, STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS, i, ol, current_render_el->header.layout); } }
                }
            } else { fprintf(debug_file, "WARN: Style ID %d for Element %d is invalid.\n", current_render_el->header.style_id, i); }
        }

        // Apply Direct Properties SECOND
        if (doc.properties && i < doc.header.element_count && doc.properties[i]) {
            for (int j = 0; j < current_render_el->header.property_count; j++) {
                KrbProperty* prop = &doc.properties[i][j]; if (!prop || !prop->value) continue;
                 if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->fg_color = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->border_color = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if (prop->value_type == VAL_TYPE_BYTE && prop->size == 1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size == 4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); }
                 else if (prop->property_id == PROP_ID_TEXT_CONTENT && prop->value_type == VAL_TYPE_STRING && prop->size == 1 && prop->value) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(current_render_el->text); current_render_el->text = strdup(doc.strings[idx]); } else { fprintf(debug_file, "WARN: Element %d text string index %d invalid.\n", i, idx); } }
                 else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size == 1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
            }
        }
    }

    // --- Build Parent/Child Tree (HACK) ---
    fprintf(debug_file, "WARN: Using TEMPORARY HACK for tree building (assumes sequential order)\n");
    RenderElement* parent_stack[MAX_ELEMENTS]; int stack_top = -1;
    for (int i = 0; i < doc.header.element_count; i++) {
        while (stack_top >= 0) { RenderElement* p = parent_stack[stack_top]; if (p->child_count >= p->header.child_count) stack_top--; else break; }
        if (stack_top >= 0) { RenderElement* cp = parent_stack[stack_top]; elements[i].parent = cp; if (cp->child_count < MAX_ELEMENTS) cp->children[cp->child_count++] = &elements[i]; else { fprintf(debug_file, "WARN: Exceeded MAX_CHILDREN for element %ld\n", cp - elements); } }
        if (elements[i].header.child_count > 0) { if (stack_top + 1 < MAX_ELEMENTS) parent_stack[++stack_top] = &elements[i]; else { fprintf(debug_file, "WARN: Exceeded MAX_ELEMENTS for parent stack depth at element %d\n", i); } }
    }

    // --- Find Roots ---
    RenderElement* root_elements[MAX_ELEMENTS]; int root_count = 0;
    for(int i = 0; i < doc.header.element_count; ++i) { if (!elements[i].parent) { if (root_count < MAX_ELEMENTS) root_elements[root_count++] = &elements[i]; else { fprintf(debug_file, "WARN: Exceeded MAX_ELEMENTS for root elements.\n"); break; } } }
    if (root_count == 0 && doc.header.element_count > 0) { fprintf(debug_file, "ERROR: No root element found.\n"); krb_free_document(&doc); free(elements); if (debug_file != stderr) fclose(debug_file); return 1; }
    else if (root_count > 0) { fprintf(debug_file, "INFO: Found %d root(s). First is Elem %ld.\n", root_count, root_elements[0] - elements); if ((doc.header.flags & FLAG_HAS_APP)) { if(app_element != root_elements[0] || root_count > 1) { fprintf(debug_file, "INFO: App flag set, forcing App Elem 0 as single root.\n"); root_elements[0] = app_element; root_count = 1;} } }

    // --- Init Window ---
    fprintf(debug_file, "INFO: Initializing window %dx%d '%s'\n", window_width, window_height, window_title ? window_title : "KRB Renderer");
    InitWindow(window_width, window_height, window_title ? window_title : "KRB Renderer");
    if (resizable) SetWindowState(FLAG_WINDOW_RESIZABLE);
    SetTargetFPS(60);

    // --- Main Loop ---
    while (!WindowShouldClose()) {

        // --- Input & Update ---
        Vector2 mousePos = GetMousePosition();
        bool cursor_set_to_hand = false; // Track if cursor was set this frame

        // --- Interaction Check ---
        SetMouseCursor(MOUSE_CURSOR_DEFAULT); // Reset cursor each frame
        for (int i = doc.header.element_count - 1; i >= 0; --i) { // Check top-most first
            RenderElement* el = &elements[i];
            // Check if interactive AND has been rendered (has non-zero size)
            if (el->is_interactive && el->render_w > 0 && el->render_h > 0) {
                Rectangle elementRect = { (float)el->render_x, (float)el->render_y, (float)el->render_w, (float)el->render_h };
                if (CheckCollisionPointRec(mousePos, elementRect)) {
                    SetMouseCursor(MOUSE_CURSOR_POINTING_HAND);
                    cursor_set_to_hand = true;
                    break; // Found the topmost interactive element under cursor
                }
            }
        }

        // --- Drawing ---
        if (resizable) {
            window_width = GetScreenWidth(); window_height = GetScreenHeight();
            if (app_element) { app_element->render_w = window_width; app_element->render_h = window_height; }
        }

        BeginDrawing();
        Color clear_color = BLACK;
        if (app_element) clear_color = app_element->bg_color; else if (root_count > 0) clear_color = root_elements[0]->bg_color;
        ClearBackground(clear_color);

        // Render roots (recalculates layout and render bounds)
        for (int i = 0; i < root_count; ++i) {
            if (root_elements[i]) {
                render_element(root_elements[i], 0, 0, window_width, window_height, scale_factor, debug_file);
            }
        }

        // DrawFPS(10, 10); // Optional

        EndDrawing();
    }

    // --- Cleanup ---
    fprintf(debug_file, "INFO: Closing window and cleaning up.\n");
    CloseWindow();
    for (int i = 0; i < doc.header.element_count; i++) { free(elements[i].text); }
    free(elements); elements = NULL;
    free(window_title); window_title = NULL;
    krb_free_document(&doc);
    if (debug_file != stderr) fclose(debug_file);
    return 0;
}