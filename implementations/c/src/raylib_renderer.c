#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <errno.h>
#include <math.h>

// Include the new header FIRST
#include "renderer.h" // Contains RenderElement, includes krb.h, raylib.h

// --- Basic Definitions (Can be moved to header if needed elsewhere) ---
#define DEFAULT_WINDOW_WIDTH 800
#define DEFAULT_WINDOW_HEIGHT 600
#define DEFAULT_SCALE_FACTOR 1.0f
#define BASE_FONT_SIZE 20
// #define STYLE_PROP_ID_LAYOUT_ALIGNMENT_GUESS 0x07 // If used

// --- Helper Functions ---

// Definition for read_u16 (declared in renderer.h)
// Ensure it's NOT static
uint16_t read_u16(const void* data) {
    if (!data) return 0;
    const unsigned char* p = (const unsigned char*)data;
    return (uint16_t)(p[0] | (p[1] << 8)); // Little-endian
}

// --- Rendering Function ---

// Definition for render_element (declared in renderer.h)
// Ensure it's NOT static
void render_element(RenderElement* el, int parent_content_x, int parent_content_y, int parent_content_width, int parent_content_height, float scale_factor, FILE* debug_file) {
    // Error check: element is NULL
    if (!el) return;

    // --- 1. Calculate Element Intrinsic Size ---
    int intrinsic_w = (int)(el->header.width * scale_factor);
    int intrinsic_h = (int)(el->header.height * scale_factor);

    // Recalculate size if it's a text element and dimensions are zero
    if (el->header.type == ELEM_TYPE_TEXT && el->text) { // Use defined type from krb.h
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        if (font_size < 1) font_size = 1; // Ensure font size is at least 1
        int text_width_measured = (el->text[0] != '\0') ? MeasureText(el->text, font_size) : 0;
        if (el->header.width == 0) intrinsic_w = text_width_measured + (int)(8 * scale_factor); // Add some padding guess
        if (el->header.height == 0) intrinsic_h = font_size + (int)(8 * scale_factor); // Add some padding guess
    }

    // Ensure dimensions are not negative and have minimum 1px if explicitly set > 0
    if (intrinsic_w < 0) intrinsic_w = 0;
    if (intrinsic_h < 0) intrinsic_h = 0;
    if (el->header.width > 0 && intrinsic_w == 0) intrinsic_w = 1; // Min 1px width if specified > 0
    if (el->header.height > 0 && intrinsic_h == 0) intrinsic_h = 1; // Min 1px height if specified > 0


    // --- 2. Determine Element Position and Final Size ---
    // This depends heavily on the layout model (flow vs absolute).
    // For now, use explicit position if given, otherwise start at parent content origin.
    // Layout logic in section 9 will adjust flow positions.
    int final_x, final_y;
    int final_w = intrinsic_w;
    int final_h = intrinsic_h;
    bool has_pos = (el->header.pos_x != 0 || el->header.pos_y != 0);
    bool is_absolute = (el->header.layout & LAYOUT_ABSOLUTE_BIT); // Check absolute flag

    if (is_absolute || has_pos) { // Treat explicit pos or absolute flag as absolute positioning relative to parent content area
        final_x = parent_content_x + (int)(el->header.pos_x * scale_factor);
        final_y = parent_content_y + (int)(el->header.pos_y * scale_factor);
    } else { // Flow layout - initial position before adjustments in section 9
        // The parent calling this function should pass the correct starting flow position (current_x/current_y)
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

    // Ensure borders don't overlap completely if element is very small
    if (el->render_h > 0 && top_bw + bottom_bw >= el->render_h) { top_bw = el->render_h > 1 ? 1 : el->render_h; bottom_bw = 0; }
    if (el->render_w > 0 && left_bw + right_bw >= el->render_w) { left_bw = el->render_w > 1 ? 1 : el->render_w; right_bw = 0; }

    // --- 5. Debug Logging ---
    // Use original_index from RenderElement struct for better identification
    if (debug_file) {
        fprintf(debug_file, "DEBUG RENDER: Elem %d (Type=0x%02X) @(%d,%d) Size=%dx%d Borders=[%d,%d,%d,%d] Text='%s' Align=%d Layout=0x%02X Interact=%d\n",
                el->original_index, el->header.type, el->render_x, el->render_y, el->render_w, el->render_h,
                top_bw, right_bw, bottom_bw, left_bw,
                el->text ? el->text : "NULL", el->text_alignment, el->header.layout, el->is_interactive);
    }

    // --- 6. Drawing Background & Borders ---
    bool draw_background = true;
    // Don't draw background for pure text nodes with no visible border/padding effect yet
    // More robust check needed if padding affects background visibility
    // if (el->header.type == ELEM_TYPE_TEXT && top_bw == 0 && right_bw == 0 && bottom_bw == 0 && left_bw == 0) {
    //     draw_background = false;
    // }

    // Draw background only if element has positive dimensions
    if (draw_background && el->render_w > 0 && el->render_h > 0) {
        DrawRectangle(el->render_x, el->render_y, el->render_w, el->render_h, bg_color);
    }

    // Draw borders (only if element has size)
    if (el->render_w > 0 && el->render_h > 0) {
        if (top_bw > 0) DrawRectangle(el->render_x, el->render_y, el->render_w, top_bw, border_color);
        if (bottom_bw > 0) DrawRectangle(el->render_x, el->render_y + el->render_h - bottom_bw, el->render_w, bottom_bw, border_color);

        // Calculate area for side borders between top and bottom
        int side_border_y = el->render_y + top_bw;
        int side_border_height = el->render_h - top_bw - bottom_bw;
        if (side_border_height < 0) side_border_height = 0; // Prevent negative height

        if (left_bw > 0) DrawRectangle(el->render_x, side_border_y, left_bw, side_border_height, border_color);
        if (right_bw > 0) DrawRectangle(el->render_x + el->render_w - right_bw, side_border_y, right_bw, side_border_height, border_color);
    }

    // --- 7. Calculate Content Area (inside borders) ---
    int content_x = el->render_x + left_bw;
    int content_y = el->render_y + top_bw;
    int content_width = el->render_w - left_bw - right_bw;
    int content_height = el->render_h - top_bw - bottom_bw;
    // Ensure content dimensions are not negative
    if (content_width < 0) content_width = 0;
    if (content_height < 0) content_height = 0;

    // --- 8. Draw Content (Text) ---
    if (el->text && el->text[0] != '\0' && content_width > 0 && content_height > 0) {
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        if (font_size < 1) font_size = 1;
        int text_width_measured = MeasureText(el->text, font_size);

        int text_draw_x = content_x; // Default: Left alignment (0)
        if (el->text_alignment == 1) { // Center alignment (1)
            text_draw_x = content_x + (content_width - text_width_measured) / 2;
        } else if (el->text_alignment == 2) { // Right alignment (2)
            text_draw_x = content_x + content_width - text_width_measured;
        }
        // Basic vertical centering
        int text_draw_y = content_y + (content_height - font_size) / 2;

        // Simple check to prevent drawing starting outside content bounds
        if (text_draw_x < content_x) text_draw_x = content_x;
        if (text_draw_y < content_y) text_draw_y = content_y;

        // Use Scissor test for proper clipping within the content area
        BeginScissorMode(content_x, content_y, content_width, content_height);
        DrawText(el->text, text_draw_x, text_draw_y, font_size, fg_color);
        EndScissorMode();
    }

    // --- 9. Layout and Render Children ---
    // Only proceed if there are children and the parent has a valid content area
    if (el->child_count > 0 && content_width > 0 && content_height > 0) {
        uint8_t direction = el->header.layout & LAYOUT_DIRECTION_MASK; // 00=Row, 01=Col, etc.
        uint8_t alignment = (el->header.layout & LAYOUT_ALIGNMENT_MASK) >> 2; // 00=Start, 01=Center, etc.
        // bool wrap = (el->header.layout & LAYOUT_WRAP_BIT); // TODO: Implement Wrap
        // TODO: Implement Grow, etc.

        // --- Simplified Flow Layout (Row/Column, Start/Center/End/SpaceBetween) ---
        int current_x = content_x; // Start position for child layout within content area
        int current_y = content_y;
        int total_child_width_scaled = 0;  // Used for Row layout alignment
        int total_child_height_scaled = 0; // Used for Col layout alignment
        int flow_child_count = 0;          // Number of children in the normal flow (not absolute)
        int child_sizes[MAX_ELEMENTS][2]; // Cache calculated child sizes [width, height]

        // First pass: Calculate total size of flow children and cache sizes
        for (int i = 0; i < el->child_count; i++) {
             RenderElement* child = el->children[i];
             if (!child) continue; // Skip if child pointer is null

             // Check if child is absolutely positioned (skip for flow calculations)
             bool child_is_absolute = (child->header.layout & LAYOUT_ABSOLUTE_BIT);
             bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);
             if (child_is_absolute || child_has_pos) {
                 // Mark size as 0 for flow calculation, actual rendering happens later
                 child_sizes[i][0] = 0;
                 child_sizes[i][1] = 0;
                 continue; // Skip the rest of flow calculation for this child
             }

             // Calculate child intrinsic size (similar to parent calculation)
             int child_w = (int)(child->header.width * scale_factor);
             int child_h = (int)(child->header.height * scale_factor);
             if (child->header.type == ELEM_TYPE_TEXT && child->text) {
                 int fs = (int)(BASE_FONT_SIZE * scale_factor); if(fs<1)fs=1;
                 int tw = (child->text[0]!='\0') ? MeasureText(child->text, fs):0;
                 if (child->header.width == 0) child_w = tw + (int)(8 * scale_factor); // Add padding guess
                 if (child->header.height == 0) child_h = fs + (int)(8 * scale_factor); // Add padding guess
             }

             // <<< INSERTED CORRECTED CHECKS HERE >>>
             if (child_w < 0) child_w = 0;
             if (child_h < 0) child_h = 0;
             if (child->header.width > 0 && child_w == 0) child_w = 1; // Min 1px if specified
             if (child->header.height > 0 && child_h == 0) child_h = 1; // Min 1px if specified
             // <<< END OF INSERTED CHECKS >>>

             // Cache the calculated size
             child_sizes[i][0] = child_w;
             child_sizes[i][1] = child_h;

             // Accumulate size based on flow direction
             if (direction == 0x00 || direction == 0x02) { // Row or RowReverse
                 total_child_width_scaled += child_w;
             } else { // Column or ColumnReverse
                 total_child_height_scaled += child_h;
             }
             flow_child_count++;
        } // End of first pass loop

        // Calculate starting position based on alignment (for the whole block of flow children)
        if (direction == 0x00 || direction == 0x02) { // Row Flow
            if (alignment == 0x01) { // Center Align (Horizontal)
                 current_x = content_x + (content_width - total_child_width_scaled) / 2;
            } else if (alignment == 0x02) { // End Align (Horizontal)
                 current_x = content_x + content_width - total_child_width_scaled;
            } // Start Align (0x00) keeps current_x = content_x
              // SpaceBetween (0x03) calculation is handled during placement below
            if (current_x < content_x) current_x = content_x; // Don't let alignment push start outside bounds
        } else { // Column Flow
             if (alignment == 0x01) { // Center Align (Vertical)
                 current_y = content_y + (content_height - total_child_height_scaled) / 2;
             } else if (alignment == 0x02) { // End Align (Vertical)
                 current_y = content_y + content_height - total_child_height_scaled;
             } // Start Align (0x00) keeps current_y = content_y
               // SpaceBetween (0x03) calculation is handled during placement below
             if (current_y < content_y) current_y = content_y; // Don't let alignment push start outside bounds
        }

        // Calculate spacing per item for SpaceBetween alignment
        float space_between = 0;
        if (alignment == 0x03 && flow_child_count > 1) { // SpaceBetween requires more than one flow child
            if (direction == 0x00 || direction == 0x02) { // Row
                 space_between = (float)(content_width - total_child_width_scaled) / (flow_child_count - 1);
            } else { // Column
                 space_between = (float)(content_height - total_child_height_scaled) / (flow_child_count - 1);
            }
            if (space_between < 0) space_between = 0; // Avoid negative spacing if children overflow
        }

        // Second pass: Position and render children recursively
        int flow_children_processed = 0;
        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i];
            if (!child) continue;

            // Get cached size
            int child_w = child_sizes[i][0];
            int child_h = child_sizes[i][1];

            // Determine origin for the recursive call
            int child_render_origin_x, child_render_origin_y;

            // Check positioning type again
            bool child_is_absolute = (child->header.layout & LAYOUT_ABSOLUTE_BIT);
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);

            if (child_is_absolute || child_has_pos) {
                // Absolute positioning: Origin is parent's content area.
                // The child's own render_element call will add its own pos_x/y offset.
                child_render_origin_x = content_x;
                child_render_origin_y = content_y;
                // Render absolutely positioned child, passing parent's content bounds
                render_element(child, child_render_origin_x, child_render_origin_y, content_width, content_height, scale_factor, debug_file);

            } else { // Flow Layout positioning
                // Calculate the child's top-left position based on current flow pos and cross-axis alignment
                if (direction == 0x00 || direction == 0x02) { // Row Flow -> Align Vertically
                    child_render_origin_x = current_x; // Position along the main axis
                    // Align on the cross axis (Vertical)
                    if (alignment == 0x01) child_render_origin_y = content_y + (content_height - child_h) / 2; // Center V
                    else if (alignment == 0x02) child_render_origin_y = content_y + content_height - child_h;   // Align Bottom
                    else child_render_origin_y = content_y; // Align Top (Start) is default
                } else { // Column Flow -> Align Horizontally
                    child_render_origin_y = current_y; // Position along the main axis
                    // Align on the cross axis (Horizontal)
                    if (alignment == 0x01) child_render_origin_x = content_x + (content_width - child_w) / 2; // Center H
                    else if (alignment == 0x02) child_render_origin_x = content_x + content_width - child_w;   // Align Right
                    else child_render_origin_x = content_x; // Align Left (Start) is default
                }

                // Simple boundary check (prevent starting outside parent content)
                if (child_render_origin_x < content_x) child_render_origin_x = content_x;
                if (child_render_origin_y < content_y) child_render_origin_y = content_y;

                // Recursively render the flow child at its calculated origin
                // Pass the PARENT'S content dimensions, not the child's, for bounds.
                render_element(child, child_render_origin_x, child_render_origin_y, content_width, content_height, scale_factor, debug_file);


                // Update the current position for the *next* flow child
                if (direction == 0x00 || direction == 0x02) { // Row Flow
                    current_x += child_w; // Move right by child width
                    // Add space if SpaceBetween and not the last flow child
                    if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) {
                         current_x += (int)roundf(space_between);
                    }
                } else { // Column Flow
                    current_y += child_h; // Move down by child height
                    // Add space if SpaceBetween and not the last flow child
                     if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) {
                         current_y += (int)roundf(space_between);
                    }
                }
                flow_children_processed++;
            } // End flow vs absolute check
        } // End of second pass loop (rendering children)
    } // End of child rendering block
} // End of render_element function


