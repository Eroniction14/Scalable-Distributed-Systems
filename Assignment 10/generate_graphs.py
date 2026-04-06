"""
generate_graphs.py — Generate report graphs from load test CSV results.

Usage:
    pip install pandas matplotlib
    python generate_graphs.py

Expects CSV files in ./results/ directory with naming convention:
    {mode}_{quorum}_W{write_pct}.csv
    e.g., leader_W5R1_W10.csv
"""

import os
import glob
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker
import numpy as np

RESULTS_DIR = "results"
GRAPHS_DIR = "graphs"
os.makedirs(GRAPHS_DIR, exist_ok=True)

# ─── Load all CSVs ────────────────────────────────────────────────────────────

def load_all_results():
    """Load all CSV files and tag with config metadata."""
    frames = []
    for f in glob.glob(f"{RESULTS_DIR}/*.csv"):
        name = os.path.splitext(os.path.basename(f))[0]
        parts = name.split("_")
        # e.g. leader_W5R1_W10 -> mode=leader, quorum=W5R1, write_pct=10
        if len(parts) < 3:
            continue
        mode = parts[0]
        quorum = parts[1]
        wp = parts[2].replace("W", "")

        df = pd.read_csv(f)
        df["mode"] = mode
        df["quorum"] = quorum
        df["write_pct"] = int(wp)
        df["config"] = f"{mode} {quorum}"
        frames.append(df)

    if not frames:
        print(f"No CSV files found in {RESULTS_DIR}/")
        return pd.DataFrame()
    return pd.concat(frames, ignore_index=True)


# ─── Graph 1: Latency Distribution (CDF) per config ──────────────────────────

def plot_latency_cdf(df):
    """CDF of read and write latency for each config+write_pct combo."""
    configs = df["config"].unique()
    write_pcts = sorted(df["write_pct"].unique())

    for wp in write_pcts:
        fig, axes = plt.subplots(1, 2, figsize=(14, 5))
        fig.suptitle(f"Latency CDF — Write Ratio: {wp}%", fontsize=14, fontweight="bold")

        for req_type, ax in zip(["read", "write"], axes):
            for cfg in configs:
                subset = df[(df["config"] == cfg) & (df["write_pct"] == wp) & (df["type"] == req_type)]
                if subset.empty:
                    continue
                lats = np.sort(subset["latency_ms"].values)
                cdf = np.arange(1, len(lats) + 1) / len(lats)
                ax.plot(lats, cdf, label=cfg, linewidth=1.5)

            ax.set_xlabel("Latency (ms)")
            ax.set_ylabel("CDF")
            ax.set_title(f"{req_type.capitalize()} Latency")
            ax.legend(fontsize=8)
            ax.grid(True, alpha=0.3)
            ax.set_xlim(left=0)

        plt.tight_layout()
        fname = f"{GRAPHS_DIR}/latency_cdf_wp{wp}.png"
        plt.savefig(fname, dpi=150)
        plt.close()
        print(f"  Saved {fname}")


# ─── Graph 2: Latency Box Plots ──────────────────────────────────────────────

def plot_latency_boxes(df):
    """Box plots showing latency distributions including long tails."""
    configs = df["config"].unique()
    write_pcts = sorted(df["write_pct"].unique())

    for req_type in ["read", "write"]:
        fig, axes = plt.subplots(1, len(write_pcts), figsize=(5 * len(write_pcts), 5),
                                  sharey=False)
        fig.suptitle(f"{req_type.capitalize()} Latency Distribution", fontsize=14, fontweight="bold")

        if len(write_pcts) == 1:
            axes = [axes]

        for ax, wp in zip(axes, write_pcts):
            data = []
            labels = []
            for cfg in configs:
                subset = df[(df["config"] == cfg) & (df["write_pct"] == wp) & (df["type"] == req_type)]
                if not subset.empty:
                    data.append(subset["latency_ms"].values)
                    labels.append(cfg.replace(" ", "\n"))

            if data:
                bp = ax.boxplot(data, labels=labels, showfliers=True, patch_artist=True)
                colors = plt.cm.Set2(np.linspace(0, 1, len(data)))
                for patch, color in zip(bp["boxes"], colors):
                    patch.set_facecolor(color)
                    patch.set_alpha(0.7)

            ax.set_title(f"Write Ratio: {wp}%")
            ax.set_ylabel("Latency (ms)")
            ax.grid(True, alpha=0.3, axis="y")

        plt.tight_layout()
        fname = f"{GRAPHS_DIR}/latency_box_{req_type}.png"
        plt.savefig(fname, dpi=150)
        plt.close()
        print(f"  Saved {fname}")


