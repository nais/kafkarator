#!/usr/bin/env uv run

# Read-only report of Kafka Streams internal topics.
#
# Kafka Streams applications create internal topics named
#   <namespace>.<name>_stream_<...>-changelog / -repartition
# where <namespace>.<name> identifies the Stream custom resource that
# owns the topics. This script lists all topics in the Aiven pools,
# finds the ones matching the `_stream_` pattern, and maps each one
# back to its Stream resource in the clusters.
#
# Topics whose prefix has no live Stream resource are reported as
# orphaned. When an orphaned prefix closely resembles a live Stream
# (e.g. renamed from `foo_stream_` to `foo-stream_stream_`), the
# report says so explicitly, to make clear that the topics are
# leftovers and not in use by the renamed application.
#
# To run this application, use:
#   uv run topic_reporter.py
#   uv run topic_reporter.py --verbose               # also list owned topics
#   uv run topic_reporter.py --include aap.          # scoped run
#
# /// script
# requires-python = ">=3.12"
# dependencies = [
#   "rich",
# ]
# ///

import argparse
import difflib
import json
import subprocess
import sys

from rich import print
from rich.table import Table

STREAM_SEPARATOR = "_stream_"


def run_json(cmd):
    with subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE) as proc:
        stdout, stderr = proc.communicate()
        if proc.returncode != 0:
            raise RuntimeError(f"{' '.join(cmd)} failed: {stderr.decode().strip()}")
        return json.loads(stdout)


def live_streams(contexts):
    """Map '<namespace>.<name>_stream_' prefix to a list of Stream CRs with that
    prefix. The same prefix can exist in several clusters/pools (e.g. a team
    running the same stream in both dev and prod)."""
    streams = {}
    for context in contexts:
        data = run_json(
            [
                "kubectl",
                "get",
                "streams.kafka.nais.io",
                "--all-namespaces",
                "--output",
                "json",
                "--context",
                context,
            ]
        )
        for item in data["items"]:
            metadata = item["metadata"]
            prefix = f"{metadata['namespace']}.{metadata['name']}{STREAM_SEPARATOR}"
            streams.setdefault(prefix, []).append(
                {
                    "namespace": metadata["namespace"],
                    "name": metadata["name"],
                    "pool": item["spec"]["pool"],
                    "context": context,
                }
            )
    return streams


def matching_stream(candidates, project):
    """The stream whose pool matches `project`, or None. When several streams
    share a prefix, a topic in `project` belongs to the one pointing at
    `project`; if none do, the topics are leftovers from a stream that has
    moved to another pool."""
    for stream in candidates:
        if stream["pool"] == project:
            return stream
    return None


def list_topics(project, service):
    data = run_json(
        [
            "avn",
            "service",
            "topic-list",
            service,
            "--project",
            project,
            "--json",
        ]
    )
    return [t["topic_name"] for t in data]


def stream_prefix(topic_name):
    """The owning '<namespace>.<name>_stream_' prefix of a stream topic, or None."""
    if topic_name.startswith("__") or STREAM_SEPARATOR not in topic_name:
        return None
    return topic_name.split(STREAM_SEPARATOR, 1)[0] + STREAM_SEPARATOR


def closest_stream(prefix, live_prefixes):
    """The live stream prefix closest to `prefix`, if it shares the same
    '<namespace>.' part and is nearly identical after normalizing '-' to '_'.
    A cutoff of 0.9 catches simple renames (e.g. 'vedtak_stream_' ->
    'vedtak-stream_stream_') without matching different apps in the team."""
    namespace = prefix.split(".", 1)[0]
    candidates = {
        p.replace("-", "_"): p
        for p in live_prefixes
        if p.split(".", 1)[0] == namespace
    }
    matches = difflib.get_close_matches(
        prefix.replace("-", "_"), candidates, n=1, cutoff=0.95
    )
    return candidates[matches[0]] if matches else None


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "--contexts",
        default="dev-gcp,prod-gcp",
        help="Comma-separated kubectl contexts to scan for Stream resources",
    )
    parser.add_argument(
        "--projects",
        default="nav-dev,nav-prod",
        help="Comma-separated Aiven projects (pools) to scan",
    )
    parser.add_argument(
        "--include",
        default=None,
        metavar="PREFIX",
        help="Only consider topics starting with this prefix (for a scoped run)",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Also list stream topics that belong to a live Stream resource",
    )
    options = parser.parse_args()

    contexts = options.contexts.split(",")
    projects = options.projects.split(",")

    print(f"Collecting Stream resources from contexts: {', '.join(contexts)}")
    streams = live_streams(contexts)
    print(f"Found {sum(len(v) for v in streams.values())} live Stream resources")

    for project in projects:
        service = f"{project}-kafka"
        print(f"\n[bold]Project {project} (service {service})[/bold]")

        topics = list_topics(project, service)
        owned, orphans = [], []
        mismatched = {}
        non_stream = 0
        for topic in topics:
            prefix = stream_prefix(topic)
            if prefix is None:
                non_stream += 1
                continue
            if options.include and not topic.startswith(options.include):
                continue
            if prefix in streams:
                candidates = streams[prefix]
                stream = matching_stream(candidates, project)
                if stream is not None:
                    owned.append((topic, prefix, stream))
                else:
                    # Streams with this prefix exist, but all point to other pools.
                    other = candidates[0]
                    mismatched.setdefault(
                        (other["namespace"], other["name"], other["context"], other["pool"]), 0
                    )
                    mismatched[
                        (other["namespace"], other["name"], other["context"], other["pool"])
                    ] += 1
            else:
                orphans.append((topic, prefix))

        print(
            f"{len(topics)} topics total: {len(owned)} stream topics owned by a live Stream, "
            f"{len(orphans)} orphaned stream topics, {non_stream} non-stream topics"
        )

        for (namespace, name, context, pool), count in sorted(mismatched.items()):
            print(
                f"[yellow]WARNING: {count} topics in pool {project} look like Stream "
                f"{namespace}/{name} ({context}), but that Stream points to pool {pool} "
                f"and no Stream with this prefix points to {project}[/yellow]"
            )

        orphan_table = Table(title=f"Orphaned stream topics in {project}")
        orphan_table.add_column("Topic", overflow="fold")
        orphan_table.add_column("Status")
        orphan_table.add_column("Closest live Stream", overflow="fold")
        for topic, prefix in sorted(orphans):
            near = closest_stream(prefix, streams)
            if near is None:
                status, closest = "orphaned", ""
            else:
                near_stream = matching_stream(streams[near], project) or streams[near][0]
                closest = f"{near_stream['namespace']}/{near_stream['name']} ({near_stream['context']}, pool {near_stream['pool']})"
                status = "renamed?" if near_stream["pool"] == project else "owned in other pool?"
            orphan_table.add_row(topic, status, closest)
        print(orphan_table)
        print(f"[bold]{len(orphans)} orphaned stream topics[/bold]")

        if options.verbose and owned:
            owned_table = Table(title=f"Stream topics with a live Stream in {project}")
            owned_table.add_column("Topic", overflow="fold")
            owned_table.add_column("Stream")
            owned_table.add_column("Context")
            owned_table.add_column("Pool")
            for topic, prefix, stream in sorted(owned):
                owned_table.add_row(
                    topic,
                    f"{stream['namespace']}/{stream['name']}",
                    stream["context"],
                    stream["pool"],
                )
            print(owned_table)


if __name__ == "__main__":
    try:
        main()
    except RuntimeError as e:
        print(f"[red]{e}[/red]")
        sys.exit(1)
