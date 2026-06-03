#!/usr/bin/env -S uv run
#
# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "requests",
#   "rich",
#   "humanize",
#   "philiprehberger-duration",
# ]
# ///

"""Analyze Kafka logs from last 24 hours to discover which topics are not reducing in size when the log-cleaner compacts it.

Calls the Loki API to get logs from the last 24 hours and extracts topic-name and size reduction for each instance.
Prints a summary of average and median size reduction for each topic/partition.
"""

import argparse
import enum
import json
import re
import subprocess
import time
from collections import defaultdict
from dataclasses import dataclass
from datetime import timedelta, datetime

from philiprehberger_duration import parse, Duration
import humanize
import requests
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn, TimeElapsedColumn, track
from rich.table import Table

LOKI_URL = "https://loki.nav.cloud.nais.io/loki/api/v1/query_range"
LOKI_QUERY = '{service_name="aiven-klog", k8s_cluster_name="prod"} |~ "(?i)Log cleaner thread 0 cleaned log"'
LOKI_ORG = "nais"
KUBE_CONTEXT = "nav-prod"
LIMIT = 5000

_RE_TOPIC = re.compile(r"cleaned log\s+(\S+)", re.IGNORECASE)
_RE_TOPIC_PARTITION = re.compile(r"(\S+)-(\d+)")
_RE_SIZE = re.compile(r"([\d.]+)%\s+size reduction", re.IGNORECASE)

console = Console()


@dataclass
class LogRecord:
    topic: str
    partition: int
    size_pct: float


@dataclass(order=True)
class TopicStats:
    name: str
    compacted: bool
    samples: int
    min_pct: float
    avg_pct: float
    median_pct: float
    max_pct: float

    def topic_row(self):
        return self.name, str(self.compacted), str(self.samples), f"{self.min_pct:.1f}", f"{self.avg_pct:.1f}", f"{self.median_pct:.1f}", f"{self.max_pct:.1f}"


def find_compacted_topics():
    cmd = [
        "kubectl",
        "get", "topic",
        "--all-namespaces",
        "--context", KUBE_CONTEXT,
        "--output", "json",
    ]
    with console.status(f"Executing {' '.join(cmd)}"):
        output = subprocess.check_output(cmd)
        data = json.loads(output)

    for item in track(data["items"], description=f"Parsing topics from {KUBE_CONTEXT}", console=console):
        cleanup_policy = item["spec"].get("config", {}).get("cleanupPolicy", "delete")
        if "compact" in cleanup_policy:
            yield item["status"].get("fullyQualifiedName", "ERROR")


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

        count = 0
        latest_ts = current_start

        for stream in result:
            for ts, line in stream.get("values", []):
                ts_int = int(ts)
                if ts_int > latest_ts:
                    latest_ts = ts_int
                count += 1
                yield line

        if count == 0:
            break

        current_start = latest_ts + 1


def parse_line(line: str) -> LogRecord | None:
    """Return (topic_partition, size_pct) or None if not parseable."""
    topic_match = _RE_TOPIC.search(line)
    size_match = _RE_SIZE.search(line)
    if topic_match and size_match:
        topic_partition_match = _RE_TOPIC_PARTITION.match(topic_match.group(1))
        if not topic_partition_match:
            return None
        return LogRecord(topic=topic_partition_match.group(1), partition=int(topic_partition_match.group(2)),
                         size_pct=float(size_match.group(1)))
    return None


def median(values: list[float]) -> float:
    s = sorted(values)
    n = len(s)
    mid = n // 2
    if n % 2 == 0:
        return (s[mid - 1] + s[mid]) / 2
    return s[mid]


def parse_since(value: str) -> timedelta:
    """Parse a duration string like '24h' or '7d' into seconds."""
    seconds = parse(value)
    return timedelta(seconds=seconds)