# ─── Graph 3: Stale Reads Summary ────────────────────────────────────────────

def plot_stale_reads(df):
    """Bar chart of stale read percentage per config and write ratio."""
    reads = df[df["type"] == "read"].copy()
    if reads.empty:
        print("  No read data for stale reads graph")
        return

    summary = reads.groupby(["config", "write_pct"]).agg(
        total=("stale", "count"),
        stale=("stale", "sum"),
    ).reset_index()
    summary["stale_pct"] = summary["stale"] / summary["total"] * 100

    configs = summary["config"].unique()
    write_pcts = sorted(summary["write_pct"].unique())

    fig, ax = plt.subplots(figsize=(12, 5))
    x = np.arange(len(write_pcts))
    width = 0.8 / len(configs)

    for i, cfg in enumerate(configs):
        sub = summary[summary["config"] == cfg]
        vals = [sub[sub["write_pct"] == wp]["stale_pct"].values[0]
                if wp in sub["write_pct"].values else 0
                for wp in write_pcts]
        bars = ax.bar(x + i * width, vals, width, label=cfg, alpha=0.8)
        # Add count labels on bars
        for bar, wp in zip(bars, write_pcts):
            row = sub[sub["write_pct"] == wp]
            if not row.empty:
                count = int(row["stale"].values[0])
                if count > 0:
                    ax.text(bar.get_x() + bar.get_width() / 2, bar.get_height() + 0.3,
                            str(count), ha="center", va="bottom", fontsize=7)

    ax.set_xlabel("Write Ratio (%)")
    ax.set_ylabel("Stale Reads (%)")
    ax.set_title("Stale Read Percentage by Configuration", fontweight="bold")
    ax.set_xticks(x + width * len(configs) / 2)
    ax.set_xticklabels([f"{wp}%" for wp in write_pcts])
    ax.legend(fontsize=8)
    ax.grid(True, alpha=0.3, axis="y")

    plt.tight_layout()
    fname = f"{GRAPHS_DIR}/stale_reads.png"
    plt.savefig(fname, dpi=150)
    plt.close()
    print(f"  Saved {fname}")


# ─── Graph 4: Read-Write Interval Distribution ───────────────────────────────

def plot_rw_intervals(df):
    """Histogram of time intervals between reading and writing the same key."""
    reads = df[(df["type"] == "read") & (df["rw_interval_ms"] > 0)].copy()
    if reads.empty:
        print("  No RW interval data")
        return

    configs = reads["config"].unique()
    write_pcts = sorted(reads["write_pct"].unique())

    for wp in write_pcts:
        fig, ax = plt.subplots(figsize=(10, 5))
        for cfg in configs:
            subset = reads[(reads["config"] == cfg) & (reads["write_pct"] == wp)]
            if subset.empty:
                continue
            ax.hist(subset["rw_interval_ms"].values, bins=50, alpha=0.5,
                    label=cfg, density=True)

        ax.set_xlabel("Time Since Last Write to Same Key (ms)")
        ax.set_ylabel("Density")
        ax.set_title(f"Read-Write Interval Distribution — Write Ratio: {wp}%", fontweight="bold")
        ax.legend(fontsize=8)
        ax.grid(True, alpha=0.3)

        plt.tight_layout()
        fname = f"{GRAPHS_DIR}/rw_interval_wp{wp}.png"
        plt.savefig(fname, dpi=150)
        plt.close()
        print(f"  Saved {fname}")


# ─── Main ─────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print("Loading results...")
    df = load_all_results()
    if df.empty:
        exit(1)

    print(f"Loaded {len(df)} records from {df['config'].nunique()} configs\n")

    print("Generating latency CDF plots...")
    plot_latency_cdf(df)

    print("Generating latency box plots...")
    plot_latency_boxes(df)

    print("Generating stale reads summary...")
    plot_stale_reads(df)

    print("Generating read-write interval distributions...")
    plot_rw_intervals(df)

    print(f"\nAll graphs saved to {GRAPHS_DIR}/")