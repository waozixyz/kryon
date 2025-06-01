#!/usr/bin/env bash

# Script to compile all .kry files in the 'examples' directory to .krb
# Assumes 'kryc' is in your PATH or you provide the full path to it.

KRYC_COMMAND="kryc" # Or "/path/to/your/kryc"
# Assumes the script is run from the root of the repository,
# and the examples are in a subdirectory named "examples".
EXAMPLES_DIR="examples"

# Check if the examples directory exists
if [ ! -d "$EXAMPLES_DIR" ]; then
  echo "Error: Directory '$EXAMPLES_DIR' not found."
  echo "Please ensure you are running this script from the root of the repository,"
  echo "and the examples directory exists at './examples/'."
  exit 1
fi

# Check if kryc is available
if ! command -v "$KRYC_COMMAND" &> /dev/null; then
    echo "Error: '$KRYC_COMMAND' command not found."
    echo "Please ensure it's in your PATH or update the KRYC_COMMAND variable in this script."
    exit 1
fi

echo "Searching for .kry files in '$EXAMPLES_DIR' and its subdirectories..."
echo "Using compiler: '$KRYC_COMMAND'"

# Initialize a counter for successful compilations
success_count=0
error_count=0
processed_any_files=false # Flag to check if any .kry files were found

# Find all .kry files in the specified directory and its subdirectories
# -print0 and xargs -0 handle filenames with spaces or special characters
find "$EXAMPLES_DIR" -type f -name "*.kry" -print0 | while IFS= read -r -d $'\0' kry_file; do
  processed_any_files=true # Mark that we found at least one file
  # Get the directory and base name of the .kry file
  dir_name=$(dirname "$kry_file")
  base_name=$(basename "$kry_file" .kry)
  
  # Construct the output .krb file path
  krb_file="${dir_name}/${base_name}.krb"
  
  echo "----------------------------------------"
  echo "Compiling: '$kry_file'"
  echo "Outputting to: '$krb_file'"
  
  # Run the kryc compiler
  # CORRECTED: Assumes: kryc <input.kry> <output.krb>
  if "$KRYC_COMMAND" "$kry_file" "$krb_file"; then
    echo "Successfully compiled '$kry_file' to '$krb_file'"
    success_count=$((success_count + 1))
  else
    # kryc already prints its usage message on error, so we don't need to repeat it.
    echo "ERROR compiling '$kry_file'. Check kryc output above."
    error_count=$((error_count + 1))
    # To stop on the first error, uncomment the next line:
    # exit 1 
  fi
done

echo "----------------------------------------"
if [ "$error_count" -gt 0 ]; then
  echo "Compilation process finished with $error_count error(s)."
  echo "$success_count file(s) compiled successfully."
  exit 1 # Exit with an error code if any compilation failed
else
  if [ "$processed_any_files" = false ]; then # Check if we actually processed any files
    echo "No .kry files found to compile in '$EXAMPLES_DIR'."
  else
    echo "All $success_count .kry file(s) processed successfully."
  fi
  exit 0
fi