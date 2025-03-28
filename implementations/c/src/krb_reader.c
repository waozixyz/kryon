#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h> // For perror
#include "krb.h"

// Internal helper to read element header (handles potential read errors)
static int read_element_header_internal(FILE* file, KrbElementHeader* element) {
    unsigned char buffer[16];
    if (fread(buffer, 1, 16, file) != 16) {
        fprintf(stderr, "Error: Failed to read 16 bytes for element header at offset %ld\n", ftell(file) - 16);
        return 0; // Indicate failure
    }
    element->type = buffer[0];
    element->id = buffer[1];
    element->pos_x = (uint16_t)(buffer[2] | (buffer[3] << 8));
    element->pos_y = (uint16_t)(buffer[4] | (buffer[5] << 8));
    element->width = (uint16_t)(buffer[6] | (buffer[7] << 8));
    element->height = (uint16_t)(buffer[8] | (buffer[9] << 8));
    element->layout = buffer[10];
    element->style_id = buffer[11];
    element->property_count = buffer[12];
    element->child_count = buffer[13];
    element->event_count = buffer[14]; // <<< This count is now crucial
    element->animation_count = buffer[15];
    // printf("DEBUG: Read element - type=0x%02X, style_id=%d, width=%d, height=%d, props=%d, children=%d, events=%d\n",
    //        element->type, element->style_id, element->width, element->height, element->property_count, element->child_count, element->event_count);
    return 1; // Indicate success
}

// Internal helper to read a single property (handles potential read errors)
static int read_property_internal(FILE* file, KrbProperty* prop) {
    unsigned char buffer[3];
    long prop_header_offset = ftell(file); // Store position before read

    if (fread(buffer, 1, 3, file) != 3) {
        fprintf(stderr, "Error: Failed to read 3 bytes for property header at offset %ld\n", prop_header_offset);
        prop->value = NULL; // Ensure value is NULL on error
        prop->size = 0;
        return 0; // Indicate failure
    }
    prop->property_id = buffer[0];
    prop->value_type = buffer[1];
    prop->size = buffer[2];

    if (prop->size > 0) {
        prop->value = malloc(prop->size);
        if (!prop->value) {
             perror("Error: Failed to allocate memory for property value");
             return 0; // Indicate failure
        }
        if (fread(prop->value, 1, prop->size, file) != prop->size) {
            fprintf(stderr, "Error: Failed to read %u bytes for property value (id=0x%02X) at offset %ld\n",
                    prop->size, prop->property_id, ftell(file) - prop->size);
            free(prop->value);
            prop->value = NULL;
            return 0; // Indicate failure
        }
    } else {
        prop->value = NULL; // No value data if size is 0
    }
    // printf("DEBUG: Read property - id=0x%02X, type=0x%02X, size=%d\n", prop->property_id, prop->value_type, prop->size);
    return 1; // Indicate success
}


// Reads the main file header. Returns 1 on success, 0 on failure.
int read_header(FILE* file, KrbHeader* header) {
    // --- THIS FUNCTION REMAINS UNCHANGED ---
    unsigned char buffer[38]; // Size of KrbHeader
    if (!file || !header) return 0;
    if (fseek(file, 0, SEEK_SET) != 0) { perror("Error seeking to start of file"); return 0; }
    size_t bytes_read = fread(buffer, 1, sizeof(buffer), file);
    if (bytes_read < sizeof(buffer)) { fprintf(stderr, "Error: Failed to read %zu-byte header, got %zu bytes\n", sizeof(buffer), bytes_read); return 0; }
    memcpy(header->magic, buffer + 0, 4);
    header->version         = (uint16_t)(buffer[4] | (buffer[5] << 8));
    header->flags           = (uint16_t)(buffer[6] | (buffer[7] << 8));
    header->element_count   = (uint16_t)(buffer[8] | (buffer[9] << 8));
    header->style_count     = (uint16_t)(buffer[10] | (buffer[11] << 8));
    header->animation_count = (uint16_t)(buffer[12] | (buffer[13] << 8));
    header->string_count    = (uint16_t)(buffer[14] | (buffer[15] << 8));
    header->resource_count  = (uint16_t)(buffer[16] | (buffer[17] << 8));
    header->element_offset  = (uint32_t)(buffer[18] | (buffer[19] << 8) | (buffer[20] << 16) | (buffer[21] << 24));
    header->style_offset    = (uint32_t)(buffer[22] | (buffer[23] << 8) | (buffer[24] << 16) | (buffer[25] << 24));
    header->animation_offset= (uint32_t)(buffer[26] | (buffer[27] << 8) | (buffer[28] << 16) | (buffer[29] << 24));
    header->string_offset   = (uint32_t)(buffer[30] | (buffer[31] << 8) | (buffer[32] << 16) | (buffer[33] << 24));
    header->total_size      = (uint32_t)(buffer[34] | (buffer[35] << 8) | (buffer[36] << 16) | (buffer[37] << 24));
    if (memcmp(header->magic, "KRB1", 4) != 0) { fprintf(stderr, "Error: Invalid magic number. Expected 'KRB1', got '%.4s'\n", header->magic); return 0; }
    if (header->version != 0x0001) { fprintf(stderr, "Error: Unsupported KRB version 0x%04X\n", header->version); return 0; }
    if (header->element_offset < sizeof(KrbHeader) && header->element_count > 0) { fprintf(stderr, "Error: Element offset (%u) overlaps header.\n", header->element_offset); return 0; }
    return 1;
    // --- END OF UNCHANGED read_header ---
}


