#!/bin/zsh

zsh -e -o pipefail <<'ZSH'
echo "Step 1: Finding branches that have been merged into master..."

# Get the current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "Current branch: $CURRENT_BRANCH"

# Get list of merged branches, excluding master
MERGED_BRANCHES=$(git branch --merged master | grep -v "^\*" | grep -v "master" | sed 's/^[ \t]*//')

# Check if we found any merged branches
if [[ -z "$MERGED_BRANCHES" ]]; then
  echo "No merged branches found to delete."
  exit 0
fi

# Show branches that will be deleted
echo "\nThe following branches will be deleted:"
echo "$MERGED_BRANCHES" | sed 's/^/  - /'

# Ask for confirmation
echo "\nStep 2: Confirm deletion"
echo -n "Do you want to delete these branches? (y/N): "
read CONFIRM

if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
  echo "Operation cancelled."
  exit 0
fi

echo "\nStep 3: Deleting merged branches..."

# Delete each branch
for BRANCH in $MERGED_BRANCHES; do
  echo "Deleting branch: $BRANCH"
  git branch -d "$BRANCH"
done

echo "\nStep 4: Cleanup complete!"
echo "All merged branches have been deleted."

# Show remaining branches
echo "\nRemaining branches:"
git branch
ZSH
