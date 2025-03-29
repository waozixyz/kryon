#ifndef KRB_RENDERER_H
#define KRB_RENDERER_H

#include <stdio.h>
#include <stdbool.h> // For bool type
#include "raylib.h"  // Needs Texture2D definition
#include "krb.h"     // Needs KRB types like KrbElementHeader, ELEM_TYPE_*, etc.

// Define MAX_ELEMENTS if not already defined (should match krb.h)
#ifndef MAX_ELEMENTS
#define MAX_ELEMENTS 256
#endif


#define MAX_LINE_LENGTH 512

// Define INVALID_RESOURCE_INDEX (used by renderer logic)
#ifndef INVALID_RESOURCE_INDEX
#define INVALID_RESOURCE_INDEX 0xFF // Or choose another suitable invalid value
#endif

// --- Renderer Structure (Holds processed data ready for rendering) ---
typedef struct RenderElement {
    KrbElementHeader header;      // Copy of the header
    char* text;                   // Resolved text string (allocated)
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
    int original_index;           // Original index from KrbDocument (useful for mapping)

    // --- Resource Handling (Fields added for textures) ---
    uint8_t resource_index;       // Index into KrbDocument's resources array (0-based), or INVALID_RESOURCE_INDEX
    Texture2D texture;            // Loaded Raylib texture (if type is Image and resource loaded)
    bool texture_loaded;          // Flag indicating if texture was loaded successfully

} RenderElement;


// --- Public Function Declarations ---

// (read_u16 is now krb_read_u16_le in krb.h/c)

// The core recursive rendering function
void render_element(RenderElement* el,
                    int parent_content_x,
                    int parent_content_y,
                    int parent_content_width,
                    int parent_content_height,
                    float scale_factor,
                    FILE* debug_file); // Optional debug output file

// Consider adding more functions here if you break down the original main(), e.g.:
// RenderElement* krb_prepare_render_tree(KrbDocument* doc, FILE* debug_file, Color* out_default_bg, ...);
// void krb_free_render_tree(RenderElement* elements, int count);


#endif // KRB_RENDERER_H