// Reads the entire KRB document structure into memory.
// Returns 1 on success, 0 on failure. Caller must call krb_free_document.
int krb_read_document(FILE* file, KrbDocument* doc) {
     if (!file || !doc) return 0;

    memset(doc, 0, sizeof(KrbDocument)); // Clear doc structure

    if (!read_header(file, &doc->header)) {
        return 0; // Header read failed or validation failed
    }

    // --- Validate App element presence if Flag Bit 6 is set ---
    // --- THIS SECTION REMAINS UNCHANGED ---
    if ((doc->header.flags & FLAG_HAS_APP) && doc->header.element_count > 0) {
        long original_pos = ftell(file);
        if (fseek(file, doc->header.element_offset, SEEK_SET) != 0) { perror("Error seeking to element offset for App check"); return 0; }
        unsigned char first_type;
        if (fread(&first_type, 1, 1, file) != 1) { fprintf(stderr, "Error: Failed to read first element type for App check\n"); fseek(file, original_pos, SEEK_SET); return 0; }
        if (first_type != ELEM_TYPE_APP) { fprintf(stderr, "Error: Header flag indicates App element, but first element is type 0x%02X, not 0x00\n", first_type); fseek(file, original_pos, SEEK_SET); return 0; }
        if (fseek(file, original_pos, SEEK_SET) != 0) { perror("Error seeking back after App check"); return 0; }
    }
    // --- END OF UNCHANGED APP CHECK ---

    // --- Read Elements, Properties, and Events --- // <<< MODIFIED SECTION TITLE
    if (doc->header.element_count > 0) {
        if (doc->header.element_offset < sizeof(KrbHeader)) { /* Already checked in read_header */ return 0; }

        // Allocate memory for element headers, property pointers, AND EVENT POINTERS <<< MODIFIED
        doc->elements = calloc(doc->header.element_count, sizeof(KrbElementHeader));
        doc->properties = calloc(doc->header.element_count, sizeof(KrbProperty*));
        doc->events = calloc(doc->header.element_count, sizeof(KrbEvent*)); // <<< ALLOCATE EVENT POINTER ARRAY
        if (!doc->elements || !doc->properties || !doc->events) { // <<< CHECK EVENT ALLOCATION
            perror("Error: Failed to allocate memory for elements, properties, or events pointers");
            krb_free_document(doc); // Attempt cleanup
            return 0;
        }

        // Seek to the start of the element data section
        if (fseek(file, doc->header.element_offset, SEEK_SET) != 0) {
             perror("Error seeking to element data section");
             krb_free_document(doc); return 0;
        }

        for (uint16_t i = 0; i < doc->header.element_count; i++) {
            // Read the header for this element
            if (!read_element_header_internal(file, &doc->elements[i])) {
                fprintf(stderr, "Failed reading header for element index %u\n", i);
                krb_free_document(doc); return 0;
            }

            // Read properties for this element (Unchanged Logic)
            if (doc->elements[i].property_count > 0) {
                doc->properties[i] = calloc(doc->elements[i].property_count, sizeof(KrbProperty));
                if (!doc->properties[i]) {
                    perror("Error: Failed to allocate property array");
                    fprintf(stderr, "Allocation failed for element index %u\n", i);
                    krb_free_document(doc); return 0;
                }
                for (uint8_t j = 0; j < doc->elements[i].property_count; j++) {
                    if (!read_property_internal(file, &doc->properties[i][j])) {
                         fprintf(stderr, "Failed reading property %u for element index %u\n", j, i);
                         krb_free_document(doc); return 0;
                    }
                }
            } else {
                doc->properties[i] = NULL; // No properties for this element
            }
            // File pointer is now AFTER the properties for element i

            // --- Read Events for this element --- // <<< NEW SECTION
            if (doc->elements[i].event_count > 0) {
                doc->events[i] = calloc(doc->elements[i].event_count, sizeof(KrbEvent));
                if (!doc->events[i]) {
                    perror("Error: Failed to allocate event array");
                    fprintf(stderr, "Allocation failed for element index %u\n", i);
                    krb_free_document(doc); return 0;
                }
                // Read all event entries for this element directly
                size_t events_to_read = doc->elements[i].event_count;
                size_t events_read = fread(doc->events[i], sizeof(KrbEvent), events_to_read, file);
                if (events_read != events_to_read) {
                    fprintf(stderr, "Error: Failed to read %zu events for element %u (read %zu)\n", events_to_read, i, events_read);
                    krb_free_document(doc); return 0; // Frees partially read events too
                }
                // printf("DEBUG: Read %u events for element %u\n", doc->elements[i].event_count, i);
            } else {
                doc->events[i] = NULL; // No events for this element
            }
            // File pointer is now AFTER the events for element i

            // --- Calculate and skip over Animation and Child references ONLY --- // <<< MODIFIED SECTION
            long bytes_to_skip = 0;
            // Spec: Animation Refs section comes after Events
            bytes_to_skip += (long)doc->elements[i].animation_count * 2; // Size: Index(1) + Trigger(1) = 2
            // Spec: Child Refs section comes after Animation Refs
            bytes_to_skip += (long)doc->elements[i].child_count * 2; // Size: Child Offset(2) = 2

            if (bytes_to_skip > 0) {
                 // printf("DEBUG: Element %u skipping %ld bytes for refs (A:%u, C:%u)\n",
                 //        i, bytes_to_skip, doc->elements[i].animation_count, doc->elements[i].child_count);
                 if (fseek(file, bytes_to_skip, SEEK_CUR) != 0) {
                     perror("Error skipping reference data (animations/children) after element events");
                     fprintf(stderr, "Error occurred after element index %u\n", i);
                     krb_free_document(doc); return 0;
                 }
            }
            // File pointer is now positioned at the start of the next element (or end of section)
        } // End loop through elements
    }

    // --- Read Styles and their Properties ---
    // --- THIS SECTION REMAINS UNCHANGED ---
    if (doc->header.style_count > 0) {
        if (doc->header.style_offset < sizeof(KrbHeader)) { fprintf(stderr, "Error: Style offset (%u) overlaps header.\n", doc->header.style_offset); krb_free_document(doc); return 0; }
        doc->styles = calloc(doc->header.style_count, sizeof(KrbStyle));
        if (!doc->styles) { perror("Error: Failed to allocate memory for styles"); krb_free_document(doc); return 0; }
        if (fseek(file, doc->header.style_offset, SEEK_SET) != 0) { perror("Error seeking to style data section"); krb_free_document(doc); return 0; }
        for (uint16_t i = 0; i < doc->header.style_count; i++) {
            unsigned char style_header_buf[3];
            if (fread(style_header_buf, 1, 3, file) != 3) { fprintf(stderr, "Error: Failed to read header for style index %u\n", i); krb_free_document(doc); return 0; }
            doc->styles[i].id = style_header_buf[0];
            doc->styles[i].name_index = style_header_buf[1];
            doc->styles[i].property_count = style_header_buf[2];
            doc->styles[i].properties = NULL;
            if (doc->styles[i].property_count > 0) {
                doc->styles[i].properties = calloc(doc->styles[i].property_count, sizeof(KrbProperty));
                if (!doc->styles[i].properties) { perror("Error: Failed to allocate style property array"); fprintf(stderr, "Allocation failed for style index %u\n", i); krb_free_document(doc); return 0; }
                for (uint8_t j = 0; j < doc->styles[i].property_count; j++) {
                     if (!read_property_internal(file, &doc->styles[i].properties[j])) { fprintf(stderr, "Failed reading property %u for style index %u\n", j, i); krb_free_document(doc); return 0; }
                }
            }
        }
    }
    // --- END OF UNCHANGED STYLES SECTION ---

    // TODO: Read Animations (Skipped for now)

    // --- Read Strings ---
    // --- THIS SECTION REMAINS UNCHANGED ---
    if (doc->header.string_count > 0) {
        if (doc->header.string_offset < sizeof(KrbHeader)) { fprintf(stderr, "Error: String offset (%u) overlaps header.\n", doc->header.string_offset); krb_free_document(doc); return 0; }
        doc->strings = calloc(doc->header.string_count, sizeof(char*));
        if (!doc->strings) { perror("Error: Failed to allocate strings pointer array"); krb_free_document(doc); return 0; }
        if (fseek(file, doc->header.string_offset, SEEK_SET) != 0) { perror("Error seeking to string data section"); krb_free_document(doc); return 0; }
        unsigned char stc_bytes[2];
        if (fread(stc_bytes, 1, 2, file) != 2) { fprintf(stderr, "Error: Failed to read string table count bytes\n"); krb_free_document(doc); return 0; }
        uint16_t string_table_count = (uint16_t)(stc_bytes[0] | (stc_bytes[1] << 8));
        if (string_table_count != doc->header.string_count) {
             fprintf(stderr, "Warning: String count in header (%u) differs from table count (%u). Using header count.\n", doc->header.string_count, string_table_count);
             if (string_table_count < doc->header.string_count) { fprintf(stderr,"Error: String table count is less than header count. Cannot proceed reliably.\n"); krb_free_document(doc); return 0; }
        }
        for (uint16_t i = 0; i < doc->header.string_count; i++) {
            uint8_t length;
            if (fread(&length, 1, 1, file) != 1) { fprintf(stderr, "Error: Failed to read string length for index %u\n", i); krb_free_document(doc); return 0; }
            doc->strings[i] = malloc(length + 1);
            if (!doc->strings[i]) { perror("Error: Failed to allocate memory for string"); fprintf(stderr, "Allocation failed for string index %u\n", i); krb_free_document(doc); return 0; }
            if (length > 0) {
                if (fread(doc->strings[i], 1, length, file) != length) { fprintf(stderr, "Error: Failed to read %u bytes for string data for index %u\n", length, i); krb_free_document(doc); return 0; }
            }
            doc->strings[i][length] = '\0';
        }
    }
    // --- END OF UNCHANGED STRINGS SECTION ---

    // TODO: Read Resources (Skipped for now)

    return 1; // Success!
}

