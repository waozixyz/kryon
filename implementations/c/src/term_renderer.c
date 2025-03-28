#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <errno.h>
#include <math.h> // Include math.h for round, fmin
#include <termbox.h>
#include "krb.h" // Assume this defines Krb* structs and PROP_ID_*

#define MAX_ELEMENTS 256 // Local definition if not in krb.h

// --- KRB Property ID Defines (Ensure these match your krb.h/spec) ---
#ifndef PROP_ID_BG_COLOR
#define PROP_ID_BG_COLOR 0x01
#define PROP_ID_FG_COLOR 0x02
#define PROP_ID_BORDER_COLOR 0x03
#define PROP_ID_BORDER_WIDTH 0x04
#define PROP_ID_TEXT_CONTENT 0x08
#define PROP_ID_TEXT_ALIGNMENT 0x0B
// App Specific
#define PROP_ID_WINDOW_WIDTH 0x20
#define PROP_ID_WINDOW_HEIGHT 0x21
#define PROP_ID_WINDOW_TITLE 0x22
#define PROP_ID_RESIZABLE 0x23
#define PROP_ID_KEEP_ASPECT 0x24
// ... other IDs if needed
#endif
// --- End Property ID defines ---


// Structure specifically for rendering, holding resolved values
typedef struct RenderElement {
    KrbElementHeader header;
    char* text;
    uint32_t bg_color;      // Store raw uint32_t from KRB (Style/Prop), 0 means unset/inherit
    uint32_t fg_color;      // Store raw uint32_t from KRB (Style/Prop), 0 means unset/inherit
    uint32_t border_color;  // Store raw uint32_t from KRB (Style/Prop), 0 means unset/inherit
    uint8_t border_widths[4]; // T, R, B, L
    uint8_t text_alignment; // 0=left, 1=center, 2=right
    struct RenderElement* parent;
    struct RenderElement* children[MAX_ELEMENTS];
    int child_count;
    // --- Add App-specific properties read during processing ---
    uint16_t app_design_width;
    uint16_t app_design_height;
    bool app_resizable;
    bool app_keep_aspect;
    // --- End App-specific ---
} RenderElement;

// --- Helper to get uint16 property value (assumes Little Endian KRB storage) ---
uint16_t get_property_u16(KrbProperty* props, uint8_t count, uint8_t prop_id, uint16_t default_val) {
    for (uint8_t i = 0; i < count; ++i) {
        if (props[i].property_id == prop_id && props[i].value_type == 0x02 && props[i].size == 2 && props[i].value) {
            uint8_t* bytes = (uint8_t*)props[i].value;
            return (uint16_t)(bytes[0] | (bytes[1] << 8)); // KRB uses LE
        }
    }
    return default_val;
}

// --- Helper to get uint32 property value ---
// Reads the uint32 color value directly from the KRB data.
// Assumes KRB stores the #RRGGBBAA value directly as a 32-bit integer (e.g., Big Endian if R is MSB).
// If KRB stores it Little Endian (AABBGGRR), adjust the reading logic.
// Let's ASSUME KRB stores it Big Endian (R is MSB) as per common hex notation.
uint32_t get_property_u32_color(KrbProperty* prop) {
    if (prop && prop->value_type == 0x03 && prop->size == 4 && prop->value) {
        uint8_t* bytes = (uint8_t*)prop->value;
        // Assuming KRB stores color bytes directly in R,G,B,A order
        return ((uint32_t)bytes[0] << 24) | // R
               ((uint32_t)bytes[1] << 16) | // G
               ((uint32_t)bytes[2] << 8)  | // B
               ((uint32_t)bytes[3]);        // A
    }
    return 0; // Return 0 if property is invalid or not a color
}


// --- Helper to get uint8/bool property value ---
bool get_property_bool(KrbProperty* props, uint8_t count, uint8_t prop_id, bool default_val) {
     for (uint8_t i = 0; i < count; ++i) {
        if (props[i].property_id == prop_id && props[i].value_type == 0x01 && props[i].size == 1 && props[i].value) {
            return (*(uint8_t*)props[i].value) != 0;
        }
    }
    return default_val;
}

