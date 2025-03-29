#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <errno.h>
#include <math.h> 
#include <libgen.h> 

#include "renderer.h" 

// --- Basic Definitions ---
#define DEFAULT_WINDOW_WIDTH 800
#define DEFAULT_WINDOW_HEIGHT 600
#define DEFAULT_SCALE_FACTOR 1.0f
#define BASE_FONT_SIZE 20
// #define INVALID_RESOURCE_INDEX 0xFF // Defined in renderer.h now

// --- Helper Functions ---
// read_u16/read_u32 are now krb_read_u16_le/krb_read_u32_le in krb.h/c

// --- Rendering Function ---
void render_element(RenderElement* el, int parent_content_x, int parent_content_y, int parent_content_width, int parent_content_height, float scale_factor, FILE* debug_file) {
    if (!el) return;

    // --- Calculate Intrinsic Size ---
    int intrinsic_w = (int)(el->header.width * scale_factor);
    int intrinsic_h = (int)(el->header.height * scale_factor);

    if (el->header.type == ELEM_TYPE_TEXT && el->text) {
        int font_size = (int)(BASE_FONT_SIZE * scale_factor); if (font_size < 1) font_size = 1;
        int text_width_measured = (el->text[0] != '\0') ? MeasureText(el->text, font_size) : 0;
        if (el->header.width == 0) intrinsic_w = text_width_measured + (int)(8 * scale_factor); // Add some padding
        if (el->header.height == 0) intrinsic_h = font_size + (int)(8 * scale_factor); // Add some padding
    }
    else if (el->header.type == ELEM_TYPE_IMAGE && el->texture_loaded) { // Use new field
        if (el->header.width == 0) intrinsic_w = (int)(el->texture.width * scale_factor); // Use new field
        if (el->header.height == 0) intrinsic_h = (int)(el->texture.height * scale_factor); // Use new field
    }

    // Clamp minimum size
    if (intrinsic_w < 0) intrinsic_w = 0;
    if (intrinsic_h < 0) intrinsic_h = 0;
    if (el->header.width > 0 && intrinsic_w == 0) intrinsic_w = 1; // Ensure non-zero if specified
    if (el->header.height > 0 && intrinsic_h == 0) intrinsic_h = 1;

    // --- Determine Final Position & Size (Layout) ---
    int final_x, final_y;
    int final_w = intrinsic_w;
    int final_h = intrinsic_h;
    bool has_pos = (el->header.pos_x != 0 || el->header.pos_y != 0); // Explicit position set
    bool is_absolute = (el->header.layout & LAYOUT_ABSOLUTE_BIT);

    // Absolute positioning (or if explicit pos_x/y is set, treat as absolute relative to parent content)
    if (is_absolute || has_pos) {
        final_x = parent_content_x + (int)(el->header.pos_x * scale_factor);
        final_y = parent_content_y + (int)(el->header.pos_y * scale_factor);
    }
    // Flow layout - position determined by parent's layout logic (passed via el->render_x/y)
    else if (el->parent != NULL) {
        final_x = el->render_x; // Use pre-calculated flow position
        final_y = el->render_y;
    }
    // Root element in flow layout - defaults to parent content origin
    else {
        final_x = parent_content_x;
        final_y = parent_content_y;
    }

    // Store final calculated render coordinates
    el->render_x = final_x;
    el->render_y = final_y;
    el->render_w = final_w;
    el->render_h = final_h;

    // --- Apply Styling ---
    Color bg_color = el->bg_color;
    Color fg_color = el->fg_color; // Used for text
    Color border_color = el->border_color;
    int top_bw = (int)(el->border_widths[0] * scale_factor);
    int right_bw = (int)(el->border_widths[1] * scale_factor);
    int bottom_bw = (int)(el->border_widths[2] * scale_factor);
    int left_bw = (int)(el->border_widths[3] * scale_factor);

    // Clamp borders if they exceed element size
    if (el->render_h > 0 && top_bw + bottom_bw >= el->render_h) { top_bw = el->render_h > 1 ? 1 : el->render_h; bottom_bw = 0; }
    if (el->render_w > 0 && left_bw + right_bw >= el->render_w) { left_bw = el->render_w > 1 ? 1 : el->render_w; right_bw = 0; }

    // Debug Logging
    if (debug_file) {
        fprintf(debug_file, "DEBUG RENDER: Elem %d (Type=0x%02X) @(%d,%d) Size=%dx%d Borders=[%d,%d,%d,%d] Layout=0x%02X ResIdx=%d\n",
                el->original_index, el->header.type, el->render_x, el->render_y, el->render_w, el->render_h,
                top_bw, right_bw, bottom_bw, left_bw, el->header.layout, el->resource_index); // Use new field
    }

    // --- Draw Background (unless it's just text) ---
    bool draw_background = (el->header.type != ELEM_TYPE_TEXT);
    if (draw_background && el->render_w > 0 && el->render_h > 0) {
        DrawRectangle(el->render_x, el->render_y, el->render_w, el->render_h, bg_color);
    }

    // --- Draw Borders ---
    if (el->render_w > 0 && el->render_h > 0) {
        if (top_bw > 0) DrawRectangle(el->render_x, el->render_y, el->render_w, top_bw, border_color);
        if (bottom_bw > 0) DrawRectangle(el->render_x, el->render_y + el->render_h - bottom_bw, el->render_w, bottom_bw, border_color);
        int side_border_y = el->render_y + top_bw;
        int side_border_height = el->render_h - top_bw - bottom_bw; if (side_border_height < 0) side_border_height = 0;
        if (left_bw > 0) DrawRectangle(el->render_x, side_border_y, left_bw, side_border_height, border_color);
        if (right_bw > 0) DrawRectangle(el->render_x + el->render_w - right_bw, side_border_y, right_bw, side_border_height, border_color);
    }

    // --- Calculate Content Area ---
    int content_x = el->render_x + left_bw;
    int content_y = el->render_y + top_bw;
    int content_width = el->render_w - left_bw - right_bw;
    int content_height = el->render_h - top_bw - bottom_bw;
    if (content_width < 0) content_width = 0;
    if (content_height < 0) content_height = 0;

    // --- Draw Content (Text or Image) ---
    if (content_width > 0 && content_height > 0) {
        BeginScissorMode(content_x, content_y, content_width, content_height);

        // Draw Text

        if ((el->header.type == ELEM_TYPE_TEXT || el->header.type == ELEM_TYPE_BUTTON) && el->text && el->text[0] != '\0') {
            int font_size = (int)(BASE_FONT_SIZE * scale_factor); if (font_size < 1) font_size = 1;
            int text_width_measured = MeasureText(el->text, font_size);
            int text_draw_x = content_x;
            if (el->text_alignment == 1) text_draw_x = content_x + (content_width - text_width_measured) / 2; // Center
            else if (el->text_alignment == 2) text_draw_x = content_x + content_width - text_width_measured;   // End/Right
            int text_draw_y = content_y + (content_height - font_size) / 2; // Vertical center

            if (text_draw_x < content_x) text_draw_x = content_x; // Clamp
            if (text_draw_y < content_y) text_draw_y = content_y; // Clamp

            if (debug_file) fprintf(debug_file, "  -> Drawing Text (Type %02X) '%s' (align=%d) at (%d,%d) within content (%d,%d %dx%d)\n", el->header.type, el->text, el->text_alignment, text_draw_x, text_draw_y, content_x, content_y, content_width, content_height);
            DrawText(el->text, text_draw_x, text_draw_y, font_size, fg_color);
        }
        
        // Draw Image
        else if (el->header.type == ELEM_TYPE_IMAGE && el->texture_loaded) { // Use new field
             if (debug_file) fprintf(debug_file, "  -> Drawing Image Texture (ResIdx %d) within content (%d,%d %dx%d)\n", el->resource_index, content_x, content_y, content_width, content_height); // Use new field
             // Simple stretch draw for now
             Rectangle sourceRec = { 0.0f, 0.0f, (float)el->texture.width, (float)el->texture.height }; // Use new field
             Rectangle destRec = { (float)content_x, (float)content_y, (float)content_width, (float)content_height };
             Vector2 origin = { 0.0f, 0.0f };
             DrawTexturePro(el->texture, sourceRec, destRec, origin, 0.0f, WHITE); // Use new field
             // TODO: Add aspect ratio handling if needed (e.g., letterboxing/pillarboxing)
        }

        EndScissorMode();
    }

    // --- Layout and Render Children ---
    if (el->child_count > 0 && content_width > 0 && content_height > 0) {
        uint8_t direction = el->header.layout & LAYOUT_DIRECTION_MASK; // 00=Row, 01=Col, etc.
        uint8_t alignment = (el->header.layout & LAYOUT_ALIGNMENT_MASK) >> 2; // 00=Start, 01=Center, etc.
        int current_flow_x = content_x;
        int current_flow_y = content_y;
        int total_child_width_scaled = 0;  // Total width of flow children in this row/column
        int total_child_height_scaled = 0; // Total height of flow children in this row/column
        int flow_child_count = 0;          // Number of children participating in flow layout
        int child_sizes[MAX_ELEMENTS][2];  // Store calculated sizes [width, height]

        if (debug_file) fprintf(debug_file, "  Layout Children of Elem %d: Count=%d Dir=%d Align=%d Content=(%d,%d %dx%d)\n", el->original_index, el->child_count, direction, alignment, content_x, content_y, content_width, content_height);

        // Pass 1: Calculate sizes and total dimensions of flow children
        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i];
            if (!child) continue;
            bool child_is_absolute = (child->header.layout & LAYOUT_ABSOLUTE_BIT);
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);

            // Skip absolute/positioned children for flow calculations
            if (child_is_absolute || child_has_pos) {
                child_sizes[i][0] = 0; child_sizes[i][1] = 0;
                continue;
            }

            // Calculate child intrinsic size (similar logic to parent)
            int child_w = (int)(child->header.width * scale_factor);
            int child_h = (int)(child->header.height * scale_factor);
            if (child->header.type == ELEM_TYPE_TEXT && child->text) {
                int fs = (int)(BASE_FONT_SIZE * scale_factor); if(fs<1)fs=1;
                int tw = (child->text[0]!='\0') ? MeasureText(child->text, fs):0;
                if (child->header.width == 0) child_w = tw + (int)(8 * scale_factor);
                if (child->header.height == 0) child_h = fs + (int)(8 * scale_factor);
            }
            else if (child->header.type == ELEM_TYPE_IMAGE && child->texture_loaded) { // Use new field
                if (child->header.width == 0) child_w = (int)(child->texture.width * scale_factor); // Use new field
                if (child->header.height == 0) child_h = (int)(child->texture.height * scale_factor); // Use new field
            }
            // Clamp and ensure minimum size
            if (child_w < 0) child_w = 0;
            if (child_h < 0) child_h = 0; // Fix misleading indentation warning
            if (child->header.width > 0 && child_w == 0) child_w = 1;
            if (child->header.height > 0 && child_h == 0) child_h = 1;

            child_sizes[i][0] = child_w;
            child_sizes[i][1] = child_h;

            // Accumulate total size based on flow direction
            if (direction == 0x00 || direction == 0x02) { // Row or RowReverse
                total_child_width_scaled += child_w;
            } else { // Column or ColumnReverse
                total_child_height_scaled += child_h;
            }
            flow_child_count++;
        }

        // Pass 2: Calculate starting position based on alignment
        if (direction == 0x00 || direction == 0x02) { // Row flow
            if (alignment == 0x01) { current_flow_x = content_x + (content_width - total_child_width_scaled) / 2; } // Center
            else if (alignment == 0x02) { current_flow_x = content_x + content_width - total_child_width_scaled; }   // End
            // else: Start (default)
            if (current_flow_x < content_x) current_flow_x = content_x; // Clamp
        } else { // Column flow
            if (alignment == 0x01) { current_flow_y = content_y + (content_height - total_child_height_scaled) / 2; } // Center
            else if (alignment == 0x02) { current_flow_y = content_y + content_height - total_child_height_scaled; }   // End
            // else: Start (default)
            if (current_flow_y < content_y) current_flow_y = content_y; // Clamp
        }

        // Calculate spacing for SpaceBetween
        float space_between = 0;
        if (alignment == 0x03 && flow_child_count > 1) { // SpaceBetween
            if (direction == 0x00 || direction == 0x02) { // Row
                space_between = (float)(content_width - total_child_width_scaled) / (flow_child_count - 1);
            } else { // Column
                space_between = (float)(content_height - total_child_height_scaled) / (flow_child_count - 1);
            }
            if (space_between < 0) space_between = 0; // Avoid negative spacing
        }

        // Pass 3: Position and render children
        int flow_children_processed = 0;
        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i];
            if (!child) continue;

            bool child_is_absolute = (child->header.layout & LAYOUT_ABSOLUTE_BIT);
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);

            // Render absolutely positioned children directly relative to parent content area
            if (child_is_absolute || child_has_pos) {
                render_element(child, content_x, content_y, content_width, content_height, scale_factor, debug_file);
            }
            // Position and render flow children
            else {
                int child_w = child_sizes[i][0];
                int child_h = child_sizes[i][1];
                int child_final_x, child_final_y;

                // Determine position based on flow direction and alignment
                if (direction == 0x00 || direction == 0x02) { // Row Flow
                    child_final_x = current_flow_x;
                    // Vertical alignment within the row
                    if (alignment == 0x01) child_final_y = content_y + (content_height - child_h) / 2; // Center
                    else if (alignment == 0x02) child_final_y = content_y + content_height - child_h; // End (bottom)
                    else child_final_y = content_y; // Start (top)
                } else { // Column Flow
                    child_final_y = current_flow_y;
                    // Horizontal alignment within the column
                    if (alignment == 0x01) child_final_x = content_x + (content_width - child_w) / 2; // Center
                    else if (alignment == 0x02) child_final_x = content_x + content_width - child_w; // End (right)
                    else child_final_x = content_x; // Start (left)
                }

                // Assign calculated position to child before rendering
                child->render_x = child_final_x;
                child->render_y = child_final_y;
                // Width/Height already calculated in Pass 1 and stored in child_sizes

                render_element(child, content_x, content_y, content_width, content_height, scale_factor, debug_file);

                // Advance flow position for the next child
                if (direction == 0x00 || direction == 0x02) { // Row
                    current_flow_x += child_w;
                    if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) {
                        current_flow_x += (int)roundf(space_between); // Add space between
                    }
                } else { // Column
                    current_flow_y += child_h;
                    if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) {
                        current_flow_y += (int)roundf(space_between); // Add space between
                    }
                }
                flow_children_processed++;
            }
        }
    } // End child rendering

    if (debug_file) fprintf(debug_file, "  Finished Render Elem %d\n", el->original_index);
}

