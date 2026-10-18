# Git commands

Run these commands from the TryNAS project root.

## Merge `embeded` into `main` and push

Commit your work on `embeded` before switching branches.

```sh
git switch main
git merge --no-ff embeded -m "$(git log -1 --format=%B embeded)"
git push origin main
```

- `git switch main` switches to the main branch.
- `git merge --no-ff embeded` merges `embeded` with a merge commit. The `-m`
  argument uses the latest commit message from `embeded` as the merge message.
- `git push origin main` sends the updated main branch to GitHub. The release
  workflow runs automatically on pushes to `main`.

## Check changes and history

| Task | Command |
| --- | --- |
| Show the current branch and changed files | `git status` |
| Show unstaged changes | `git diff` |
| Show staged changes | `git diff --cached` |
| Show recent commits | `git log --oneline -10` |
| Show the latest commit message on `embeded` | `git log -1 --format=%B embeded` |
| List local branches | `git branch` |
| List local and remote branches | `git branch -a` |
| Show remote URLs | `git remote -v` |

## Work on a branch and commit

```sh
git switch embeded
git status
git diff
git add path/to/file
git diff --cached
git commit -m "Describe the change"
git push -u origin embeded
```

Replace `path/to/file` with the files you want to commit. After the first push
sets the upstream branch, use `git push` for subsequent pushes.

To create a new branch from the current commit:

```sh
git switch -c feature-name
```

## Get updates from GitHub

```sh
git fetch origin
git switch main
git pull --ff-only origin main
```

`git fetch` updates remote branch information. `git pull --ff-only` updates
local `main` only when no merge commit is needed. If it fails, inspect
`git log --oneline --graph --all -20` to understand the diverging history.

## Resolve merge conflicts

If a merge reports conflicts, edit the conflicting files, remove the conflict
markers, then stage the resolved files and finish the merge:

```sh
git status
git add path/to/resolved-file
git merge --continue
```

To cancel an unfinished merge:

```sh
git merge --abort
```

## Temporarily save unfinished work

```sh
git stash push -u -m "Work in progress"
git stash list
git stash pop
```

`-u` includes untracked files. `git stash pop` restores the most recent stash;
resolve any conflicts before continuing.

## Unstage a file

```sh
git restore --staged path/to/file
```

This removes the file from the next commit while keeping its working changes.

## GitHub builds and releases

The workflow is defined in [release.yml](.github/workflows/release.yml).

- Pushing to `main` builds and tests TryNAS, reserves an incremental version
  tag, and publishes the release when the checks succeed.
- After a successful `main` release, the workflow deploys the project landing
  page to https://rajibdpi.github.io/trynas/. Enable **Settings → Pages →
  Build and deployment → GitHub Actions** once before the first deployment.
- Pushing a version tag such as `v1.2.3` builds a draft release.
- The workflow can also run manually for an existing version tag, with an
  option to publish after validation.
- Release notes include download links for Windows, macOS, and Linux on
  AMD64 and ARM64, plus checksums and installation files.

To create and push a version tag, replace `v1.2.3` with an unused version:

```sh
git tag v1.2.3
git push origin v1.2.3
```

See [installation.md](installation.md#github-releases) for release setup and
publishing details.
