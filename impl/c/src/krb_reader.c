#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>   // For perror, strerror
#include <stdbool.h> // Add include for bool type
#include "krb.h"     // Include the header file

// --- Helper Functions ---

// Read little-endian uint16_t
uint16_t krb_read_u16_le(const void* data) {
    if (!data) return 0;
    const unsigned char* p = (const unsigned char*)data;
    return (uint16_t)(p[0] | (p[1] << 8));
}

// Read little-endian uint32_t
uint32_t krb_read_u32_le(const void* data) {
    if (!data) return 0;
    const unsigned char* p = (const unsigned char*)data;
    return (uint32_t)(p[0] | (p[1] << 8) | (p[2] << 16) | (p[3] << 24));
}


// --- Internal Read Helpers ---

// Reads the main file header (KRB v0.2 - 42 bytes)
static bool read_header_internal(FILE* file, KrbHeader* header) {
    unsigned char buffer[42]; // <-- Size updated to 42
    if (!file || !header) return false;
    if (fseek(file, 0, SEEK_SET) != 0) {
        perror("Error seeking to start of file"); return false;
    }
    size_t bytes_read = fread(buffer, 1, sizeof(buffer), file);
    if (bytes_read < sizeof(buffer)) {
        fprintf(stderr, "Error: Failed to read %zu-byte header, got %zu bytes\n", sizeof(buffer), bytes_read);
        return false;
    }

    // Parse header fields using helpers for endianness
    memcpy(header->magic, buffer + 0, 4);
    header->version         = krb_read_u16_le(buffer + 4);
    header->flags           = krb_read_u16_le(buffer + 6);
    header->element_count   = krb_read_u16_le(buffer + 8);
    header->style_count     = krb_read_u16_le(buffer + 10);
    header->animation_count = krb_read_u16_le(buffer + 12);
    header->string_count    = krb_read_u16_le(buffer + 14);
    header->resource_count  = krb_read_u16_le(buffer + 16);
    header->element_offset  = krb_read_u32_le(buffer + 18);
    header->style_offset    = krb_read_u32_le(buffer + 22);
    header->animation_offset= krb_read_u32_le(buffer + 26);
    header->string_offset   = krb_read_u32_le(buffer + 30);
    header->resource_offset = krb_read_u32_le(buffer + 34); // <-- Read new field
    header->total_size      = krb_read_u32_le(buffer + 38); // <-- Read moved field

    // Basic Validations
    if (memcmp(header->magic, "KRB1", 4) != 0) {
        fprintf(stderr, "Error: Invalid magic number. Expected 'KRB1', got '%.4s'\n", header->magic);
        return false;
    }
    // Allow reading slightly different 0.x versions but warn
    uint8_t major = (header->version & 0x00FF);
    uint8_t minor = (header->version >> 8);
    if (major != KRB_SPEC_VERSION_MAJOR || minor != KRB_SPEC_VERSION_MINOR) {
        fprintf(stderr, "Warning: KRB version mismatch. Expected %d.%d, got %d.%d. Parsing continues but may be unreliable.\n",
                KRB_SPEC_VERSION_MAJOR, KRB_SPEC_VERSION_MINOR, major, minor);
    }
    // Check reasonable offset values
    uint32_t min_offset = sizeof(KrbHeader);
    if (header->element_count > 0 && header->element_offset < min_offset) { fprintf(stderr, "Error: Element offset (%u) overlaps header (%u).\n", header->element_offset, min_offset); return false; }
    if (header->style_count > 0 && header->style_offset < min_offset) { fprintf(stderr, "Error: Style offset (%u) overlaps header.\n", header->style_offset); return false; }
    // Add checks for other offsets if needed...
    if (header->resource_count > 0 && header->resource_offset < min_offset) { fprintf(stderr, "Error: Resource offset (%u) overlaps header.\n", header->resource_offset); return false; }


    return true;
}


