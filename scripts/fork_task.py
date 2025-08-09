#!/usr/bin/env python3
"""
fork_task.py  —  Copy the current .tasks/<timestamp>.<taskName>.md file
                 to .tasks/<NOW>.<taskName>.md

Usage:  python fork_task.py <absolute-path-to-current-file>
"""

import re
import shutil
import sys
import subprocess
from datetime import datetime
from pathlib import Path

def dasherize(text):
    """
    Convert a string to a file path friendly format by replacing non-alphanumeric
    characters with dashes and removing consecutive dashes.
    """
    # Replace non-alphanumeric characters (except dashes) with dashes
    dasherized = re.sub(r'[^a-zA-Z0-9-]', '-', text)
    # Replace consecutive dashes with a single dash
    dasherized = re.sub(r'-+', '-', dasherized)
    # Remove leading and trailing dashes
    dasherized = dasherized.strip('-')
    return dasherized


def get_current_commit():
    """
    Get the current commit hash using git command
    """
    try:
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            check=True
        )
        return result.stdout.strip()
    except subprocess.CalledProcessError:
        return "unknown"


def update_frontmatter_with_commit(file_path, commit_hash):
    """
    Update the frontmatter of a file with the commit hash.
    If the commit field exists, update it. Otherwise, add it as the last line of the frontmatter.
    """
    with open(file_path, 'r') as f:
        content = f.read()

    # Check if the file has frontmatter (enclosed in ```)
    frontmatter_match = re.search(r'^```(.+?)```', content, re.DOTALL)
    if not frontmatter_match:
        return

    frontmatter = frontmatter_match.group(1)
    commit_match = re.search(r'^commit\s*=\s*"(.+?)"', frontmatter, re.MULTILINE)

    if commit_match:
        # Update existing commit field
        updated_frontmatter = re.sub(
            r'^commit\s*=\s*"(.+?)"',
            f'commit = "{commit_hash}"',
            frontmatter,
            flags=re.MULTILINE
        )
    else:
        # Add commit field as the last line of frontmatter
        updated_frontmatter = frontmatter.rstrip() + f'\ncommit = "{commit_hash}"\n'

    # Replace the old frontmatter with the updated one
    updated_content = content.replace(frontmatter_match.group(1), updated_frontmatter)

    with open(file_path, 'w') as f:
        f.write(updated_content)


def main():
    if len(sys.argv) < 2:
        sys.exit("Need a file path argument")

    src = Path(sys.argv[1]).resolve()
    provided_name = sys.argv[2] if len(sys.argv) > 2 else ""

    # derive default task name from the file:
    parts = src.name.split(".")
    default_name = ".".join(parts[1:-1]) if len(parts) >= 3 else ""

    task_name = provided_name or default_name

    # Dasherize the task name to make it file path friendly
    task_name = dasherize(task_name)

    # ── build destination path ------------------------------------------
    now = datetime.now().strftime("%Y-%m-%dT%H-%M")   # e.g. 2025-04-26T10-45
    dest = src.parent / f"{now}.{task_name}.md"

    # ── copy -------------------------------------------------------------
    shutil.copy2(src, dest)

    # ── update frontmatter with commit hash -----------------------------
    commit_hash = get_current_commit()
    update_frontmatter_with_commit(dest, commit_hash)

    # print(f"✅  Created {dest}")

if __name__ == "__main__":
    main()
