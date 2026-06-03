#!/usr/bin/env -S uv run
#
# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "requests",
#   "rich",
#   "humanize",
# ]
# ///

"""Analyze Kafka logs from last 24 hours to discover which topics are not reducing in size when the log-cleaner compacts it.

Calls the Loki API to get logs from the last 24 hours and extracts topic-name and size reduction for each instance.
Prints a summary of average and median size reduction for each topic/partition.
"""

import argparse
import re
import time
from datetime import timedelta
from collections import defaultdict

import requests
import humanize
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn, TimeElapsedColumn
from rich.table import Table

LOKI_URL = "https://loki.nav.cloud.nais.io/loki/api/v1/query_range"
LOKI_QUERY = '{service_name="aiven-klog", k8s_cluster_name="prod"} |~ "(?i)Log cleaner thread 0 cleaned log"'
LOKI_ORG = "nais"
LIMIT = 5000

_RE_TOPIC = re.compile(r"cleaned log\s+(\S+)", re.IGNORECASE)
_RE_SIZE = re.compile(r"([\d.]+)%\s+size reduction", re.IGNORECASE)

console = Console()


def fetch_log_lines(start_ns: int, end_ns: int):
    """Generator that paginates Loki query_range results and yields individual log lines."""
    params = {
        "query": LOKI_QUERY,
        "start": str(start_ns),
        "end": str(end_ns),
        "limit": str(LIMIT),
        "direction": "forward",
    }
    headers = {"X-Scope-OrgID": LOKI_ORG}

    current_start = start_ns

    while True:
        params["start"] = str(current_start)
        resp = requests.get(LOKI_URL, params=params, headers=headers, timeout=30)
        resp.raise_for_status()
        data = resp.json()

        result = data.get("data", {}).get("result", [])
        if not result:
            break

        batch_lines = []
        latest_ts = current_start

        for stream in result:
            for ts, line in stream.get("values", []):
                ts_int = int(ts)
                if ts_int > latest_ts:
                    latest_ts = ts_int
                batch_lines.append((ts_int, line))

        if not batch_lines:
            break

        batch_lines.sort(key=lambda x: x[0])

        for _, line in batch_lines:
            yield line

        current_start = latest_ts + 1


def parse_line(line: str):
    """Return (topic_partition, size_pct) or None if not parseable."""
    topic_match = _RE_TOPIC.search(line)
    size_match = _RE_SIZE.search(line)
    if topic_match and size_match:
        return topic_match.group(1), float(size_match.group(1))
    return None


def median(values: list[float]) -> float:
    s = sorted(values)
    n = len(s)
    mid = n // 2
    if n % 2 == 0:
        return (s[mid - 1] + s[mid]) / 2
    return s[mid]


def parse_since(value: str) -> int:
    """Parse a duration string like '24h' or '7d' into seconds."""
    m = re.fullmatch(r"(\d+)([hd])", value.strip().lower())
    if not m:
        raise argparse.ArgumentTypeError(f"Invalid --since value '{value}'. Use e.g. '24h' or '7d'.")
    amount, unit = int(m.group(1)), m.group(2)
    return amount * (3600 if unit == "h" else 86400)


def main():
    parser = argparse.ArgumentParser(description="Analyze Kafka log-cleaner compaction results from Loki.")
    parser.add_argument("--since", default="24h", type=parse_since,
                        metavar="DURATION",
                        help="How far back to query (e.g. 24h, 7d). Default: 24h")
    args = parser.parse_args()

    now_ns = int(time.time() * 1e9)
    start_ns = now_ns - args.since * 1_000_000_000
    period = humanize.naturaldelta(timedelta(seconds=args.since))

    stats: dict[str, list[float]] = defaultdict(list)

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        TimeElapsedColumn(),
        console=console,
    ) as progress:
        task = progress.add_task(f"Fetching Loki logs for {period} …", total=None)
        for line in fetch_log_lines(start_ns, now_ns):
            result = parse_line(line)
            if result:
                topic_partition, size_pct = result
                topic = re.sub(r"-\d+$", "", topic_partition)
                if topic.startswith("__"):
                    continue
                stats[topic_partition].append(size_pct)
            progress.update(task, description=f"Fetching Loki logs for {period} … ({sum(len(v) for v in stats.values())} entries)")

    if not stats:
        console.print("[yellow]No matching log entries found.[/yellow]")
        return

    rows = []
    for tp, values in stats.items():
        avg = sum(values) / len(values)
        med = median(values)
        rows.append((tp, len(values), min(values), avg, med, max(values)))

    def tp_sort_key(r):
        tp = r[0]
        m = re.match(r"^(.*)-(\d+)$", tp)
        if m:
            return (m.group(1), int(m.group(2)))
        return (tp, 0)

    rows.sort(key=tp_sort_key)

    tp_table = Table(title="Topic-Partition Size Reduction", show_lines=False)
    tp_table.add_column("Topic-Partition", style="cyan", no_wrap=True)
    tp_table.add_column("Samples", justify="right")
    tp_table.add_column("Min %", justify="right")
    tp_table.add_column("Avg %", justify="right")
    tp_table.add_column("Median %", justify="right")
    tp_table.add_column("Max %", justify="right")

    for tp, samples, mn, avg, med, mx in rows:
        tp_table.add_row(tp, str(samples), f"{mn:.1f}", f"{avg:.1f}", f"{med:.1f}", f"{mx:.1f}")

    console.print()
    console.print(tp_table)

    buckets = {
        "0.0% size reduction":  0,
        "<10% size reduction":  0,
        "<50% size reduction":  0,
        "<90% size reduction":  0,
        "≥90% size reduction":  0,
    }
    for _, _, _, avg, _, _ in rows:
        if avg == 0.0:
            buckets["0.0% size reduction"] += 1
        elif avg < 10:
            buckets["<10% size reduction"] += 1
        elif avg < 50:
            buckets["<50% size reduction"] += 1
        elif avg < 90:
            buckets["<90% size reduction"] += 1
        else:
            buckets["≥90% size reduction"] += 1

    bucket_table = Table(title="Summary buckets (by average size reduction)", show_lines=False)
    bucket_table.add_column("Bucket")
    bucket_table.add_column("Count", justify="right")

    for label, count in buckets.items():
        bucket_table.add_row(label, str(count))
    bucket_table.add_section()
    bucket_table.add_row("[bold]Total[/bold]", f"[bold]{sum(buckets.values())}[/bold]")

    console.print()
    console.print(bucket_table)


if __name__ == "__main__":
    main()
