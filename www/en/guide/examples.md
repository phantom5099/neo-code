---
title: Usage Examples
description: Four copy-pasteable scenarios to help you get comfortable with NeoCode quickly.
---

# Usage Examples

All examples below are based on NeoCode's currently implemented TUI, tools, and Gateway capabilities. You can copy them directly into a session.

---

## Scenario 1: Understand an unfamiliar project

**Goal**: You just cloned a repo and want a quick overview of the directory structure and module responsibilities.

**Prompt**:

```text
Please read the current project directory structure and summarize each module's responsibilities. If you see an internal/ directory, explain the boundaries of each subpackage in particular.
```

**Expected behavior**:

- NeoCode will call file-read tools to scan directories
- Provide a structured summary of module responsibilities
- If the tree is deep, it may ask whether to continue expanding

**Key decision points**:

- If it requests reading many files, choose **Allow** (read-only, no risk)
- If it suggests running `find` or `tree`, choose **Allow** (read_only classification)

---

## Scenario 2: Locate and fix a bug

**Goal**: A test fails and you want to find the root cause and a fix.

**Prompt (step 1 — provide context)**:

```text
I see the following test failure. Please locate the root cause and propose a fix:
```

Paste the failure log after this line.

**Prompt (step 2 — ask it to apply)**:

```text
Please modify the corresponding file to fix this issue and describe the verification steps. Only change one file.
```

**Expected behavior**:

- Read the relevant source and test files
- Pinpoint the specific function or logic error
- Provide a code diff with the fix
- Remind you how to run the test to verify

**Key decision points**:

- When it requests file modifications, choose **Ask** or **Allow** (local_mutation, controllable)
- When it requests running the test command, choose **Allow** (necessary verification step)

---

## Scenario 3: Add tests for a function

**Goal**: Generate unit tests for a function that currently lacks them.

**Prompt**:

```text
Please generate unit tests for the ReadFile function in internal/tools/file.go, covering the happy path, file-not-found, and permission-denied cases. Write the tests to internal/tools/file_test.go.
```

**Expected behavior**:

- Read the target function source to understand inputs and outputs
- Generate corresponding test cases
- After writing the file, ask whether to run verification

**Prompt (verification)**:

```text
Please run the tests for this package. If any fail, analyze and fix them.
```

**Key decision points**:

- File writes are local_mutation; **Ask** is recommended (confirm path and content are correct)
- Running tests is read_only / local_mutation; **Allow** is fine

---

## Scenario 4: Add a new feature / endpoint

**Goal**: Add a new endpoint or feature point to existing code.

**Prompt (design phase)**:

```text
I want to add a health-check endpoint in internal/gateway that returns the current runtime status. Please propose an implementation plan, including which files need to be modified.
```

**Prompt (implementation phase)**:

```text
Please implement according to the plan, keeping the style consistent with existing code. After finishing, run go build ./... to verify compilation passes.
```

**Expected behavior**:

- Analyze the existing gateway route registration pattern
- Add a new handler and register the route
- Run the build to verify

**Key decision points**:

- For multi-file modifications, **Ask** each time to confirm
- Running `go build` is read_only (verification only), **Allow** is fine

---

## Next steps

- Permission decision details: [Tools & permissions](./tools-permissions)
- Configure models and providers: [Configuration](./configuration)
- Daily operations: [Daily use](./daily-use)
- Something wrong: [Troubleshooting](./troubleshooting)
