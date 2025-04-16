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

WORKER_COUNTS=(0 1 2 3 4 5 6 7 8 9 10) # 0 means auto

HYPERFINE_CMD="hyperfine --warmup 1 --runs 10"

for WORKERS in "${WORKER_COUNTS[@]}"; do
    HYPERFINE_CMD+=" 'mkbrr create \"$FILE_PATH\" --workers $WORKERS'"
done

eval "$HYPERFINE_CMD"

echo "Benchmarking complete."