// Frees all memory allocated within the KrbDocument structure.
void krb_free_document(KrbDocument* doc) {
    if (!doc) return;

    // --- Free Element Data (Properties and Events) --- // <<< MODIFIED
    if (doc->elements) { // Only proceed if elements were allocated
        for (uint16_t i = 0; i < doc->header.element_count; i++) {
            // Free properties for this element
            if (doc->properties && doc->properties[i]) { // Check properties pointer array AND specific element's array
                 for (uint8_t j = 0; j < doc->elements[i].property_count; j++) {
                     if (doc->properties[i][j].value) {
                         free(doc->properties[i][j].value);
                         // doc->properties[i][j].value = NULL;
                     }
                 }
                free(doc->properties[i]);
                // doc->properties[i] = NULL;
            }
            // Free events for this element // <<< NEW SECTION
            if (doc->events && doc->events[i]) { // Check events pointer array AND specific element's array
                // KrbEvent struct itself doesn't contain pointers, just need to free the array
                free(doc->events[i]);
                // doc->events[i] = NULL;
            }
        }
    }

    // Free the top-level pointer arrays
    if (doc->properties) {
        free(doc->properties);
        // doc->properties = NULL;
    }
    if (doc->events) { // <<< FREE EVENT POINTER ARRAY
        free(doc->events);
        // doc->events = NULL;
    }
    if (doc->elements) {
        free(doc->elements);
        // doc->elements = NULL;
    }


    // Free style properties (Unchanged Logic)
    if (doc->styles) {
        for (uint16_t i = 0; i < doc->header.style_count; i++) {
            if (doc->styles[i].properties) {
                for (uint8_t j = 0; j < doc->styles[i].property_count; j++) {
                    if (doc->styles[i].properties[j].value) {
                        free(doc->styles[i].properties[j].value);
                    }
                }
                free(doc->styles[i].properties);
            }
        }
        free(doc->styles);
        // doc->styles = NULL;
    }

    // Free strings (Unchanged Logic)
    if (doc->strings) {
        for (uint16_t i = 0; i < doc->header.string_count; i++) {
            if (doc->strings[i]) {
                free(doc->strings[i]);
            }
        }
        free(doc->strings);
        // doc->strings = NULL;
    }

    // TODO: Free animations if allocated
    // TODO: Free resources if allocated

    // Optional: Zero out header (Doesn't hurt)
    // memset(&doc->header, 0, sizeof(KrbHeader));
}