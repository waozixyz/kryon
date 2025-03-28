#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include "raylib.h" // Include Raylib first
#include "krb.h"    // Include your KRB header

// Use the definition from krb.h if suitable, or redefine if needed for renderer only
#ifndef MAX_ELEMENTS
#define MAX_ELEMENTS 256
#endif
#define DEFAULT_WINDOW_WIDTH 800
#define DEFAULT_WINDOW_HEIGHT 600
#define DEFAULT_SCALE_FACTOR 1.0f
#define BASE_FONT_SIZE 20 // Base size for text rendering

// --- KRB Layout Bit Definitions (Based on Spec) ---
#define LAYOUT_DIRECTION_MASK 0x03 // Bits 0-1
#define LAYOUT_ALIGNMENT_MASK 0x0C // Bits 2-3
#define LAYOUT_WRAP_BIT       0x10 // Bit 4
#define LAYOUT_GROW_BIT       0x20 // Bit 5
#define LAYOUT_ABSOLUTE_BIT   0x40 // Bit 6
// --- End KRB Definitions ---

// --- Guessed Style Property ID for Layout (based on KRB dump) ---
#define STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS 0x07 // Property ID seen in containerstyle dump

// Structure specifically for rendering, holding resolved values
typedef struct RenderElement {
    KrbElementHeader header;      // Copy of the header - MAY BE MODIFIED by style application
    char* text;                   // Resolved text string (if applicable)
    Color bg_color;               // Resolved background color
    Color fg_color;               // Resolved foreground/text color
    Color border_color;           // Resolved border color
    uint8_t border_widths[4];     // Resolved border widths [T, R, B, L]
    uint8_t text_alignment;       // Resolved text alignment (0=L, 1=C, 2=R)
    struct RenderElement* parent; // Pointer to parent RenderElement
    struct RenderElement* children[MAX_ELEMENTS]; // Array of pointers to children
    int child_count;              // Actual number of children linked
} RenderElement;


// Helper function
uint16_t read_u16(const void* data) {
    if (!data) return 0;
    const unsigned char* p = (const unsigned char*)data;
    return (uint16_t)(p[0] | (p[1] << 8)); // Little-endian
}

