param(
    [string]$Go = "go"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root ".test-bin"
$daemonExe = Join-Path $binDir "ai-dev-manager-network-daemon.exe"
$testExe = Join-Path $binDir "ai-dev-manager-network-tests.exe"

New-Item -ItemType Directory -Force -Path $binDir | Out-Null

function Invoke-NetworkTestPackage {
    param(
        [Parameter(Mandatory = $true)][string]$Package,
        [Parameter(Mandatory = $true)][string]$Pattern
    )

    & $Go test -c -o $testExe $Package
    if ($LASTEXITCODE -ne 0) {
        throw "failed to build fixed network test binary for $Package"
    }

    & $testExe "-test.run=$Pattern" "-test.count=1"
    if ($LASTEXITCODE -ne 0) {
        throw "network acceptance tests failed for $Package"
    }
}

Push-Location $root
try {
    & $Go build -o $daemonExe ./cmd/ai-dev-manager
    if ($LASTEXITCODE -ne 0) {
        throw "failed to build fixed network acceptance daemon"
    }

    $oldAcceptance = $env:ADM_NETWORK_ACCEPTANCE
    $oldDaemonExe = $env:ADM_TEST_DAEMON_EXECUTABLE
    try {
        $env:ADM_NETWORK_ACCEPTANCE = "1"
        $env:ADM_TEST_DAEMON_EXECUTABLE = $daemonExe

        # All real-listener tests are compiled to the same stable test-exe path
        # instead of Go's per-run Temp\go-build\*.test.exe paths. The production
        # daemon used by cross-process CLI tests also has its own stable path.
        # This preserves the network coverage while avoiding a new Windows
        # firewall/application authorization identity on every `go test` run.
        Invoke-NetworkTestPackage ./cmd/ai-dev-manager '^TestCLI(ForegroundServeExposesWorkspaceAndStopsOnCancel|DaemonLifecycleAcrossProcesses|PersistentRuntimeLifecycleAcrossDaemon|RuntimeDockerExposurePersistsAcrossDaemonRestart|DaemonCleanRestartReconcilesDesiredRuntimes|DaemonCrashRestartReclaimsStaleLeaseAndReconciles|UpDownPSAndCtlRestart|CtlShutdownClearsDesiredRuntimes|AgentRunLifecycleAcrossInvocationsAndRestart|AgentVerifyWorkflowAcrossInvocations|AgentGSDWorkflowAcrossInvocations|AgentParallelVerifyWorktreesAcrossInvocations|EnvironmentLifecycleAcrossDaemonRestart|EnvironmentIncludeChangesAndForceAcrossRestart|EnvironmentWriterAndBaseFactsAcrossRestart|GatewayDiscoveryStableAcrossDaemonRestart|GatewayDockerExposureAcrossDaemonRestart|GatewayEnvironmentLifecycleAcrossRestart|GatewayWriterSafeMutationExecAndVerifyAcrossRestart)$'
        Invoke-NetworkTestPackage ./internal/host '^TestManager(RunsTwoIsolatedWorkspaceMCPInstances|RejectsNonLoopbackAndDuplicateInstanceID|HostsExternalRuntimeContract)$'
        Invoke-NetworkTestPackage ./internal/controlplane '^TestService(RunsTwoPersistedWorkspaceMCPInstances|ConfiguredMCPActivationHonorsWorkspaceDisable)$'
        Invoke-NetworkTestPackage ./internal/daemon '^Test(RuntimeOwner.*|GatewayOwner.*|RunHealthStopAndCleanup)$'
        Invoke-NetworkTestPackage ./internal/mcp '^TestActivatorProbesStreamableHTTP$'
    }
    finally {
        if ($null -eq $oldAcceptance) { Remove-Item Env:ADM_NETWORK_ACCEPTANCE -ErrorAction SilentlyContinue } else { $env:ADM_NETWORK_ACCEPTANCE = $oldAcceptance }
        if ($null -eq $oldDaemonExe) { Remove-Item Env:ADM_TEST_DAEMON_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:ADM_TEST_DAEMON_EXECUTABLE = $oldDaemonExe }
    }
}
finally {
    Pop-Location
}
