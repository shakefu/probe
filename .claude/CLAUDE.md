# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Terraform provider written in Go using the [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework). The provider checks whether AWS resources exist without failing when they don't, using native AWS SDK calls.

See `context/terraform-provider-probe-design.md` for the full design document.

## Development Commands

### Building

```bash
go build                              # Build the provider binary
go build -o terraform-provider-probe  # Build with explicit output name
go install                            # Install to $GOPATH/bin
```

### Testing

Uses Go's standard testing package with [terraform-plugin-testing](https://developer.hashicorp.com/terraform/plugin/testing) for acceptance tests.

```bash
# Unit tests
go test ./...                         # Run all tests
go test -v ./...                      # Verbose output
go test -run 'TestName' ./...         # Run specific test

# Acceptance tests (creates real resources or uses LocalStack)
TF_ACC=1 go test ./... -v             # Run acceptance tests

# Test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out      # View HTML report
go tool cover -func=coverage.out      # Coverage by function
```

### Local Provider Testing

To test the provider locally with Terraform:

```bash
# Build and install to local plugin directory
go build -o ~/.terraform.d/plugins/registry.terraform.io/shakefu/probe/0.1.0/darwin_arm64/terraform-provider-probe

# Or use dev overrides in ~/.terraformrc:
provider_installation {
  dev_overrides {
    "shakefu/probe" = "/path/to/terraform-provider-probe"
  }
  direct {}
}
```

## Agent Session Protocol

Each session starts with no memory of previous work. Follow this protocol:

1. **Verify clean state**: Run `git status`, `git stash list`, check for unpushed commits. Ask user if any pending changes exist.
2. **Create feature branch**: Checkout main, pull latest changes on main branch, create branch `<type>/<description>-<session-id>`
3. **Start baseline tests**: Run `go test ./...` with `run_in_background: true` (skip for docs/context-only changes)
4. **Find current task**: Read `context/current-task.json` for active work
5. **Review recent history**: Run `git log --oneline -5`
6. **Execute**: Find first task where `status=pending` and `blocked_by=null`, complete it, update status

**Using background tests**: After making code changes, use `BashOutput` to check results. Start a new background test run after edits to verify changes.

**Key context files:**

- `context/current-task.json` - Active task and plan file
- `context/terraform-provider-probe-design.md` - Provider design document

**Archived files** (in `context/completed/`): completed implementation plans

### Task Stash Stack

The `context/` directory is not in version control (gitignored). Task files are local working state only.

When interrupted by higher-priority work:

**Push** (preserve current, start new):

1. Rename `current-task.json` → `current-task-stash{N}.json` (skip if current task is null/empty)
2. Create new `current-task.json` with new task
3. **Do not start the new task** without confirmation - user typically wants fresh sessions

**Pop** (resume after completing current):

1. Delete/clear `current-task.json`
2. Rename highest-numbered stash back to `current-task.json`

Stack order: highest number = oldest task.

### Archiving Completed Plans

When all tasks in a plan are complete:

1. `mv context/<plan>.json context/completed/`
2. Update `current-task.json` to next plan or clear

## Agent Effectiveness Guidelines

