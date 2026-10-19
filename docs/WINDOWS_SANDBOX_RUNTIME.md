# Windows Sandbox Runtime

## Goal

ADM must let an Agent build, test and debug project code without granting that
code the interactive Windows user's filesystem authority.

The security target is both confidentiality and integrity:

- project code may read/write the current temporary managed worktree;
- project code may read/write ADM-owned sandbox cache/build/profile directories;
- explicitly approved tool directories may be read/executed;
- the interactive user's profile, other projects, SSH material, browser data,
  ADM Admin configuration and unrelated drives must not become readable merely
  because a script runs;
- the whole child process tree must remain inside the execution boundary.

## Why a restricted token alone is not enough

Restricted tokens, Low Integrity and Job Objects are useful defense-in-depth,
but they are not by themselves the strong read boundary ADM needs. A process
that still runs as the interactive user can retain read authority that is
unacceptable for an Agent development sandbox.

The strong Windows design therefore uses a dedicated local sandbox identity.
NTFS access is granted to that identity only for the task worktree and ADM-owned
runtime directories. Restricted-token/low-integrity hardening may additionally
be applied inside that identity.

## Strong sandbox lifecycle

1. One-time Admin setup creates an ADM-owned local sandbox account with a random
   credential. The account is not an administrator and is not used interactively.
2. The credential is stored encrypted using Windows-protected local storage; it
   is never injected into Agent-visible state or process environment.
3. For each execution ADM creates private profile/temp/cache/build directories.
4. ADM grants the sandbox SID only the minimum required access to:
   - current temporary managed worktree: modify;
   - private sandbox runtime directories: modify;
   - explicitly approved tool roots: read/execute.
5. ADM constructs a sanitized environment rather than inheriting the interactive
   user's environment wholesale.
6. The process starts as the sandbox identity and is attached to a Job Object.
   Closing/cancelling the execution terminates the owned process tree.
7. ACL grants that were created for the execution are revoked after the process
   tree is gone.
8. Agent execution fails closed if strong sandbox setup is unavailable. An
   unsandboxed host fallback remains an explicit Admin decision, never an
   automatic fallback.

## Environment policy

The sandbox keeps only operating-system/tool-discovery variables that are
needed for normal development. User profile locations and build caches are
redirected into ADM-owned directories.

Variables that commonly carry credentials or inject startup code are not
inherited by default. Examples include cloud tokens, SSH agent sockets,
database URLs, NODE_OPTIONS, shell startup hooks and user Git configuration.

This policy intentionally prefers a clean build environment over silently
sharing the user's secrets.

## Development compatibility

System-wide Go/Git/Node/Java installations should normally remain readable and
executable to the sandbox identity. Tools installed only inside the interactive
user's private profile require an explicit tool-root grant or a separate
sandbox-visible installation.

Caches are private to the sandbox by default. Shared read-only caches can be
added later after their mutation and symlink behavior is audited.

## Networking

The first strong-filesystem milestone does not claim network isolation.
Dependency download access can remain available, but the sandbox receives no
user credentials by default. Docker daemons, SSH agents and other privileged
local IPC endpoints are separate capabilities and must not be inherited
implicitly.

Network allowlisting/firewall isolation is a later layer.

## Rollout

The sandbox is introduced behind a capability/status seam first. Existing
Verifier execution is not switched until Windows end-to-end tests prove:

- current worktree read/write succeeds;
- another ADM workspace cannot be read or modified;
- the real user profile cannot be read;
- Go test/build and Wails build work;
- child processes are terminated on cancellation;
- secrets are absent from the child environment;
- setup/teardown survives restart and upgrade.

Only after those tests pass should Agent Verifier execution default to the
strong sandbox.