// Reads element header (17 bytes for v0.3)
static bool read_element_header_internal(FILE* file, KrbElementHeader* element) {
    if (!file || !element) {
        fprintf(stderr, "Error: NULL file or element pointer passed to read_element_header_internal.\n");
        return false;
    }

    // Read all 17 bytes directly into the struct
    // This relies on KrbElementHeader being defined with `#pragma pack(push, 1)` in krb.h
    size_t expected_size = sizeof(KrbElementHeader);
    if (expected_size != 17) {
         // Safety check in case the struct definition wasn't updated correctly
         fprintf(stderr, "Error: KrbElementHeader size mismatch! Expected 17, got %zu. Check krb.h definition and packing.\n", expected_size);
         return false;
    }

    size_t bytes_read = fread(element, 1, expected_size, file);
    if (bytes_read != expected_size) {
        fprintf(stderr, "Error: Failed to read %zu bytes for element header, got %zu.\n", expected_size, bytes_read);
        if (feof(file)) { fprintf(stderr, "  (End of file reached prematurely)\n"); }
        else if (ferror(file)) { perror("  (File read error)"); }
        return false;
    }

    // Correct endianness for multi-byte fields AFTER reading the whole struct buffer
    // Assumes krb_read_u16_le can correctly read from potentially unaligned memory
    // (which is generally okay on x86 but good practice to handle safely if needed).
    // It reads bytes directly, so alignment isn't usually an issue here.
    element->pos_x = krb_read_u16_le(&element->pos_x);
    element->pos_y = krb_read_u16_le(&element->pos_y);
    element->width = krb_read_u16_le(&element->width);
    element->height = krb_read_u16_le(&element->height);
    // Single byte fields (type, id, layout, style_id, counts) are already correct endianness

    return true;
}


// Reads a single property (unchanged internal logic)
static bool read_property_internal(FILE* file, KrbProperty* prop) {
    unsigned char buffer[3]; // ID(1)+Type(1)+Size(1)
    long prop_header_offset = ftell(file);
    if (fread(buffer, 1, 3, file) != 3) {
        fprintf(stderr, "Error: Failed reading property header @ %ld\n", prop_header_offset);
        prop->value = NULL; prop->size = 0; return false;
    }
    prop->property_id = buffer[0]; prop->value_type = buffer[1]; prop->size = buffer[2];
    prop->value = NULL;
    if (prop->size > 0) {
        prop->value = malloc(prop->size);
        if (!prop->value) { perror("malloc prop value"); return false; }
        if (fread(prop->value, 1, prop->size, file) != prop->size) {
            fprintf(stderr, "Error: Failed reading %u bytes prop value (ID 0x%02X) @ %ld\n", prop->size, prop->property_id, ftell(file) - prop->size);
            free(prop->value); prop->value = NULL; return false;
        }
    }
    return true;
}

// --- Public API Functions ---

