#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <errno.h>
#include <math.h>

// Include the renderer header
#include "renderer.h" // Includes krb.h, raylib.h, RenderElement, render_element()

// --->>> INCLUDE THE GENERATED HEADER WITH EMBEDDED DATA <<<---
#include "button_krb_data.h" // Provides get_embedded_krb_data() and _len()

// --- Add Default Definitions ---
#define DEFAULT_WINDOW_WIDTH 800
#define DEFAULT_WINDOW_HEIGHT 600
#define DEFAULT_SCALE_FACTOR 1.0f
// --- End Default Definitions ---

// --- Event Handling Logic (Stays the same) ---
void handleButtonClick() {
    printf("------------------------------------\n");
    printf(">>> C Event Handler: Button Clicked! <<<\n");
    printf("------------------------------------\n");
}
typedef void (*KrbEventHandlerFunc)();
typedef struct { const char* name; KrbEventHandlerFunc func; } EventHandlerMapping;
EventHandlerMapping event_handlers[] = {
    { "handleButtonClick", handleButtonClick },
    { NULL, NULL }
};
KrbEventHandlerFunc find_handler(const char* name) {
    if (!name) return NULL;
    for (int i = 0; event_handlers[i].name != NULL; i++) {
        if (strcmp(event_handlers[i].name, name) == 0) return event_handlers[i].func;
    }
    fprintf(stderr, "Warning: Handler function not found for name: %s\n", name);
    return NULL;
}

