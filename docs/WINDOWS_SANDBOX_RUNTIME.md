# Windows Sandbox Runtime

## Goal

ADM must let an Agent build, test and debug project code without granting that
code the interactive Windows user's filesystem authority.

The target is both confidentiality and integrity:

- project code can read/write the selected temporary managed worktree;
- project code can read/write ADM-owned sandbox profile/temp/cache/build roots;
- explicitly approved tool roots can be exposed when a tool is installed in a
  user-private location;
- the interactive user's profile, other projects, SSH material, browser data,
  ADM Admin state and unrelated user files are not implicitly visible;
- the complete child process tree is owned and terminable by ADM.

## Windows 10+ primary boundary: AppContainer

The primary Windows backend uses a classic AppContainer profile. AppContainer
is available on Windows 8+ and provides a restricted process identity whose
access to user resources is default-deny. ADM creates a profile identity for
one sandbox scope and grants that profile SID access only to selected paths.

This is a better confidentiality boundary than launching project code as a
normal secondary local user. A normal local account can still receive access
through broad ACL entries such as Users or Everyone, so a separate account
alone does not satisfy the requirement that project code must not read
unrelated user data.

For the same reason, a Restricted Token by itself is not treated as the file
boundary. Restricted tokens, Low Integrity and mitigation policies remain
useful defense-in-depth.

Windows 11 has newer experimental composable sandbox APIs that can provide
AppContainer plus bound filesystem policy directly. ADM may use those APIs as
an optional backend after feature detection, but the product cannot depend on
them because the supported development host includes Windows 10.

## Sandbox lifecycle

1. ADM derives an AppContainer profile name from the temporary Environment
   identity, so unrelated Agent tasks do not share filesystem grants.
2. ADM creates or opens the profile and records its SID.
3. ADM creates private profile/temp/cache/build directories.
4. The AppContainer SID receives:
   - current temporary managed worktree: read/write/execute;
   - private sandbox runtime roots: read/write/execute;
   - explicit user-private tool roots: read/execute only when required.
5. ADM builds a sanitized environment rather than inheriting the interactive
   user's environment wholesale.
6. ADM launches the build/test/debug process with
   PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES for that AppContainer.
7. The process starts suspended, is attached to a Job Object, and is resumed
   only after the process-tree boundary exists.
8. Cancellation or Gateway shutdown closes/terminates the Job Object.
9. ACL grants and the AppContainer profile are removed when the temporary
   Environment is safely cleaned up.

Verifier integration stays disabled until the complete lifecycle is proven by
Windows end-to-end tests. There is no automatic fallback to normal host
execution when sandbox creation fails.

## Environment policy

The sandbox keeps a small set of operating-system and tool-discovery variables
required by compilers. Profile locations and common build caches are redirected
into ADM-owned directories.

Arbitrary inherited variables are intentionally dropped. This prevents common
credential and startup-code channels such as cloud tokens, SSH agent sockets,
database URLs, NODE_OPTIONS and the interactive user's Git configuration from
silently crossing into project code.

The policy favors a reproducible clean build over ambient user state. A future
secret capability can inject a named secret for one approved Verifier without
turning normal environment inheritance back on.

## Tool compatibility

System-wide Go, Git, Node, Java and similar installations should normally
remain executable because Windows grants AppContainer-compatible read/execute
access to system/application resources. Tools installed only inside the
interactive user's private profile may need a narrow read/execute grant for
their installation root.

ADM will discover required tool roots from the configured Verifier executable;
it will not expose the whole user profile simply to make one tool work.

Go/Wails/Node compatibility is an explicit release gate, not an assumption.

## Filesystem ACL rules

ACL changes are scoped to ADM-owned AppContainer SIDs and selected directories.
ADM never replaces the user's full DACL and never adds a deny ACE for the
interactive user.

The initial implementation uses a unique AppContainer profile per sandbox
scope. This allows cleanup to revoke that SID's access without disturbing user
or system entries.

Before Verifier integration, Windows E2E tests must prove both directions:

- allowed worktree/cache files can be created, read, modified and deleted;
- a sibling ADM project and a file in the real user profile cannot be opened.

## Process boundary

AppContainer controls resource access but process lifetime still needs an
independent boundary. The launcher will use a Job Object with
JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE. The initial process is created suspended,
assigned to the job, then resumed, eliminating the window where a child could
escape before assignment.

GUI/debug support will additionally evaluate Windows desktop/UI restrictions.
Headless test/build is the first target.

## Networking and privileged IPC

Network policy is intentionally separate from the first filesystem milestone.
An AppContainer has no network capability unless ADM grants one. Dependency
download can later use an explicit outbound-network capability or a controlled
dependency-fetch phase.

Docker daemon access, SSH agents, named pipes and other privileged local IPC
are separate capabilities and are never inherited just because an executable
is present on PATH.

## Rollout gates

Existing Verifier execution is not switched until Windows E2E proves:

- current worktree read/write succeeds;
- another ADM workspace cannot be read or modified;
- the real user profile cannot be read;
- sanitized child environment contains no test secret;
- Go test/build works;
- Wails build works;
- Node/Bun build works where configured;
- child and grandchild processes terminate on cancellation;
- ACL/profile cleanup survives failure, restart and upgrade;
- failure is closed: sandbox setup errors never downgrade to normal host
  execution automatically.

Only then should Agent Verifier execution default to this backend.
