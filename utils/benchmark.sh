#!/bin/bash

# Script to benchmark mkbrr with different number of workers and collect performance metrics

# --- Basic System Info ---
OS_TYPE=$(uname -s)
CPU_MODEL="Unknown"
if [[ "$OS_TYPE" == "Darwin" ]]; then
    CPU_MODEL=$(sysctl -n machdep.cpu.brand_string)
elif [[ "$OS_TYPE" == "Linux" ]]; then
    CPU_MODEL=$(grep 'Model name' /proc/cpuinfo | head -n 1 | awk -F': ' '{print $2}')
fi
# --- End System Info ---

# --- Function to get drive type ---
get_drive_type() {
    local target_path=$1
    local drive_type="Unknown"
    local mount_point device

    # Find the mount point for the target path
    if [[ "$OS_TYPE" == "Darwin" ]]; then
        mount_point=$(df "$target_path" | awk 'NR==2 {print $9}')
        device=$(df "$target_path" | awk 'NR==2 {print $1}')
        # Use diskutil to check if it's Solid State
        if diskutil info "$device" | grep -q "Solid State:.*Yes"; then
            drive_type="SSD"
        elif diskutil info "$device" | grep -q "Solid State:.*No"; then
            # Check if it reports rotational rate for HDDs
            if diskutil info "$device" | grep -q "Device Block Size"; then # A crude check, assumes HDDs have blocks reported differently sometimes
                drive_type="HDD"
            fi
        fi
        # Fallback using system_profiler if diskutil was inconclusive
        if [[ "$drive_type" == "Unknown" ]]; then
            storage_info=$(system_profiler SPStorageDataType -xml 2>/dev/null)
            physical_drive=$(echo "$storage_info" | plutil -extract "SPStorageDataType"."$mount_point"."physical_drive"."com.apple.corestorage.pv" xml1 -o - - | plutil -p - | head -n 1 | awk '{print $NF}' | tr -d '"')
            if [[ -n "$physical_drive" ]]; then
                medium_type=$(echo "$storage_info" | plutil -extract "SPStorageDataType"."$physical_drive"."physical_drive"."medium_type" xml1 -o - - | plutil -p - | awk '{print $NF}' | tr -d '"')
                if [[ "$medium_type" == "ssd" ]]; then
                    drive_type="SSD"
                elif [[ "$medium_type" == "hdd" || "$medium_type" == "rotational" ]]; then
                    drive_type="HDD"
                fi
            fi
        fi
    elif [[ "$OS_TYPE" == "Linux" ]]; then
        device=$(df "$target_path" | awk 'NR==2 {print $1}')
        # lsblk might show partition, find the parent device
        base_device=$(lsblk -no pkname "$device" 2>/dev/null)
        if [[ -z "$base_device" ]]; then
            # If no parent, it might be the device itself (e.g., /dev/nvme0n1)
            base_device_path="/dev/$device"
        else
            base_device_path="/dev/$base_device"
        fi

        if [[ -e "$base_device_path" ]]; then
            # Check rotational status (0: SSD/NVMe, 1: HDD)
            rota=$(lsblk -d -no ROTA "$base_device_path" 2>/dev/null | head -n 1 | awk '{print $1}')
            if [[ "$rota" == "0" ]]; then
                drive_type="SSD/NVMe"
            elif [[ "$rota" == "1" ]]; then
                drive_type="HDD"
            fi
        else
            # Fallback if device path check fails
            drive_type="Unknown (Linux path error)"
        fi
    else
        drive_type="Unknown (Unsupported OS for drive type detection)"
    fi
    echo "$drive_type"
}
# --- End Drive Type Function ---

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

# Create results directory
RESULTS_DIR="benchmark_results_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

# --- Save System Info ---
echo "OS: $OS_TYPE" >"$RESULTS_DIR/system_info.txt"
echo "CPU: $CPU_MODEL" >>"$RESULTS_DIR/system_info.txt"
# --- Get and Save Drive Type ---
DETECTED_DRIVE_TYPE=$(get_drive_type "$FILE_PATH")
echo "Drive Type: $DETECTED_DRIVE_TYPE" >>"$RESULTS_DIR/system_info.txt"
echo "Benchmarking on drive type: $DETECTED_DRIVE_TYPE"
# --- End Save Drive Type ---
# --- End Save System Info ---

# Worker counts to test
WORKER_COUNTS=(0 2 4 8 16 32) # 0 means auto

# Function to get current memory usage in MB
get_memory_usage() {
    ps -o rss= -p $1 | awk '{print $1/1024}'
}

