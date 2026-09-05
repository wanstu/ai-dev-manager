# Phase 28 Context — Agent File Editing Ergonomics

## Why this phase exists

Phase 27 closed the project-toolchain/verifier gap and the real external Agent can now finish the edit → verifier loop through `@pjadm`. Continued Windows dogfood exposed the next concrete friction point in the Agent-facing file surface: exact edit is byte/newline exact, while Agent-provided text commonly uses LF even when the repository file is CRLF.

Observed failure:

```text
file bytes use CRLF
Agent old_text uses LF
        ↓
exact edit
        ↓
invalid_edit
```

This is not evidence for fuzzy patching. The desired behavior is a narrow newline-compatibility rule around the existing exact-edit contract.

## Product boundary

`edit` remains an exact replacement operation. Phase 28 must not introduce fuzzy matching, whitespace normalization, approximate context matching, patch heuristics, or silent replacement-count relaxation.

The only tolerated equivalence is a clearly identifiable LF ↔ CRLF representation difference. The write must preserve the target file's existing newline style.

## Required behavior

For a file with a consistent newline style:

- exact byte/string match continues to win unchanged;
- when exact `old_text` has zero matches, ADM may retry only by adapting LF ↔ CRLF to the file's existing style;
- the adapted `old_text` must still match exactly `expected_replacements` times;
- adapted `new_text` must use the file's existing newline style;
- files with mixed newline styles do not receive compatibility fallback;
- bare-CR text is not normalized;
- whitespace, indentation and all non-newline bytes remain exact.

If exact matching finds a non-zero but unexpected number of replacements, return `invalid_edit` directly rather than trying newline adaptation. This keeps `expected_replacements` strict and avoids masking ambiguous edits.

## Why ranged read is not part of this phase

A fresh dogfood read of the ~104 KB `main_test.go` succeeds under the existing read limit, so there is no evidence-backed need for ranged read in Phase 28. Large-file read ergonomics should remain deferred until a real limit blocks a normal coding task.

## Success path

1. Unit tests cover CRLF file + LF request and LF file + CRLF request.
2. Replacement output preserves the file newline style.
3. Multiple-match / wrong `expected_replacements` remains rejected without modifying the file.
4. Mixed-newline and non-newline whitespace differences remain rejected.
5. Existing runtime/gateway edit behavior remains compatible.
6. Full test/race/vet/build gate passes.

## Non-goals

- fuzzy patch or approximate context matching;
- whitespace-insensitive edits;
- automatic formatter invocation;
- generic patch language;
- ranged read without new dogfood evidence;
- Process/logs/ports/dev-server work.
