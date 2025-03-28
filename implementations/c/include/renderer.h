#ifndef KRB_RENDERER_H
#define KRB_RENDERER_H

#include <stdio.h>
#include <stdbool.h>
#include "raylib.h"
#include "krb.h" // Needs KRB types like KrbElementHeader, ELEM_TYPE_*, etc.

// Define MAX_ELEMENTS if not already defined (should be in krb.h)
#ifndef MAX_ELEMENTS
#define MAX_ELEMENTS 256
#endif

// --- Renderer Structure (Needed by users of the renderer) ---
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

    // --- Runtime Rendering Data ---
    int render_x;                 // Final calculated X position on screen
    int render_y;                 // Final calculated Y position on screen
    int render_w;                 // Final calculated width on screen
    int render_h;                 // Final calculated height on screen
    bool is_interactive;          // Flag indicating if this element responds to hover/click
    int original_index;           // Original index from KrbDocument (useful for mapping events)

} RenderElement;


// --- Public Function Declarations ---

// Helper to read Little Endian uint16_t (might be useful externally)
uint16_t read_u16(const void* data);

// The core recursive rendering function
void render_element(RenderElement* el,
                    int parent_content_x,
                    int parent_content_y,
                    int parent_content_width,
                    int parent_content_height,
                    float scale_factor,
                    FILE* debug_file);

// Consider adding more functions here if you break down the original main(), e.g.:
// RenderElement* krb_prepare_render_tree(KrbDocument* doc, FILE* debug_file, Color* out_default_bg, ...);
// void krb_free_render_tree(RenderElement* elements, int count);


#endif // KRB_RENDERER_H