// --- Rendering Function (Simplified Positioning, Relies on Corrected Header) ---
void render_element(RenderElement* el, int parent_content_x, int parent_content_y, int parent_content_width, int parent_content_height, float scale_factor, FILE* debug_file) {
    if (!el) return;

    // --- 1. Calculate Element Intrinsic Size ---
    int intrinsic_w = (int)(el->header.width * scale_factor);
    int intrinsic_h = (int)(el->header.height * scale_factor);

    if (el->header.type == 0x02 && el->text) { // Auto-size Text
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        int text_width_measured = (el->text && el->text[0] != '\0') ? MeasureText(el->text, font_size) : 0;
        if (intrinsic_w == 0) intrinsic_w = text_width_measured + (int)(8 * scale_factor); // Basic padding
        if (intrinsic_h == 0) intrinsic_h = font_size + (int)(8 * scale_factor);
    }
    if (intrinsic_w < 0) intrinsic_w = 0;
    if (intrinsic_h < 0) intrinsic_h = 0;

    // --- 2. Determine Element Position and Final Size ---
    // *** SIMPLIFIED POSITIONING LOGIC ***
    // If pos_x/pos_y are provided in the header, use them relative to the parent's content origin.
    // Otherwise, the position is determined solely by the parent's layout logic (passed via parent_content_x/y).
    int final_x, final_y;
    int final_w = intrinsic_w;
    int final_h = intrinsic_h;
    bool has_pos = (el->header.pos_x != 0 || el->header.pos_y != 0);

    if (has_pos) {
        // Treat as absolute relative to parent content origin if pos_x/y are set.
        // This assumes parent_content_x/y represent the origin for this element's coord system.
        final_x = parent_content_x + (int)(el->header.pos_x * scale_factor);
        final_y = parent_content_y + (int)(el->header.pos_y * scale_factor);
         fprintf(debug_file, "DEBUG: Positioning element %p using header pos (%d,%d) relative to parent content origin (%d,%d) -> Final=(%d,%d)\n",
                 (void*)el, el->header.pos_x, el->header.pos_y, parent_content_x, parent_content_y, final_x, final_y);
    } else {
        // No pos_x/pos_y, position is determined by parent's layout (passed as parent_content_x/y).
        final_x = parent_content_x;
        final_y = parent_content_y;
         fprintf(debug_file, "DEBUG: Positioning element %p using parent-calculated layout origin (%d,%d) (no header pos)\n", (void*)el, final_x, final_y);
    }
    // *** END SIMPLIFIED POSITIONING LOGIC ***

    // TODO: Handle Grow flag

    // --- 3. Get Resolved Styles ---
    Color bg_color = el->bg_color;
    Color fg_color = el->fg_color;
    Color border_color = el->border_color;
    int top_bw = (int)(el->border_widths[0] * scale_factor);
    int right_bw = (int)(el->border_widths[1] * scale_factor);
    int bottom_bw = (int)(el->border_widths[2] * scale_factor);
    int left_bw = (int)(el->border_widths[3] * scale_factor);

    // --- 4. Debug Logging ---
    fprintf(debug_file, "Render: Type=0x%02X @(%d,%d) Size=%dx%d Borders=[%d,%d,%d,%d] Text='%s' Align=%d LayoutByte=0x%02X\n",
            el->header.type, final_x, final_y, final_w, final_h,
            top_bw, right_bw, bottom_bw, left_bw,
            el->text ? el->text : "NULL", el->text_alignment, el->header.layout); // Log LayoutByte

    // --- 5. Drawing Background & Borders ---
    if (el->header.type != 0x02) { // Don't draw BG for Text
        if (final_w > 0 && final_h > 0) {
            DrawRectangle(final_x, final_y, final_w, final_h, bg_color);
            // Draw Borders
            if (top_bw > 0) DrawRectangle(final_x, final_y, final_w, top_bw, border_color);
            if (bottom_bw > 0) DrawRectangle(final_x, final_y + final_h - bottom_bw, final_w, bottom_bw, border_color);
            int side_border_y = final_y + top_bw;
            int side_border_height = final_h - top_bw - bottom_bw;
            if (side_border_height < 0) side_border_height = 0;
            if (left_bw > 0) DrawRectangle(final_x, side_border_y, left_bw, side_border_height, border_color);
            if (right_bw > 0) DrawRectangle(final_x + final_w - right_bw, side_border_y, right_bw, side_border_height, border_color);
        }
    }

    // Calculate Content Area
    int content_x = final_x + left_bw;
    int content_y = final_y + top_bw;
    int content_width = final_w - left_bw - right_bw;
    int content_height = final_h - top_bw - bottom_bw;
    if (content_width < 0) content_width = 0;
    if (content_height < 0) content_height = 0;

    // --- 6. Draw Content (e.g., Text) ---
    if (el->header.type == 0x02 && el->text && content_width > 0 && content_height > 0) {
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        int text_width_measured = (el->text && el->text[0] != '\0') ? MeasureText(el->text, font_size) : 0;

        // Calc text draw pos within content area based on text_alignment
        int text_draw_x = content_x; // Default Left (0)
        if (el->text_alignment == 1) { // Center (1)
            text_draw_x = content_x + (content_width - text_width_measured) / 2;
        } else if (el->text_alignment == 2) { // Right (2)
            text_draw_x = content_x + content_width - text_width_measured;
        }
        // Vertical centering
        int text_draw_y = content_y + (content_height - font_size) / 2;

        fprintf(debug_file, "DEBUG: Text Draw: Element=%p Content=(%d,%d %dx%d) TextWidth=%d Align=%d FinalDraw=(%d,%d)\n",
                (void*)el, content_x, content_y, content_width, content_height, text_width_measured, el->text_alignment, text_draw_x, text_draw_y);

        BeginScissorMode(content_x, content_y, content_width, content_height);
        DrawText(el->text, text_draw_x, text_draw_y, font_size, fg_color);
        EndScissorMode();
    }

    // --- 7. Layout and Render Children ---
    // This logic now relies on el->header.layout having the correct alignment bits applied (potentially by the heuristic in main).
    if (el->child_count > 0 && content_width > 0 && content_height > 0) {
        uint8_t direction = el->header.layout & LAYOUT_DIRECTION_MASK;
        uint8_t alignment = (el->header.layout & LAYOUT_ALIGNMENT_MASK) >> 2; // Use potentially corrected alignment

        fprintf(debug_file,"Layout Children: Container=%p LayoutByte=0x%02X -> Dir=%d Align=%d Children=%d ParentContent=(%d,%d %dx%d)\n",
                (void*)el, el->header.layout, direction, alignment, el->child_count, content_x, content_y, content_width, content_height);

        // Pre-calculate sizes of flow children
        int total_child_width_scaled = 0;
        int total_child_height_scaled = 0;
        int flow_child_count = 0;
        int child_sizes[MAX_ELEMENTS][2];

        for (int i = 0; i < el->child_count; i++) {
             RenderElement* child = el->children[i];
             if (!child) continue;

             int child_w = (int)(child->header.width * scale_factor);
             int child_h = (int)(child->header.height * scale_factor);
             if (child->header.type == 0x02 && child->text) { // Auto-size text child
                 int font_size = (int)(BASE_FONT_SIZE * scale_factor);
                 int text_w = (child->text && child->text[0] != '\0') ? MeasureText(child->text, font_size) : 0;
                 if (child_w == 0) child_w = text_w + (int)(8 * scale_factor);
                 if (child_h == 0) child_h = font_size + (int)(8 * scale_factor);
             }
             if (child_w < 0) child_w = 0;
             if (child_h < 0) child_h = 0;
             child_sizes[i][0] = child_w;
             child_sizes[i][1] = child_h;

             // Only include flow children in size calculation
             bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);
             if (!child_has_pos) { // Treat children without pos as flow
                 if (direction == 0x00 || direction == 0x02) total_child_width_scaled += child_w;
                 else total_child_height_scaled += child_h;
                 flow_child_count++;
             }
        }
         fprintf(debug_file, "Layout Children: Flow Child Count=%d, TotalFlowSize=(%d x %d)\n",
                 flow_child_count, total_child_width_scaled, total_child_height_scaled);


        // Calculate starting position based on main-axis alignment
        int current_x = content_x;
        int current_y = content_y;

        if (direction == 0x00 || direction == 0x02) { // Row
            if (alignment == 0x01) current_x = content_x + (content_width - total_child_width_scaled) / 2; // Center
            else if (alignment == 0x02) current_x = content_x + content_width - total_child_width_scaled; // End
            // Start (0) or SpaceBetween (3) handled by initial current_x
            if (current_x < content_x) current_x = content_x; // Clamp
             fprintf(debug_file, "Layout Children: Align=%d (H). StartX=%d\n", alignment, current_x);
        } else { // Column
             if (alignment == 0x01) current_y = content_y + (content_height - total_child_height_scaled) / 2; // Center
             else if (alignment == 0x02) current_y = content_y + content_height - total_child_height_scaled; // End
             // Start (0) or SpaceBetween (3) handled by initial current_y
             if (current_y < content_y) current_y = content_y; // Clamp
             fprintf(debug_file, "Layout Children: Align=%d (V). StartY=%d\n", alignment, current_y);
        }

        // Calculate spacing for SpaceBetween
        float space_between = 0;
        // ... (SpaceBetween calculation - unchanged) ...
        if (alignment == 0x03 /* SpaceBetween */ && flow_child_count > 1) {
            if (direction == 0x00 || direction == 0x02) { // Row
                space_between = (float)(content_width - total_child_width_scaled) / (flow_child_count - 1);
            } else { // Column
                space_between = (float)(content_height - total_child_height_scaled) / (flow_child_count - 1);
            }
            if (space_between < 0) space_between = 0;
        }

         // Iterate and Render Children
         int flow_children_processed = 0;
         for (int i = 0; i < el->child_count; i++) {
             RenderElement* child = el->children[i];
             if (!child) continue;

             int child_w = child_sizes[i][0];
             int child_h = child_sizes[i][1];
             int child_render_origin_x, child_render_origin_y;
             bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);

             if (child_has_pos) {
                 // Child has pos_x/y -> Position it relative to parent content origin
                 // Pass the parent content origin (content_x, content_y)
                 child_render_origin_x = content_x;
                 child_render_origin_y = content_y;
                 fprintf(debug_file,"  Child %d (%p) HAS POS: Size=%dx%d. Passing parent content origin (%d,%d)\n",
                         i, (void*)child, child_w, child_h, child_render_origin_x, child_render_origin_y);
             } else {
                 // Child is Flow -> Position based on parent layout flow and cross-axis alignment
                 child_render_origin_x = current_x; // From main-axis flow
                 child_render_origin_y = current_y; // From main-axis flow

                 // Apply Cross-axis alignment (based on PARENT's alignment)
                 if (direction == 0x00 || direction == 0x02) { // Row: Align vertically
                     if (alignment == 0x01) child_render_origin_y = content_y + (content_height - child_h) / 2; // Center V
                     else if (alignment == 0x02) child_render_origin_y = content_y + content_height - child_h; // End V
                     else child_render_origin_y = content_y; // Start V
                 } else { // Column: Align horizontally
                     if (alignment == 0x01) child_render_origin_x = content_x + (content_width - child_w) / 2; // Center H
                     else if (alignment == 0x02) child_render_origin_x = content_x + content_width - child_w; // End H
                     else child_render_origin_x = content_x; // Start H
                 }

                 // Clamp
                 if(child_render_origin_x < content_x) child_render_origin_x = content_x;
                 if(child_render_origin_y < content_y) child_render_origin_y = content_y;

                 fprintf(debug_file,"  Child %d (%p) FLOW: Layout Pos (%d,%d) Size=%dx%d\n",
                         i, (void*)child, child_render_origin_x, child_render_origin_y, child_w, child_h);

                 // Advance main-axis position for the *next* flow child
                 if (direction == 0x00 || direction == 0x02) { // Row
                     current_x += child_w;
                     if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) current_x += (int)space_between;
                 } else { // Column
                     current_y += child_h;
                      if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) current_y += (int)space_between;
                 }
                 flow_children_processed++;
             }

             // Render the child, passing the calculated origin
             render_element(child, child_render_origin_x, child_render_origin_y, content_width, content_height, scale_factor, debug_file);

         } // End child loop
    } // End child rendering block
}


