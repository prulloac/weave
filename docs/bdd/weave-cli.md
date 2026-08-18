# BDD: Weave CLI

Human-readable behavior test cases for the Weave CLI commands (`mount`, `unmount`, `status`, meta).

**Spec reference:** [weave-cli.md](../specs/weave-cli.md)
**Playwright spec:** `tests/e2e/weave-cli.spec.ts`

---

## Feature: Mount a Bundle via Browser

As a user, I want to mount an OKF bundle and browse the resulting explorer in my browser.

### Scenario: Browse the mounted graph

- **Given** the Weave CLI has mounted the test fixture bundle on port 4322
- **When** I navigate to `http://localhost:4322/`
- **Then** I see the heading "OKF Bundle Explorer"
- **And** the concept count is displayed as 5
- **And** the concept card for `concepts/okf-bundle` is visible
- **And** at least one index entry is visible

---

## Feature: Mount Error Handling

As a user, I want clear error messages when something goes wrong with mounting.

### Scenario: Mounting a nonexistent path fails with exit code 2

- **Given** a path that does not exist on the filesystem
- **When** I run `weave mount <nonexistent-path>`
- **Then** the process exits with code 2

---

## Feature: Port Conflict Resolution

As a user, I want Weave to automatically find a free port if my requested port is busy.

### Scenario: Mounting on a busy port picks the next free port

- **Given** a port is already in use by another process
- **When** I run `weave mount <path> --port <busy-port>`
- **Then** the server starts on a different port than the one requested
- **And** the URL reflects the actual port used
- **And** unmounting the actual port succeeds

---

## Feature: Status Lists Active Mounts

As a user with multiple mounts, I want to see which ports are active and where they point.

### Scenario: List active mounts

- **Given** a bundle is currently mounted
- **When** I run `weave status`
- **Then** the output contains the mount URL
- **And** the output contains the port number

---

## Feature: Unmount Preserves Target Repository

As a user, I want confidence that Weave never modifies my original files.

### Scenario: Unmount leaves the target untouched and removes artifacts

- **Given** a Git repository with tracked files
- **When** I mount the repository, then unmount it
- **Then** `git status --porcelain` produces the same output as before mounting
- **And** file `mtime` values are unchanged
- **And** the temporary artifact directory no longer exists
- **And** the unmount output reports the removed artifact path

---

## Feature: Unmount Unknown Port

As a user, I want a clear response when trying to unmount a port that isn't mounted.

### Scenario: Unmounting an unknown port exits with code 5

- **Given** no mount is running on a specific port
- **When** I run `weave unmount <unknown-port>`
- **Then** the process exits with code 5

---

## Feature: Version and Help Output

As a user, I want to verify the CLI version and see usage instructions.

### Scenario: --version and --help exit cleanly

- **When** I run `weave --version`
- **Then** the process exits with code 0
- **And** the output contains `weave 0.0.1`

- **When** I run `weave --help`
- **Then** the process exits with code 0
- **And** the output contains a usage message
