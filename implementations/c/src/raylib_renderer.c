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
void render_element(RenderElement* el, int parent_content_x, int parent_content_y, int parent_content_width, int parent_content_height, float scale_factor, FILE* debug_file) {
    if (!el) return;

    int intrinsic_w = (int)(el->header.width * scale_factor);
    int intrinsic_h = (int)(el->header.height * scale_factor);

    if (el->header.type == ELEM_TYPE_TEXT && el->text) {
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

    int final_x, final_y;
    int final_w = intrinsic_w;
    int final_h = intrinsic_h;
    bool has_pos = (el->header.pos_x != 0 || el->header.pos_y != 0);
    bool is_absolute = (el->header.layout & LAYOUT_ABSOLUTE_BIT);

    if (is_absolute || has_pos) {
        final_x = parent_content_x + (int)(el->header.pos_x * scale_factor);
        final_y = parent_content_y + (int)(el->header.pos_y * scale_factor);
    } else if (el->parent == NULL) {
        final_x = parent_content_x;
        final_y = parent_content_y;
    } else {
        final_x = el->render_x;
        final_y = el->render_y;
    }

    el->render_x = final_x;
    el->render_y = final_y;
    el->render_w = final_w;
    el->render_h = final_h;

    Color bg_color = el->bg_color;
    Color fg_color = el->fg_color;
    Color border_color = el->border_color;
    int top_bw = (int)(el->border_widths[0] * scale_factor);
    int right_bw = (int)(el->border_widths[1] * scale_factor);
    int bottom_bw = (int)(el->border_widths[2] * scale_factor);
    int left_bw = (int)(el->border_widths[3] * scale_factor);

    if (el->render_h > 0 && top_bw + bottom_bw >= el->render_h) { top_bw = el->render_h > 1 ? 1 : el->render_h; bottom_bw = 0; }
    if (el->render_w > 0 && left_bw + right_bw >= el->render_w) { left_bw = el->render_w > 1 ? 1 : el->render_w; right_bw = 0; }

    if (debug_file) {
        fprintf(debug_file, "DEBUG RENDER: Elem %d (Type=0x%02X) Initial Pos=(%d,%d) Size=%dx%d Borders=[%d,%d,%d,%d] Text='%s' Align=%d Layout=0x%02X Interact=%d\n",
                el->original_index, el->header.type, el->render_x, el->render_y, el->render_w, el->render_h,
                top_bw, right_bw, bottom_bw, left_bw,
                el->text ? el->text : "NULL", el->text_alignment, el->header.layout, el->is_interactive);
    }

    bool draw_background = (el->header.type != ELEM_TYPE_TEXT);
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

    int content_x = el->render_x + left_bw;
    int content_y = el->render_y + top_bw;
    int content_width = el->render_w - left_bw - right_bw;
    int content_height = el->render_h - top_bw - bottom_bw;
    if (content_width < 0) content_width = 0;
    if (content_height < 0) content_height = 0;

    if (el->text && el->text[0] != '\0' && content_width > 0 && content_height > 0) {
        int font_size = (int)(BASE_FONT_SIZE * scale_factor);
        if (font_size < 1) font_size = 1;
        int text_width_measured = MeasureText(el->text, font_size);
        int text_draw_x = content_x;
        if (el->text_alignment == 1) {
            text_draw_x = content_x + (content_width - text_width_measured) / 2;
        } else if (el->text_alignment == 2) {
            text_draw_x = content_x + content_width - text_width_measured;
        }
        int text_draw_y = content_y + (content_height - font_size) / 2;
        if (text_draw_x < content_x) text_draw_x = content_x;
        if (text_draw_y < content_y) text_draw_y = content_y;

        if (debug_file) {
            fprintf(debug_file, "  -> Drawing Text '%s' (align=%d -> center=%d) at (%d,%d) within content (%d,%d %dx%d)\n",
                    el->text, el->text_alignment, (el->text_alignment == 1), text_draw_x, text_draw_y,
                    content_x, content_y, content_width, content_height);
        }

        BeginScissorMode(content_x, content_y, content_width, content_height);
        DrawText(el->text, text_draw_x, text_draw_y, font_size, fg_color);
        EndScissorMode();
    }

    if (el->child_count > 0 && content_width > 0 && content_height > 0) {
        uint8_t direction = el->header.layout & LAYOUT_DIRECTION_MASK;
        uint8_t alignment = (el->header.layout & LAYOUT_ALIGNMENT_MASK) >> 2;
        int current_flow_x = content_x;
        int current_flow_y = content_y;
        int total_child_width_scaled = 0;
        int total_child_height_scaled = 0;
        int flow_child_count = 0;
        int child_sizes[MAX_ELEMENTS][2];

        if (debug_file) fprintf(debug_file, "  Layout Children of Elem %d: Count=%d Dir=%d Align=%d (LayoutByte=0x%02X) Content=(%d,%d %dx%d)\n", el->original_index, el->child_count, direction, alignment, el->header.layout, content_x, content_y, content_width, content_height);

        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i];
            if (!child) continue;
            bool child_is_absolute = (child->header.layout & LAYOUT_ABSOLUTE_BIT);
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);
            if (child_is_absolute || child_has_pos) {
                child_sizes[i][0] = 0; child_sizes[i][1] = 0;
                continue;
            }
            int child_w = (int)(child->header.width * scale_factor);
            int child_h = (int)(child->header.height * scale_factor);
            if (child->header.type == ELEM_TYPE_TEXT && child->text) {
                int fs = (int)(BASE_FONT_SIZE * scale_factor); if(fs<1)fs=1;
                int tw = (child->text[0]!='\0') ? MeasureText(child->text, fs):0;
                if (child->header.width == 0) child_w = tw + (int)(8 * scale_factor);
                if (child->header.height == 0) child_h = fs + (int)(8 * scale_factor);
            }
            if (child_w < 0) child_w = 0;
            if (child_h < 0) child_h = 0;
            if (child->header.width > 0 && child_w == 0) child_w = 1;
            if (child->header.height > 0 && child_h == 0) child_h = 1;
            child_sizes[i][0] = child_w;
            child_sizes[i][1] = child_h;
            if (direction == 0x00 || direction == 0x02) {
                total_child_width_scaled += child_w;
            } else {
                total_child_height_scaled += child_h;
            }
            flow_child_count++;
        }

        if (direction == 0x00 || direction == 0x02) {
            if (alignment == 0x01) { current_flow_x = content_x + (content_width - total_child_width_scaled) / 2; }
            else if (alignment == 0x02) { current_flow_x = content_x + content_width - total_child_width_scaled; }
            if (current_flow_x < content_x) current_flow_x = content_x;
        } else {
            if (alignment == 0x01) { current_flow_y = content_y + (content_height - total_child_height_scaled) / 2; }
            else if (alignment == 0x02) { current_flow_y = content_y + content_height - total_child_height_scaled; }
            if (current_flow_y < content_y) current_flow_y = content_y;
        }

        float space_between = 0;
        if (alignment == 0x03 && flow_child_count > 1) {
            if (direction == 0x00 || direction == 0x02) { space_between = (float)(content_width - total_child_width_scaled) / (flow_child_count - 1); }
            else { space_between = (float)(content_height - total_child_height_scaled) / (flow_child_count - 1); }
            if (space_between < 0) space_between = 0;
        }

        int flow_children_processed = 0;
        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i];
            if (!child) continue;
            int child_w = child_sizes[i][0];
            int child_h = child_sizes[i][1];
            int child_final_x, child_final_y;
            bool child_is_absolute = (child->header.layout & LAYOUT_ABSOLUTE_BIT);
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);

            if (child_is_absolute || child_has_pos) {
                render_element(child, content_x, content_y, content_width, content_height, scale_factor, debug_file);
            } else {
                if (direction == 0x00 || direction == 0x02) {
                    child_final_x = current_flow_x;
                    if (alignment == 0x01) child_final_y = content_y + (content_height - child_h) / 2;
                    else if (alignment == 0x02) child_final_y = content_y + content_height - child_h;
                    else child_final_y = content_y;
                } else {
                    child_final_y = current_flow_y;
                    if (alignment == 0x01) child_final_x = content_x + (content_width - child_w) / 2;
                    else if (alignment == 0x02) child_final_x = content_x + content_width - child_w;
                    else child_final_x = content_x;
                }
                child->render_x = child_final_x;
                child->render_y = child_final_y;
                child->render_w = child_w;
                child->render_h = child_h;
                render_element(child, content_x, content_y, content_width, content_height, scale_factor, debug_file);
                if (direction == 0x00 || direction == 0x02) {
                    current_flow_x += child_w;
                    if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) {
                        current_flow_x += (int)roundf(space_between);
                    }
                } else {
                    current_flow_y += child_h;
                    if (alignment == 0x03 && flow_children_processed < flow_child_count - 1) {
                        current_flow_y += (int)roundf(space_between);
                    }
                }
                flow_children_processed++;
            }
        }
    }
    if (debug_file) fprintf(debug_file, "  Finished Rendering Elem %d\n", el->original_index);
}

