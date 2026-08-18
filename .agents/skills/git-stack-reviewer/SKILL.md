---
name: git-stack-reviewer
description: Manages stacked pull requests using GitHub CLI (`gh stack`), isolates feature pipelines with Git worktrees, and handles atomic conventional commits.
---

# Git, Worktree & Stacked PR Protocol

You are in **Git & Release Operations Mode**. Your goal is to keep the working environment clean using Git worktrees, structure pipeline deliverables into linear stacked PRs (`gh stack`), and maintain atomic commit hygiene.

## 1. Worktree Environment Isolation

Use Git worktrees to isolate separate features, spikes, or concurrent pipeline stages without dirtying the primary working tree or switching branches mid-task.

- **Create Feature Worktree:**
  ```bash
  git worktree add ../<feature-name> -b <feature-branch>
  ```

- **Inspect Active Worktrees:**

  ```bash
  git worktree list
  ```

- **Clean Up Worktree:** After merging or completing work, detach and delete the worktree folder:

  ```bash
  git worktree remove ../<feature-name>
  ```

## 2. Stacked Pull Requests (gh stack)

Break down large SDD → BDD → TDD pipelines into small, dependent, and easily reviewable pull requests using GitHub's native stacked PR commands.

- **Stack Layering Strategy:**

  - **Layer 1 (Spec):** `docs(spec): add specification contract for <feature>`

  - **Layer 2 (BDD):** `docs(bdd): add BDD test cases for <feature>`

  - **Layer 3 (E2E):** `test(e2e): add failing behavior tests for <feature>`

  - **Layer 4 (TDD):** `feat(<scope>): implement component and pass unit tests`

- **GitHub Stack CLI Operations:**

  - **Save Stack Layer:** Save local changes to the current stack step:

    ```bash
    gh stack save -m "docs(spec): add spec for <feature>"
    ```

  - **View Stack Status:** Check the current stack state and linked branches:

    ```bash
    gh stack status
    ```

  - **Submit Stack:** Push all layers and generate stacked pull requests on GitHub:

    ```bash
    gh stack submit
    ```

  - **Sync Stack:** Rebase and sync stacked branches after upstream reviews or edits:

    ```bash
    gh stack sync
    ```

## 3. Atomic Staging & Conventional Commits

- **Stage Selectively:** Never blindly run git add .. Stage only the exact files tied to the current pipeline phase or stack layer.

- **Commit Formatting:**

  - `docs(spec): ...`
  - `test(e2e): ...`
  - `test(unit): ...`
  - `feat(<scope>): ...`
  - `refactor(<scope>): ...`
  - `fix(<scope>): ...`

## Hand-off Criteria

Verify that all worktrees are cleanly unmounted when done, commits are atomic, and stacked PRs are pushed and linked via gh stack submit.
