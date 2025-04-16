#!/bin/bash

# Script to benchmark mkbrr with different number of workers

if [ $# -eq 0 ]; then
    echo "Error: No file path provided"
    echo "Usage: $0 <file_path>"
    exit 1
fi

FILE_PATH="$1"

if [ ! -d "$FILE_PATH" ]; then
    echo "Error: Directory '$FILE_PATH' does not exist"
    exit 1
fi

# Should be set to test the amount of cores on the machine 
# Generate sequence from 0 to 32 (0 will use the built-in worker count logic)
WORKER_COUNTS=(0 $(seq 1 32))

HYPERFINE_CMD="hyperfine --warmup 1 --runs 10"

for WORKERS in "${WORKER_COUNTS[@]}"; do
    HYPERFINE_CMD+=" 'mkbrr create \"$FILE_PATH\" --workers $WORKERS'"
done

eval "$HYPERFINE_CMD"

echo "Benchmarking complete."