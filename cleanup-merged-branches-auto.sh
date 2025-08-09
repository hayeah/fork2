#!/bin/zsh

zsh -e -o pipefail <<'ZSH'
echo "Step 1: Finding branches that have been merged into master..."

# Get the current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "Current branch: $CURRENT_BRANCH"

# Get list of merged branches, excluding master and the current branch
MERGED_BRANCHES=$(git branch --merged master | grep -v "^\*" | grep -v "master" | sed 's/^[ \t]*//')

# Check if we found any merged branches
if [[ -z "$MERGED_BRANCHES" ]]; then
  echo "No merged branches found to delete."
  exit 0
fi

# Show branches that will be deleted
echo "\nThe following branches will be deleted:"
echo "$MERGED_BRANCHES" | sed 's/^/  - /'

echo "\nStep 2: Deleting merged branches..."

# Delete each branch
for BRANCH in $MERGED_BRANCHES; do
  echo "Deleting branch: $BRANCH"
  git branch -d "$BRANCH"
done

echo "\nStep 3: Cleanup complete!"
echo "All merged branches have been deleted."

# Show remaining branches
echo "\nRemaining branches:"
git branch

# Optional: Check for remote branches that could be cleaned up
echo "\nStep 4: Checking for remote branches that could be cleaned up..."
git remote prune origin --dry-run

echo "\nTo remove these stale remote-tracking branches, run:"
echo "git remote prune origin"

# Optional: To also delete the remote branches themselves
echo "\nTo delete the remote branches themselves (if you have permission), you would run:"
echo "git push origin --delete BRANCH_NAME"
ZSH