Based on [Anthropic's "Effective Harnesses for Long-Running Agents"](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents):

- **Follow recommendations precisely** - Read entire sources before proposing solutions; don't paraphrase without justification
- **If corrected, acknowledge and fix** - Don't defend substitutions that contradict the source
- **Work on one task at a time** - Avoid scope creep and doing too much at once
- **Documentation is part of implementation** - When adding functionality, update all related docs (module docs, function docs, user-facing docs) in the same change. Don't defer documentation to later.

### Code Search Tools

Prefer the cgc MCP tools (`mcp__cgc__*`) over built-in Glob/Grep for finding code:

- `mcp__cgc__find_code` - Find functions, classes, or code by keyword
- `mcp__cgc__analyze_code_relationships` - Find callers, callees, class hierarchy, etc.

Use built-in tools (Glob, Grep, Read) for simple file lookups or when cgc isn't indexed.

### Use JSON for Structured Tracking

- **Use JSON, not YAML or markdown** for task lists and progress tracking
- **Do not substitute formats** - If a reference says "use JSON", use JSON
- **Include explicit status fields**: `"status": "pending|in_progress|complete"`
- **Add step-by-step descriptions** that can be verified

Example structure:

```json
{
  "plan_name": "Feature or Project Name",
  "last_updated": "YYYY-MM-DD",
  "session_startup": [
    "Run git status to check branch",
    "Run go test ./... to verify baseline",
    "Read this file to find current task",
    "Find first task where status=pending and blocked_by=null"
  ],
  "tasks": [
    {
      "id": "task-id-kebab-case",
      "name": "Human readable task name",
      "status": "pending",
      "priority": 1,
      "blocked_by": null,
      "output_file": "path/to/output.go",
      "steps": [
        "Concrete step 1 with specific action",
        "Concrete step 2 with verification command"
      ],
      "acceptance_criteria": [
        "File exists at expected path",
        "Tests pass: go test ./...",
        "No lint warnings"
      ]
    },
    {
      "id": "dependent-task",
      "name": "Task that depends on first",
      "status": "pending",
      "priority": 2,
      "blocked_by": "task-id-kebab-case",
      "steps": ["..."],
      "acceptance_criteria": ["..."]
    }
  ],
  "notes": [
    "Tasks with blocked_by=null can run in parallel",
    "Update status to complete and last_updated when done"
  ]
}
```

### Plan-to-Plan Pattern

When a task requires reading excessive files/lines that would consume too much context, break it into sub-plans.

**How it works:**

1. Parent plan includes task: `"Create sub-plan for X"` → complete when sub-plan file created
2. Next parent task: `"Complete sub-plan-X.json"` → stays `in_progress`
3. Update `current-task.json` to point to sub-plan
4. Work through sub-plan tasks
5. When sub-plan is done: archive it, update `current-task.json` back to parent, mark parent task complete

**Rules:**

- Sub-plans are functionally separate (no parent references needed)
- Archive sub-plans separately when complete
- Nesting depth is unlimited, but confirm with the user at 3+ levels deep

### Feature Completion Criteria

Do not mark features complete prematurely:

1. **Tests exist and pass** - Both unit and acceptance tests where applicable
2. **All acceptance criteria met** - Check each explicitly
3. **Tests actually run and pass** - Run `go test ./...`, don't assume
4. **Documentation updated** - Add new features to relevant docs

### Zero Narration

Do not narrate actions. Tool calls are structured output - the user sees them directly. Text output wastes context.

**Never output:**

- Action announcements ("Let me...", "I'll now...", "I'm going to...")
- Summaries of what was done
- Confirmations of success (visible from tool output)
- Explanations of routine operations

**Only output text when:**

- Asking a question that requires user input
- Reporting an error that blocks progress
- A decision point requires user choice

Otherwise: execute silently.

## Code Quality & Pre-commit

Pre-commit hooks (configured in `.pre-commit-config.yaml`) automatically run:

- `go-fmt` and `go-mod-tidy`
- Conventional commit validation
- Terraform fmt/validate
- JSON/YAML/markdown checks

**Before every commit**, run:

```bash
pre-commit run --all-files    # Run all hooks
go fmt ./...                  # Format Go code
go vet ./...                  # Run go vet
go mod tidy                   # Clean up dependencies
```

## Commit Message Requirements

All commits must follow **conventional commits**:

```text
<type>(<scope>): <description>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

Examples: `feat: add S3 bucket support`, `fix: handle nil properties`, `docs: update README`

Breaking changes: `feat!: description` or `BREAKING CHANGE:` in footer

## Committing Guidelines for Claude Code

1. **Run pre-commit before every commit** - Catches formatting and linting issues
2. **NEVER commit/push without explicit user approval**
3. **Avoid hardcoding values that change** - No version numbers, dates, or timestamps in tests
4. **When fixing tests** - Understand what's being validated, fix the underlying issue
5. **Keep summaries brief** - 1-2 sentences, no code samples unless requested
6. **NEVER add "co-authored-by" to commit messages**

## Documentation Style Guide

- Follow [Go documentation comments](https://go.dev/doc/comment) and [Effective Go](https://go.dev/doc/effective_go)
- No emojis or hype language
- No specific numbers that will change (versions, coverage percentages)
- Review for consistency and accuracy when done

### Go Documentation Comments

Go uses special comment conventions for documentation:

- Package comments: `// Package name provides...` at the top of the file
- Exported symbols: Comments starting with the symbol name
- Examples: `func ExampleFunctionName()` for executable examples

```bash
go doc ./...                    # View documentation for all packages
go doc internal/provider        # View documentation for provider package
```

## Important Notes

- Binary name is `terraform-provider-probe`
- Provider address is `registry.terraform.io/shakefu/probe`
- Uses native AWS SDK calls for resource existence checks
- Automatically detects LocalStack at `localhost:4566`
- Use `go mod tidy` to clean up dependencies
- Use `go mod verify` to verify dependencies haven't been modified
