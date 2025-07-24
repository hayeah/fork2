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

# Done Notification

- After you are done with a task, notify the user with a concise message.
  - If you wrote code (after lint, test, and commit), run `fork.py status`, which reports:
    - project name
    - workspace number
    - one-line HEAD commit msg
  - If you you finished some task, but new code, write your message.
- exec `say.py --voice shimmer "{project name} {workspace number} is ready for review. {concise msg}."`
  - This will play a voice msg to the user. It shouldn't be too long.