// --- Standalone Main Application Logic ---
// This section is only compiled if BUILD_STANDALONE_RENDERER is defined
#ifdef BUILD_STANDALONE_RENDERER

int main(int argc, char* argv[]) {
    // --- Setup ---
    if (argc != 2) {
        printf("Usage: %s <krb_file>\n", argv[0]);
        return 1;
    }
    const char* krb_file_path = argv[1];

    FILE* debug_file = fopen("krb_render_debug_standalone.log", "w");
    if (!debug_file) {
        debug_file = stderr; // Fallback to stderr
        fprintf(stderr, "Warning: Could not open debug log file.\n");
    }

    fprintf(debug_file, "INFO (Standalone): Opening KRB file: %s\n", krb_file_path);
    FILE* file = fopen(krb_file_path, "rb");
    if (!file) {
        fprintf(stderr, "ERROR: Could not open file '%s': %s\n", krb_file_path, strerror(errno));
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }

    // --- Parsing ---
    KrbDocument doc = {0};
    fprintf(debug_file, "INFO (Standalone): Reading KRB document...\n");
    // Assumes krb_reader functions are linked or compiled in
    if (!krb_read_document(file, &doc)) {
        fprintf(stderr, "ERROR: Failed to parse KRB document '%s'\n", krb_file_path);
        fclose(file);
        krb_free_document(&doc);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    fclose(file);
    fprintf(debug_file, "INFO (Standalone): Parsed KRB OK - Elements=%u, Styles=%u, Strings=%u, Flags=0x%04X\n",
            doc.header.element_count, doc.header.style_count, doc.header.string_count, doc.header.flags);


    if (doc.header.element_count == 0) {
        fprintf(debug_file, "WARN (Standalone): No elements found. Exiting.\n");
        krb_free_document(&doc);
        if (debug_file != stderr) fclose(debug_file);
        return 0;
    }

    // --- Prepare Render Elements ---
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) {
        perror("ERROR (Standalone): Failed to allocate memory for render elements");
        krb_free_document(&doc);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }

    // --- Process App & Defaults ---
    Color default_bg = BLACK, default_fg = RAYWHITE, default_border = GRAY;
    int window_width = DEFAULT_WINDOW_WIDTH, window_height = DEFAULT_WINDOW_HEIGHT;
    float scale_factor = DEFAULT_SCALE_FACTOR;
    char* window_title = NULL; bool resizable = false;
    RenderElement* app_element = NULL;

    if ((doc.header.flags & FLAG_HAS_APP) && doc.header.element_count > 0 && doc.elements[0].type == ELEM_TYPE_APP) {
        app_element = &elements[0];
        app_element->header = doc.elements[0];
        app_element->original_index = 0; // Set original index for App element
        app_element->text = NULL;
        app_element->parent = NULL;
        app_element->child_count = 0;
        for(int k=0; k<MAX_ELEMENTS; ++k) app_element->children[k] = NULL;
        // Set initial render bounds for App based on potentially parsed window size
        app_element->is_interactive = false; // App root usually isn't interactive
        fprintf(debug_file, "INFO (Standalone): Processing App Element (Index 0)\n");

        // Apply App Style as default baseline
        if (app_element->header.style_id > 0 && app_element->header.style_id <= doc.header.style_count) {
             int style_idx = app_element->header.style_id - 1;
             if (doc.styles && style_idx >= 0) {
                KrbStyle* app_style = &doc.styles[style_idx];
                for(int j=0; j<app_style->property_count; ++j) {
                    KrbProperty* prop = &app_style->properties[j]; if (!prop || !prop->value) continue;
                    if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_bg=(Color){c[0],c[1],c[2],c[3]}; }
                    else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_fg=(Color){c[0],c[1],c[2],c[3]}; }
                    else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_border=(Color){c[0],c[1],c[2],c[3]}; }
                }
             } else { fprintf(debug_file, "WARN (Standalone): App Style ID %d is invalid.\n", app_element->header.style_id); }
        }
         // Set resolved colors on App element itself too
         app_element->bg_color = default_bg;
         app_element->fg_color = default_fg;
         app_element->border_color = default_border;
         memset(app_element->border_widths, 0, 4); // App usually has no border widths itself

        // Apply App direct properties (overriding defaults/style)
        // Crucially, read window size properties here to potentially override defaults
        if (doc.properties && doc.properties[0]) {
            for (int j = 0; j < app_element->header.property_count; j++) {
                KrbProperty* prop = &doc.properties[0][j]; if (!prop || !prop->value) continue;
                if (prop->property_id == PROP_ID_WINDOW_WIDTH && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { window_width = read_u16(prop->value); app_element->header.width = window_width; } // Update window_width
                else if (prop->property_id == PROP_ID_WINDOW_HEIGHT && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { window_height = read_u16(prop->value); app_element->header.height = window_height; } // Update window_height
                else if (prop->property_id == PROP_ID_WINDOW_TITLE && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(window_title); window_title = strdup(doc.strings[idx]); } }
                else if (prop->property_id == PROP_ID_RESIZABLE && prop->value_type == VAL_TYPE_BYTE && prop->size == 1) { resizable = *(uint8_t*)prop->value; }
                else if (prop->property_id == PROP_ID_SCALE_FACTOR && prop->value_type == VAL_TYPE_PERCENTAGE && prop->size == 2) { uint16_t sf = read_u16(prop->value); scale_factor = sf / 256.0f; }
                else if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; app_element->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
                // Add other App properties if needed (Icon, Version, Author etc.)
            }
        }
        // Set initial render size for App element AFTER potentially reading window size props
        app_element->render_w = window_width;
        app_element->render_h = window_height;
        app_element->render_x = 0;
        app_element->render_y = 0;

        fprintf(debug_file, "INFO (Standalone): Processed App Element props. Window: %dx%d, Title: '%s'\n", window_width, window_height, window_title ? window_title : "(None)");

    } else {
        fprintf(debug_file, "WARN (Standalone): No App element found or KRB lacks App flag. Using default window settings.\n");
        window_title = strdup("KRB Standalone Renderer"); // Default title
    }


    // --- Populate & Process Remaining RenderElements ---
     for (int i = 0; i < doc.header.element_count; i++) {
        if (app_element && i == 0) continue; // Skip App element if already processed

        RenderElement* current_render_el = &elements[i];
        current_render_el->header = doc.elements[i];
        current_render_el->original_index = i; // Store original index

        // Init with defaults inherited from App or global defaults
        current_render_el->text = NULL;
        current_render_el->bg_color = default_bg;
        current_render_el->fg_color = default_fg;
        current_render_el->border_color = default_border;
        memset(current_render_el->border_widths, 0, 4); // Default no border width
        current_render_el->text_alignment = 0; // Default left align
        current_render_el->parent = NULL; // Will be set by tree building
        current_render_el->child_count = 0; // Initialize child count
        for(int k=0; k<MAX_ELEMENTS; ++k) current_render_el->children[k] = NULL; // Nullify children pointers
        // Initial render bounds will be calculated later by parent layout
        current_render_el->render_x = 0;
        current_render_el->render_y = 0;
        current_render_el->render_w = 0;
        current_render_el->render_h = 0;

        // Standalone renderer might just check type for hover cursor
        current_render_el->is_interactive = (current_render_el->header.type == ELEM_TYPE_BUTTON);
        if (current_render_el->is_interactive) {
            fprintf(debug_file, "DEBUG (Standalone): Element %d (Type 0x%02X) marked interactive.\n", i, current_render_el->header.type);
        }

        // Apply Style FIRST (Overrides defaults)
        if (current_render_el->header.style_id > 0 && current_render_el->header.style_id <= doc.header.style_count) {
            int style_idx = current_render_el->header.style_id - 1;
             if (doc.styles && style_idx >= 0) {
                 KrbStyle* style = &doc.styles[style_idx];
                 for(int j=0; j<style->property_count; ++j) {
                     KrbProperty* prop = &style->properties[j]; if (!prop || !prop->value) continue;
                     // Apply relevant style properties to RenderElement fields
                     if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->bg_color=(Color){c[0],c[1],c[2],c[3]}; }
                     else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->fg_color=(Color){c[0],c[1],c[2],c[3]}; }
                     else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->border_color=(Color){c[0],c[1],c[2],c[3]}; }
                     else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if(prop->value_type == VAL_TYPE_BYTE && prop->size==1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size==4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); }
                     else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                     // Add other style property applications here
                 }
             } else { fprintf(debug_file, "WARN (Standalone): Style ID %d for Element %d is invalid.\n", current_render_el->header.style_id, i); }
        }

        // Apply Direct Properties SECOND (Overrides style and defaults)
        if (doc.properties && i < doc.header.element_count && doc.properties[i]) {
             for (int j = 0; j < current_render_el->header.property_count; j++) {
                 KrbProperty* prop = &doc.properties[i][j]; if (!prop || !prop->value) continue;
                 // Apply relevant direct properties to RenderElement fields
                 if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->bg_color=(Color){c[0],c[1],c[2],c[3]}; }
                 else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->fg_color=(Color){c[0],c[1],c[2],c[3]}; }
                 else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->border_color=(Color){c[0],c[1],c[2],c[3]}; }
                 else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if(prop->value_type == VAL_TYPE_BYTE && prop->size==1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size==4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); }
                 else if (prop->property_id == PROP_ID_TEXT_CONTENT && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(current_render_el->text); current_render_el->text = strdup(doc.strings[idx]); } else { fprintf(debug_file, "WARN (Standalone): Element %d text string index %d invalid.\n", i, idx); } }
                 else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                 // Add other direct property applications here
            }
        }
    } // End loop processing elements

    // --- Build Parent/Child Tree (HACK - same as example) ---
    fprintf(debug_file, "WARN (Standalone): Using TEMPORARY HACK for tree building.\n");
    RenderElement* parent_stack[MAX_ELEMENTS]; int stack_top = -1;
    for (int i = 0; i < doc.header.element_count; i++) {
        // Pop parents from stack if their children are fully processed
        while (stack_top >= 0) {
            RenderElement* p = parent_stack[stack_top];
            // Check against the *original* header's child count
            if (p->child_count >= p->header.child_count) {
                stack_top--; // This parent is done, pop it
            } else {
                break; // Current parent on stack still needs children
            }
        }
        // Assign parent to current element if stack is not empty
        if (stack_top >= 0) {
            RenderElement* cp = parent_stack[stack_top];
            elements[i].parent = cp;
            if (cp->child_count < MAX_ELEMENTS) {
                 // Increment child count *after* assigning
                 cp->children[cp->child_count++] = &elements[i];
            } else {
                 fprintf(debug_file, "WARN (Standalone): Exceeded MAX_ELEMENTS children for element %d\n", cp->original_index);
                 // Decide how to handle this - maybe stop adding children
            }
        }
        // Push current element onto stack if it expects children
        if (elements[i].header.child_count > 0) {
            if (stack_top + 1 < MAX_ELEMENTS) {
                parent_stack[++stack_top] = &elements[i];
            } else {
                 fprintf(debug_file, "WARN (Standalone): Exceeded MAX_ELEMENTS for parent stack depth at element %d\n", i);
                 // Decide how to handle this - maybe stop processing
            }
        }
    } // End tree building loop


    // --- Find Roots ---
    RenderElement* root_elements[MAX_ELEMENTS]; int root_count = 0;
    for(int i = 0; i < doc.header.element_count; ++i) {
        if (!elements[i].parent) { // Elements with no parent are roots
            if (root_count < MAX_ELEMENTS) {
                 root_elements[root_count++] = &elements[i];
            } else {
                 fprintf(debug_file, "WARN (Standalone): Exceeded MAX_ELEMENTS for root elements.\n");
                 break;
            }
        }
    }
    if (root_count == 0 && doc.header.element_count > 0) {
        fprintf(stderr, "ERROR (Standalone): No root element found in KRB.\n");
        krb_free_document(&doc);
        free(elements);
        if(debug_file!=stderr) fclose(debug_file);
        return 1;
    }
    // If App flag is set, ensure the app element is the single root
    if (root_count > 0 && app_element && root_elements[0] != app_element) {
        fprintf(debug_file, "INFO (Standalone): App flag set, forcing App Elem 0 as single root.\n");
        root_elements[0] = app_element;
        root_count = 1;
    }
    fprintf(debug_file, "INFO (Standalone): Found %d root element(s).\n", root_count);


    // --- Init Raylib Window ---
    fprintf(debug_file, "INFO (Standalone): Initializing window %dx%d Title: '%s'\n", window_width, window_height, window_title ? window_title : "KRB Renderer");
    InitWindow(window_width, window_height, window_title ? window_title : "KRB Standalone Renderer");
    if (resizable) SetWindowState(FLAG_WINDOW_RESIZABLE);
    SetTargetFPS(60);

    // --- Main Loop (Standalone) ---
    fprintf(debug_file, "INFO (Standalone): Entering main loop...\n");
    while (!WindowShouldClose()) {

        // --- Input (Standalone might have minimal input handling) ---
        Vector2 mousePos = GetMousePosition();
        // No specific event callback handling in standalone mode

        // --- Update (Window Resizing) ---
         if (resizable && IsWindowResized()) {
            window_width = GetScreenWidth(); window_height = GetScreenHeight();
            // Update root/app element size if necessary for layout recalculation
            if (app_element) { app_element->render_w = window_width; app_element->render_h = window_height; }
             fprintf(debug_file, "INFO (Standalone): Window resized to %dx%d\n", window_width, window_height);
        }

        // --- Interaction Check (Hover only for cursor change in standalone) ---
        SetMouseCursor(MOUSE_CURSOR_DEFAULT); // Reset cursor each frame
        // Iterate top-down (reverse order) to find topmost interactive element under cursor
        for (int i = doc.header.element_count - 1; i >= 0; --i) {
             RenderElement* el = &elements[i];
             // Check only interactive elements that have been rendered (have size)
             if (el->is_interactive && el->render_w > 0 && el->render_h > 0) {
                Rectangle elementRect = { (float)el->render_x, (float)el->render_y, (float)el->render_w, (float)el->render_h };
                if (CheckCollisionPointRec(mousePos, elementRect)) {
                    SetMouseCursor(MOUSE_CURSOR_POINTING_HAND); // Change cursor on hover
                    break; // Found the topmost element, stop checking
                }
            }
        }


        // --- Drawing ---
        BeginDrawing();
        Color clear_color = BLACK; // Default clear color
        if (app_element) {
            clear_color = app_element->bg_color; // Use App background
        } else if (root_count > 0) {
            clear_color = root_elements[0]->bg_color; // Use first root's background if no App
        }
        ClearBackground(clear_color);

        // Render roots (recalculates layout and render bounds each frame)
        // Pass debug_file to the render function
        for (int i = 0; i < root_count; ++i) {
            if (root_elements[i]) {
                // Call the globally available render_element function
                render_element(root_elements[i], 0, 0, window_width, window_height, scale_factor, debug_file);
            }
        }

        EndDrawing();
    } // End main loop

    // --- Cleanup ---
    fprintf(debug_file, "INFO (Standalone): Closing window and cleaning up...\n");
    CloseWindow();

    // Free RenderElement text strings
    for (int i = 0; i < doc.header.element_count; i++) {
        free(elements[i].text); // free(NULL) is safe
    }
    free(elements); // Free the array of RenderElements
    free(window_title); // Free the window title string
    krb_free_document(&doc); // Free all data parsed from KRB

    if (debug_file != stderr) {
        fclose(debug_file);
    }

    printf("Standalone renderer finished.\n");
    return 0;
}

#endif // BUILD_STANDALONE_RENDERER