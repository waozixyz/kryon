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
    element->event_count = buffer[14];
    element->animation_count = buffer[15];
    // printf("DEBUG: Read element - type=0x%02X, style_id=%d, width=%d, height=%d, props=%d, children=%d\n",
    //        element->type, element->style_id, element->width, element->height, element->property_count, element->child_count);
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

        // Optional: Handle endianness for multi-byte types *if needed* here.
        // Assumes compiler and reader have same endianness for now, or that
        // the renderer will handle LE interpretation of the byte buffer.
        // Example for short:
        // if (prop->value_type == 0x02 && prop->size == 2) {
        //     uint8_t* bytes = (uint8_t*)prop->value;
        //     *(uint16_t*)prop->value = (uint16_t)(bytes[0] | (bytes[1] << 8)); // Assuming LE target
        // }

    } else {
        prop->value = NULL; // No value data if size is 0
    }
    // printf("DEBUG: Read property - id=0x%02X, type=0x%02X, size=%d\n", prop->property_id, prop->value_type, prop->size);
    return 1; // Indicate success
}


// Reads the main file header. Returns 1 on success, 0 on failure.
int read_header(FILE* file, KrbHeader* header) {
    unsigned char buffer[38]; // Size of KrbHeader

    if (!file || !header) return 0;

    // Ensure we are at the beginning of the file
    if (fseek(file, 0, SEEK_SET) != 0) {
        perror("Error seeking to start of file");
        return 0;
    }

    // Read the header data
    size_t bytes_read = fread(buffer, 1, sizeof(buffer), file);
    if (bytes_read < sizeof(buffer)) {
        fprintf(stderr, "Error: Failed to read %zu-byte header, got %zu bytes\n", sizeof(buffer), bytes_read);
        return 0;
    }

    // --- Populate Header struct from buffer (Little Endian) ---
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
    // --- End Population ---

    // Validate Magic and Version
    if (memcmp(header->magic, "KRB1", 4) != 0) {
        fprintf(stderr, "Error: Invalid magic number. Expected 'KRB1', got '%.4s'\n", header->magic);
        return 0;
    }
     if (header->version != 0x0001) { // Check for 1.0 (LE 01 00)
         fprintf(stderr, "Error: Unsupported KRB version 0x%04X\n", header->version);
        return 0;
    }
     // Basic sanity checks on offsets and counts
     if (header->element_offset < sizeof(KrbHeader) && header->element_count > 0) {
         fprintf(stderr, "Error: Element offset (%u) overlaps header.\n", header->element_offset); return 0;
     }
     // Add more checks as needed (e.g., style_offset >= element_offset + element_data_size)


    return 1; // Success
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
    if ((doc->header.flags & (1 << 6)) && doc->header.element_count > 0) {
        // Temporarily seek, read first byte, then seek back
        long original_pos = ftell(file);
        if (fseek(file, doc->header.element_offset, SEEK_SET) != 0) {
            perror("Error seeking to element offset for App check"); return 0;
        }
        unsigned char first_type;
        if (fread(&first_type, 1, 1, file) != 1) {
            fprintf(stderr, "Error: Failed to read first element type for App check\n");
             fseek(file, original_pos, SEEK_SET); // Try to restore position
            return 0;
        }
        if (first_type != 0x00) {
            fprintf(stderr, "Error: Header flag indicates App element, but first element is type 0x%02X, not 0x00\n", first_type);
             fseek(file, original_pos, SEEK_SET); // Try to restore position
            return 0;
        }
        // Seek back to where we were after reading the header
        if (fseek(file, original_pos, SEEK_SET) != 0) {
             perror("Error seeking back after App check"); return 0;
        }
    }

    // --- Read Elements and their Properties ---
    if (doc->header.element_count > 0) {
        if (doc->header.element_offset < sizeof(KrbHeader)) { /* Already checked in read_header */ return 0; }

        // Allocate memory for element headers and property array pointers
        doc->elements = calloc(doc->header.element_count, sizeof(KrbElementHeader));
        doc->properties = calloc(doc->header.element_count, sizeof(KrbProperty*)); // Array of *pointers*
        if (!doc->elements || !doc->properties) {
            perror("Error: Failed to allocate memory for elements or properties pointers");
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

            // Read properties for this element
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

            // *** Calculate and skip over Event, Animation, and Child references ***
            long bytes_to_skip = 0;
            // Spec: Events section comes after Properties
            bytes_to_skip += (long)doc->elements[i].event_count * 2; // Size: Type(1) + CallbackID(1) = 2
            // Spec: Animation Refs section comes after Events
            bytes_to_skip += (long)doc->elements[i].animation_count * 2; // Size: Index(1) + Trigger(1) = 2
            // Spec: Child Refs section comes after Animation Refs
            bytes_to_skip += (long)doc->elements[i].child_count * 2; // Size: Child Offset(2) = 2

            if (bytes_to_skip > 0) {
                 // printf("DEBUG: Element %u skipping %ld bytes for refs (E:%u, A:%u, C:%u)\n",
                 //        i, bytes_to_skip, doc->elements[i].event_count, doc->elements[i].animation_count, doc->elements[i].child_count);
                 if (fseek(file, bytes_to_skip, SEEK_CUR) != 0) {
                     perror("Error skipping reference data after element properties");
                     fprintf(stderr, "Error occurred after element index %u\n", i);
                     krb_free_document(doc); return 0;
                 }
            }
        } // End loop through elements
    }

    // --- Read Styles and their Properties ---
    if (doc->header.style_count > 0) {
         if (doc->header.style_offset < (doc->header.element_offset + (/* need total element size here - complex */ 0))) {
             // Basic check: must be after header
             if (doc->header.style_offset < sizeof(KrbHeader)) {
                  fprintf(stderr, "Error: Style offset (%u) overlaps header.\n", doc->header.style_offset);
                  krb_free_document(doc); return 0;
             }
             // A better check requires knowing the total size of the element section.
         }

        doc->styles = calloc(doc->header.style_count, sizeof(KrbStyle));
        if (!doc->styles) {
            perror("Error: Failed to allocate memory for styles");
            krb_free_document(doc); return 0;
        }

        if (fseek(file, doc->header.style_offset, SEEK_SET) != 0) {
            perror("Error seeking to style data section");
            krb_free_document(doc); return 0;
        }

        for (uint16_t i = 0; i < doc->header.style_count; i++) {
            unsigned char style_header_buf[3]; // ID(1) + NameIndex(1) + PropCount(1)
            if (fread(style_header_buf, 1, 3, file) != 3) {
                fprintf(stderr, "Error: Failed to read header for style index %u\n", i);
                krb_free_document(doc); return 0;
            }
            doc->styles[i].id = style_header_buf[0];
            doc->styles[i].name_index = style_header_buf[1];
            doc->styles[i].property_count = style_header_buf[2];
            doc->styles[i].properties = NULL; // Initialize

            // printf("DEBUG: Read style %u - id=%u, name_index=%u, props=%u\n",
            //        i, doc->styles[i].id, doc->styles[i].name_index, doc->styles[i].property_count);

            if (doc->styles[i].property_count > 0) {
                doc->styles[i].properties = calloc(doc->styles[i].property_count, sizeof(KrbProperty));
                 if (!doc->styles[i].properties) {
                    perror("Error: Failed to allocate style property array");
                    fprintf(stderr, "Allocation failed for style index %u\n", i);
                    krb_free_document(doc); return 0;
                }
                for (uint8_t j = 0; j < doc->styles[i].property_count; j++) {
                     if (!read_property_internal(file, &doc->styles[i].properties[j])) {
                         fprintf(stderr, "Failed reading property %u for style index %u\n", j, i);
                         krb_free_document(doc); return 0;
                    }
                }
            }
        } // End loop through styles
    }

    // TODO: Read Animations

    // --- Read Strings ---
    if (doc->header.string_count > 0) {
         if (doc->header.string_offset < sizeof(KrbHeader)) { /* Add better check */
              fprintf(stderr, "Error: String offset (%u) overlaps header.\n", doc->header.string_offset);
              krb_free_document(doc); return 0;
         }

        doc->strings = calloc(doc->header.string_count, sizeof(char*));
        if (!doc->strings) {
             perror("Error: Failed to allocate strings pointer array");
             krb_free_document(doc); return 0;
        }

        if (fseek(file, doc->header.string_offset, SEEK_SET) != 0) {
             perror("Error seeking to string data section");
             krb_free_document(doc); return 0;
        }

        // Read string table count (LE format)
        unsigned char stc_bytes[2];
        if (fread(stc_bytes, 1, 2, file) != 2) {
            fprintf(stderr, "Error: Failed to read string table count bytes\n");
            krb_free_document(doc); return 0;
        }
        uint16_t string_table_count = (uint16_t)(stc_bytes[0] | (stc_bytes[1] << 8)); // LE

        // Validate count consistency
        if (string_table_count != doc->header.string_count) {
             fprintf(stderr, "Warning: String count in header (%u) differs from table count (%u). Using header count.\n",
                    doc->header.string_count, string_table_count);
             // Trust header count for allocation, but be careful reading
             if (string_table_count < doc->header.string_count) {
                  fprintf(stderr,"Error: String table count is less than header count. Cannot proceed reliably.\n");
                  krb_free_document(doc); return 0;
             }
             // If table count is more, we might read too far, but using header count for loop is safer.
        }

        for (uint16_t i = 0; i < doc->header.string_count; i++) { // Loop using header count
            uint8_t length;
            if (fread(&length, 1, 1, file) != 1) {
                fprintf(stderr, "Error: Failed to read string length for index %u\n", i);
                krb_free_document(doc); return 0;
            }

            doc->strings[i] = malloc(length + 1); // Allocate space for string + null terminator
            if (!doc->strings[i]) {
                 perror("Error: Failed to allocate memory for string");
                 fprintf(stderr, "Allocation failed for string index %u\n", i);
                 krb_free_document(doc); return 0;
            }

            if (length > 0) {
                if (fread(doc->strings[i], 1, length, file) != length) {
                    fprintf(stderr, "Error: Failed to read %u bytes for string data for index %u\n", length, i);
                    // free(doc->strings[i]); // krb_free_document will handle this
                    // doc->strings[i] = NULL;
                    krb_free_document(doc); return 0;
                }
            }
            doc->strings[i][length] = '\0'; // Ensure null termination

            // printf("DEBUG: Read string %u - length=%u, value='%s'\n", i, length, doc->strings[i]);
        } // End loop through strings
    }

    // TODO: Read Resources

    // Final check: ensure we didn't read past the reported total size?
    // long current_pos = ftell(file);
    // if (current_pos > doc->header.total_size) {
    //     fprintf(stderr, "Warning: Read past reported total file size (%ld > %u)\n", current_pos, doc->header.total_size);
    // }


    return 1; // Success!
}

