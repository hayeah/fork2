- If you are in a git work tree, YOU MUST not cd out of the work tree into the root repo, or other work trees.
  - The work trees are typically named `.forks/{project_name}_001`, `{project_name}_002`. DO NOT cd or make changes out of these by accident.

# Commit

- Once you are satisfied with the code, commit it.
- Always run pre-commit to clean up before commit.
- If I ask you to make changes, create a new commit after you are happy with the changes.

## Pre Commit

- go format

## Commit Messages

- MUST NOT include:

```
🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```
