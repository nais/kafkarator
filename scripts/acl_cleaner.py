#!/usr/bin/env python3
import argparse
from fnmatch import fnmatch

from alive_progress import alive_it

from common import AivenKafka, User


def find_unused_acls(topics, acls):
    for acl in acls:
        if any((acl.topic.startswith("__"),
                "_stream_" in acl.topic,
                acl.topic in topics)):
            continue
        yield acl


def find_unused_users(acls, users: set[User]):
    acl_patterns = {acl.username for acl in acls}
    for user in alive_it(users):
        if user.username == "avnadmin":
            continue
        if not any(fnmatch(user.username, pattern) for pattern in acl_patterns):
            yield user.username


def main(project, dry_run):
    aiven = AivenKafka(project, dry_run=dry_run)

    service = aiven.get_service()

    acls_to_delete = set(find_unused_acls(service.topics, service.acls))
    print(f"Found {len(acls_to_delete)} ACLs referencing non-existing topics")
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