// Reads the entire KRB document structure into memory.
bool krb_read_document(FILE* file, KrbDocument* doc) {
     if (!file || !doc) return false;
     memset(doc, 0, sizeof(KrbDocument)); // Clear doc structure

     // Read and validate header
     if (!read_header_internal(file, &doc->header)) {
         return false;
     }
     // Store parsed version components
     doc->version_major = (doc->header.version & 0x00FF);
     doc->version_minor = (doc->header.version >> 8);


     // Validate App element presence if flag is set
     if ((doc->header.flags & FLAG_HAS_APP) && doc->header.element_count > 0) {
         long original_pos = ftell(file); // Should be right after header
         if (fseek(file, doc->header.element_offset, SEEK_SET) != 0) { perror("seek App check"); krb_free_document(doc); return false; }
         unsigned char first_type;
         if (fread(&first_type, 1, 1, file) != 1) { fprintf(stderr, "read App check failed\n"); fseek(file, original_pos, SEEK_SET); krb_free_document(doc); return false; }
         if (first_type != ELEM_TYPE_APP) { fprintf(stderr, "Error: FLAG_HAS_APP set, but first elem type 0x%02X != 0x00\n", first_type); fseek(file, original_pos, SEEK_SET); krb_free_document(doc); return false; }
         if (fseek(file, original_pos, SEEK_SET) != 0) { perror("seek back App check"); krb_free_document(doc); return false; }
     }

     // --- Read Elements, Properties, and Events ---
     if (doc->header.element_count > 0) {
         if (doc->header.element_offset == 0) { fprintf(stderr, "Error: Zero element offset with non-zero count.\n"); krb_free_document(doc); return false; }

         // Allocate memory for element headers, property pointers, and event pointers
         doc->elements = calloc(doc->header.element_count, sizeof(KrbElementHeader));
         doc->properties = calloc(doc->header.element_count, sizeof(KrbProperty*));
         doc->events = calloc(doc->header.element_count, sizeof(KrbEventFileEntry*)); // Use file entry struct
         if (!doc->elements || !doc->properties || !doc->events) {
             perror("calloc elements/props/events ptrs"); krb_free_document(doc); return false;
         }

         if (fseek(file, doc->header.element_offset, SEEK_SET) != 0) {
              perror("seek element data"); krb_free_document(doc); return false;
         }

         for (uint16_t i = 0; i < doc->header.element_count; i++) {
             // Read element header
             if (!read_element_header_internal(file, &doc->elements[i])) {
                 fprintf(stderr, "Failed reading header elem %u\n", i); krb_free_document(doc); return false;
             }

             // Read properties
             doc->properties[i] = NULL; // Default to NULL
             if (doc->elements[i].property_count > 0) {
                 doc->properties[i] = calloc(doc->elements[i].property_count, sizeof(KrbProperty));
                 if (!doc->properties[i]) { perror("calloc props elem"); fprintf(stderr, "Elem %u\n", i); krb_free_document(doc); return false; }
                 for (uint8_t j = 0; j < doc->elements[i].property_count; j++) {
                     if (!read_property_internal(file, &doc->properties[i][j])) {
                          fprintf(stderr, "Failed reading prop %u elem %u\n", j, i); krb_free_document(doc); return false;
                     }
                 }
             }

             // Read Events
             doc->events[i] = NULL; // Default to NULL
             if (doc->elements[i].event_count > 0) {
                 doc->events[i] = calloc(doc->elements[i].event_count, sizeof(KrbEventFileEntry));
                 if (!doc->events[i]) { perror("calloc events elem"); fprintf(stderr, "Elem %u\n", i); krb_free_document(doc); return false; }
                 size_t events_to_read = doc->elements[i].event_count;
                 size_t events_read = fread(doc->events[i], sizeof(KrbEventFileEntry), events_to_read, file);
                 if (events_read != events_to_read) {
                     fprintf(stderr, "Error: Read %zu/%zu events elem %u\n", events_read, events_to_read, i); krb_free_document(doc); return false;
                 }
             }

             // Skip Animation Refs and Child Refs
             long bytes_to_skip = (long)doc->elements[i].animation_count * 2 // Anim Index(1)+Trigger(1)
                                + (long)doc->elements[i].child_count * 2;   // Child Offset(2)
             if (bytes_to_skip > 0) {
                  if (fseek(file, bytes_to_skip, SEEK_CUR) != 0) {
                      perror("seek skip refs"); fprintf(stderr, "Elem %u\n", i); krb_free_document(doc); return false;
                  }
             }
         } // End element loop
     }

     // --- Read Styles ---
     if (doc->header.style_count > 0) {
         if (doc->header.style_offset == 0) { fprintf(stderr, "Error: Zero style offset with non-zero count.\n"); krb_free_document(doc); return false; }
         doc->styles = calloc(doc->header.style_count, sizeof(KrbStyle));
         if (!doc->styles) { perror("calloc styles"); krb_free_document(doc); return false; }
         if (fseek(file, doc->header.style_offset, SEEK_SET) != 0) { perror("seek styles"); krb_free_document(doc); return false; }

         for (uint16_t i = 0; i < doc->header.style_count; i++) {
             unsigned char style_header_buf[3]; // ID(1)+NameIdx(1)+PropCount(1)
             if (fread(style_header_buf, 1, 3, file) != 3) { fprintf(stderr, "Failed read style header %u\n", i); krb_free_document(doc); return false; }
             doc->styles[i].id = style_header_buf[0]; // 1-based ID
             doc->styles[i].name_index = style_header_buf[1]; // 0-based index
             doc->styles[i].property_count = style_header_buf[2];
             doc->styles[i].properties = NULL;

             if (doc->styles[i].property_count > 0) {
                 doc->styles[i].properties = calloc(doc->styles[i].property_count, sizeof(KrbProperty));
                 if (!doc->styles[i].properties) { perror("calloc style props"); fprintf(stderr, "Style %u\n", i); krb_free_document(doc); return false; }
                 for (uint8_t j = 0; j < doc->styles[i].property_count; j++) {
                      if (!read_property_internal(file, &doc->styles[i].properties[j])) { fprintf(stderr, "Failed read prop %u style %u\n", j, i); krb_free_document(doc); return false; }
                 }
             }
         }
     }

     // --- Read Animations (Skipped) ---
     // TODO: Implement animation reading if needed

     // --- Read Strings ---
     if (doc->header.string_count > 0) {
         if (doc->header.string_offset == 0) { fprintf(stderr, "Error: Zero string offset with non-zero count.\n"); krb_free_document(doc); return false; }
         doc->strings = calloc(doc->header.string_count, sizeof(char*));
         if (!doc->strings) { perror("calloc strings ptrs"); krb_free_document(doc); return false; }
         if (fseek(file, doc->header.string_offset, SEEK_SET) != 0) { perror("seek strings"); krb_free_document(doc); return false; }

         unsigned char stc_bytes[2]; // Read count from table start
         if (fread(stc_bytes, 1, 2, file) != 2) { fprintf(stderr, "Failed read string table count\n"); krb_free_document(doc); return false; }
         uint16_t table_count = krb_read_u16_le(stc_bytes);
         if (table_count != doc->header.string_count) { fprintf(stderr, "Warning: Header string count %u != table count %u\n", doc->header.string_count, table_count); }
         // We trust the header count for allocation, but table count could be used for read loop bounds if preferred

         for (uint16_t i = 0; i < doc->header.string_count; i++) {
             uint8_t length;
             if (fread(&length, 1, 1, file) != 1) { fprintf(stderr, "Failed read str len %u\n", i); krb_free_document(doc); return false; }
             doc->strings[i] = malloc(length + 1); // +1 for null terminator
             if (!doc->strings[i]) { perror("malloc string"); fprintf(stderr, "String %u\n", i); krb_free_document(doc); return false; }
             if (length > 0) {
                 if (fread(doc->strings[i], 1, length, file) != length) { fprintf(stderr, "Failed read %u bytes str %u\n", length, i); krb_free_document(doc); return false; }
             }
             doc->strings[i][length] = '\0'; // Null terminate
         }
     }

     // --- Read Resources --- // <<< NEW SECTION
     if (doc->header.resource_count > 0) {
         if (doc->header.resource_offset == 0) { fprintf(stderr, "Error: Zero resource offset with non-zero count.\n"); krb_free_document(doc); return false; }
         doc->resources = calloc(doc->header.resource_count, sizeof(KrbResource));
         if (!doc->resources) { perror("calloc resources"); krb_free_document(doc); return false; }
         if (fseek(file, doc->header.resource_offset, SEEK_SET) != 0) { perror("seek resources"); krb_free_document(doc); return false; }

         unsigned char resc_bytes[2]; // Read count from table start
         if (fread(resc_bytes, 1, 2, file) != 2) { fprintf(stderr, "Failed read resource table count\n"); krb_free_document(doc); return false; }
         uint16_t table_res_count = krb_read_u16_le(resc_bytes);
         if (table_res_count != doc->header.resource_count) { fprintf(stderr, "Warning: Header resource count %u != table count %u\n", doc->header.resource_count, table_res_count); }

         for (uint16_t i = 0; i < doc->header.resource_count; i++) {
             unsigned char res_entry_buf[4]; // Buffer for external format: Type(1)+NameIdx(1)+Format(1)+DataIdx(1)
             if (fread(res_entry_buf, 1, 4, file) != 4) {
                 fprintf(stderr, "Error: Failed read resource entry %u\n", i);
                 krb_free_document(doc); return false;
             }
             doc->resources[i].type = res_entry_buf[0];
             doc->resources[i].name_index = res_entry_buf[1]; // 0-based string index
             doc->resources[i].format = res_entry_buf[2];
             // Assuming external format for now based on spec v0.2 and compiler impl
             if (doc->resources[i].format == RES_FORMAT_EXTERNAL) {
                 doc->resources[i].data_string_index = res_entry_buf[3]; // 0-based string index
             } else if (doc->resources[i].format == RES_FORMAT_INLINE) {
                 fprintf(stderr, "Error: Inline resource parsing not yet implemented (Res %u).\n", i);
                 // TODO: Read size(2 bytes), read data(N bytes)
                 krb_free_document(doc); return false;
             } else {
                 fprintf(stderr, "Error: Unknown resource format 0x%02X for resource %u\n", doc->resources[i].format, i);
                 krb_free_document(doc); return false;
             }
         }
     } // <<< END NEW RESOURCE SECTION

     return true; // Success!
}

