#!/usr/bin/env uv run

# To run this application, use:
#   uv run broadband_invoices.py
#
# /// script
# requires-python = ">=3.12"
# dependencies = [
#   "rich",
#   "requests",
# ]
# ///

import argparse
from fnmatch import fnmatch

from rich.progress import track
from rich import print

from common import AivenKafka, User


def find_unused_acls(topics, acls):
    for acl in track(acls, description="Searching for unused ACLs"):
        if any((acl.topic.startswith("__"),
                "_stream_" in acl.topic,
                acl.topic in topics)):
            continue
        yield acl


def find_unused_users(acls, users: set[User]):
    acl_patterns = {acl.username for acl in acls}
    for user in track(users, description="Searching for unused users"):
        if user.username == "avnadmin":
            continue
        if not any(fnmatch(user.username, pattern) for pattern in acl_patterns):
            yield user.username


def find_old_acls(acls):
    for acl in track(acls, description="Searching for old ACLs"):
        if "." in acl.username:
            yield acl


def main(project, dry_run):
    aiven = AivenKafka(project, dry_run=dry_run)

    service = aiven.get_service()

    unused_acls = set(find_unused_acls(service.topics, service.acls))
    print(f"Found {len(unused_acls)} ACLs referencing non-existing topics")
    old_acls = set(find_old_acls(service.acls))
    print(f"Found {len(old_acls)} ACLs using the old username convention")

    acls_to_delete = unused_acls | old_acls
    aiven.delete_acls(acls_to_delete)

    acls = aiven.get_acls()
    if dry_run:
        acls.difference_update(acls_to_delete)
    users_to_delete = set(find_unused_users(acls, service.users))
    print(f"Found {len(users_to_delete)} users with no associated ACLs")
    aiven.delete_users(users_to_delete)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("-n", "--dry-run", action="store_true", help="Make no actual changes")
    parser.add_argument("project", action="store", help="Aiven project to process")
    options = parser.parse_args()
    main(options.project, options.dry_run)