// --- Standalone Main Application Logic ---
#ifdef BUILD_STANDALONE_RENDERER

int main(int argc, char* argv[]) {
    // --- Setup ---
    if (argc != 2) { printf("Usage: %s <krb_file>\n", argv[0]); return 1; }
    const char* krb_file_path = argv[1];
    char* krb_file_path_copy = strdup(krb_file_path); // Create a copy for dirname
    if (!krb_file_path_copy) { perror("Failed to duplicate krb_file_path"); return 1; }
    const char* krb_dir = dirname(krb_file_path_copy); // Extract directory

    FILE* debug_file = fopen("krb_render_debug_standalone.log", "w");
    if (!debug_file) { debug_file = stderr; fprintf(stderr, "Warn: No debug log.\n"); }
    setvbuf(debug_file, NULL, _IOLBF, BUFSIZ);
    fprintf(debug_file, "INFO: Opening KRB: %s\n", krb_file_path);
    fprintf(debug_file, "INFO: Base Directory: %s\n", krb_dir);
    FILE* file = fopen(krb_file_path, "rb");
    if (!file) { fprintf(stderr, "ERROR: Cannot open '%s': %s\n", krb_file_path, strerror(errno)); free(krb_file_path_copy); if (debug_file != stderr) fclose(debug_file); return 1; }

    // --- Parsing ---
    KrbDocument doc = {0};
    fprintf(debug_file, "INFO: Reading KRB document...\n");
    if (!krb_read_document(file, &doc)) {
        fprintf(stderr, "ERROR: Failed parse KRB '%s'\n", krb_file_path);
        fclose(file); krb_free_document(&doc); free(krb_file_path_copy); if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    fclose(file);
    fprintf(debug_file, "INFO: Parsed KRB OK - Ver=%u.%u Elements=%u Styles=%u Strings=%u Resources=%u Flags=0x%04X\n",
            doc.version_major, doc.version_minor, doc.header.element_count, doc.header.style_count, doc.header.string_count, doc.header.resource_count, doc.header.flags);
    if (doc.header.element_count == 0) {
        fprintf(debug_file, "WARN: No elements. Exiting.\n");
        krb_free_document(&doc); free(krb_file_path_copy); if (debug_file != stderr) fclose(debug_file);
        return 0;
    }
    if (doc.version_major != KRB_SPEC_VERSION_MAJOR || doc.version_minor != KRB_SPEC_VERSION_MINOR) {
         fprintf(stderr, "WARN: KRB version mismatch! Doc is %d.%d, Reader expects %d.%d. Parsing continues...\n",
                 doc.version_major, doc.version_minor, KRB_SPEC_VERSION_MAJOR, KRB_SPEC_VERSION_MINOR);
         fprintf(debug_file, "WARN: KRB version mismatch! Doc is %d.%d, Reader expects %d.%d.\n",
                 doc.version_major, doc.version_minor, KRB_SPEC_VERSION_MAJOR, KRB_SPEC_VERSION_MINOR);
     }


    // --- Prepare Render Elements ---
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) { perror("ERROR: calloc render elements"); krb_free_document(&doc); free(krb_file_path_copy); if (debug_file != stderr) fclose(debug_file); return 1; }

    // --- Process App & Defaults ---
    Color default_bg = BLACK, default_fg = RAYWHITE, default_border = GRAY;
    int window_width = DEFAULT_WINDOW_WIDTH, window_height = DEFAULT_WINDOW_HEIGHT;
    float scale_factor = DEFAULT_SCALE_FACTOR; char* window_title = NULL; bool resizable = false;
    RenderElement* app_element = NULL;

    if ((doc.header.flags & FLAG_HAS_APP) && doc.header.element_count > 0 && doc.elements[0].type == ELEM_TYPE_APP) {
        app_element = &elements[0];
        app_element->header = doc.elements[0]; app_element->original_index = 0; app_element->text = NULL;
        app_element->parent = NULL; app_element->child_count = 0; app_element->texture_loaded = false;
        app_element->resource_index = INVALID_RESOURCE_INDEX;
        for(int k=0; k<MAX_ELEMENTS; ++k) app_element->children[k] = NULL; app_element->is_interactive = false;
        fprintf(debug_file, "INFO: Processing App Elem 0 (StyleID=%d, Props=%d)\n", app_element->header.style_id, app_element->header.property_count);
        // Apply App Style
        if (app_element->header.style_id > 0 && app_element->header.style_id <= doc.header.style_count && doc.styles) {
             int style_idx = app_element->header.style_id - 1; KrbStyle* app_style = &doc.styles[style_idx];
             fprintf(debug_file, "  Applying App Style %d\n", style_idx);
             for(int j=0; j<app_style->property_count; ++j) { KrbProperty* prop = &app_style->properties[j]; if (!prop || !prop->value) continue; if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_bg=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_fg=(Color){c[0],c[1],c[2],c[3]};} else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_border=(Color){c[0],c[1],c[2],c[3]}; } }
        } else if (app_element->header.style_id > 0) { fprintf(debug_file, "WARN: App Style ID %d invalid.\n", app_element->header.style_id); }
         app_element->bg_color = default_bg; app_element->fg_color = default_fg; app_element->border_color = default_border; memset(app_element->border_widths, 0, 4);
        // Apply App Direct Properties
        fprintf(debug_file, "  Applying App Direct Props\n");
        if (doc.properties && doc.properties[0]) {
            for (int j = 0; j < app_element->header.property_count; j++) { KrbProperty* prop = &doc.properties[0][j]; if (!prop || !prop->value) continue; if (prop->property_id == PROP_ID_WINDOW_WIDTH && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { window_width = krb_read_u16_le(prop->value); app_element->header.width = window_width; } else if (prop->property_id == PROP_ID_WINDOW_HEIGHT && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { window_height = krb_read_u16_le(prop->value); app_element->header.height = window_height; } else if (prop->property_id == PROP_ID_WINDOW_TITLE && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(window_title); window_title = strdup(doc.strings[idx]); } } else if (prop->property_id == PROP_ID_RESIZABLE && prop->value_type == VAL_TYPE_BYTE && prop->size == 1) { resizable = *(uint8_t*)prop->value; } else if (prop->property_id == PROP_ID_SCALE_FACTOR && prop->value_type == VAL_TYPE_PERCENTAGE && prop->size == 2) { uint16_t sf = krb_read_u16_le(prop->value); scale_factor = sf / 256.0f; } else if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; app_element->bg_color = (Color){c[0], c[1], c[2], c[3]}; } else if (prop->property_id == PROP_ID_ICON && prop->value_type == VAL_TYPE_RESOURCE && prop->size == 1) { /* Icon res index ignored for now */ } }
        }
        app_element->render_w = window_width; app_element->render_h = window_height; app_element->render_x = 0; app_element->render_y = 0;
        fprintf(debug_file, "INFO: Processed App. Window:%dx%d Title:'%s' Scale:%.2f\n", window_width, window_height, window_title ? window_title : "(None)", scale_factor);
    } else { fprintf(debug_file, "WARN: No App element. Using defaults.\n"); window_title = strdup("KRB Renderer (No App)"); }

    // --- Populate & Process Remaining RenderElements ---
     for (int i = 0; i < doc.header.element_count; i++) {
        if (app_element && i == 0) continue;
        RenderElement* current_render_el = &elements[i];
        current_render_el->header = doc.elements[i]; current_render_el->original_index = i; current_render_el->text = NULL;
        current_render_el->bg_color = default_bg; current_render_el->fg_color = default_fg; current_render_el->border_color = default_border;
        memset(current_render_el->border_widths, 0, 4); current_render_el->text_alignment = 0; current_render_el->parent = NULL;
        current_render_el->child_count = 0; current_render_el->texture_loaded = false;
        current_render_el->resource_index = INVALID_RESOURCE_INDEX;
        for(int k=0; k<MAX_ELEMENTS; ++k) current_render_el->children[k] = NULL;
        current_render_el->render_x = 0; current_render_el->render_y = 0; current_render_el->render_w = 0; current_render_el->render_h = 0;
        current_render_el->is_interactive = (current_render_el->header.type == ELEM_TYPE_BUTTON || current_render_el->header.type == ELEM_TYPE_INPUT);
        fprintf(debug_file, "INFO: Processing Elem %d (Type=0x%02X, StyleID=%d, Props=%d)\n", i, current_render_el->header.type, current_render_el->header.style_id, current_render_el->header.property_count);
        // Apply Style
        if (current_render_el->header.style_id > 0 && current_render_el->header.style_id <= doc.header.style_count && doc.styles) {
            int style_idx = current_render_el->header.style_id - 1; KrbStyle* style = &doc.styles[style_idx];
            fprintf(debug_file, "  Applying Style %d (Props=%d)\n", style_idx, style->property_count);
            for(int j=0; j<style->property_count; ++j) { KrbProperty* prop = &style->properties[j]; if (!prop || !prop->value) continue; if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->bg_color=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->fg_color=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->border_color=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if(prop->value_type == VAL_TYPE_BYTE && prop->size==1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size==4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); } else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; } }
        } else if (current_render_el->header.style_id > 0) { fprintf(debug_file, "WARN: Style ID %d invalid.\n", current_render_el->header.style_id); }
        // Apply Direct Properties
        fprintf(debug_file, "  Applying Direct Props (Count=%d)\n", current_render_el->header.property_count);
        if (doc.properties && i < doc.header.element_count && doc.properties[i]) {
             for (int j = 0; j < current_render_el->header.property_count; j++) { KrbProperty* prop = &doc.properties[i][j]; if (!prop || !prop->value) continue; if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->bg_color=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->fg_color=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->border_color=(Color){c[0],c[1],c[2],c[3]}; } else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if(prop->value_type == VAL_TYPE_BYTE && prop->size==1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size==4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); } else if (prop->property_id == PROP_ID_TEXT_CONTENT && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(current_render_el->text); current_render_el->text = strdup(doc.strings[idx]); } else { fprintf(debug_file, "WARN: Elem %d text index %d invalid.\n", i, idx); } } else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; fprintf(debug_file, "    Text Align set to: %d\n", current_render_el->text_alignment);} else if (prop->property_id == PROP_ID_IMAGE_SOURCE && prop->value_type == VAL_TYPE_RESOURCE && prop->size == 1) { current_render_el->resource_index = *(uint8_t*)prop->value; fprintf(debug_file, "    Image Source Res Idx: %d\n", current_render_el->resource_index); } else if (prop->property_id == PROP_ID_ICON && prop->value_type == VAL_TYPE_RESOURCE && prop->size == 1) { fprintf(debug_file, "    App Icon Res Idx: %d (Ignored)\n", *(uint8_t*)prop->value); } }
        }
        fprintf(debug_file, "  Finished Elem %d. Text='%s' Align=%d ResIdx=%d\n", i, current_render_el->text ? current_render_el->text : "NULL", current_render_el->text_alignment, current_render_el->resource_index);
    }

    // --- Build Parent/Child Tree ---
    fprintf(debug_file, "INFO: Building element tree...\n");
    RenderElement* parent_stack[MAX_ELEMENTS]; int stack_top = -1;
    for (int i = 0; i < doc.header.element_count; i++) {
        RenderElement* current_el = &elements[i];
        while (stack_top >= 0) { RenderElement* p = parent_stack[stack_top]; if (p->child_count >= p->header.child_count) stack_top--; else break; }
        if (stack_top >= 0) { RenderElement* p = parent_stack[stack_top]; current_el->parent = p; if (p->child_count < MAX_ELEMENTS) p->children[p->child_count++] = current_el; else fprintf(debug_file, "WARN: Max children parent %d.\n", p->original_index); }
        if (current_el->header.child_count > 0) { if (stack_top + 1 < MAX_ELEMENTS) parent_stack[++stack_top] = current_el; else fprintf(debug_file, "WARN: Max stack depth elem %d.\n", i); }
    }
    fprintf(debug_file, "INFO: Finished building element tree.\n");

    // --- Find Roots ---
    RenderElement* root_elements[MAX_ELEMENTS]; int root_count = 0;
    for(int i = 0; i < doc.header.element_count; ++i) { if (!elements[i].parent) { if (root_count < MAX_ELEMENTS) root_elements[root_count++] = &elements[i]; else { fprintf(debug_file, "WARN: Max roots.\n"); break; } } }
    if (root_count == 0 && doc.header.element_count > 0) { fprintf(stderr, "ERROR: No root found!\n"); fprintf(debug_file, "ERROR: No root!\n"); krb_free_document(&doc); free(elements); if(window_title) free(window_title); free(krb_file_path_copy); if(debug_file!=stderr) fclose(debug_file); return 1; }
    fprintf(debug_file, "INFO: Found %d root(s).\n", root_count);

    // --- Init Raylib Window ---
    fprintf(debug_file, "INFO: Init window %dx%d Title: '%s'\n", window_width, window_height, window_title ? window_title : "KRB Renderer");
    InitWindow(window_width, window_height, window_title ? window_title : "KRB Renderer");
    if (resizable) SetWindowState(FLAG_WINDOW_RESIZABLE);
    SetTargetFPS(60);

    // --- Load Textures ---
    fprintf(debug_file, "INFO: Loading textures...\n");
    for (int i = 0; i < doc.header.element_count; ++i) {
        RenderElement* el = &elements[i];
        if (el->header.type == ELEM_TYPE_IMAGE && el->resource_index != INVALID_RESOURCE_INDEX) {
            if (el->resource_index >= doc.header.resource_count || !doc.resources) { fprintf(stderr, "ERROR: Elem %d invalid res idx %d (max %d).\n", i, el->resource_index, doc.header.resource_count - 1); continue; }
            KrbResource* res = &doc.resources[el->resource_index];
            if (res->format == RES_FORMAT_EXTERNAL) {
                if (res->data_string_index >= doc.header.string_count || !doc.strings || !doc.strings[res->data_string_index]) { fprintf(stderr, "ERROR: Res %d invalid data str idx %d.\n", el->resource_index, res->data_string_index); continue; }
                const char* relative_path = doc.strings[res->data_string_index];

                // Construct the full path relative to the KRB file directory
                char full_path[MAX_LINE_LENGTH]; // Use defined limit
                // Check if krb_dir is "." (current directory) - avoid "./image.png" if possible
                if (strcmp(krb_dir, ".") == 0) {
                    snprintf(full_path, sizeof(full_path), "%s", relative_path);
                } else {
                    snprintf(full_path, sizeof(full_path), "%s/%s", krb_dir, relative_path);
                }
                full_path[sizeof(full_path) - 1] = '\0'; // Ensure null termination

                fprintf(debug_file, "  Loading texture Elem %d (Res %d): '%s' (Relative: '%s')\n", i, el->resource_index, full_path, relative_path);

                el->texture = LoadTexture(full_path);
                if (IsTextureReady(el->texture)) {
                    el->texture_loaded = true;
                    fprintf(debug_file, "    -> OK (ID: %u, %dx%d)\n", el->texture.id, el->texture.width, el->texture.height);
                } else {
                    fprintf(stderr, "ERROR: Failed load texture: %s\n", full_path);
                    fprintf(debug_file, "    -> FAILED: %s\n", full_path);
                    el->texture_loaded = false;
                }
            } else {
                fprintf(debug_file, "WARN: Inline res NI Elem %d (Res %d).\n", i, el->resource_index);
            }
        }
    }
    fprintf(debug_file, "INFO: Finished loading textures.\n");


    // --- Main Loop ---
    fprintf(debug_file, "INFO: Entering main loop...\n");
    while (!WindowShouldClose()) {
        Vector2 mousePos = GetMousePosition();
        if (resizable && IsWindowResized()) { window_width = GetScreenWidth(); window_height = GetScreenHeight(); if (app_element && app_element->parent == NULL) { app_element->render_w = window_width; app_element->render_h = window_height; } fprintf(debug_file, "INFO: Resized %dx%d.\n", window_width, window_height); }
        SetMouseCursor(MOUSE_CURSOR_DEFAULT);
        for (int i = doc.header.element_count - 1; i >= 0; --i) { RenderElement* el = &elements[i]; if (el->is_interactive && el->render_w > 0 && el->render_h > 0) { Rectangle r = { (float)el->render_x, (float)el->render_y, (float)el->render_w, (float)el->render_h }; if (CheckCollisionPointRec(mousePos, r)) { SetMouseCursor(MOUSE_CURSOR_POINTING_HAND); break; } } }

        BeginDrawing();
        Color clear_color = (app_element) ? app_element->bg_color : BLACK; ClearBackground(clear_color);
        fflush(debug_file);
        if (root_count > 0) {
            for (int i = 0; i < root_count; ++i) { if (root_elements[i]) render_element(root_elements[i], 0, 0, window_width, window_height, scale_factor, debug_file); else fprintf(debug_file, "WARN: Root %d NULL.\n", i); }
        } else { DrawText("No roots.", 10, 10, 20, RED); fprintf(debug_file, "--- FRAME SKIPPED (No roots) ---\n"); }
        EndDrawing();
    }

    // --- Cleanup ---
    fprintf(debug_file, "INFO: Closing & cleanup...\n"); CloseWindow();
    fprintf(debug_file, "INFO: Unloading textures...\n");
    for (int i = 0; i < doc.header.element_count; i++) { if (elements[i].texture_loaded) { fprintf(debug_file, "  Unload Elem %d TexID %u\n", i, elements[i].texture.id); UnloadTexture(elements[i].texture); } free(elements[i].text); }
    free(elements); free(window_title);
    krb_free_document(&doc);
    free(krb_file_path_copy); // Free the duplicated path
    if (debug_file != stderr) { fclose(debug_file); }
    printf("Standalone renderer finished.\n");
    return 0;
}

#endif // BUILD_STANDALONE_RENDERER