// Convert KRB RGBA (passed as uint32_t with R in MSB) to Termbox color
int rgb_to_tb_color(uint32_t rgba, FILE* debug_file) {
    // Extract components assuming R is MSB: RRGGBBAA
    // Example: #191970FF -> 0x191970FF -> R=25, G=25, B=112, A=255
    // Example: #00FFFFFF -> 0x00FFFFFF -> R=0, G=255, B=255, A=255
    uint8_t r = (rgba >> 24) & 0xFF;
    uint8_t g = (rgba >> 16) & 0xFF;
    uint8_t b = (rgba >> 8) & 0xFF;
    uint8_t a = rgba & 0xFF;

    // Add more verbose logging
    fprintf(debug_file, "DEBUG CONVERT: Input RGBA=0x%08X (R=%d, G=%d, B=%d, A=%d) -> ", rgba, r, g, b, a);

    if (a < 128) {
         fprintf(debug_file, "TB_DEFAULT (Alpha < 128)\n");
         return TB_DEFAULT;
    }

    // --- Direct Match for Example Colors (More Robust) ---
    // appstyle background: #191919FF -> R=25 G=25 B=25
    if (r < 60 && g < 60 && b < 60) { fprintf(debug_file, "TB_BLACK\n"); return TB_BLACK; }
    // appstyle text: #FFFF00FF -> R=255 G=255 B=0
    if (r > 200 && g > 200 && b < 50) { fprintf(debug_file, "TB_YELLOW | TB_BOLD\n"); return TB_YELLOW | TB_BOLD; }
    // containerstyle background: #191970FF -> R=25 G=25 B=112
    if (r < 50 && g < 50 && b > 90 && b < 140) { fprintf(debug_file, "TB_BLUE\n"); return TB_BLUE; } // Match specific blue range
    // containerstyle border: #00FFFFFF -> R=0 G=255 B=255
    if (r < 50 && g > 200 && b > 200) { fprintf(debug_file, "TB_CYAN | TB_BOLD\n"); return TB_CYAN | TB_BOLD; }
    // --- End Direct Match ---

    // General mapping (copied from previous good version)
    if (r > 200 && g > 200 && b > 200) { fprintf(debug_file, "TB_WHITE\n"); return TB_WHITE; }
    if (r > 200 && g < 100 && b < 100) { fprintf(debug_file, "TB_RED | TB_BOLD\n"); return TB_RED | TB_BOLD; }
    if (r < 100 && g > 200 && b < 100) { fprintf(debug_file, "TB_GREEN | TB_BOLD\n"); return TB_GREEN | TB_BOLD; }
    if (r < 100 && g < 100 && b > 200) { fprintf(debug_file, "TB_BLUE | TB_BOLD\n"); return TB_BLUE | TB_BOLD; }
    if (r > 150 && g < 100 && b > 150) { fprintf(debug_file, "TB_MAGENTA | TB_BOLD\n"); return TB_MAGENTA | TB_BOLD; }
    if (r < 100 && g > 150 && b > 150) { fprintf(debug_file, "TB_CYAN | TB_BOLD\n"); return TB_CYAN | TB_BOLD; }
    if (r > 100 && g > 100 && b > 100) { fprintf(debug_file, "TB_WHITE (normal)\n"); return TB_WHITE; }
    if (r > 120 && g < 70 && b < 70) { fprintf(debug_file, "TB_RED\n"); return TB_RED; }
    if (r < 70 && g > 120 && b < 70) { fprintf(debug_file, "TB_GREEN\n"); return TB_GREEN; }
    if (r > 120 && g > 120 && b < 70) { fprintf(debug_file, "TB_YELLOW\n"); return TB_YELLOW; }
    if (r < 70 && g < 70 && b > 120) { fprintf(debug_file, "TB_BLUE\n"); return TB_BLUE; } // Already caught brighter blue
    if (r > 100 && g < 70 && b > 100) { fprintf(debug_file, "TB_MAGENTA\n"); return TB_MAGENTA; }
    if (r < 70 && g > 100 && b > 100) { fprintf(debug_file, "TB_CYAN\n"); return TB_CYAN; }

    // Fallback if no suitable match
    fprintf(debug_file, "TB_DEFAULT (Fallback)\n");
    return TB_DEFAULT;
}

// Strips surrounding quotes and allocates a new string. Caller must free.
char* strip_quotes(const char* input) {
    // ... (implementation unchanged) ...
    if (!input) return NULL; size_t len = strlen(input);
    if (len >= 2 && input[0] == '"' && input[len - 1] == '"') {
        char* stripped = malloc(len - 1); if (!stripped) { perror("strip_quotes malloc"); return NULL; }
        strncpy(stripped, input + 1, len - 2); stripped[len - 2] = '\0'; return stripped;
    }
    char* dup = strdup(input); if (!dup) { perror("strip_quotes strdup"); } return dup;
}