def main():
    parser = argparse.ArgumentParser(description="Analyze Kafka log-cleaner compaction results from Loki.")
    parser.add_argument("--since", default="24h", type=parse_since,
                        metavar="DURATION",
                        help="How far back to query (e.g. 24h, 7d). Default: 24h")
    args = parser.parse_args()

    now = datetime.now()
    now_ns = int(now.timestamp() * 1_000_000_000)
    start = now - args.since
    start_ns = int(start.timestamp() * 1_000_000_000)
    period = humanize.naturaldelta(args.since)

    compacted_topics = list(find_compacted_topics())

    stats: dict[str, list[LogRecord]] = defaultdict(list)

    with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            TimeElapsedColumn(),
            console=console,
    ) as progress:
        task = progress.add_task(f"Fetching Loki logs for {period} …", total=None)
        for line in fetch_log_lines(start_ns, now_ns):
            record = parse_line(line)
            if record:
                if record.topic.startswith("__"):
                    continue
                stats[record.topic].append(record)
            progress.update(task,
                            description=f"Fetching Loki logs for {period} … ({sum(len(v) for v in stats.values())} entries)")

    if not stats:
        console.print("[yellow]No matching log entries found.[/yellow]")
        return

    rows = []
    for topic, records in stats.items():
        values = [r.size_pct for r in records]
        avg = sum(values) / len(records)
        med = median(values)
        rows.append(TopicStats(
            name=topic,
            compacted=topic in compacted_topics,
            samples=len(values),
            min_pct=min(values),
            avg_pct=avg,
            median_pct=med,
            max_pct=max(values))
        )

    rows.sort()

    tp_table = Table(title="Topic Size Reduction", show_lines=False)
    tp_table.add_column("Topic", style="cyan", no_wrap=True)
    tp_table.add_column("Compacted?")
    tp_table.add_column("Samples", justify="right")
    tp_table.add_column("Min %", justify="right")
    tp_table.add_column("Avg %", justify="right")
    tp_table.add_column("Median %", justify="right")
    tp_table.add_column("Max %", justify="right")

    sus_table = Table(title="Compacted topics with no size reduction", show_lines=False)
    sus_table.add_column("Topic", style="cyan", no_wrap=True)
    sus_table.add_column("Samples", justify="right")

    for row in rows:
        tp_table.add_row(*row.topic_row())
        if row.compacted and row.avg_pct == 0.0:
            sus_table.add_row(row.name, str(row.samples))

    console.print()
    console.print(tp_table)
    console.print()
    console.print(sus_table)

    buckets = {
        "0.0% size reduction": [0, 0, 0],
        "<10% size reduction": [0, 0, 0],
        "<50% size reduction": [0, 0, 0],
        "<90% size reduction": [0, 0, 0],
        "≥90% size reduction": [0, 0, 0],
    }
    for row in rows:
        idx = int(row.compacted) + 1
        if row.avg_pct == 0.0:
            buckets["0.0% size reduction"][0] += 1
            buckets["0.0% size reduction"][idx] += 1
        elif row.avg_pct < 10:
            buckets["<10% size reduction"][0] += 1
            buckets["<10% size reduction"][idx] += 1
        elif row.avg_pct < 50:
            buckets["<50% size reduction"][0] += 1
            buckets["<50% size reduction"][idx] += 1
        elif row.avg_pct < 90:
            buckets["<90% size reduction"][0] += 1
            buckets["<90% size reduction"][idx] += 1
        else:
            buckets["≥90% size reduction"][0] += 1
            buckets["≥90% size reduction"][idx] += 1

    bucket_table = Table(title="Summary buckets (by average size reduction)", show_lines=False)
    bucket_table.add_column("Bucket")
    bucket_table.add_column("Count", justify="right")
    bucket_table.add_column("Compacted", justify="right")
    bucket_table.add_column("Normal", justify="right")

    for label, counts in buckets.items():
        bucket_table.add_row(label, *(str(count) for count in counts))
    bucket_table.add_section()
    bucket_table.add_row("[bold]Total[/bold]", f"[bold]{sum(v[0] for v in buckets.values())}[/bold]")

    console.print()
    console.print(bucket_table)


if __name__ == "__main__":
    main()
