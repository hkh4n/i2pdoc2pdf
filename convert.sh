#!/bin/bash

# Set IFS to handle filenames with spaces
IFS=$'\n'

# Function to check if file is HTML and add extension if needed
process_file() {
    local file="$1"
    
    # Skip if file already ends in .html or .htm
    if [[ "$file" =~ \.(html|htm)$ ]]; then
        return
    fi
    
    # Check file type using magic bytes
    filetype=$(file -b --mime-type "$file")
    
    if [[ "$filetype" == "text/html" ]]; then
        echo "Found HTML file: $file"
        
        # Create new filename with .html extension
        new_name="${file}.html"
        
        # Check if target filename already exists
        if [ -e "$new_name" ]; then
            echo "Warning: Cannot rename '$file' to '$new_name' - target file already exists"
            return
        fi
        
        # Rename the file
        mv -v "$file" "$new_name"
    fi
}

# Check if directory is provided
if [ "$#" -eq 1 ]; then
    search_dir="$1"
else
    search_dir="."
fi

# Verify directory exists
if [ ! -d "$search_dir" ]; then
    echo "Error: Directory '$search_dir' not found"
    exit 1
fi

# Find all regular files and process them
find "$search_dir" -type f -print0 | while IFS= read -r -d '' file; do
    process_file "$file"
done

echo "Processing complete!"