// --- Rendering Function (Termbox - WITH SCALING) ---
void render_element(RenderElement* el,
                    int parent_content_x, int parent_content_y,
                    int parent_content_width, int parent_content_height,
                    double scale_x, double scale_y, // Scaling factors
                    int offset_x, int offset_y,     // Offsets for centering (if keep_aspect)
                    uint32_t default_bg_color_u32, // Default BG from app/parent cascade
                    uint32_t default_fg_color_u32, // Default FG from app/parent cascade
                    FILE* debug_file)
{
    if (!el) return;

    // --- Special Handling for App Element (Type 0x00) ---
    if (el->header.type == 0x00) {
        // App element itself doesn't get rendered as a box, it defines the window/defaults.
        // We just use its resolved colors to pass down as defaults to children.
        uint32_t app_bg = (el->bg_color != 0) ? el->bg_color : default_bg_color_u32;
        uint32_t app_fg = (el->fg_color != 0) ? el->fg_color : default_fg_color_u32;
        fprintf(debug_file, "DEBUG RENDER: Processing App Element %p. Effective Defaults: BG=0x%08X, FG=0x%08X. Passing parent area (%d,%d %dx%d) scale/offset to children.\n",
                (void*)el, app_bg, app_fg, parent_content_x, parent_content_y, parent_content_width, parent_content_height);

        // Layout and Render children (logic unchanged)
        if (el->child_count > 0 && parent_content_width > 0 && parent_content_height > 0) {
            // ... (Calculate scaled sizes, layout origin, alignment, spacing) ...
            uint8_t direction = el->header.layout & LAYOUT_DIRECTION_MASK; uint8_t alignment = (el->header.layout & LAYOUT_ALIGNMENT_MASK) >> 2;
            int total_child_width = 0, total_child_height = 0, flow_child_count = 0; int child_scaled_sizes[MAX_ELEMENTS][2];
            for (int i = 0; i < el->child_count; i++) { /* ... calc scaled_w/h ... */
                 RenderElement* child = el->children[i]; if (!child) continue; int scaled_w = (int)round(child->header.width*scale_x); int scaled_h=(int)round(child->header.height*scale_y);
                 if(child->header.type==0x02&&child->text){if(child->header.width==0)scaled_w=strlen(child->text)+2; if(child->header.height==0)scaled_h=1;} if(child->header.type==0x01&&child->header.width==0)scaled_w=3;if(child->header.type==0x01&&child->header.height==0)scaled_h=3;if(child->header.width>0&&scaled_w<=0)scaled_w=1;if(child->header.height>0&&scaled_h<=0)scaled_h=1;if(scaled_w<0)scaled_w=0;if(scaled_h<0)scaled_h=0; child_scaled_sizes[i][0]=scaled_w; child_scaled_sizes[i][1]=scaled_h; bool child_has_pos=(child->header.pos_x!=0||child->header.pos_y!=0); if(!child_has_pos){if(direction==0||direction==2)total_child_width+=scaled_w; else total_child_height+=scaled_h; flow_child_count++;}
            }
            int current_x = parent_content_x + offset_x, current_y = parent_content_y + offset_y; int available_flow_width = parent_content_width - offset_x*2; int available_flow_height = parent_content_height - offset_y*2;
            if(direction==0||direction==2){ if(alignment==1)current_x=parent_content_x+offset_x+(available_flow_width-total_child_width)/2; else if(alignment==2)current_x=parent_content_x+parent_content_width-offset_x-total_child_width; if(current_x<parent_content_x+offset_x)current_x=parent_content_x+offset_x;} else { if(alignment==1)current_y=parent_content_y+offset_y+(available_flow_height-total_child_height)/2; else if(alignment==2)current_y=parent_content_y+parent_content_height-offset_y-total_child_height; if(current_y<parent_content_y+offset_y)current_y=parent_content_y+offset_y;}
            int space_between=0; if(alignment==3&&flow_child_count>1){if(direction==0||direction==2)space_between=(available_flow_width-total_child_width)/(flow_child_count-1);else space_between=(available_flow_height-total_child_height)/(flow_child_count-1);if(space_between<0)space_between=0;}

            int flow_children_processed = 0;
            for (int i = 0; i < el->child_count; i++) {
                RenderElement* child = el->children[i]; if (!child) continue;
                int child_w = child_scaled_sizes[i][0], child_h = child_scaled_sizes[i][1];
                int child_render_origin_x, child_render_origin_y;
                bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);
                 if(child_has_pos){ child_render_origin_x=parent_content_x+offset_x; child_render_origin_y=parent_content_y+offset_y;} else {child_render_origin_x=current_x; child_render_origin_y=current_y; if(direction==0||direction==2){if(alignment==1)child_render_origin_y=parent_content_y+offset_y+(available_flow_height-child_h)/2;else if(alignment==2)child_render_origin_y=parent_content_y+parent_content_height-offset_y-child_h; else child_render_origin_y=parent_content_y+offset_y;} else {if(alignment==1)child_render_origin_x=parent_content_x+offset_x+(available_flow_width-child_w)/2; else if(alignment==2)child_render_origin_x=parent_content_x+parent_content_width-offset_x-child_w; else child_render_origin_x=parent_content_x+offset_x;} if(child_render_origin_x<parent_content_x+offset_x)child_render_origin_x=parent_content_x+offset_x; if(child_render_origin_y<parent_content_y+offset_y)child_render_origin_y=parent_content_y+offset_y; if(direction==0||direction==2){current_x+=child_w; if(alignment==3&&flow_children_processed<flow_child_count-1)current_x+=space_between;}else{current_y+=child_h; if(alignment==3&&flow_children_processed<flow_child_count-1)current_y+=space_between;} flow_children_processed++;}

                // Pass App's resolved colors as the defaults for the child
                render_element(child, child_render_origin_x, child_render_origin_y, parent_content_width, parent_content_height, scale_x, scale_y, offset_x, offset_y, app_bg, app_fg, debug_file);
            }
        }
        return; // Done with App element
    }
    // --- End Special Handling for App Element ---


    // --- Processing for non-App elements ---

    // --- 1. Calculate Scaled Size --- (Unchanged)
    int scaled_w = (int)round(el->header.width*scale_x); int scaled_h=(int)round(el->header.height*scale_y);
    if(el->header.type==0x02&&el->text){int len=strlen(el->text); if(el->header.width==0)scaled_w=len+2; if(el->header.height==0)scaled_h=1;} if(el->header.type==0x01&&el->header.width==0)scaled_w=3;if(el->header.type==0x01&&el->header.height==0)scaled_h=3;if(el->header.width>0&&scaled_w<=0)scaled_w=1;if(el->header.height>0&&scaled_h<=0)scaled_h=1;if(scaled_w<0)scaled_w=0;if(scaled_h<0)scaled_h=0;

    // --- 2. Determine Final Position and Size --- (Unchanged)
    int final_x, final_y; int final_w = scaled_w; int final_h = scaled_h; bool has_pos=(el->header.pos_x!=0||el->header.pos_y!=0);
    if(has_pos){int scaled_pos_x=(int)round(el->header.pos_x*scale_x); int scaled_pos_y=(int)round(el->header.pos_y*scale_y); final_x=parent_content_x+scaled_pos_x; final_y=parent_content_y+scaled_pos_y;} else {final_x=parent_content_x; final_y=parent_content_y;}

    // --- 3. Clipping --- (Unchanged)
    int term_w = tb_width(); int term_h = tb_height();
    if(final_x>=term_w||final_y>=term_h){fprintf(debug_file,"WARN RENDER: Skipping elem %p starting outside bounds (%d,%d >= %d,%d)\n", (void*)el, final_x, final_y, term_w, term_h); return;} if(final_x<0){final_w+=final_x;final_x=0;} if(final_y<0){final_h+=final_y;final_y=0;} if(final_x+final_w>term_w)final_w=term_w-final_x; if(final_y+final_h>term_h)final_h=term_h-final_y; if(final_w<=0||final_h<=0){fprintf(debug_file,"WARN RENDER: Skipping elem %p with zero clipped size (%dx%d) at (%d,%d)\n", (void*)el, final_w, final_h, final_x, final_y); return;}

    // --- 4. Resolve Colors for this Element ---
    uint32_t use_bg = (el->bg_color != 0) ? el->bg_color : default_bg_color_u32;
    uint32_t use_fg = (el->fg_color != 0) ? el->fg_color : default_fg_color_u32;
    uint32_t use_border = el->border_color; // Check if border color was set
    if (use_border == 0) {
        // If not set, use a fallback (e.g., same as foreground, or a default gray)
        // Let's use the resolved foreground color if border isn't set explicitly
        // use_border = use_fg;
        // Or use a hardcoded gray:
        use_border = 0x808080FF; // Gray
    }

    // Convert resolved uint32 colors to Termbox attributes
    int tb_bg = rgb_to_tb_color(use_bg, debug_file);
    int tb_fg = rgb_to_tb_color(use_fg, debug_file);
    int tb_border = rgb_to_tb_color(use_border, debug_file);

    // Border widths (Unchanged)
    int top_bw = (el->border_widths[0] > 0) ? 1 : 0; int right_bw = (el->border_widths[1] > 0) ? 1 : 0; int bottom_bw = (el->border_widths[2] > 0) ? 1 : 0; int left_bw = (el->border_widths[3] > 0) ? 1 : 0;
    if(top_bw+bottom_bw>=final_h){top_bw=(final_h>0);bottom_bw=0;} if(left_bw+right_bw>=final_w){left_bw=(final_w>0);right_bw=0;}

    // --- 5. Debug Logging ---
    fprintf(debug_file, "DEBUG RENDER: Elem %p: Type=0x%02X @(%d,%d) FinalSize=%dx%d Borders=[%d,%d,%d,%d] Colors=(BG:0x%08X->%d, FG:0x%08X->%d, BRDR:0x%08X->%d)\n",
            (void*)el, el->header.type, final_x, final_y, final_w, final_h, top_bw, right_bw, bottom_bw, left_bw,
            use_bg, tb_bg, use_fg, tb_fg, use_border, tb_border);

    // --- 6. Drawing Background & Borders ---
    for (int j = 0; j < final_h; ++j) {
        for (int i = 0; i < final_w; ++i) {
            bool is_border = false;
            int border_char = ' ';
            int char_fg = tb_fg; // Default to element's foreground
            int char_bg = tb_bg; // Default to element's background

            // Check if current cell is part of the border
            if (top_bw > 0 && j < top_bw) is_border = true;
            else if (bottom_bw > 0 && j >= final_h - bottom_bw) is_border = true;
            else if (left_bw > 0 && i < left_bw) is_border = true;
            else if (right_bw > 0 && i >= final_w - right_bw) is_border = true;

            if (is_border) {
                // Use border color for FG, keep element BG for border cell BG
                char_fg = tb_border;
                // Simple border chars
                if ((j < top_bw || j >= final_h - bottom_bw) && (i<left_bw || i>=final_w-right_bw)) border_char = '+'; // Corners
                else if (j < top_bw || j >= final_h - bottom_bw) border_char = '-'; // Top/Bottom edge
                else if (i < left_bw || i >= final_w - right_bw) border_char = '|'; // Left/Right edge
                else border_char = '?'; // Should not happen?
            } else {
                // Not a border cell, fill with background
                 border_char = ' ';
                 // Use element's default fg/bg (already set)
            }
             // Draw the cell
             tb_change_cell(final_x + i, final_y + j, border_char, char_fg, char_bg);
        }
    }


    // --- 7. Calculate Content Area --- (Unchanged)
    int content_x=final_x+left_bw; int content_y=final_y+top_bw; int content_width=final_w-left_bw-right_bw; int content_height=final_h-top_bw-bottom_bw; if(content_width<0)content_width=0; if(content_height<0)content_height=0;

    // --- 8. Draw Content (Text) --- (Unchanged logic, uses resolved tb_fg, tb_bg)
    if(el->text && el->text[0] != '\0' && content_width > 0 && content_height > 0){
        int text_len=strlen(el->text); int text_draw_x=content_x; if(el->text_alignment==1)text_draw_x=content_x+(content_width-text_len)/2;else if(el->text_alignment==2)text_draw_x=content_x+content_width-text_len; if(text_draw_x<content_x)text_draw_x=content_x;if(text_draw_x>=content_x+content_width&&text_len>0)text_draw_x=content_x+content_width-1; int text_draw_y=content_y+(content_height-1)/2; if(text_draw_y<content_y)text_draw_y=content_y;if(text_draw_y>=content_y+content_height)text_draw_y=content_y+content_height-1;
        for(int i=0;i<text_len;++i){
            int char_x=text_draw_x+i;
            if(char_x>=content_x&&char_x<content_x+content_width){
                 if(text_draw_y>=content_y&&text_draw_y<content_y+content_height){
                    tb_change_cell(char_x,text_draw_y,el->text[i],tb_fg,tb_bg); // Uses resolved fg/bg
                 }
            } else if (char_x >= content_x + content_width) { break; }
        }
    }


    // --- 9. Layout and Render Children --- (Pass resolved colors down)
    if (el->child_count > 0 && content_width > 0 && content_height > 0) {
        // ... (Calculate scaled sizes, layout origin, alignment, spacing - unchanged) ...
        uint8_t direction=el->header.layout&LAYOUT_DIRECTION_MASK; uint8_t alignment=(el->header.layout&LAYOUT_ALIGNMENT_MASK)>>2;
        int total_child_width=0,total_child_height=0,flow_child_count=0; int child_scaled_sizes[MAX_ELEMENTS][2];
        for(int i=0;i<el->child_count;i++){/* ... calc scaled_cw/ch ... */
            RenderElement* child=el->children[i]; if(!child)continue; int scaled_cw=(int)round(child->header.width*scale_x); int scaled_ch=(int)round(child->header.height*scale_y); if(child->header.type==0x02&&child->text){if(child->header.width==0)scaled_cw=strlen(child->text)+2;if(child->header.height==0)scaled_ch=1;}if(child->header.type==0x01&&child->header.width==0)scaled_cw=3;if(child->header.type==0x01&&child->header.height==0)scaled_ch=3;if(child->header.width>0&&scaled_cw<=0)scaled_cw=1;if(child->header.height>0&&scaled_ch<=0)scaled_ch=1;if(scaled_cw<0)scaled_cw=0;if(scaled_ch<0)scaled_ch=0; child_scaled_sizes[i][0]=scaled_cw;child_scaled_sizes[i][1]=scaled_ch; bool child_has_pos=(child->header.pos_x!=0||child->header.pos_y!=0); if(!child_has_pos){if(direction==0||direction==2)total_child_width+=scaled_cw;else total_child_height+=scaled_ch; flow_child_count++;}}
        int current_x=content_x;int current_y=content_y; if(direction==0||direction==2){if(alignment==1)current_x=content_x+(content_width-total_child_width)/2;else if(alignment==2)current_x=content_x+content_width-total_child_width; if(current_x<content_x)current_x=content_x;}else{if(alignment==1)current_y=content_y+(content_height-total_child_height)/2;else if(alignment==2)current_y=content_y+content_height-total_child_height; if(current_y<content_y)current_y=content_y;}
        int space_between=0; if(alignment==3&&flow_child_count>1){if(direction==0||direction==2)space_between=(content_width-total_child_width)/(flow_child_count-1);else space_between=(content_height-total_child_height)/(flow_child_count-1);if(space_between<0)space_between=0;}

        int flow_children_processed = 0;
        for (int i = 0; i < el->child_count; i++) {
            RenderElement* child = el->children[i]; if (!child) continue;
            int child_w = child_scaled_sizes[i][0], child_h = child_scaled_sizes[i][1];
            int child_render_origin_x, child_render_origin_y;
            bool child_has_pos = (child->header.pos_x != 0 || child->header.pos_y != 0);
            if(child_has_pos){/* ... absolute origin ... */ child_render_origin_x=content_x;child_render_origin_y=content_y;} else {/* ... flow origin + cross-axis + clamp ... */ child_render_origin_x=current_x;child_render_origin_y=current_y; if(direction==0||direction==2){if(alignment==1)child_render_origin_y=content_y+(content_height-child_h)/2;else if(alignment==2)child_render_origin_y=content_y+content_height-child_h; else child_render_origin_y=content_y;} else {if(alignment==1)child_render_origin_x=content_x+(content_width-child_w)/2; else if(alignment==2)child_render_origin_x=content_x+content_width-child_w; else child_render_origin_x=content_x;} if(child_render_origin_x<content_x)child_render_origin_x=content_x;if(child_render_origin_y<content_y)child_render_origin_y=content_y; if(direction==0||direction==2){current_x+=child_w;if(alignment==3&&flow_children_processed<flow_child_count-1)current_x+=space_between;}else{current_y+=child_h;if(alignment==3&&flow_children_processed<flow_child_count-1)current_y+=space_between;} flow_children_processed++;}

            // Recursive call: Pass this element's resolved colors (use_bg, use_fg) as defaults for child
            render_element(child, child_render_origin_x, child_render_origin_y, content_width, content_height, scale_x, scale_y, offset_x, offset_y, use_bg, use_fg, debug_file);
        }
    }
}


