#!/usr/bin/env python3

import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import sys
import os


def plot_benchmark_results(results_dir):
    # Read summary data
    summary_path = os.path.join(results_dir, "summary.csv")
    if not os.path.exists(summary_path):
        print(f"Error: Summary file not found at {summary_path}")
        sys.exit(1)
    df = pd.read_csv(summary_path)

    # --- Read System Info ---
    sys_info_path = os.path.join(results_dir, "system_info.txt")
    os_info = "OS: N/A"
    cpu_info = "CPU: N/A"
    drive_info = "Drive Type: N/A"
    if os.path.exists(sys_info_path):
        with open(sys_info_path, "r") as f:
            lines = f.readlines()
            for line in lines:
                if line.startswith("OS:"):
                    os_info = line.strip()
                elif line.startswith("CPU:"):
                    cpu_info = line.strip()
                elif line.startswith("Drive Type:"):
                    drive_info = line.strip()

    # Combine system info for display
    system_info_display = f"{os_info} | {cpu_info}"

    # --- Explicitly convert columns to numeric (excluding Write column) ---
    numeric_cols = [
        "Actual Workers",
        "Max Memory (MB)",
        "Avg CPU (%)",
        "Max Disk Read (kB/s)",
        "Total Time (s)",
    ]  # Removed Max Disk Write
    for col in numeric_cols:
        df[col] = pd.to_numeric(df[col], errors="coerce")

    # --- Create Worker Label column ---
    def create_label(row):
        if row["Worker Count"] == 0:
            actual_str = (
                str(int(row["Actual Workers"]))
                if pd.notna(row["Actual Workers"])
                else "N/A"
            )
            return f"Auto ({actual_str})"
        else:
            return str(int(row["Worker Count"]))

    df["Worker Label"] = df.apply(create_label, axis=1)

    # --- Set style and figure ---
    plt.style.use("default")
    sns.set_theme(style="whitegrid")
    fig, axes = plt.subplots(2, 2, figsize=(18, 10))
    fig.suptitle("mkbrr Performance Benchmark: Varying Worker Counts", fontsize=16)
    # Display OS and CPU info
    fig.text(
        0.5,
        0.94,
        system_info_display,
        ha="center",
        va="bottom",
        fontsize=10,
        color="gray",
    )
    # Display Drive info below OS/CPU
    fig.text(0.5, 0.92, drive_info, ha="center", va="bottom", fontsize=10, color="gray")

    # --- Helper function to add labels ---
    def add_labels_barh(ax):
        for patch in ax.patches:
            width = patch.get_width()
            label = f"{width:.2f}" if pd.notna(width) else "N/A"
            ax.text(
                width + (ax.get_xlim()[1] * 0.01),
                patch.get_y() + patch.get_height() / 2.0,
                label,
                ha="left",
                va="center",
                fontsize=9,
            )

    # --- Find best indices ---
    best_mem_idx = df["Max Memory (MB)"].idxmin()
    best_cpu_idx = df["Avg CPU (%)"].idxmin()
    best_disk_idx = df["Max Disk Read (kB/s)"].idxmax()
    best_time_idx = df["Total Time (s)"].idxmin()

    # --- Define custom palettes ---
    mem_palette = {
        label: "green" if i == best_mem_idx else "#1f77b4"
        for i, label in enumerate(df["Worker Label"])
    }
    cpu_palette = {
        label: "green" if i == best_cpu_idx else "#1f77b4"
        for i, label in enumerate(df["Worker Label"])
    }
    # --- Palette for Disk Read ---
    disk_read_palette = {
        label: "green" if i == best_disk_idx else "#1f77b4"
        for i, label in enumerate(df["Worker Label"])
    }
    time_palette = {
        label: "green" if i == best_time_idx else "#1f77b4"
        for i, label in enumerate(df["Worker Label"])
    }

    # --- Plot 1: Memory Usage ---
    ax = axes[0, 0]
    bars = sns.barplot(
        y="Worker Label",
        x="Max Memory (MB)",
        data=df,
        ax=ax,
        orient="h",
        hue="Worker Label",
        palette=mem_palette,
        legend=False,
    )
    ax.set_title("Maximum Memory Usage")
    ax.set_ylabel("Number of Workers")
    ax.set_xlabel("Memory (MB)")
    add_labels_barh(ax)

    # --- Plot 2: CPU Usage ---
    ax = axes[0, 1]
    bars = sns.barplot(
        y="Worker Label",
        x="Avg CPU (%)",
        data=df,
        ax=ax,
        orient="h",
        hue="Worker Label",
        palette=cpu_palette,
        legend=False,
    )
    ax.set_title("Average CPU Usage")
    ax.set_ylabel("Number of Workers")
    ax.set_xlabel("CPU Usage (%)")
    add_labels_barh(ax)

    # --- Plot 3: Disk I/O (Reads Only) ---
    ax = axes[1, 0]
    # --- Use seaborn barplot for consistency and highlighting ---
    bars = sns.barplot(
        y="Worker Label",
        x="Max Disk Read (kB/s)",
        data=df,
        ax=ax,
        orient="h",
        hue="Worker Label",
        palette=disk_read_palette,
        legend=False,
    )
    ax.set_title("Maximum Disk Read I/O")
    ax.set_ylabel("Number of Workers")
    ax.set_xlabel("Read Rate (kB/s)")
    # --- Remove legend if it exists (seaborn might add one with hue) ---
    # ax.get_legend().remove() # Alternative way, but legend=False should prevent it
    add_labels_barh(ax)

    # --- Plot 4: Total Time ---
    ax = axes[1, 1]
    bars = sns.barplot(
        y="Worker Label",
        x="Total Time (s)",
        data=df,
        ax=ax,
        orient="h",
        hue="Worker Label",
        palette=time_palette,
        legend=False,
    )
    ax.set_title("Total Processing Time")
    ax.set_ylabel("Number of Workers")
    ax.set_xlabel("Time (seconds)")
    add_labels_barh(ax)

    # --- Adjust layout and save ---
    plt.tight_layout(rect=[0, 0.03, 1, 0.90])  # Adjust top boundary for new text
    output_path = os.path.join(results_dir, "benchmark_results.png")
    plt.savefig(output_path, dpi=300, bbox_inches="tight")
    print(f"Plot saved to: {output_path}")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python plot_benchmark.py <results_directory>")
        sys.exit(1)
    results_dir = sys.argv[1]
    plot_benchmark_results(results_dir)
