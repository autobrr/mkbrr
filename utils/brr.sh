#!/bin/bash

# Script to benchmark mkbrr with different number of workers

FILE_PATH="/path/to/source/data"


WORKER_COUNTS=(1 2 3 4 5 6 7 8)


COMMANDS=("hyperfine" "--warmup" "1" "--runs" "5")


for WORKERS in "${WORKER_COUNTS[@]}"; do
  COMMAND="mkbrr create '$FILE_PATH' --workers $WORKERS"
  COMMANDS+=("$COMMAND")
done

"${COMMANDS[@]}"

echo "Benchmarking complete."