// Frees all memory allocated within the KrbDocument structure.
void krb_free_document(KrbDocument* doc) {
    if (!doc) return;

    // Free Element Data (Properties and Events)
    if (doc->elements) {
        for (uint16_t i = 0; i < doc->header.element_count; i++) {
            // Free properties
            if (doc->properties && doc->properties[i]) {
                 for (uint8_t j = 0; j < doc->elements[i].property_count; j++) {
                     if (doc->properties[i][j].value) {
                         free(doc->properties[i][j].value);
                     }
                 }
                free(doc->properties[i]);
            }
            // Free events
            if (doc->events && doc->events[i]) {
                free(doc->events[i]);
            }
        }
    }
    // Free top-level pointer arrays
    if (doc->properties) free(doc->properties);
    if (doc->events) free(doc->events);
    if (doc->elements) free(doc->elements);

    // Free Styles
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
    }

    // Free Strings
    if (doc->strings) {
        for (uint16_t i = 0; i < doc->header.string_count; i++) {
            if (doc->strings[i]) {
                free(doc->strings[i]);
            }
        }
        free(doc->strings);
    }

    // Free Resources // <<< NEW SECTION
    if (doc->resources) {
        // Currently KrbResource struct itself doesn't hold allocated data (paths are refs into strings array)
        // If inline resources were implemented, their data would need freeing here.
        free(doc->resources);
    } // <<< END NEW RESOURCE SECTION

    // TODO: Free animations if allocated

    // Optional: Zero out the doc struct itself after freeing members
    // memset(doc, 0, sizeof(KrbDocument));
}