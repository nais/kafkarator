#!/usr/bin/env -S uv run
#
# /// script
# requires-python = ">=3.12"
# dependencies = [
#   "rich",
#   "requests",
# ]
# ///

"""Investigate an application on Aiven Kafka

Looks for these things:
- Service users referenced in the kubernetes clusters
- Active service users in Aiven
- Matching ACLs
"""
import argparse
import subprocess
from fnmatch import fnmatch

from rich import print

from common import get_secrets_in, AivenKafka


def main(options):
    if options.context is None:
        options.context = subprocess.check_output(["kubectl", "config", "current-context"]).strip().decode("utf-8")
    if options.pool is None:
        options.pool = "-".join(options.context.split('-')[0:2])
    usernames = {secret.username for secret in
                 get_secrets_in(options.context, options.team, selector=f"app={options.app}")}
    print(f"Found {len(usernames)} users in {options.context} with app={options.app}")
    for username in usernames:
        print(f" * {username}")
    aiven_kafka = AivenKafka(options.pool)
    service = aiven_kafka.get_service(options.team)
    identify_users(service, options.team, usernames)
    identify_acls(service, options.team, usernames)


def identify_acls(service, team, usernames):
    found = False
    for acl in sorted(service.acls, key=lambda a: a.username):
        if any(fnmatch(u, acl.username) for u in usernames):
            print(f"Found matching ACL in Aiven: {acl}")
            found = True
    if not found:
        print(f"No matching ACLs found in Aiven for {team}")


def identify_users(service, team, usernames):
    found = False
    for user in service.users:
        if user.username in usernames:
            print(f"Found matching user in Aiven: {user}")
            found = True
    if not found:
        print(f"No matching users found in Aiven for {team}")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("team", type=str, help="Team to investigate")
    parser.add_argument("app", type=str, help="Application to investigate")
    parser.add_argument("--context", action="store", help="Kubernetes context to use")
    parser.add_argument("--pool", action="store", help="Aiven project to use")
    options = parser.parse_args()
    main(options)
