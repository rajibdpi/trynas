"""Reserve the next stable release tag, or reuse this commit's existing tag."""

import os
import re
import subprocess


def select_tag(tags, commit):
    versions = []
    for name, sha in tags:
        match = re.fullmatch(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", name)
        if match:
            versions.append((tuple(map(int, match.groups())), name, sha))
    existing = [entry for entry in versions if entry[2] == commit]
    if existing:
        return max(existing)[1], True
    if not versions:
        return "v1.0.0", False
    major, minor, patch = max(versions)[0]
    return f"v{major}.{minor}.{patch + 1}", False


def reserve_tag(repo, commit):
    for _ in range(5):
        listing = subprocess.run(
            ["gh", "api", "--paginate", f"repos/{repo}/tags?per_page=100",
             "--jq", '.[] | [.name, .commit.sha] | @tsv'],
            check=True, capture_output=True, text=True,
        )
        tags = [line.split("\t") for line in listing.stdout.splitlines() if line]
        tag, exists = select_tag(tags, commit)
        if exists:
            return tag
        result = subprocess.run(
            ["gh", "api", "--method", "POST", f"repos/{repo}/git/refs",
             "-f", f"ref=refs/tags/{tag}", "-f", f"sha={commit}"],
            capture_output=True, text=True,
        )
        if result.returncode == 0:
            return tag
        # A concurrent release may have reserved the same version. Refresh and retry.
        if "HTTP 422" not in result.stderr:
            raise RuntimeError(result.stderr)
    raise RuntimeError("Could not reserve a release tag after five attempts")


if __name__ == "__main__":
    tag = reserve_tag(os.environ["GH_REPO"], os.environ["GITHUB_SHA"])
    with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as env:
        env.write(f"RELEASE_TAG={tag}\n")
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
            output.write(f"tag={tag}\n")
    print(f"Release tag: {tag}")
