# Build Task: {{task.title}}

## Story Context

**Story**: {{story.title}}

{{story.description}}

## Your Task

{{task.title}}

{{task.description}}

## Instructions

You are an AI agent performing a build gate for the Loom Kanban board. Your job is to run the project build, report results, and take appropriate follow-up action.

### 1. Prepare the Environment

- Clone or navigate to the repository: `{{context.repo_url}}`
- Work within the directory: `{{context.workdir}}`

### 2. Run the Build

Execute the following build command:

```
{{context.build_command}}
```

Capture the full output (stdout and stderr) of the build command.

### 3. If the Build Succeeds

1. Use **complete_work** to mark this task as **Done**.
2. Add a comment containing:
   - A summary that the build passed.
   - Key build artifacts or output (e.g., binary paths, bundle sizes, warnings).
   - The full build output if it is short, or a truncated version with key details if very long.

### 4. If the Build Fails

1. Use **complete_work** to mark this task as **Done** (the build attempt is complete even though it failed).
2. Add a comment containing:
   - A summary that the build failed.
   - The **full** error output from the build command — do not truncate error messages.
3. Create a new **fix** task:
   - **Type**: `code`
   - **Title**: "Fix build failure"
   - **Description**: Include the error output and a summary of what needs to be fixed.
4. Create a new **rerun build** task:
   - **Type**: `build`
   - **Title**: "Rebuild project"
   - **Description**: "Re-run the build after build fixes are applied."
   - **Dependencies**: This rerun task must depend on the fix task created above.
5. Add a comment on the fix task referencing the failed build task for full context.

## Rerun Scenarios

If this is a rerun build (e.g., after a fix), the last build comment is provided below for reference:

```
{{last_build_comment}}
```

Review the previous failure details, verify the build command runs in a clean state, and report whether the issue has been resolved.

## Important Reminders

- Always run the exact build command specified — do not substitute or modify it.
- A failed build attempt is still a **completed** task; the task status should be **Done**, not Blocked.
- Do not attempt to fix build errors yourself unless a separate fix task has been assigned to you.