// --- Main Application ---
int main(int argc, char* argv[]) {
    // --- Setup ---
    // No longer need command-line arguments for the KRB file path
    (void)argc; // Suppress unused parameter warning
    (void)argv; // Suppress unused parameter warning

    FILE* debug_file = fopen("krb_render_debug_example.log", "w");
    if (!debug_file) {
        debug_file = stderr;
        fprintf(stderr, "Warning: Could not open krb_render_debug_example.log, writing debug to stderr.\n");
    }

    // --- Access Embedded KRB Data ---
    // Use the functions provided by the included button_krb_data.h
    unsigned char *krb_data_buffer = get_embedded_krb_data();
    unsigned int krb_data_size = get_embedded_krb_data_len();

    fprintf(debug_file, "INFO: Using embedded KRB data (Size: %u bytes)\n", krb_data_size);

    // --- Create In-Memory Stream ---
    // Use fmemopen (POSIX) to treat the memory buffer like a file stream
    // NOTE: fmemopen might not be available on non-POSIX systems (like native Windows without MinGW/Cygwin)
    //       If targeting such systems, you would need to adapt krb_read_document
    //       to accept a buffer and size directly, or implement a custom stream.
    FILE* file = fmemopen(krb_data_buffer, krb_data_size, "rb"); // "rb" for read binary
    if (!file) {
        perror("ERROR: Could not create in-memory stream with fmemopen");
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }

    // --- Parsing (Uses the in-memory FILE* stream) ---
    KrbDocument doc = {0};
    fprintf(debug_file, "INFO: Reading KRB document from memory...\n");
    // The krb_reader functions are linked separately (as per Makefile)
    if (!krb_read_document(file, &doc)) { // Pass the memory stream handle
        fprintf(stderr, "ERROR: Failed to parse embedded KRB data\n");
        fclose(file); // Close the memory stream
        krb_free_document(&doc);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    fclose(file); // Close the memory stream after successful parsing
    fprintf(debug_file, "INFO: Parsed embedded KRB OK - Elements=%u, Styles=%u, Strings=%u, EventsRead=%s\n",
            doc.header.element_count, doc.header.style_count, doc.header.string_count,
            doc.events ? "Yes" : "No");

    if (doc.header.element_count == 0) {
        fprintf(stderr, "ERROR: No elements found in KRB data.\n");
        krb_free_document(&doc);
        if(debug_file!=stderr) fclose(debug_file);
        return 0;
    }

    // --- Prepare Render Elements ---
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    if (!elements) {
        perror("ERROR: Failed to allocate memory for render elements");
        krb_free_document(&doc);
        if(debug_file!=stderr) fclose(debug_file);
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
        fprintf(debug_file, "INFO: Processing App Element (Index 0)\n");

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
             } else { fprintf(debug_file, "WARN: App Style ID %d is invalid.\n", app_element->header.style_id); }
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
                // Use read_u16 (declared in renderer.h, defined in raylib_renderer.c)
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

        fprintf(debug_file, "INFO: Processed App Element props. Window: %dx%d, Title: '%s'\n", window_width, window_height, window_title ? window_title : "(None)");

    } else {
        fprintf(debug_file, "WARN: No App element found or KRB lacks App flag. Using default window settings.\n");
        window_title = strdup("KRB Button Example"); // Default title
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

        // Set interactivity based on element type (e.g., buttons)
        current_render_el->is_interactive = (current_render_el->header.type == ELEM_TYPE_BUTTON);
        if (current_render_el->is_interactive) {
            fprintf(debug_file, "DEBUG: Element %d (Type 0x%02X) marked interactive.\n", i, current_render_el->header.type);
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
             } else { fprintf(debug_file, "WARN: Style ID %d for Element %d is invalid.\n", current_render_el->header.style_id, i); }
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
                 else if (prop->property_id == PROP_ID_TEXT_CONTENT && prop->value_type == VAL_TYPE_STRING && prop->size == 1) { uint8_t idx = *(uint8_t*)prop->value; if (idx < doc.header.string_count && doc.strings[idx]) { free(current_render_el->text); current_render_el->text = strdup(doc.strings[idx]); } else { fprintf(debug_file, "WARN: Element %d text string index %d invalid.\n", i, idx); } }
                 else if (prop->property_id == PROP_ID_TEXT_ALIGNMENT && prop->value_type == VAL_TYPE_ENUM && prop->size==1 && prop->value) { current_render_el->text_alignment = *(uint8_t*)prop->value; }
                 // Add other direct property applications here
            }
        }
    } // End loop processing elements

    // --- Build Parent/Child Tree ---
    // This logic seems correct based on the KrbElementHeader structure having child_count.
    RenderElement* parent_stack[MAX_ELEMENTS]; int stack_top = -1;
    for (int i = 0; i < doc.header.element_count; i++) {
        // Pop parents from stack if their children are fully processed
        while (stack_top >= 0) {
            RenderElement* p = parent_stack[stack_top];
            // Check against the *original* header's child count stored in the RenderElement copy
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
                 // Increment child count *after* assigning the child pointer
                 cp->children[cp->child_count++] = &elements[i];
            } else {
                 fprintf(debug_file, "WARN: Exceeded MAX_ELEMENTS children for element %d\n", cp->original_index);
                 // Decide how to handle this - maybe stop adding children
            }
        }
        // Push current element onto stack if it expects children
        if (elements[i].header.child_count > 0) {
            if (stack_top + 1 < MAX_ELEMENTS) {
                parent_stack[++stack_top] = &elements[i];
            } else {
                 fprintf(debug_file, "WARN: Exceeded MAX_ELEMENTS for parent stack depth at element %d\n", i);
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
                 fprintf(debug_file, "WARN: Exceeded MAX_ELEMENTS for root elements.\n");
                 break;
            }
        }
    }
    if (root_count == 0 && doc.header.element_count > 0) {
        fprintf(stderr, "ERROR: No root element found in KRB.\n");
        krb_free_document(&doc);
        free(elements);
        if(debug_file!=stderr) fclose(debug_file);
        return 1;
    }
    // If App flag is set, ensure the app element is the single root
    if (root_count > 0 && app_element && root_elements[0] != app_element) {
        fprintf(debug_file, "INFO: App flag set, forcing App Elem 0 as single root.\n");
        root_elements[0] = app_element;
        root_count = 1;
    }
    fprintf(debug_file, "INFO: Found %d root element(s).\n", root_count);


    // --- Init Raylib Window ---
    InitWindow(window_width, window_height, window_title ? window_title : "KRB Button Example");
    if (resizable) SetWindowState(FLAG_WINDOW_RESIZABLE);
    SetTargetFPS(60);
    fprintf(debug_file, "INFO: Entering main loop...\n");

    // --- Main Loop ---
    while (!WindowShouldClose()) {
        Vector2 mousePos = GetMousePosition();
        bool mouse_clicked = IsMouseButtonPressed(MOUSE_BUTTON_LEFT);

        // --- Window Resizing ---
         if (resizable && IsWindowResized()) {
            window_width = GetScreenWidth(); window_height = GetScreenHeight();
            // Update root/app element size if necessary for layout recalculation
            if (app_element) { app_element->render_w = window_width; app_element->render_h = window_height; }
             fprintf(debug_file, "INFO: Window resized to %dx%d\n", window_width, window_height);
        }

        // --- Interaction Check & Callback Execution ---
        SetMouseCursor(MOUSE_CURSOR_DEFAULT); // Reset cursor each frame
        // Iterate top-down (reverse order) to find topmost interactive element under cursor
        for (int i = doc.header.element_count - 1; i >= 0; --i) {
             RenderElement* el = &elements[i];
             // Check only interactive elements that have been rendered (have size)
             if (el->is_interactive && el->render_w > 0 && el->render_h > 0) {
                Rectangle elementRect = { (float)el->render_x, (float)el->render_y, (float)el->render_w, (float)el->render_h };
                if (CheckCollisionPointRec(mousePos, elementRect)) {
                    SetMouseCursor(MOUSE_CURSOR_POINTING_HAND); // Change cursor on hover
                    if (mouse_clicked) {
                         int original_idx = el->original_index;
                         // Check if event data exists for this element
                         if (doc.events && original_idx < doc.header.element_count && doc.events[original_idx]) {
                            // Iterate through events defined for this element
                            for (int k = 0; k < doc.elements[original_idx].event_count; k++) {
                                KrbEvent* event = &doc.events[original_idx][k];
                                // Check if it's a click event
                                if (event->event_type == EVENT_TYPE_CLICK) {
                                    // Get the callback name string index
                                    uint8_t callback_idx = event->callback_id;
                                    // Validate index and get string
                                    if (callback_idx < doc.header.string_count && doc.strings[callback_idx]) {
                                        const char* handler_name = doc.strings[callback_idx];
                                        // Find the corresponding C function
                                        KrbEventHandlerFunc handler_func = find_handler(handler_name);
                                        // Execute if found
                                        if (handler_func) {
                                            fprintf(debug_file, "INFO: Executing click handler '%s' for element %d\n", handler_name, original_idx);
                                            handler_func();
                                        } else {
                                            fprintf(debug_file, "WARN: Click handler '%s' not found for element %d\n", handler_name, original_idx);
                                        }
                                    } else {
                                         fprintf(debug_file, "WARN: Invalid callback string index %d for element %d\n", callback_idx, original_idx);
                                    }
                                    // Assuming only one click handler per element for now
                                    goto end_interaction_check; // Exit loops after handling click
                                }
                            } // End loop through events for the element
                         } else {
                             fprintf(debug_file, "DEBUG: Clicked interactive element %d, but no event data found/defined.\n", original_idx);
                         }
                    } // End if mouse_clicked
                    // If hovering or click handled, no need to check elements underneath
                    break; // Exit hover check loop (found topmost element under cursor)
                } // End if collision check
            } // End if interactive and rendered
        } // End loop through elements (reverse)
        end_interaction_check:; // Label for goto


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
                // Call the globally available render_element function (defined in raylib_renderer.c)
                render_element(root_elements[i], 0, 0, window_width, window_height, scale_factor, debug_file);
            }
        }

        EndDrawing();
    } // End main loop

    // --- Cleanup ---
    fprintf(debug_file, "INFO: Closing window and cleaning up...\n");
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

    printf("Button example finished.\n");
    return 0;
}