// --- Main Application Logic ---
int main(int argc, char* argv[]) {
    // --- Setup ---
    if (argc != 2) { printf("Usage: %s <krb_file>\n", argv[0]); return 1; }
    FILE* debug_file = fopen("krb_render_debug.log", "w");
    if (!debug_file) { debug_file = stderr; }
    fprintf(debug_file, "INFO: Opening KRB file: %s\n", argv[1]);
    FILE* file = fopen(argv[1], "rb");
    if (!file) { fprintf(debug_file, "ERROR: Could not open file '%s'\n", argv[1]); if (debug_file != stderr) fclose(debug_file); return 1; }

    // --- Parsing ---
    KrbDocument doc = {0};
    fprintf(debug_file, "INFO: Reading KRB document...\n");
    if (!krb_read_document(file, &doc)) {
        fprintf(debug_file, "ERROR: Failed to parse KRB document\n");
        // Proper cleanup
        fclose(file);
        krb_free_document(&doc);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    fclose(file); file = NULL;
    fprintf(debug_file, "INFO: Parsed KRB OK - Elements=%u, Styles=%u, Strings=%u, Flags=0x%04X\n",
            doc.header.element_count, doc.header.style_count, doc.header.string_count, doc.header.flags);
    if (doc.header.element_count == 0) { /* Handle no elements */ return 0; }

    // --- Prepare Render Elements ---
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) { /* Handle alloc error */ krb_free_document(&doc); if (debug_file != stderr) fclose(debug_file); return 1; }

    // --- Process App & Defaults ---
    Color default_bg = BLACK, default_fg = RAYWHITE, default_border = GRAY;
    int window_width = DEFAULT_WINDOW_WIDTH, window_height = DEFAULT_WINDOW_HEIGHT;
    float scale_factor = DEFAULT_SCALE_FACTOR;
    char* window_title = NULL; bool resizable = false;
    RenderElement* app_element = NULL;

    if ((doc.header.flags & (1 << 6)) && doc.header.element_count > 0 && doc.elements[0].type == 0x00) {
        app_element = &elements[0];
        app_element->header = doc.elements[0]; // Initial copy
        fprintf(debug_file, "INFO: Processing App Element (Index 0)\n");
        // Apply App Style to defaults
        if (app_element->header.style_id > 0 && app_element->header.style_id <= doc.header.style_count) {
             KrbStyle* app_style = &doc.styles[app_element->header.style_id - 1];
             for (int j = 0; j < app_style->property_count; j++) { /* Apply BG/FG defaults */
                KrbProperty* prop = &app_style->properties[j];
                 if (!prop || !prop->value) continue;
                 if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; default_bg = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == 0x02 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; default_fg = (Color){c[0], c[1], c[2], c[3]}; }
             }
        }
        // Apply defaults/style results to App element itself
        app_element->bg_color = default_bg; app_element->fg_color = default_fg; app_element->border_color = default_border; memset(app_element->border_widths, 0, 4);
        // Apply App direct properties
        if (doc.properties && doc.properties[0]) {
             for (int j = 0; j < app_element->header.property_count; j++) {
                 KrbProperty* prop = &doc.properties[0][j]; if (!prop || !prop->value) continue;
                 // Apply Window props, Scale, Title, Resizable, direct BG/FG overrides etc.
                 if (prop->property_id == 0x20 && prop->value_type == 0x02 && prop->size == 2) { uint16_t w = read_u16(prop->value); window_width = app_element->header.width = w; }
                 else if (prop->property_id == 0x21 && prop->value_type == 0x02 && prop->size == 2) { uint16_t h = read_u16(prop->value); window_height = app_element->header.height = h; }
                 else if (prop->property_id == 0x22 && prop->value_type == 0x04 && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count) { free(window_title); window_title = strdup(doc.strings[idx]); } }
                 else if (prop->property_id == 0x23 && prop->value_type == 0x01 && prop->size == 1) { resizable = *(uint8_t*)prop->value; }
                 else if (prop->property_id == 0x25 && prop->value_type == 0x06 && prop->size == 2) { uint16_t sf = read_u16(prop->value); scale_factor = sf / 256.0f; }
                 else if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; app_element->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                 // Add checks for FG 0x02, BorderColor 0x03, BorderWidth 0x04 if needed for App element
             }
        }
    } else { fprintf(debug_file, "WARN: No App element found or flag not set.\n"); }

    // --- Populate & Process Remaining RenderElements ---
    for (int i = 0; i < doc.header.element_count; i++) {
        if (app_element && i == 0) continue; // Skip App

        RenderElement* current_render_el = &elements[i];
        current_render_el->header = doc.elements[i]; // Base header

        // Init with defaults
        current_render_el->text = NULL; current_render_el->bg_color = default_bg; current_render_el->fg_color = default_fg;
        current_render_el->border_color = default_border; memset(current_render_el->border_widths, 0, 4);
        current_render_el->text_alignment = 0; current_render_el->parent = NULL; current_render_el->child_count = 0;
        for(int k=0; k<MAX_ELEMENTS; ++k) current_render_el->children[k] = NULL;

        fprintf(debug_file, "INFO: Processing Element %d: type=0x%02X\n", i, current_render_el->header.type);

        // Apply Style FIRST (including layout heuristic)
        if (current_render_el->header.style_id > 0 && current_render_el->header.style_id <= doc.header.style_count) {
            KrbStyle* style = &doc.styles[current_render_el->header.style_id - 1];
            fprintf(debug_file, "DEBUG: Applying style %d (ID=%d) to Element %d\n", current_render_el->header.style_id - 1, style->id, i);
            for (int j = 0; j < style->property_count; j++) {
                KrbProperty* prop = &style->properties[j]; if (!prop || !prop->value) continue;
                // Apply visual props
                 if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == 0x02 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->fg_color = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == 0x03 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->border_color = (Color){c[0], c[1], c[2], c[3]}; }
                 else if (prop->property_id == 0x04) { if (prop->value_type == 0x01 && prop->size == 1) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == 0x08 && prop->size == 4) memcpy(current_render_el->border_widths, prop->value, 4); }
                 else if (prop->property_id == 0x0B && prop->value_type == 0x09 && prop->size == 1) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                 // *** LAYOUT HEURISTIC APPLICATION ***
                 else if (prop->property_id == STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS && prop->value_type == 0x09 && prop->size == 1) {
                      uint8_t style_align = *(uint8_t*)prop->value; // 0=Start, 1=Center, 2=End
                      if (style_align <= 3) {
                           uint8_t original_layout = current_render_el->header.layout;
                           current_render_el->header.layout &= ~LAYOUT_ALIGNMENT_MASK; // Clear existing alignment bits
                           current_render_el->header.layout |= (style_align << 2);     // Set new alignment bits
                           fprintf(debug_file, "WARN: Applied Layout Alignment %d from Style (Heuristic ID 0x%02X) to Element %d header. Layout 0x%02X -> 0x%02X\n",
                                   style_align, STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS, i, original_layout, current_render_el->header.layout);
                      }
                 }
            }
        }

        // Apply Direct Properties SECOND (override style)
        if (doc.properties && i < doc.header.element_count && doc.properties[i]) {
             for (int j = 0; j < current_render_el->header.property_count; j++) {
                KrbProperty* prop = &doc.properties[i][j]; if (!prop || !prop->value) continue;
                // Apply visual overrides
                if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                else if (prop->property_id == 0x02 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->fg_color = (Color){c[0], c[1], c[2], c[3]}; }
                else if (prop->property_id == 0x03 && prop->value_type == 0x03 && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; current_render_el->border_color = (Color){c[0], c[1], c[2], c[3]}; }
                else if (prop->property_id == 0x04) { if (prop->value_type == 0x01 && prop->size == 1) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == 0x08 && prop->size == 4) memcpy(current_render_el->border_widths, prop->value, 4); }
                // Apply text overrides
                else if (prop->property_id == 0x08 && prop->value_type == 0x04 && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count) { free(current_render_el->text); current_render_el->text = strdup(doc.strings[idx]); } }
                else if (prop->property_id == 0x0B && prop->value_type == 0x09 && prop->size == 1) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                 // Direct properties should ideally also be able to override layout bytes if specified, but not handled here yet.
            }
        }
         fprintf(debug_file, "DEBUG: Element %d Final Processed: Layout=0x%02X Align=%d Text='%s'\n", i,
                 current_render_el->header.layout, current_render_el->text_alignment, current_render_el->text ? current_render_el->text : "NULL");
    }

    // --- Build Parent/Child Tree (HACK - unchanged) ---
    fprintf(debug_file, "WARN: Using TEMPORARY HACK for tree building (assumes sequential order)\n");
    for (int i = 0; i < doc.header.element_count; i++) { /* ... HACK ... */
        RenderElement* parent_el = &elements[i]; int expected_children = parent_el->header.child_count; int children_linked = 0;
        for (int k = i + 1; k < doc.header.element_count && children_linked < expected_children; k++) {
             RenderElement* potential_child = &elements[k];
             if (potential_child->parent == NULL) {
                 if (parent_el->child_count < MAX_ELEMENTS) { parent_el->children[parent_el->child_count++] = potential_child; potential_child->parent = parent_el; children_linked++; }
                 else { fprintf(debug_file, "WARN: Exceeded MAX_ELEMENTS for children of element %d\n", i); break; }
             } else { break; }
        }
         if (children_linked != expected_children) fprintf(debug_file, "WARN: Element %d header expected %d children, but only linked %d using HACK\n", i, expected_children, children_linked);
    }


    // --- Find Roots ---
    RenderElement* root_elements[MAX_ELEMENTS]; int root_count = 0;
    for(int i = 0; i < doc.header.element_count; ++i) { if (elements[i].parent == NULL) { if (root_count < MAX_ELEMENTS) root_elements[root_count++] = &elements[i]; else break; } }
    // Validate roots vs App flag
    if (root_count == 0 && doc.header.element_count > 0) { fprintf(debug_file, "ERROR: No root element found.\n"); }
    else if (root_count > 0) { fprintf(debug_file, "INFO: Found %d root(s). First=%p(idx %ld)\n", root_count, (void*)root_elements[0], root_elements[0] - elements); if ((doc.header.flags & (1 << 6)) && (root_elements[0] != app_element || root_count > 1)) { fprintf(debug_file, "ERROR: App flag/root mismatch.\n"); root_count = 0; } }

    // --- Init Window ---
    fprintf(debug_file, "INFO: Initializing window %dx%d '%s'\n", window_width, window_height, window_title ? window_title : "KRB Renderer");
    InitWindow(window_width, window_height, window_title ? window_title : "KRB Renderer");
    if (resizable) SetWindowState(FLAG_WINDOW_RESIZABLE);
    SetTargetFPS(60);

    // --- Main Loop ---
    while (!WindowShouldClose()) {
        if (resizable) { window_width = GetScreenWidth(); window_height = GetScreenHeight(); }
        BeginDrawing();
        Color clear_color = BLACK;
        if (app_element) clear_color = app_element->bg_color; else if (root_count > 0) clear_color = root_elements[0]->bg_color;
        ClearBackground(clear_color);

        // Render roots
        for (int i = 0; i < root_count; ++i) {
            // Pass 0,0 as initial parent content origin for roots
            render_element(root_elements[i], 0, 0, window_width, window_height, scale_factor, debug_file);
        }

        // DrawFPS(10, 10); // Removed FPS counter

        EndDrawing();
    }

    // --- Cleanup ---
    fprintf(debug_file, "INFO: Closing window and cleaning up.\n");
    CloseWindow();
    for (int i = 0; i < doc.header.element_count; i++) { free(elements[i].text); }
    free(elements); elements = NULL;
    free(window_title);
    krb_free_document(&doc);
    if (debug_file != stderr) fclose(debug_file);
    return 0;
}