# Function to get CPU usage percentage
get_cpu_usage() {
    ps -o %cpu= -p $1
}

# Function to get disk I/O stats (macOS compatible)
get_disk_io() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS version - Get total MB/s (col 3), output as KB/s Read, 0.0 Write (CSV)
        iostat -d 1 1 | awk 'NR==3 {printf "%.2f,%.2f\n", $3 * 1024, 0.0}'
    else
        # Linux version - Output KB/s Read (col 3), KB/s Write (col 4) (CSV)
        iostat -d -k 1 1 | grep -v "Device" | awk '{printf "%.2f,%.2f\n", $3, $4}'
    fi
}

echo "Starting benchmark for: $FILE_PATH"
echo "Results will be saved in: $RESULTS_DIR"

# Run benchmarks for each worker count
for WORKERS in "${WORKER_COUNTS[@]}"; do
    echo "Testing with $WORKERS workers..."

    # Start the process in background
    # Ensure mkbrr command exists or is in PATH
    if ! command -v mkbrr &>/dev/null; then
        echo "Error: mkbrr command not found. Make sure it is built and in your PATH."
        # Attempt to find it in the build directory as a fallback
        if [ -f "./build/mkbrr" ]; then
            echo "Using ./build/mkbrr"
            MKBRR_CMD="./build/mkbrr"
        else
            exit 1
        fi
    else
        MKBRR_CMD="mkbrr"
    fi

    $MKBRR_CMD create "$FILE_PATH" --workers $WORKERS >"$RESULTS_DIR/output_${WORKERS}w.log" 2>&1 &
    PID=$!

    # Create CSV file for metrics
    echo "timestamp,memory_mb,cpu_percent,disk_read_kb,disk_write_kb" >"$RESULTS_DIR/metrics_${WORKERS}w.csv"

    # Monitor process until it completes
    while kill -0 $PID 2>/dev/null; do
        timestamp=$(date +%s)
        memory=$(get_memory_usage $PID)
        cpu=$(get_cpu_usage $PID)
        disk_io=$(get_disk_io)

        echo "$timestamp,$memory,$cpu,$disk_io" >>"$RESULTS_DIR/metrics_${WORKERS}w.csv"
        sleep 1
    done

    # Wait for process to complete
    wait $PID
    echo "Completed test with $WORKERS workers"
done

# Generate summary report
echo "Generating summary report..."
echo "Worker Count,Actual Workers,Max Memory (MB),Avg CPU (%),Max Disk Read (kB/s),Max Disk Write (kB/s),Total Time (s)" >"$RESULTS_DIR/summary.csv"

for WORKERS in "${WORKER_COUNTS[@]}"; do
    # --- Determine Actual Workers used ---
    actual_workers=$WORKERS # Default
    if [ "$WORKERS" -eq 0 ]; then
        # Parse the log file for the line like "Concurrency: Using X worker(s)"
        auto_workers=$(grep "Concurrency: Using" "$RESULTS_DIR/output_0w.log" | awk '{print $3}')
        if [[ "$auto_workers" =~ ^[0-9]+$ ]]; then # Check if it's a number
            actual_workers=$auto_workers
        else
            actual_workers="N/A" # Fallback if parsing fails
        fi
    fi

    # Extract metrics from CSV
    max_memory=$(awk -F',' 'NR>1 && $2 != "" {if($2>max)max=$2} END {print max+0}' "$RESULTS_DIR/metrics_${WORKERS}w.csv")
    avg_cpu=$(awk -F',' 'NR>1 && $3 != "" {sum+=$3; count++} END {if (count > 0) print sum/count; else print 0}' "$RESULTS_DIR/metrics_${WORKERS}w.csv")
    max_read=$(awk -F',' 'NR>1 && $4 != "" {if($4>max)max=$4} END {print max+0}' "$RESULTS_DIR/metrics_${WORKERS}w.csv")
    max_write=$(awk -F',' 'NR>1 && $5 != "" {if($5>max)max=$5} END {print max+0}' "$RESULTS_DIR/metrics_${WORKERS}w.csv")

    # Get total time from output log - remove trailing non-numeric chars
    total_time=$(grep "elapsed" "$RESULTS_DIR/output_${WORKERS}w.log" | awk '{print $NF}' | sed 's/[^0-9.]*$//')

    # --- Add actual_workers to the output line ---
    echo "$WORKERS,$actual_workers,$max_memory,$avg_cpu,$max_read,$max_write,$total_time" >>"$RESULTS_DIR/summary.csv"
done

echo "Benchmark complete! Results saved in $RESULTS_DIR"
echo "Summary report: $RESULTS_DIR/summary.csv"
