#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Check if exactly two arguments are provided
usage() {
    echo "Description: move checkpoint files from one directory to another"
    echo "Usage: $0 source_file destination_file [--run]"
    echo "Example: $0 /var/flow/from-folder/checkpoint.000010 /var/flow/to-folder/root.checkpoint [--run]"
    echo "The above command will preview the checkpoint files to be moved including its 17 subfiles to the destination folder and rename them."
    echo "Preview mode is default. Use --run to actually move the files."
    exit 1
}

# Check if at least two arguments are provided
if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    usage
fi

# Assign arguments to variables
source_file_pattern=$1
destination_file_base=$2
run_mode=false

# Check for run mode
if [ "$#" -eq 3 ] && [ "$3" == "--run" ]; then
    run_mode=true
elif [ "$#" -eq 3 ]; then
    usage
fi

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

        # Preview or move the file
        if [ "$run_mode" = true ]; then
            echo "Moving: $(realpath "$file") -> $(realpath "$destination_file")"
            mv "$file" "$destination_file"
        else
            echo "Preview: $(realpath "$file") -> $(realpath "$destination_file")"
        fi
    fi
done


if [ "$run_mode" = true ]; then
    echo "Checkpoint files have been moved successfully."
else
    echo "Preview complete. No files have been moved. add --run flag to move the files."
fi
