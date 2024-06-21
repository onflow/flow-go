#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Check if exactly two arguments are provided
if [ "$#" -ne 2 ]; then
    echo "Description: move checkpoint files from one directory to another"
    echo "Usage: $0 source_file destination_file"
    echo "Example: $0 /var/flow/from-folder/checkpoint.000010 /var/flow/to-folder/root.checkpoint"
    echo "The above command will move the checkpoint files including its 17 subfiles to the destination folder and rename them"

    exit 1
fi

# Assign arguments to variables
source_file_pattern=$1
destination_file_base=$2

# Extract the basename from the source file pattern
source_base=$(basename "$source_file_pattern")

# Extract the directory and base name from the destination file base
destination_directory=$(dirname "$destination_file_base")
destination_base=$(basename "$destination_file_base")

# Create the destination directory if it doesn't exist
mkdir -p "$destination_directory"

# Loop through the source files and move them to the destination directory
for file in "$source_file_pattern"*;
do
    if [ -f "$file" ]; then
        # Get the file extension (if any)
        extension="${file#$source_file_pattern}"
        # Construct the destination file name
        destination_file="$destination_directory/$destination_base$extension"
        # Move the file
        echo "Moved: $(realpath "$file") -> $(realpath "$destination_file")"

        echo "Moved: $file -> $destination_file"
    fi
done

echo "Checkpoint files are moved successfully."
