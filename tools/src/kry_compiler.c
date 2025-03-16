#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// Hardcoded for simplicity: matches the "Hello World" .krb
void write_krb(FILE* output) {
    // File Header (32 bytes, Small File Mode)
    uint8_t header[] = {
        0x4B, 0x52, 0x42, 0x31, // Magic: "KRB1"
        0x01, 0x00,             // Version: 1.0
        0x00, 0x00,             // Flags: 0 (Small File Mode)
        0x01, 0x00,             // Element Count: 1
        0x00, 0x00,             // Style Count: 0
        0x00, 0x00,             // Animation Count: 0
        0x01, 0x00,             // String Count: 1
        0x00, 0x00,             // Resource Count: 0
        0x20, 0x00, 0x00, 0x00, // Element Offset: 32
        0x00, 0x00, 0x00, 0x00, // Style Offset: 0
        0x00, 0x00, 0x00, 0x00, // Animation Offset: 0
        0x3C, 0x00, 0x00, 0x00, // String Offset: 60
        0x00, 0x00, 0x00, 0x00, // Resource Offset: 0
        0x47, 0x00              // Total Size: 71 bytes
    };
    fwrite(header, sizeof(header), 1, output);

    // Element Block (19 bytes)
    uint8_t element[] = {
        0x02,                   // Type: Text
        0x00,                   // ID: 0 (none)
        0x0A, 0x00,             // Position X: 10
        0x0A, 0x00,             // Position Y: 10
        0x64, 0x00,             // Width: 100
        0x14, 0x00,             // Height: 20
        0x40,                   // Layout: Absolute (Bit 6 = 1)
        0x00,                   // Style ID: 0
        0x01,                   // Property Count: 1
        0x00,                   // Child Count: 0
        0x00,                   // Event Count: 0
        0x00,                   // Animation Count: 0
        0x08,                   // Property ID: TextContent
        0x04,                   // Value Type: String
        0x01,                   // Size: 1 byte
        0x00                    // Value: String Index 0
    };
    fwrite(element, sizeof(element), 1, output);

    // String Table (20 bytes)
    uint8_t string_table[] = {
        0x01, 0x00,             // String Count: 1
        0x0B,                   // Length: 11 bytes
        0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x20, 0x57, 0x6F, 0x72, 0x6C, 0x64, // "Hello World"
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 // Padding to 20 bytes
    };
    fwrite(string_table, sizeof(string_table), 1, output);
}

// Basic validation of .kry file (stub for now)
int validate_kry(FILE* input) {
    char buffer[256];
    if (!fgets(buffer, sizeof(buffer), input)) {
        printf("Error: Empty or invalid .kry file\n");
        return 0;
    }
    // For this simple example, just check if it starts with "text {"
    if (strncmp(buffer, "text {", 6) != 0) {
        printf("Error: Expected 'text {' at start of .kry file\n");
        return 0;
    }
    return 1;
}

int main(int argc, char* argv[]) {
    if (argc != 3) {
        printf("Usage: %s <input.kry> <output.krb>\n", argv[0]);
        return 1;
    }

    FILE* input = fopen(argv[1], "r");
    if (!input) {
        printf("Error: Could not open input file %s\n", argv[1]);
        return 1;
    }

    FILE* output = fopen(argv[2], "wb");
    if (!output) {
        printf("Error: Could not open output file %s\n", argv[2]);
        fclose(input);
        return 1;
    }

    // Validate .kry file (stub for now)
    if (!validate_kry(input)) {
        fclose(input);
        fclose(output);
        return 1;
    }

    // Write hardcoded .krb output
    write_krb(output);

    printf("Compiled %s to %s successfully\n", argv[1], argv[2]);

    fclose(input);
    fclose(output);
    return 0;
}
