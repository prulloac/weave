# BDD: Feedback Controls

Human-readable behavior cases for quiet, verbose, progress and ASCII output.

**Spec reference:** [optimization.md § O2](../specs/optimization.md)
**Go tests:** `cmd/weave/feedback_bdd_test.go`, `cmd/weave/progress_test.go`

---

## Feature: Quiet

### Scenario: Successful human output is suppressed

- **When** I run `weave list <path> --quiet`
- **Then** the command exits `0`
- **And** stdout and stderr are empty

### Scenario: Errors still print

- **When** I run `weave show <path> missing --quiet`
- **Then** the command exits `1`
- **And** stderr contains the error and hint

### Scenario: No match stays silent

- **When** I run `weave search <path> zzzz --quiet`
- **Then** the command exits `1`
- **And** stdout and stderr are empty

### Scenario: JSON is never suppressed

- **When** I run `weave version --quiet --json`
- **Then** stdout is one envelope

### Scenario: Quiet and verbose are exclusive

- **When** I pass `--quiet --verbose`
- **Then** the command exits `2`

---

## Feature: Verbose

### Scenario: Paths, database, cache and timing

- **When** I run a command with `--verbose`
- **Then** stderr names the bundle
- **And** names the database or `--no-db`
- **And** names the cache state
- **And** prints elapsed time

### Scenario: Sources are listed

- **When** I use `--source b=<path> --verbose`
- **Then** stderr lists the source alias and path

---

## Feature: Progress

### Scenario: TTY write progress

- **When** a write command runs on a TTY
- **Then** stderr shows an elapsed progress line
- **And** the line is cleared when the command finishes

### Scenario: No progress off-TTY or with JSON

- **When** stderr is not a terminal or `--json` is set
- **Then** no progress line is written

---

## Feature: ASCII and color

### Scenario: ASCII arrows

- **When** I pass `--ascii`
- **Then** human output uses `->` instead of `→`
- **And** uses `|` instead of `·`

### Scenario: NO_COLOR

- **When** `NO_COLOR` is set
- **Then** output contains no ANSI escapes
- **And** meaning never depends on color
