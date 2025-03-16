# Kryon Project
A binary UI format with multi-language implementations.

## Structure
- `docs/`: Specifications
- `examples/`: Sample .kry and .krb files
- `implementations/`: Language-specific readers and renderers
- `tools/`: Utilities (e.g., kry to krb compiler)

## Build and Run
### Compiler
gcc -o kry_compiler tools/src/kry_compiler.c
./kry_compiler examples/hello_world.kry examples/hello_world.krb

### C Reader
gcc -o krb_reader implementations/c/src/readers/krb_reader.c
./krb_reader examples/hello_world.krb
