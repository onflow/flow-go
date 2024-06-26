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
source_directory=$(dirname "$source_file_pattern")
destination_directory=$(dirname "$destination_file_base")
destination_base=$(basename "$destination_file_base")

# Create the destination directory if it doesn't exist
mkdir -p "$destination_directory"

# Define the expected files
expected_files=(
    "$source_base"
    "$source_base.001"
    "$source_base.002"
    "$source_base.003"
    "$source_base.004"
    "$source_base.005"
    "$source_base.006"
    "$source_base.007"
    "$source_base.008"
    "$source_base.009"
    "$source_base.010"
    "$source_base.011"
    "$source_base.012"
    "$source_base.013"
    "$source_base.014"
    "$source_base.015"
    "$source_base.016"
)

# Check if all expected files are present
missing_files=()
for expected_file in "${expected_files[@]}"; do
    full_expected_file="$source_directory/$expected_file"
    if [ ! -f "$full_expected_file" ]; then
        missing_files+=("$full_expected_file")
    fi
done

if [ "${#missing_files[@]}" -ne 0 ]; then
    echo "Error: The following expected files are missing:"
    for file in "${missing_files[@]}"; do
        echo "  $file"
    done
    exit 1
fi

# Loop through the expected files and preview/move them to the destination directory
for file in "${expected_files[@]}"; do
    full_source_file="$source_directory/$file"
    if [ -f "$full_source_file" ]; then
        # Get the file extension (if any)
        extension="${file#$source_base}"
        # Construct the destination file name
        destination_file="$destination_directory/$destination_base$extension"
        # Preview or move the file
        if [ "$run_mode" = true ]; then
            echo "Moving: $(realpath "$full_source_file") -> $(realpath "$destination_file")"
            mv "$full_source_file" "$destination_file"
        else
            echo "Preview: $(realpath "$full_source_file") -> $(realpath "$destination_file")"
        fi
    fi
done


if [ "$run_mode" = true ]; then
    echo "Checkpoint files have been moved successfully."
else
    echo "Preview complete. No files have been moved. add --run flag to move the files."
fi
