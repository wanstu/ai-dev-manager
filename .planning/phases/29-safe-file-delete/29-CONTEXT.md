# Phase 29 Context — Safe File Delete

## Why this phase exists

Post-restart Phase 28 dogfood exposed the next concrete Agent-facing gap. After creating a temporary CRLF file through `@pjadm write` and validating `@pjadm edit`, the Agent had no file-delete tool and had to fall back to structured `git clean -f -- <path>` solely to remove the temporary file.

That workaround is evidence of a missing basic development primitive rather than a reason to expand shell/Git usage. Normal code changes also need to remove obsolete source/config/test files without routing through Git-specific cleanup commands.

## Product boundary

Phase 29 adds one narrow mutation primitive: delete one file inside the validated Environment.

It does **not** add recursive directory deletion, rename/move, arbitrary filesystem commands, trash/recycle-bin semantics, or automatic Git staging/commit behavior.

Deletion remains subject to the same Runtime write policy, workspace containment, blocked-path rules and Environment single-writer ownership as `write` and `edit`.

## Safety rules

- read-only Runtime cannot delete;
- path must stay inside the Runtime workspace;
- `.git/**` and `.ai-dev-manager/runtime/**` remain blocked;
- deleting the workspace root is forbidden;
- ordinary directories are rejected rather than recursively removed;
- a missing path returns the existing structured `not_found` Runtime error;
- only a single filesystem entry is removed per call;
- Gateway mutation still requires the current `writer_owner` and only successful deletion renews writer/activity.

A symlink path may only be removed when it passes the existing write-target containment policy; deletion removes the link entry itself, not the linked target.

## Success path

A real external Agent should be able to:

1. `write` a temporary file;
2. `delete` that file using only `environment_id + writer_owner + path`;
3. observe the file disappear via `read`/`git_status`;
4. receive structured failures for directory, blocked, outside-workspace and missing-path cases;
5. run the configured project verifier afterward.

## Non-goals

- no recursive delete;
- no directory tree cleanup API;
- no rename/move in this phase;
- no fuzzy path matching;
- no Git staging/commit/push automation;
- no Process/log/port work.

## Exit criteria

Phase 29 is complete when `files.delete` exists as a capability from Native Runtime through the ADM Gateway, is writer-guarded and path-policy-safe, focused tests and full verifier/race/vet/build gates pass, and post-restart real `@pjadm` dogfood can create then delete a file without using `exec`/Git cleanup.