// --- Standalone Main Application Logic ---
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
        debug_file = stderr;
        fprintf(stderr, "Warning: Could not open debug log file.\n");
    }
    setvbuf(debug_file, NULL, _IOLBF, BUFSIZ);
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
    if (!krb_read_document(file, &doc)) {
        fprintf(stderr, "ERROR: Failed to parse KRB document '%s'\n", krb_file_path);
        fclose(file); krb_free_document(&doc);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    fclose(file);
    fprintf(debug_file, "INFO (Standalone): Parsed KRB OK - Elements=%u, Styles=%u, Strings=%u, Flags=0x%04X\n",
            doc.header.element_count, doc.header.style_count, doc.header.string_count, doc.header.flags);

    if (doc.header.element_count == 0) {
        fprintf(debug_file, "WARN (Standalone): No elements found. Exiting.\n");
        krb_free_document(&doc); if (debug_file != stderr) fclose(debug_file);
        return 0;
    }

    // --- Prepare Render Elements ---
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) {
        perror("ERROR (Standalone): Failed to allocate memory for render elements");
        krb_free_document(&doc); if (debug_file != stderr) fclose(debug_file);
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
        app_element->original_index = 0;
        app_element->text = NULL; app_element->parent = NULL; app_element->child_count = 0;
        for(int k=0; k<MAX_ELEMENTS; ++k) app_element->children[k] = NULL;
        app_element->is_interactive = false;
        fprintf(debug_file, "INFO (Standalone): Processing App Element (Index 0, StyleID=%d, Props=%d)\n", app_element->header.style_id, app_element->header.property_count);

        // Apply App Style as default baseline
        if (app_element->header.style_id > 0 && app_element->header.style_id <= doc.header.style_count) {
             int style_idx = app_element->header.style_id - 1;
             if (doc.styles && style_idx >= 0) {
                KrbStyle* app_style = &doc.styles[style_idx];
                fprintf(debug_file, "  Applying App Style %d\n", style_idx);
                for(int j=0; j<app_style->property_count; ++j) {
                    KrbProperty* prop = &app_style->properties[j]; if (!prop || !prop->value) continue;
                    if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_bg=(Color){c[0],c[1],c[2],c[3]}; }
                    else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_fg=(Color){c[0],c[1],c[2],c[3]};}
                    else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; default_border=(Color){c[0],c[1],c[2],c[3]}; }
                }
             } else { fprintf(debug_file, "WARN (Standalone): App Style ID %d is invalid.\n", app_element->header.style_id); }
        }
         app_element->bg_color = default_bg; app_element->fg_color = default_fg; app_element->border_color = default_border;
         memset(app_element->border_widths, 0, 4);

        // Apply App direct properties
        fprintf(debug_file, "  Applying App Direct Properties\n");
        if (doc.properties && doc.properties[0]) {
            for (int j = 0; j < app_element->header.property_count; j++) {
                KrbProperty* prop = &doc.properties[0][j]; if (!prop || !prop->value) continue;
                if (prop->property_id == PROP_ID_WINDOW_WIDTH && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { window_width = read_u16(prop->value); app_element->header.width = window_width; }
                else if (prop->property_id == PROP_ID_WINDOW_HEIGHT && prop->value_type == VAL_TYPE_SHORT && prop->size == 2) { window_height = read_u16(prop->value); app_element->header.height = window_height; }
                else if (prop->property_id == PROP_ID_WINDOW_TITLE && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(window_title); window_title = strdup(doc.strings[idx]); } }
                else if (prop->property_id == PROP_ID_RESIZABLE && prop->value_type == VAL_TYPE_BYTE && prop->size == 1) { resizable = *(uint8_t*)prop->value; }
                else if (prop->property_id == PROP_ID_SCALE_FACTOR && prop->value_type == VAL_TYPE_PERCENTAGE && prop->size == 2) { uint16_t sf = read_u16(prop->value); scale_factor = sf / 256.0f; }
                else if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c = (uint8_t*)prop->value; app_element->bg_color = (Color){c[0], c[1], c[2], c[3]}; }
            }
        }
        app_element->render_w = window_width; app_element->render_h = window_height;
        app_element->render_x = 0; app_element->render_y = 0;
        fprintf(debug_file, "INFO (Standalone): Processed App Element props. Window: %dx%d, Title: '%s'\n", window_width, window_height, window_title ? window_title : "(None)");
    } else {
        fprintf(debug_file, "WARN (Standalone): No App element found or KRB lacks App flag. Using default window settings.\n");
        window_title = strdup("KRB Standalone Renderer");
    }

    // --- Populate & Process Remaining RenderElements ---
     for (int i = 0; i < doc.header.element_count; i++) {
        if (app_element && i == 0) continue; // Skip App

        RenderElement* current_render_el = &elements[i];
        current_render_el->header = doc.elements[i];
        current_render_el->original_index = i;
        current_render_el->text = NULL; current_render_el->bg_color = default_bg; current_render_el->fg_color = default_fg;
        current_render_el->border_color = default_border; memset(current_render_el->border_widths, 0, 4);
        current_render_el->text_alignment = 0; current_render_el->parent = NULL; current_render_el->child_count = 0;
        for(int k=0; k<MAX_ELEMENTS; ++k) current_render_el->children[k] = NULL;
        current_render_el->render_x = 0; current_render_el->render_y = 0; current_render_el->render_w = 0; current_render_el->render_h = 0;
        current_render_el->is_interactive = (current_render_el->header.type == ELEM_TYPE_BUTTON);

        fprintf(debug_file, "INFO (Standalone): Processing Element %d (Type=0x%02X, StyleID=%d, Props=%d)\n", i, current_render_el->header.type, current_render_el->header.style_id, current_render_el->header.property_count);

        // Apply Style FIRST
        if (current_render_el->header.style_id > 0 && current_render_el->header.style_id <= doc.header.style_count) {
            int style_idx = current_render_el->header.style_id - 1;
             if (doc.styles && style_idx >= 0) {
                 KrbStyle* style = &doc.styles[style_idx];
                 fprintf(debug_file, "  Applying Style %d (Props=%d)\n", style_idx, style->property_count);
                 for(int j=0; j<style->property_count; ++j) {
                     KrbProperty* prop = &style->properties[j]; if (!prop || !prop->value) continue;
                     if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->bg_color=(Color){c[0],c[1],c[2],c[3]}; }
                     else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->fg_color=(Color){c[0],c[1],c[2],c[3]}; }
                     else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->border_color=(Color){c[0],c[1],c[2],c[3]}; }
                     else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if(prop->value_type == VAL_TYPE_BYTE && prop->size==1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size==4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); }
                     else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                 }
             } else { fprintf(debug_file, "WARN (Standalone): Style ID %d for Element %d is invalid.\n", current_render_el->header.style_id, i); }
        }

        // Apply Direct Properties SECOND
         fprintf(debug_file, "  Applying Direct Properties (Count=%d)\n", current_render_el->header.property_count);
        if (doc.properties && i < doc.header.element_count && doc.properties[i]) {
             for (int j = 0; j < current_render_el->header.property_count; j++) {
                 KrbProperty* prop = &doc.properties[i][j]; if (!prop || !prop->value) continue;
                 if (prop->property_id == PROP_ID_BG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->bg_color=(Color){c[0],c[1],c[2],c[3]}; }
                 else if (prop->property_id == PROP_ID_FG_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->fg_color=(Color){c[0],c[1],c[2],c[3]}; }
                 else if (prop->property_id == PROP_ID_BORDER_COLOR && prop->value_type == VAL_TYPE_COLOR && prop->size == 4) { uint8_t* c=(uint8_t*)prop->value; current_render_el->border_color=(Color){c[0],c[1],c[2],c[3]}; }
                 else if (prop->property_id == PROP_ID_BORDER_WIDTH) { if(prop->value_type == VAL_TYPE_BYTE && prop->size==1 && prop->value) memset(current_render_el->border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == VAL_TYPE_EDGEINSETS && prop->size==4 && prop->value) memcpy(current_render_el->border_widths, prop->value, 4); }
                 else if (prop->property_id == PROP_ID_TEXT_CONTENT && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(current_render_el->text); current_render_el->text = strdup(doc.strings[idx]); } else { fprintf(debug_file, "WARN (Standalone): Element %d text string index %d invalid.\n", i, idx); } }
                 else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; fprintf(debug_file, "      Text Align: %d\n", current_render_el->text_alignment);}
            }
        }
         fprintf(debug_file, "  Finished Processing Element %d. LayoutByte=0x%02X Text='%s' Align=%d\n", i, current_render_el->header.layout, current_render_el->text ? current_render_el->text : "NULL", current_render_el->text_alignment);
    } // End loop processing elements

    // --- Build Parent/Child Tree ---
    fprintf(debug_file, "INFO (Standalone): Building element tree...\n");
    RenderElement* parent_stack[MAX_ELEMENTS]; int stack_top = -1;
    for (int i = 0; i < doc.header.element_count; i++) {
        RenderElement* current_el = &elements[i];
        while (stack_top >= 0) {
            RenderElement* potential_parent = parent_stack[stack_top];
            if (potential_parent->child_count >= potential_parent->header.child_count) {
                stack_top--;
            } else { break; }
        }
        if (stack_top >= 0) {
            RenderElement* current_parent = parent_stack[stack_top];
            current_el->parent = current_parent;
            if (current_parent->child_count < MAX_ELEMENTS) {
                 current_parent->children[current_parent->child_count++] = current_el;
            } else { fprintf(debug_file, "WARN (Standalone): Exceeded MAX_ELEMENTS children for parent %d.\n", current_parent->original_index); }
        }
        if (current_el->header.child_count > 0) {
            if (stack_top + 1 < MAX_ELEMENTS) {
                parent_stack[++stack_top] = current_el;
            } else { fprintf(debug_file, "WARN (Standalone): Exceeded MAX_ELEMENTS for parent stack depth at element %d.\n", i); }
        }
    }
    fprintf(debug_file, "INFO (Standalone): Finished building element tree.\n");

    // --- Find Roots ---
    RenderElement* root_elements[MAX_ELEMENTS]; int root_count = 0;
    for(int i = 0; i < doc.header.element_count; ++i) {
        if (!elements[i].parent) {
             fprintf(debug_file, "  Found Root Element: Index %d (Type=0x%02X)\n", i, elements[i].header.type);
            if (root_count < MAX_ELEMENTS) { root_elements[root_count++] = &elements[i]; }
            else { fprintf(debug_file, "WARN (Standalone): Exceeded MAX_ELEMENTS for root elements.\n"); break; }
        }
    }
    if (root_count == 0 && doc.header.element_count > 0) {
        fprintf(stderr, "ERROR (Standalone): No root element found in KRB, but elements exist.\n");
        fprintf(debug_file, "ERROR (Standalone): No root element found in KRB, but elements exist.\n");
        krb_free_document(&doc); free(elements); if(window_title) free(window_title); if(debug_file!=stderr) fclose(debug_file);
        return 1;
    }
    fprintf(debug_file, "INFO (Standalone): Found %d root element(s).\n", root_count);

    // --- Init Raylib Window ---
    fprintf(debug_file, "INFO (Standalone): Initializing window %dx%d Title: '%s'\n", window_width, window_height, window_title ? window_title : "KRB Renderer");
    InitWindow(window_width, window_height, window_title ? window_title : "KRB Standalone Renderer");
    if (resizable) SetWindowState(FLAG_WINDOW_RESIZABLE);
    SetTargetFPS(60);

    // --- Main Loop ---
    fprintf(debug_file, "INFO (Standalone): Entering main loop...\n");
    while (!WindowShouldClose()) {
        Vector2 mousePos = GetMousePosition();

        if (resizable && IsWindowResized()) {
            window_width = GetScreenWidth(); window_height = GetScreenHeight();
            if (app_element && app_element->parent == NULL) { // Update App root size if it exists and is a root
                app_element->render_w = window_width;
                app_element->render_h = window_height;
            }
            fprintf(debug_file, "INFO (Standalone): Window resized to %dx%d.\n", window_width, window_height);
        }

        // Hover Check for Cursor
        SetMouseCursor(MOUSE_CURSOR_DEFAULT);
        for (int i = doc.header.element_count - 1; i >= 0; --i) {
             RenderElement* el = &elements[i];
             if (el->is_interactive && el->render_w > 0 && el->render_h > 0) {
                Rectangle elementRect = { (float)el->render_x, (float)el->render_y, (float)el->render_w, (float)el->render_h };
                if (CheckCollisionPointRec(mousePos, elementRect)) {
                    SetMouseCursor(MOUSE_CURSOR_POINTING_HAND);
                    break;
                }
            }
        }

        // --- Drawing ---
        BeginDrawing();
        Color clear_color = (app_element) ? app_element->bg_color : ((root_count > 0) ? root_elements[0]->bg_color : BLACK);
        ClearBackground(clear_color);

        fflush(debug_file); // Flush log before render loop

        if (root_count > 0) {
            fprintf(debug_file, "--- BEGIN FRAME RENDER (Roots=%d) ---\n", root_count);
            for (int i = 0; i < root_count; ++i) {
                if (root_elements[i]) {
                     fprintf(debug_file, "Rendering Root %d (Elem %d)\n", i, root_elements[i]->original_index);
                     // Pass window dimensions as the initial parent content area for roots
                     render_element(root_elements[i], 0, 0, window_width, window_height, scale_factor, debug_file);
                } else { fprintf(debug_file, "WARN: Root element %d is NULL.\n", i); }
            }
             fprintf(debug_file, "--- END FRAME RENDER ---\n");
        } else {
             DrawText("No KRB root elements found to render.", 10, 10, 20, RED);
             fprintf(debug_file, "--- FRAME RENDER SKIPPED (No roots) ---\n");
        }

        // --- REMOVED FPS COUNTER ---
        // DrawFPS(10, 10);

        EndDrawing();
    } // End main loop

    // --- Cleanup ---
    fprintf(debug_file, "INFO (Standalone): Closing window and cleaning up...\n");
    CloseWindow();
    for (int i = 0; i < doc.header.element_count; i++) { free(elements[i].text); }
    free(elements); free(window_title);
    krb_free_document(&doc);
    if (debug_file != stderr) { fclose(debug_file); }
    printf("Standalone renderer finished.\n");
    return 0;
}

#endif // BUILD_STANDALONE_RENDERER