int main(int argc, char* argv[]) {
    if (argc != 2) { printf("Usage: %s <krb_file>\n", argv[0]); return 1; }
    FILE* debug_file = fopen("krb_term_debug.log", "w"); if (!debug_file) { debug_file = stderr; }
    FILE* file = fopen(argv[1], "rb"); if (!file) { fprintf(debug_file, "Error: Could not open file %s: %s\n", argv[1], strerror(errno)); if (debug_file != stderr) fclose(debug_file); return 1; }

    KrbDocument doc = {0};
    if (!krb_read_document(file, &doc)) { /* ... error handling ... */ goto error_cleanup; }
    fclose(file); file = NULL;
    fprintf(debug_file, "INFO: Parsed KRB OK - Elements=%u, Styles=%u, Strings=%u, Flags=0x%04X\n", doc.header.element_count, doc.header.style_count, doc.header.string_count, doc.header.flags);
    if (doc.header.element_count == 0) { /* ... handle no elements ... */ goto cleanup; }

    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) { /* ... error handling ... */ goto error_cleanup; }
    RenderElement* app_element = NULL;

    uint16_t design_width = 0; uint16_t design_height = 0;
    bool app_resizable = false; bool app_keep_aspect = false;
    // Define App-level defaults BEFORE processing elements
    uint32_t app_default_bg = 0x000000FF; // Black default BG (R=0, G=0, B=0, A=255)
    uint32_t app_default_fg = 0xFFFFFFFF; // White default FG (R=255, G=255, B=255, A=255)

    // --- Process Elements, Styles, Properties ---
    for (int i = 0; i < doc.header.element_count; i++) {
        elements[i].header = doc.elements[i];
        elements[i].text = NULL;
        elements[i].text_alignment = 0;
        // Initialize colors to 0 (meaning 'unset', will inherit or use default)
        elements[i].bg_color = 0;
        elements[i].fg_color = 0;
        elements[i].border_color = 0;
        memset(elements[i].border_widths, 0, 4);
        elements[i].parent = NULL; elements[i].child_count = 0;
        for(int k=0; k<MAX_ELEMENTS; ++k) elements[i].children[k] = NULL;

        elements[i].app_design_width = 0; elements[i].app_design_height = 0;
        elements[i].app_resizable = false; elements[i].app_keep_aspect = false;

        fprintf(debug_file, "INFO: Processing Element %d: type=0x%02X, style_id=%d, layout=0x%02X\n", i, elements[i].header.type, elements[i].header.style_id, elements[i].header.layout);

        // Special handling for App element to get defaults and window props
        if (elements[i].header.type == 0x00) {
            app_element = &elements[i];
            KrbProperty* app_props = (doc.properties && i < doc.header.element_count) ? doc.properties[i] : NULL;
            uint8_t app_prop_count = app_props ? elements[i].header.property_count : 0;

            // Check App's style for base defaults first
            if (app_element->header.style_id > 0 && app_element->header.style_id <= doc.header.style_count) {
                int style_idx = app_element->header.style_id - 1;
                if (doc.styles && style_idx >= 0 && style_idx < doc.header.style_count) { // Bounds check added
                     KrbStyle* style = &doc.styles[style_idx];
                     fprintf(debug_file, "INFO: Reading App Style %d\n", app_element->header.style_id);
                     for (int p = 0; p < style->property_count; ++p) {
                         KrbProperty* prop = &style->properties[p];
                         uint32_t color_val = 0;
                         if (prop->property_id == PROP_ID_BG_COLOR && (color_val = get_property_u32_color(prop)) != 0) {
                             app_default_bg = color_val;
                             fprintf(debug_file, "INFO: App Style sets default BG to 0x%08X\n", app_default_bg);
                         }
                         if (prop->property_id == PROP_ID_FG_COLOR && (color_val = get_property_u32_color(prop)) != 0) {
                             app_default_fg = color_val;
                             fprintf(debug_file, "INFO: App Style sets default FG to 0x%08X\n", app_default_fg);
                         }
                     }
                } else {
                    fprintf(debug_file, "WARN: App Style ID %d invalid.\n", app_element->header.style_id);
                }
            }
            // Apply App's direct properties (overrides style defaults for the app itself)
             if (app_props) {
                 fprintf(debug_file, "INFO: Reading App Direct Props\n");
                for (int p = 0; p < app_prop_count; ++p) {
                    KrbProperty* prop = &app_props[p];
                    uint32_t color_val = 0;
                     if (prop->property_id == PROP_ID_BG_COLOR && (color_val = get_property_u32_color(prop)) != 0) {
                         elements[i].bg_color = color_val; // Set App's specific BG
                          fprintf(debug_file, "INFO: App Direct Prop sets BG to 0x%08X\n", elements[i].bg_color);
                     }
                     if (prop->property_id == PROP_ID_FG_COLOR && (color_val = get_property_u32_color(prop)) != 0) {
                         elements[i].fg_color = color_val; // Set App's specific FG
                          fprintf(debug_file, "INFO: App Direct Prop sets FG to 0x%08X\n", elements[i].fg_color);
                     }
                     // Read window props etc.
                    elements[i].app_design_width = get_property_u16(app_props, app_prop_count, PROP_ID_WINDOW_WIDTH, 0);
                    elements[i].app_design_height = get_property_u16(app_props, app_prop_count, PROP_ID_WINDOW_HEIGHT, 0);
                    elements[i].app_resizable = get_property_bool(app_props, app_prop_count, PROP_ID_RESIZABLE, false);
                    elements[i].app_keep_aspect = get_property_bool(app_props, app_prop_count, PROP_ID_KEEP_ASPECT, false);
                    design_width = elements[i].app_design_width; design_height = elements[i].app_design_height;
                    app_resizable = elements[i].app_resizable; app_keep_aspect = elements[i].app_keep_aspect;
                }
            }
             fprintf(debug_file, "INFO: App Element (Final): BG=0x%08X, FG=0x%08X. Design=(%d,%d), Resizable=%d, KeepAspect=%d\n",
                     elements[i].bg_color, elements[i].fg_color, design_width, design_height, app_resizable, app_keep_aspect);
        } // End App element processing

        // Apply Styles to non-app elements
        if (elements[i].header.style_id > 0 && elements[i].header.style_id <= doc.header.style_count) {
            int style_idx = elements[i].header.style_id - 1;
             if (doc.styles && style_idx >= 0 && style_idx < doc.header.style_count) { // Bounds check added
                 KrbStyle* style = &doc.styles[style_idx];
                 fprintf(debug_file, "INFO: Applying Style %d (index %d) with %d props to Element %d\n", elements[i].header.style_id, style_idx, style->property_count, i);
                 for (int p = 0; p < style->property_count; ++p) {
                    KrbProperty* prop = &style->properties[p];
                    uint32_t color_val = 0;
                    // Set the element's color field directly from style
                    if (prop->property_id == PROP_ID_BG_COLOR && (color_val = get_property_u32_color(prop)) != 0) elements[i].bg_color = color_val;
                    if (prop->property_id == PROP_ID_FG_COLOR && (color_val = get_property_u32_color(prop)) != 0) elements[i].fg_color = color_val;
                    if (prop->property_id == PROP_ID_BORDER_COLOR && (color_val = get_property_u32_color(prop)) != 0) elements[i].border_color = color_val;
                    // Border Width
                    if (prop->property_id == PROP_ID_BORDER_WIDTH) { if (prop->value_type == 0x01 && prop->size==1 && prop->value) memset(elements[i].border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == 0x08 && prop->size==4 && prop->value) memcpy(elements[i].border_widths, prop->value, 4); }
                    // Text Align
                    if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == 0x09 && prop->size==1 && prop->value) elements[i].text_alignment = *(uint8_t*)prop->value;
                 }
                 fprintf(debug_file, "DEBUG: After Style %d: BG=0x%08X, FG=0x%08X, Border=0x%08X, BW=%d\n", style_idx, elements[i].bg_color, elements[i].fg_color, elements[i].border_color, elements[i].border_widths[0]);
            } else {
                 fprintf(debug_file, "WARN: Style ID %d for Element %d invalid.\n", elements[i].header.style_id, i);
            }
        }

        // Apply Direct Properties (Overwrite styles)
        if (doc.properties && i < doc.header.element_count && doc.properties[i]) {
            KrbProperty* props = doc.properties[i]; uint8_t prop_count = elements[i].header.property_count;
            fprintf(debug_file, "INFO: Applying %d direct properties for Element %d\n", prop_count, i);
            for (int p = 0; p < prop_count; ++p) {
                KrbProperty* prop = &props[p];
                uint32_t color_val = 0;
                 // Colors: Overwrite element's color field
                 if (prop->property_id == PROP_ID_BG_COLOR && (color_val = get_property_u32_color(prop)) != 0) elements[i].bg_color = color_val;
                 if (prop->property_id == PROP_ID_FG_COLOR && (color_val = get_property_u32_color(prop)) != 0) elements[i].fg_color = color_val;
                 if (prop->property_id == PROP_ID_BORDER_COLOR && (color_val = get_property_u32_color(prop)) != 0) elements[i].border_color = color_val;
                 // Border Width
                 if (prop->property_id == PROP_ID_BORDER_WIDTH) { if (prop->value_type == 0x01 && prop->size==1 && prop->value) memset(elements[i].border_widths, *(uint8_t*)prop->value, 4); else if (prop->value_type == 0x08 && prop->size==4 && prop->value) memcpy(elements[i].border_widths, prop->value, 4); }
                 // Text Align
                 if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == 0x09 && prop->size==1 && prop->value) elements[i].text_alignment = *(uint8_t*)prop->value;
                 // Text Content
                 if (prop->property_id == PROP_ID_TEXT_CONTENT && prop->value_type == 0x04 && prop->size==1 && prop->value) {
                     uint8_t str_idx = *(uint8_t*)prop->value;
                     // KRB String table index is 0-based based on parsing log ("Strings=7" -> indices 0-6)
                     if (str_idx < doc.header.string_count && doc.strings[str_idx]) {
                         free(elements[i].text);
                         elements[i].text = strip_quotes(doc.strings[str_idx]);
                         // fprintf(debug_file, "INFO: Element %d Text (Index %d): '%s'\n", i, str_idx, elements[i].text ? elements[i].text : "NULL");
                     } else { fprintf(debug_file, "WARN: Element %d text string index %d invalid.\n", i, str_idx); }
                 }
            }
             fprintf(debug_file, "DEBUG: After Direct Props %d: BG=0x%08X, FG=0x%08X, Border=0x%08X, BW=%d, Text='%s'\n", i, elements[i].bg_color, elements[i].fg_color, elements[i].border_color, elements[i].border_widths[0], elements[i].text ? elements[i].text : "NULL");
        }
    } // End element processing loop

    // --- Build Tree (HACK) --- (Unchanged)
    fprintf(debug_file, "WARN: Using TEMPORARY HACK for tree building...\n");
    RenderElement* parent_stack[MAX_ELEMENTS]; int stack_top = -1;
    for (int i = 0; i < doc.header.element_count; i++) { /* ... HACK logic ... */
        while (stack_top >= 0) { RenderElement* p = parent_stack[stack_top]; if (p->child_count == p->header.child_count) stack_top--; else break; }
        if (stack_top >= 0) { RenderElement* current_parent = parent_stack[stack_top]; elements[i].parent = current_parent; if (current_parent->child_count < MAX_ELEMENTS) current_parent->children[current_parent->child_count++] = &elements[i]; }
        if (elements[i].header.child_count > 0) { if (stack_top + 1 < MAX_ELEMENTS) parent_stack[++stack_top] = &elements[i]; }
    }


    // --- Find Roots --- (Unchanged, including App element forcing)
    RenderElement* root_elements[MAX_ELEMENTS]; int root_count = 0;
    for(int i = 0; i < doc.header.element_count; ++i) { if (!elements[i].parent) { if (root_count < MAX_ELEMENTS) root_elements[root_count++] = &elements[i]; else break; } }
    if (root_count == 0 && doc.header.element_count > 0) { fprintf(debug_file, "ERROR: No root elements found!\n"); goto cleanup; }
    else if (root_count > 0) { fprintf(debug_file, "INFO: Found %d root(s).\n", root_count); if (app_element && (root_count > 1 || root_elements[0] != app_element)) { root_elements[0] = app_element; root_count = 1; fprintf(debug_file, "INFO: Forcing App Element as the single root.\n"); } }


    // --- Termbox Initialization ---
    if (tb_init() != 0) { fprintf(debug_file, "ERROR: Failed to initialize termbox\n"); goto error_cleanup; }

    // --- Determine Effective App Background Color for Clearing ---
    uint32_t effective_app_bg_u32 = app_default_bg; // Start with the default established from App's style
    if (app_element && app_element->bg_color != 0) {
        effective_app_bg_u32 = app_element->bg_color; // Override with App's direct BG property if set
    }
    int tb_clear_bg = rgb_to_tb_color(effective_app_bg_u32, debug_file);
    fprintf(debug_file, "INFO: Setting initial clear color based on effective App BG 0x%08X -> %d\n", effective_app_bg_u32, tb_clear_bg);

    // --- Clear Screen with App Background ---
    // Method 1: Set clear attributes (might not work on all terminals/termbox versions)
    // tb_set_clear_attributes(TB_DEFAULT, tb_clear_bg);
    // tb_clear();
    // Method 2: Manually fill the screen after standard clear
    tb_clear(); // Clear with default attributes first
    int term_w = tb_width(); int term_h = tb_height();
    // Use a default foreground for clearing, maybe from App's FG?
    uint32_t effective_app_fg_u32 = app_default_fg;
     if (app_element && app_element->fg_color != 0) {
        effective_app_fg_u32 = app_element->fg_color;
    }
    int tb_clear_fg = rgb_to_tb_color(effective_app_fg_u32, debug_file);
    fprintf(debug_file, "INFO: Manually filling background with FG=%d BG=%d\n", tb_clear_fg, tb_clear_bg);
    for (int y = 0; y < term_h; ++y) {
        for (int x = 0; x < term_w; ++x) {
            tb_change_cell(x, y, ' ', tb_clear_fg, tb_clear_bg);
        }
    }


    fprintf(debug_file, "INFO: Terminal size: %d x %d\n", term_w, term_h);

    // --- Calculate Scaling Factors --- (Unchanged)
    double scale_x = 1.0; double scale_y = 1.0; int offset_x = 0; int offset_y = 0;
    if (app_resizable && design_width > 0 && design_height > 0 && term_w > 0 && term_h > 0) { /* ... calculate scale/offset ... */
        double actual_scale_x = (double)term_w / design_width; double actual_scale_y = (double)term_h / design_height;
        if (app_keep_aspect) { scale_x = scale_y = fmin(actual_scale_x, actual_scale_y); int scaled_total_w = (int)round(design_width * scale_x); int scaled_total_h = (int)round(design_height * scale_y); offset_x = (term_w - scaled_total_w) / 2; offset_y = (term_h - scaled_total_h) / 2; if (offset_x < 0) offset_x = 0; if (offset_y < 0) offset_y = 0; }
        else { scale_x = actual_scale_x; scale_y = actual_scale_y; offset_x = 0; offset_y = 0; }
    }
    fprintf(debug_file, "INFO: Using Scale=(%.3f, %.3f), Offset=(%d, %d)\n", scale_x, scale_y, offset_x, offset_y);

    // --- Initial Render Call ---
    for (int i = 0; i < root_count; ++i) {
        // Pass App's effective BG and FG as the top-level defaults
        render_element(root_elements[i], 0, 0, term_w, term_h, scale_x, scale_y, offset_x, offset_y, effective_app_bg_u32, effective_app_fg_u32, debug_file);
    }
    tb_present();

    struct tb_event ev; fprintf(debug_file, "INFO: Rendering complete. Press any key to exit.\n"); fflush(debug_file);
    tb_poll_event(&ev); tb_shutdown();

cleanup:
    fprintf(debug_file, "INFO: Cleaning up resources...\n");
    if (elements) {
        for (int i = 0; i < doc.header.element_count; i++) free(elements[i].text);
        free(elements);
    }
    krb_free_document(&doc);
    if (debug_file && debug_file != stderr) fclose(debug_file);
    return 0;

error_cleanup:
     fprintf(debug_file, "FATAL ERROR occurred.\n");
     if(file) fclose(file);
     krb_free_document(&doc);
     if(elements) { /* Leak element text */ free(elements); }
     if (debug_file && debug_file != stderr) fclose(debug_file);
     // Attempt shutdown if tb initialized
     // tb_shutdown(); // Risky if tb_init failed
     return 1;
}