// Frees all memory allocated within the KrbDocument structure.
void krb_free_document(KrbDocument* doc) {
    if (!doc) return;

    // Free element properties
    if (doc->properties) {
        for (uint16_t i = 0; i < doc->header.element_count; i++) {
            if (doc->properties[i]) { // Check if property array for this element was allocated
                 if (doc->elements) { // Check if elements array exists before accessing its counts
                     for (uint8_t j = 0; j < doc->elements[i].property_count; j++) {
                         if (doc->properties[i][j].value) {
                             free(doc->properties[i][j].value);
                             // doc->properties[i][j].value = NULL; // Good practice
                         }
                     }
                 }
                free(doc->properties[i]);
                // doc->properties[i] = NULL; // Good practice
            }
        }
        free(doc->properties);
        // doc->properties = NULL; // Good practice
    }

     // Free element headers array
     if (doc->elements) {
         free(doc->elements);
         // doc->elements = NULL; // Good practice
     }


    // Free style properties
    if (doc->styles) {
        for (uint16_t i = 0; i < doc->header.style_count; i++) {
            if (doc->styles[i].properties) {
                for (uint8_t j = 0; j < doc->styles[i].property_count; j++) {
                    if (doc->styles[i].properties[j].value) {
                        free(doc->styles[i].properties[j].value);
                        // doc->styles[i].properties[j].value = NULL;
                    }
                }
                free(doc->styles[i].properties);
                // doc->styles[i].properties = NULL;
            }
        }
        free(doc->styles);
        // doc->styles = NULL;
    }

    // Free strings
    if (doc->strings) {
        for (uint16_t i = 0; i < doc->header.string_count; i++) {
            if (doc->strings[i]) {
                free(doc->strings[i]);
                // doc->strings[i] = NULL;
            }
        }
        free(doc->strings);
        // doc->strings = NULL;
    }

     // TODO: Free animations
     // TODO: Free resources

    // Optional: Zero out header, although doc itself is usually on stack or caller manages it
    // memset(&doc->header, 0, sizeof(KrbHeader));
}