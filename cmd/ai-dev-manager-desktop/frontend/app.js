const elements = {
  refreshButton: document.getElementById('refreshButton'),
  launchAtLogin: document.getElementById('launchAtLogin'), desktopShellHint: document.getElementById('desktopShellHint'),
  worktreeSettingsForm: document.getElementById('worktreeSettingsForm'), worktreeRoot: document.getElementById('worktreeRoot'), worktreeBranchPrefix: document.getElementById('worktreeBranchPrefix'), worktreeSettingsSaveButton: document.getElementById('worktreeSettingsSaveButton'), worktreeSettingsHint: document.getElementById('worktreeSettingsHint'), hostEnvironmentSummary: document.getElementById('hostEnvironmentSummary'), hostEnvironmentRefreshButton: document.getElementById('hostEnvironmentRefreshButton'), loggingStatusSummary: document.getElementById('loggingStatusSummary'),
  statusPanel: document.getElementById('statusPanel'),
  dashboardDataState: document.getElementById('dashboardDataState'), dashboardLastSuccess: document.getElementById('dashboardLastSuccess'), dashboardDataDetail: document.getElementById('dashboardDataDetail'),
  gatewayState: document.getElementById('gatewayState'),
  gatewayBaseURL: document.getElementById('gatewayBaseURL'),
  gatewayHealthURL: document.getElementById('gatewayHealthURL'),
  gatewayURL: document.getElementById('gatewayURL'),
  gatewayAdminURL: document.getElementById('gatewayAdminURL'),
  gatewayProcess: document.getElementById('gatewayProcess'),
  gatewayDetail: document.getElementById('gatewayDetail'),
  gatewayRefreshButton: document.getElementById('gatewayRefreshButton'),
  gatewayStartButton: document.getElementById('gatewayStartButton'),
  gatewayStopButton: document.getElementById('gatewayStopButton'),
  gatewayAllowedHosts: document.getElementById('gatewayAllowedHosts'), gatewayAccessSaveHosts: document.getElementById('gatewayAccessSaveHosts'), gatewayAdminAPIKey: document.getElementById('gatewayAdminAPIKey'), gatewayAccessSetAdminKey: document.getElementById('gatewayAccessSetAdminKey'), gatewayAccessClearAdminKey: document.getElementById('gatewayAccessClearAdminKey'), gatewayAgentAPIKey: document.getElementById('gatewayAgentAPIKey'), gatewayAccessSetAgentKey: document.getElementById('gatewayAccessSetAgentKey'), gatewayAccessClearAgentKey: document.getElementById('gatewayAccessClearAgentKey'), gatewayAccessSummary: document.getElementById('gatewayAccessSummary'),
  workspaceCount: document.getElementById('workspaceCount'), environmentCount: document.getElementById('environmentCount'),
  execCount: document.getElementById('execCount'), mcpCount: document.getElementById('mcpCount'), skillCount: document.getElementById('skillCount'), memoryCount: document.getElementById('memoryCount'),
  workspaceBadge: document.getElementById('workspaceBadge'), environmentBadge: document.getElementById('environmentBadge'), execBadge: document.getElementById('execBadge'),
  mcpBadge: document.getElementById('mcpBadge'), skillBadge: document.getElementById('skillBadge'),
  managementContextPanel: document.getElementById('managementContextPanel'), managementWorkspace: document.getElementById('managementWorkspace'), managementEnvironment: document.getElementById('managementEnvironment'), managementEnvironmentHint: document.getElementById('managementEnvironmentHint'), createEnvironmentButton: document.getElementById('createEnvironmentButton'), editEnvironmentButton: document.getElementById('editEnvironmentButton'),
  runtimeRefreshButton: document.getElementById('runtimeRefreshButton'), runtimeHint: document.getElementById('runtimeHint'), runtimeWriterOwnerInput: document.getElementById('runtimeWriterOwnerInput'), runtimeWriterAcquireButton: document.getElementById('runtimeWriterAcquireButton'), runtimeWriterReleaseButton: document.getElementById('runtimeWriterReleaseButton'), runtimeWriterHint: document.getElementById('runtimeWriterHint'), runtimeSubviewTabs: document.getElementById('runtimeSubviewTabs'), runtimeVerifierCount: document.getElementById('runtimeVerifierCount'), runtimeProcessCount: document.getElementById('runtimeProcessCount'), runtimeRunCount: document.getElementById('runtimeRunCount'), verifierList: document.getElementById('verifierList'), processList: document.getElementById('processList'), runList: document.getElementById('runList'), runtimeOutputMeta: document.getElementById('runtimeOutputMeta'), runtimeOutput: document.getElementById('runtimeOutput'),
  mcpTotalCount: document.getElementById('mcpTotalCount'), mcpDefaultCount: document.getElementById('mcpDefaultCount'), mcpEnvironmentCount: document.getElementById('mcpEnvironmentCount'), mcpIssueCount: document.getElementById('mcpIssueCount'),
  mcpForm: document.getElementById('mcpForm'), mcpName: document.getElementById('mcpName'), mcpTransport: document.getElementById('mcpTransport'), mcpEndpointField: document.getElementById('mcpEndpointField'), mcpEndpoint: document.getElementById('mcpEndpoint'),
  mcpExecutableField: document.getElementById('mcpExecutableField'), mcpExecutable: document.getElementById('mcpExecutable'), mcpArgsField: document.getElementById('mcpArgsField'), mcpArgs: document.getElementById('mcpArgs'),
  mcpAuthField: document.getElementById('mcpAuthField'), mcpAuthMode: document.getElementById('mcpAuthMode'), mcpRefsField: document.getElementById('mcpRefsField'), mcpReferencePairs: document.getElementById('mcpReferencePairs'), mcpEnvRefsField: document.getElementById('mcpEnvRefsField'), mcpEnvReferencePairs: document.getElementById('mcpEnvReferencePairs'),
  mcpHealthEnabled: document.getElementById('mcpHealthEnabled'), mcpAutoReconnect: document.getElementById('mcpAutoReconnect'), mcpProbeTimeoutField: document.getElementById('mcpProbeTimeoutField'), mcpProbeTimeout: document.getElementById('mcpProbeTimeout'), mcpCheckIntervalField: document.getElementById('mcpCheckIntervalField'), mcpCheckInterval: document.getElementById('mcpCheckInterval'), mcpReconnectIntervalField: document.getElementById('mcpReconnectIntervalField'), mcpReconnectInterval: document.getElementById('mcpReconnectInterval'), mcpDefault: document.getElementById('mcpDefault'),
  mcpEditorFlow: document.getElementById('mcpEditorFlow'), mcpEditorSummary: document.getElementById('mcpEditorSummary'), mcpEditorHint: document.getElementById('mcpEditorHint'), mcpSubmitButton: document.getElementById('mcpSubmitButton'), mcpEditCancelButton: document.getElementById('mcpEditCancelButton'),
  mcpImportForm: document.getElementById('mcpImportForm'), mcpImportFormat: document.getElementById('mcpImportFormat'), mcpImportConflict: document.getElementById('mcpImportConflict'), mcpImportDefault: document.getElementById('mcpImportDefault'), mcpImportContent: document.getElementById('mcpImportContent'),
  mcpImportApplyButton: document.getElementById('mcpImportApplyButton'), mcpImportPreview: document.getElementById('mcpImportPreview'), mcpList: document.getElementById('mcpList'),
  mcpFilter: document.getElementById('mcpFilter'), mcpStateFilter: document.getElementById('mcpStateFilter'), mcpVisibleCount: document.getElementById('mcpVisibleCount'), mcpListTotalCount: document.getElementById('mcpListTotalCount'),
  mcpSelectVisibleButton: document.getElementById('mcpSelectVisibleButton'), mcpClearSelectionButton: document.getElementById('mcpClearSelectionButton'), mcpSelectedCount: document.getElementById('mcpSelectedCount'), mcpAssignEnvironmentsButton: document.getElementById('mcpAssignEnvironmentsButton'), mcpClearMissingButton: document.getElementById('mcpClearMissingButton'), mcpSetVisibleDefaultButton: document.getElementById('mcpSetVisibleDefaultButton'), mcpUnsetVisibleDefaultButton: document.getElementById('mcpUnsetVisibleDefaultButton'), mcpEnableVisibleButton: document.getElementById('mcpEnableVisibleButton'), mcpDisableVisibleButton: document.getElementById('mcpDisableVisibleButton'), mcpBulkHint: document.getElementById('mcpBulkHint'),
  skillSourceCount: document.getElementById('skillSourceCount'), skillTotalCount: document.getElementById('skillTotalCount'), skillEnvironmentCount: document.getElementById('skillEnvironmentCount'), skillIssueCount: document.getElementById('skillIssueCount'),
  skillSourceForm: document.getElementById('skillSourceForm'), skillSourceID: document.getElementById('skillSourceID'), skillSourceDialogTitle: document.getElementById('skillSourceDialogTitle'), skillSourceRoot: document.getElementById('skillSourceRoot'), skillSupportRoots: document.getElementById('skillSupportRoots'), skillSourceDefault: document.getElementById('skillSourceDefault'), skillSourceSubmitButton: document.getElementById('skillSourceSubmitButton'), skillSubviewTabs: document.getElementById('skillSubviewTabs'), skillSubviewSkillCount: document.getElementById('skillSubviewSkillCount'), skillSubviewSourceCount: document.getElementById('skillSubviewSourceCount'), skillsPanel: document.getElementById('skillsPanel'), skillSourcesPanel: document.getElementById('skillSourcesPanel'), skillSourceList: document.getElementById('skillSourceList'), skillList: document.getElementById('skillList'),
  skillFilter: document.getElementById('skillFilter'), skillSourceFilter: document.getElementById('skillSourceFilter'), skillStateFilter: document.getElementById('skillStateFilter'), skillSourceFilterInput: document.getElementById('skillSourceFilterInput'), skillSourceVisibleCount: document.getElementById('skillSourceVisibleCount'), skillSourceListTotalCount: document.getElementById('skillSourceListTotalCount'), skillVisibleCount: document.getElementById('skillVisibleCount'), skillListTotalCount: document.getElementById('skillListTotalCount'),
  skillProbeAllButton: document.getElementById('skillProbeAllButton'), skillSelectVisibleButton: document.getElementById('skillSelectVisibleButton'), skillClearSelectionButton: document.getElementById('skillClearSelectionButton'), skillSetVisibleDefaultButton: document.getElementById('skillSetVisibleDefaultButton'), skillUnsetVisibleDefaultButton: document.getElementById('skillUnsetVisibleDefaultButton'), skillEnableVisibleButton: document.getElementById('skillEnableVisibleButton'), skillDisableVisibleButton: document.getElementById('skillDisableVisibleButton'), skillSelectedCount: document.getElementById('skillSelectedCount'), skillAssignEnvironmentsButton: document.getElementById('skillAssignEnvironmentsButton'), skillClearMissingButton: document.getElementById('skillClearMissingButton'), skillDeleteSelectedButton: document.getElementById('skillDeleteSelectedButton'), skillClearUnavailableButton: document.getElementById('skillClearUnavailableButton'), skillBulkHint: document.getElementById('skillBulkHint'),
  capabilityAssignmentDialog: document.getElementById('capabilityAssignmentDialog'), capabilityAssignmentSummary: document.getElementById('capabilityAssignmentSummary'), capabilityAssignmentSelectAllButton: document.getElementById('capabilityAssignmentSelectAllButton'), capabilityAssignmentClearButton: document.getElementById('capabilityAssignmentClearButton'), capabilityAssignmentEnvironmentList: document.getElementById('capabilityAssignmentEnvironmentList'), capabilityAssignmentEnableButton: document.getElementById('capabilityAssignmentEnableButton'), capabilityAssignmentDisableButton: document.getElementById('capabilityAssignmentDisableButton'), capabilityAssignmentResult: document.getElementById('capabilityAssignmentResult'),
  workspaceForm: document.getElementById('workspaceForm'), workspacePath: document.getElementById('workspacePath'), workspaceName: document.getElementById('workspaceName'), workspaceList: document.getElementById('workspaceList'), workspaceFilter: document.getElementById('workspaceFilter'), workspaceVisibleCount: document.getElementById('workspaceVisibleCount'), workspaceListTotalCount: document.getElementById('workspaceListTotalCount'),
  workspaceDiscoveryDialog: document.getElementById('workspaceDiscoveryDialog'), workspaceDiscoveryTitle: document.getElementById('workspaceDiscoveryTitle'), workspaceDiscoveryForm: document.getElementById('workspaceDiscoveryForm'), workspaceDiscoveryPath: document.getElementById('workspaceDiscoveryPath'), workspaceDiscoveryQuery: document.getElementById('workspaceDiscoveryQuery'), workspaceDiscoveryMaxDepth: document.getElementById('workspaceDiscoveryMaxDepth'), workspaceDiscoveryMaxEntries: document.getElementById('workspaceDiscoveryMaxEntries'), workspaceDiscoveryMaxCandidates: document.getElementById('workspaceDiscoveryMaxCandidates'), workspaceDiscoveryMaxDigestEntries: document.getElementById('workspaceDiscoveryMaxDigestEntries'), workspaceDiscoveryMaxOutputBytes: document.getElementById('workspaceDiscoveryMaxOutputBytes'), workspaceDiscoveryScanButton: document.getElementById('workspaceDiscoveryScanButton'), workspaceDiscoveryFilter: document.getElementById('workspaceDiscoveryFilter'), workspaceDiscoverySummary: document.getElementById('workspaceDiscoverySummary'), workspaceDiscoveryResult: document.getElementById('workspaceDiscoveryResult'),
  environmentForm: document.getElementById('environmentForm'), environmentWorkspace: document.getElementById('environmentWorkspace'), environmentName: document.getElementById('environmentName'), environmentRoot: document.getElementById('environmentRoot'), environmentList: document.getElementById('environmentList'), environmentFilter: document.getElementById('environmentFilter'), environmentWorkspaceFilter: document.getElementById('environmentWorkspaceFilter'), environmentVisibleCount: document.getElementById('environmentVisibleCount'), environmentListTotalCount: document.getElementById('environmentListTotalCount'), environmentFilterHint: document.getElementById('environmentFilterHint'),
  environmentWorkspaceDialog: document.getElementById('environmentWorkspaceDialog'), environmentWorkspaceDialogTitle: document.getElementById('environmentWorkspaceDialogTitle'), environmentWorkspaceForm: document.getElementById('environmentWorkspaceForm'), environmentWorkspaceSummary: document.getElementById('environmentWorkspaceSummary'), environmentWorkspaceTarget: document.getElementById('environmentWorkspaceTarget'), environmentWorkspaceReason: document.getElementById('environmentWorkspaceReason'), environmentWorkspaceSaveButton: document.getElementById('environmentWorkspaceSaveButton'),
  environmentWorkspaceRecommendationsButton: document.getElementById('environmentWorkspaceRecommendationsButton'), cleanupExpiredTemporaryEnvironmentsButton: document.getElementById('cleanupExpiredTemporaryEnvironmentsButton'), environmentWorkspaceRecommendationsDialog: document.getElementById('environmentWorkspaceRecommendationsDialog'), environmentWorkspaceRecommendationsSummary: document.getElementById('environmentWorkspaceRecommendationsSummary'), environmentWorkspaceRecommendationsList: document.getElementById('environmentWorkspaceRecommendationsList'), environmentWorkspaceRecommendationsResult: document.getElementById('environmentWorkspaceRecommendationsResult'), environmentWorkspaceRecommendationsApplyButton: document.getElementById('environmentWorkspaceRecommendationsApplyButton'),
  environmentDetailBackdrop: document.getElementById('environmentDetailBackdrop'), environmentDetailPanel: document.getElementById('environmentDetailPanel'), environmentDetailTitle: document.getElementById('environmentDetailTitle'), environmentDetailSubviewTabs: document.getElementById('environmentDetailSubviewTabs'), environmentDetail: document.getElementById('environmentDetail'), environmentDiagnostics: document.getElementById('environmentDiagnostics'), environmentDetailRoutes: document.getElementById('environmentDetailRoutes'), environmentTreeDigestSection: document.getElementById('environmentTreeDigestSection'), environmentTreeDigestButton: document.getElementById('environmentTreeDigestButton'), environmentTreeDigestResult: document.getElementById('environmentTreeDigestResult'),
  environmentMCPSelections: document.getElementById('environmentMCPSelections'), environmentSkillSelections: document.getElementById('environmentSkillSelections'), closeEnvironmentDetail: document.getElementById('closeEnvironmentDetail'),
  diagnosticsRefreshButton: document.getElementById('diagnosticsRefreshButton'), diagnosticsPageHint: document.getElementById('diagnosticsPageHint'), diagnosticsPageContent: document.getElementById('diagnosticsPageContent'),
  aboutVersion: document.getElementById('aboutVersion'), aboutRuntime: document.getElementById('aboutRuntime'),
  loadEnvironmentMemory: document.getElementById('loadEnvironmentMemory'), writeEnvironmentMemoryButton: document.getElementById('writeEnvironmentMemoryButton'), environmentMemoryScopeHint: document.getElementById('environmentMemoryScopeHint'), environmentMemoryForm: document.getElementById('environmentMemoryForm'), environmentMemoryKey: document.getElementById('environmentMemoryKey'), environmentMemoryValue: document.getElementById('environmentMemoryValue'), environmentMemoryList: document.getElementById('environmentMemoryList'),
  execForm: document.getElementById('execForm'), execExecutable: document.getElementById('execExecutable'), execList: document.getElementById('execList'), execAllowedCount: document.getElementById('execAllowedCount'), execBlockedCount: document.getElementById('execBlockedCount'), execBlockedList: document.getElementById('execBlockedList'), execClearAllBlockedButton: document.getElementById('execClearAllBlockedButton'), execAuthorizationMode: document.getElementById('execAuthorizationMode'), execAuthorizationSave: document.getElementById('execAuthorizationSave'), execAuthorizationModeLabel: document.getElementById('execAuthorizationModeLabel'),
  loadGlobalMemory: document.getElementById('loadGlobalMemory'), globalMemoryForm: document.getElementById('globalMemoryForm'), globalMemoryKey: document.getElementById('globalMemoryKey'), globalMemoryValue: document.getElementById('globalMemoryValue'), globalMemoryList: document.getElementById('globalMemoryList'),
};

let currentSnapshot = null;
let skillSources = [];
let selectedEnvironmentID = '';
let managementWorkspaceID = '';
let managementEnvironmentID = '';
let managementInspection = null;
let managementCapabilityFacts = new Map();
let skillAvailabilityByID = new Map();
let environmentSkillAvailabilityByID = new Map();
let mcpHealthByKey = new Map();
let temporaryLifecycleByEnvironmentID = new Map();
let temporaryCleanupPreviewIDs = new Set();
let gatewayAccessStatus = null;
let execAuthorizationStatus = null;
let runtimeRunsByID = new Map();
let runtimeSubview = 'verifiers';
let environmentDetailSubview = 'summary';
const environmentContextRoutes = new Set(['environments', 'runtime', 'mcp', 'skills', 'memory', 'diagnostics']);
let runtimeSubviewGeneration = 0;
let runtimeLists = {verifiers: [], processes: [], runs: []};
let runtimeListStates = {verifiers: 'unloaded', processes: 'unloaded', runs: 'unloaded'};
let runtimeListErrors = {verifiers: '', processes: '', runs: ''};
let runtimeOutputBinding = null;
let runtimePendingActionKey = '';
let runtimeWriterCredential = {environmentID: '', owner: ''};
let pendingMCPImport = null;
let mcpImportBusy = false;
let editingMCPID = '';
let editingSkillSourceID = '';
let skillSubview = 'skills';
let mcpBulkBusy = false;
let globalMemoryLoaded = false;
let globalMemoryLoading = false;
let environmentMemoryLoaded = false;
let environmentMemoryLoading = false;
let loadedEnvironmentMemoryID = '';
let statusTimer = null;
let managementNavigation = null;
let environmentDetailOpener = null;
let connectionGeneration = 0;
let environmentGeneration = 0;
let detailGeneration = 0;
let lastSnapshotSuccessAt = 0;
let skillSourcesState = 'unloaded';
let skillSourcesError = '';
let managementContextError = '';
let managementSkillAvailabilityError = '';
let selectedMCPIDs = new Set();
let selectedSkillIDs = new Set();
let selectedAssignmentEnvironmentIDs = new Set();
let capabilityAssignmentBusy = false;
let explicitSkillAvailabilityProbe = null;
let skillBulkBusy = false;
let workspaceDiscoveryWorkspaceID = '';
let workspaceDiscoveryGeneration = 0;
let workspaceDiscoveryLoading = false;
let workspaceDiscoveryReport = null;
let workspaceDiscoveryError = '';
let workspaceDiscoveryFingerprint = '';
let environmentWorkspaceEnvironmentID = '';
let environmentWorkspaceOptions = null;
let environmentWorkspaceGeneration = 0;
let environmentWorkspaceLoading = false;
let environmentWorkspaceRecommendations = [];
let environmentWorkspaceRecommendationsGeneration = 0;
let environmentWorkspaceRecommendationsLoading = false;
let environmentWorkspaceRecommendationsApplying = false;
let environmentWorkspaceRecommendationsResult = '';
let environmentTreeDigestGeneration = 0;
let environmentTreeDigestLoading = false;
let environmentTreeDigestReport = null;
let environmentTreeDigestError = '';

function desktopAdapter() {
  const adapter = window.go?.desktop?.Adapter;
  if (!adapter?.GetSnapshot) throw new Error('Wails desktop binding is not ready');
  return trackDesktopAdapter(adapter);
}
function safeArray(value) { return Array.isArray(value) ? value : []; }
function safeNumber(value, fallback = 0) { const n = Number(value); return Number.isFinite(n) ? n : fallback; }
function setMetric(element, value) { element.textContent = String(value); }
function emptyMessage(container, message) { container.replaceChildren(); container.classList.add('empty'); container.textContent = message; }
function textOrDash(value) { const text = String(value ?? '').trim(); return text || '—'; }
function optionalPositiveInteger(element) {
  const text = String(element?.value || '').trim(); if (!text) return 0;
  const value = Number.parseInt(text, 10); return Number.isFinite(value) && value > 0 ? value : 0;
}
function workspaceDiscoveryRequest() {
  return {
    path: elements.workspaceDiscoveryPath.value.trim(), query: elements.workspaceDiscoveryQuery.value.trim(),
    max_depth: optionalPositiveInteger(elements.workspaceDiscoveryMaxDepth), max_entries: optionalPositiveInteger(elements.workspaceDiscoveryMaxEntries),
    max_candidates: optionalPositiveInteger(elements.workspaceDiscoveryMaxCandidates), max_digest_entries: optionalPositiveInteger(elements.workspaceDiscoveryMaxDigestEntries),
    max_output_bytes: optionalPositiveInteger(elements.workspaceDiscoveryMaxOutputBytes),
  };
}
function workspaceDiscoveryRequestFingerprint() { return JSON.stringify(workspaceDiscoveryRequest()); }
function currentWorkspaceDiscoveryWorkspace() { return safeArray(currentSnapshot?.workspaces).find((workspace) => workspace.workspace_id === workspaceDiscoveryWorkspaceID) || null; }
function normalizeFilesystemKey(value) { return String(value || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase(); }
function absoluteCandidateRoot(workspace, relativeRoot) {
  const root = String(relativeRoot || '.').trim() || '.';
  if (/^[a-zA-Z]:[\\/]/.test(root) || root.startsWith('/')) return root;
  if (root === '.') return workspace?.path || root;
  return String(workspace?.path || '').replace(/[\\/]+$/, '') + '/' + root.replace(/^[\\/]+/, '');
}
function environmentRootForCreate(workspaceID, root) {
  const value = String(root || '').trim(); if (!value || /^[a-zA-Z]:[\\/]/.test(value) || value.startsWith('/')) return value;
  const workspace = safeArray(currentSnapshot?.workspaces).find((item) => item.workspace_id === workspaceID);
  return workspace ? absoluteCandidateRoot(workspace, value) : value;
}
function matchingCandidateEnvironments(workspace, candidate) {
  const target = normalizeFilesystemKey(absoluteCandidateRoot(workspace, candidate?.suggested_environment_root || candidate?.root));
  return safeArray(currentSnapshot?.environments).filter((environment) => environment.workspace_id === workspace?.workspace_id && normalizeFilesystemKey(environment.root) === target);
}
function resetWorkspaceDiscoveryState(message = '点击“扫描”显式开始 discovery。') {
  workspaceDiscoveryLoading = false; workspaceDiscoveryReport = null; workspaceDiscoveryError = ''; workspaceDiscoveryFingerprint = '';
  elements.workspaceDiscoveryScanButton.disabled = false;
  elements.workspaceDiscoverySummary.textContent = '尚未扫描。打开此窗口不会读取目录。';
  emptyMessage(elements.workspaceDiscoveryResult, message);
}
function openWorkspaceDiscovery(workspace, opener) {
  if (!workspace) return;
  workspaceDiscoveryGeneration++; workspaceDiscoveryWorkspaceID = workspace.workspace_id || '';
  elements.workspaceDiscoveryTitle.textContent = `Discover projects · ${workspace.name || workspace.workspace_id}`;
  elements.workspaceDiscoveryForm.reset(); elements.workspaceDiscoveryFilter.value = '';
  resetWorkspaceDiscoveryState();
  if (opener?.focus) opener.focus({preventScroll: true});
  openEditorDialog('workspaceDiscoveryDialog');
}
function workspaceDiscoveryIdentity() { return {connectionGeneration, dialogGeneration: workspaceDiscoveryGeneration, workspaceID: workspaceDiscoveryWorkspaceID}; }
function workspaceDiscoveryIdentityIsCurrent(identity) {
  return Boolean(identity && identity.connectionGeneration === connectionGeneration && identity.dialogGeneration === workspaceDiscoveryGeneration && identity.workspaceID === workspaceDiscoveryWorkspaceID && elements.workspaceDiscoveryDialog.open);
}
function renderWorkspaceDiscovery() {
  const workspace = currentWorkspaceDiscoveryWorkspace();
  elements.workspaceDiscoveryScanButton.disabled = workspaceDiscoveryLoading || !workspace;
  if (!workspace) {
    elements.workspaceDiscoverySummary.textContent = '当前 Workspace 已不可用。';
    return emptyMessage(elements.workspaceDiscoveryResult, 'Workspace 已切换或移除；关闭后重新打开 discovery。');
  }
  if (workspaceDiscoveryLoading) {
    elements.workspaceDiscoverySummary.textContent = '正在执行有界 metadata discovery…';
    return emptyMessage(elements.workspaceDiscoveryResult, '扫描中…');
  }
  if (workspaceDiscoveryError) {
    elements.workspaceDiscoverySummary.textContent = 'Discovery 失败；已保留当前选项，可显式重试。';
    return emptyMessage(elements.workspaceDiscoveryResult, workspaceDiscoveryError);
  }
  if (!workspaceDiscoveryReport) return resetWorkspaceDiscoveryState();

  const requestChanged = workspaceDiscoveryFingerprint !== workspaceDiscoveryRequestFingerprint();
  const report = workspaceDiscoveryReport, candidates = safeArray(report.candidates), matches = searchMatcher(elements.workspaceDiscoveryFilter.value);
  const visible = candidates.filter((candidate) => matches([candidate.name, candidate.root, candidate.query_match, ...safeArray(candidate.markers).map((marker) => marker.path)].filter(Boolean).join(' ')));
  const partial = Boolean(report.truncated) || String(report.coverage || '').toLowerCase().startsWith('partial');
  const omitted = safeNumber(report.omitted_candidates) + safeNumber(report.omitted_digest_entries) + safeNumber(report.omitted_markers) + safeNumber(report.omitted_diagnostics);
  const limits = report.limits || {}, scope = report.scope || {};
  const scopeText = scope.workspace_id ? `Workspace ${scope.workspace_id}` : 'Workspace scope unavailable';
  const stops = safeArray(report.stop_reasons);
  elements.workspaceDiscoverySummary.textContent = `${requestChanged ? '选项已变化 · 需要重新扫描 · ' : ''}${partial ? 'Partial' : 'Complete'} · ${scopeText} · path ${report.scan_path || '.'} · candidates ${candidates.length}${omitted ? ` · omitted ${omitted}` : ''} · visited ${safeNumber(report.visited_entries)} · limits d${safeNumber(limits.max_depth)}/e${safeNumber(limits.max_entries)}/c${safeNumber(limits.max_candidates)}/g${safeNumber(limits.max_digest_entries)}/b${safeNumber(limits.max_output_bytes)}${stops.length ? ` · stop ${stops.join(', ')}` : ''} · ${formatDateTime(report.observed_at)}`;
  elements.workspaceDiscoveryResult.replaceChildren(); elements.workspaceDiscoveryResult.classList.remove('empty');
  if (!candidates.length) return emptyMessage(elements.workspaceDiscoveryResult, report.query ? '扫描完成，但 discovery query 没有候选匹配。可缩小 path、修改 query 后显式重新扫描。' : '扫描完成，没有发现项目候选。可调整 path/budgets 后显式重新扫描。');
  if (!visible.length) return emptyMessage(elements.workspaceDiscoveryResult, '当前本地筛选没有匹配候选；不会重新扫描。');

  const digest = safeArray(report.digest);
  if (digest.length) {
    const digestPanel = document.createElement('details'); digestPanel.className = 'discovery-digest';
    const digestSummary = document.createElement('summary'); digestSummary.textContent = `Directory digest · ${digest.length} entries${safeNumber(report.omitted_digest_entries) ? ` · ${safeNumber(report.omitted_digest_entries)} omitted` : ''}`;
    const digestBody = document.createElement('div'); digestBody.className = 'discovery-digest-body';
    for (const entry of digest.slice(0, 12)) { const line = document.createElement('div'); const path = document.createElement('code'); path.textContent = entry.path || '.'; const meta = document.createElement('small'); meta.textContent = `${safeNumber(entry.observed_children)} children · ${safeNumber(entry.observed_markers)} markers${entry.children_complete === false ? ' · partial' : ''}`; line.append(path, meta); digestBody.append(line); }
    if (digest.length > 12) { const more = document.createElement('small'); more.textContent = `${digest.length - 12} more digest entries in bounded report`; digestBody.append(more); }
    digestPanel.append(digestSummary, digestBody); elements.workspaceDiscoveryResult.append(digestPanel);
  }
  for (const candidate of visible) {
    const row = document.createElement('article'); row.className = 'discovery-candidate'; row.dataset.root = candidate.root || '';
    const content = document.createElement('div'); content.className = 'item-content';
    const title = document.createElement('div'); title.className = 'item-title-line'; const strong = document.createElement('strong'); strong.textContent = candidate.name || candidate.root || 'project'; title.append(strong, stateBadge(partial ? 'partial scan' : 'candidate', partial ? 'degraded' : 'available'));
    const root = document.createElement('code'); root.textContent = candidate.root || '.';
    const evidence = document.createElement('small'); evidence.textContent = safeArray(candidate.markers).length ? `Markers: ${safeArray(candidate.markers).map((marker) => marker.path || marker.kind).join(', ')}` : (candidate.evidence || 'directory metadata');
    const matchesEnvironment = matchingCandidateEnvironments(workspace, candidate);
    const existing = document.createElement('small'); existing.textContent = matchesEnvironment.length ? `Existing Environment: ${matchesEnvironment.map((environment) => `${environment.name || environment.environment_id} (${environment.environment_id})`).join(', ')}` : 'Existing Environment: none for this exact root';
    content.append(title, root, evidence, existing);
    const actions = document.createElement('div'); actions.className = 'item-actions';
    const use = createActionButton('Use root', 'use-discovery-root', candidate.root || '.'); use.dataset.root = candidate.suggested_environment_root || candidate.root || '.'; actions.append(use);
    if (matchesEnvironment.length) { const view = createActionButton('查看匹配 Environment', 'show-discovery-environments', candidate.root || '.'); view.dataset.root = candidate.suggested_environment_root || candidate.root || '.'; actions.append(view); }
    row.append(content, actions); elements.workspaceDiscoveryResult.append(row);
  }
}
async function scanWorkspaceDiscovery() {
  if (workspaceDiscoveryLoading || !workspaceDiscoveryWorkspaceID) return;
  const identity = workspaceDiscoveryIdentity(), request = workspaceDiscoveryRequest(), fingerprint = JSON.stringify(request);
  workspaceDiscoveryLoading = true; workspaceDiscoveryError = ''; renderWorkspaceDiscovery();
  try {
    const report = await desktopAdapter().DiscoverWorkspace(identity.workspaceID, request);
    if (!workspaceDiscoveryIdentityIsCurrent(identity)) return;
    workspaceDiscoveryReport = report; workspaceDiscoveryFingerprint = fingerprint;
  } catch (error) {
    if (!workspaceDiscoveryIdentityIsCurrent(identity)) return;
    workspaceDiscoveryReport = null; workspaceDiscoveryFingerprint = ''; workspaceDiscoveryError = 'Discovery 失败：' + errorText(error);
  } finally {
    if (workspaceDiscoveryIdentityIsCurrent(identity)) { workspaceDiscoveryLoading = false; renderWorkspaceDiscovery(); }
  }
}
function resetEnvironmentTreeDigest(message = '尚未读取目录摘要。') {
  environmentTreeDigestGeneration++; environmentTreeDigestLoading = false; environmentTreeDigestReport = null; environmentTreeDigestError = '';
  elements.environmentTreeDigestButton.disabled = !selectedEnvironmentID;
  emptyMessage(elements.environmentTreeDigestResult, message);
}
function renderEnvironmentTreeDigest() {
  elements.environmentTreeDigestButton.disabled = environmentTreeDigestLoading || !selectedEnvironmentID;
  if (environmentTreeDigestLoading) return emptyMessage(elements.environmentTreeDigestResult, '正在读取有界目录摘要…');
  if (environmentTreeDigestError) return emptyMessage(elements.environmentTreeDigestResult, environmentTreeDigestError);
  const report = environmentTreeDigestReport; if (!report) return emptyMessage(elements.environmentTreeDigestResult, '尚未读取目录摘要。');
  const digest = safeArray(report.digest), partial = Boolean(report.truncated) || String(report.coverage || '').toLowerCase().startsWith('partial');
  const limits = report.limits || {}, stops = safeArray(report.stop_reasons), scope = report.scope || {};
  if (!digest.length) return emptyMessage(elements.environmentTreeDigestResult, `${partial ? 'Partial · ' : ''}目录摘要为空。${stops.length ? ` Stop: ${stops.join(', ')}.` : ''}`);
  elements.environmentTreeDigestResult.replaceChildren(); elements.environmentTreeDigestResult.classList.remove('empty');
  const summary = document.createElement('div'); summary.className = 'preview-summary'; summary.textContent = `${partial ? 'Partial' : 'Complete'} · Environment ${scope.environment_id || selectedEnvironmentID} · path ${report.scan_path || '.'} · ${digest.length} entries · visited ${safeNumber(report.visited_entries)} · limits d${safeNumber(limits.max_depth)}/e${safeNumber(limits.max_entries)}/g${safeNumber(limits.max_digest_entries)}/b${safeNumber(limits.max_output_bytes)}${stops.length ? ` · stop ${stops.join(', ')}` : ''} · ${formatDateTime(report.observed_at)}`; elements.environmentTreeDigestResult.append(summary);
  for (const entry of digest) {
    const row = document.createElement('div'); row.className = 'digest-row'; const path = document.createElement('code'); path.textContent = entry.path || '.'; const meta = document.createElement('small'); meta.textContent = `${entry.kind || 'directory'} · children ${safeNumber(entry.observed_children)} · markers ${safeNumber(entry.observed_markers)}${entry.children_complete === false ? ' · partial children' : ''}`; row.append(path, meta); elements.environmentTreeDigestResult.append(row);
  }
}
async function loadEnvironmentTreeDigest() {
  if (environmentTreeDigestLoading || !selectedEnvironmentID) return;
  const identity = {connectionGeneration, detailGeneration, environmentID: selectedEnvironmentID, digestGeneration: ++environmentTreeDigestGeneration};
  environmentTreeDigestLoading = true; environmentTreeDigestError = ''; environmentTreeDigestReport = null; renderEnvironmentTreeDigest();
  try {
    const report = await desktopAdapter().EnvironmentTreeDigest(identity.environmentID, {});
    if (identity.connectionGeneration !== connectionGeneration || identity.detailGeneration !== detailGeneration || identity.environmentID !== selectedEnvironmentID || identity.digestGeneration !== environmentTreeDigestGeneration || elements.environmentDetailPanel.hidden) return;
    environmentTreeDigestReport = report;
  } catch (error) {
    if (identity.connectionGeneration !== connectionGeneration || identity.detailGeneration !== detailGeneration || identity.environmentID !== selectedEnvironmentID || identity.digestGeneration !== environmentTreeDigestGeneration || elements.environmentDetailPanel.hidden) return;
    environmentTreeDigestError = '目录摘要读取失败：' + errorText(error);
  } finally {
    if (identity.connectionGeneration === connectionGeneration && identity.detailGeneration === detailGeneration && identity.environmentID === selectedEnvironmentID && identity.digestGeneration === environmentTreeDigestGeneration && !elements.environmentDetailPanel.hidden) { environmentTreeDigestLoading = false; renderEnvironmentTreeDigest(); }
  }
}
function formatDateTime(value) {
  if (!value) return '未知时间';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
}
function searchMatcher(raw) {
  const text = String(raw || '').trim();
  if (!text) return () => true;
  const slash = text.match(/^\/(.*)\/([dgimsuvy]*)$/);
  if (slash) {
    try {
      const flags = [...new Set(slash[2].replace(/[gy]/g, '').split(''))].join('');
      const regex = new RegExp(slash[1], flags || 'i');
      return (value) => regex.test(String(value || ''));
    } catch (_) {}
  }
  const lower = text.toLowerCase();
  return (value) => String(value || '').toLowerCase().includes(lower);
}
function visibleResourceIDs(container) { return [...container.querySelectorAll('.resource-row[data-id]')].map((row) => row.dataset.id).filter(Boolean); }
function lineValues(value) { return String(value || '').split(/\r?\n/).map((item) => item.trim()).filter(Boolean); }
function referenceMap(value) {
  const result = {};
  for (const line of lineValues(value)) {
    const index = line.indexOf('=');
    if (index <= 0 || index === line.length - 1) throw new Error(`引用映射必须使用 KEY=REFERENCE：${line}`);
    result[line.slice(0, index).trim()] = line.slice(index + 1).trim();
  }
  return result;
}
function normalizedState(value) { return String(value || 'unknown').toLowerCase().replace(/[^a-z0-9_-]+/g, '-'); }
function referenceLines(values) { return Object.entries(values || {}).sort(([a], [b]) => a.localeCompare(b)).map(([key, value]) => `${key}=${value}`).join('\n'); }
function effectiveMCPAuthMode(transport, requestedMode) {
  if (transport === 'stdio') return 'none';
  return requestedMode === 'headers' ? 'headers' : 'none';
}
function syncMCPAuthModeFromReferences() {
  if (elements.mcpTransport.value === 'stdio') elements.mcpAuthMode.value = 'none';
  else if (lineValues(elements.mcpReferencePairs.value).length && elements.mcpAuthMode.value === 'none') elements.mcpAuthMode.value = 'headers';
  syncMCPAuthForm();
}
async function writeClipboardText(text) {
  if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(text); return; }
  const field = document.createElement('textarea'); field.value = text; field.setAttribute('readonly', ''); field.style.position = 'fixed'; field.style.opacity = '0'; document.body.append(field); field.select();
  const copied = typeof document.execCommand === 'function' && document.execCommand('copy'); field.remove();
  if (!copied) throw new Error('当前 WebView 不支持写入剪贴板');
}
async function copyMCPConfig(entry) {
  const text = window.ADMMCPExport?.stringifyGenericConfig(entry);
  if (!text) throw new Error('MCP 导出组件不可用');
  await writeClipboardText(text);
  setStatus(`已复制 ${entry?.name || entry?.id || 'MCP'} 的 Generic mcpServers JSON；只包含连接配置和环境变量引用。`, 'success');
}
function humanConfigState(state) {
  return ({available: '可用', configured: '已配置', degraded: '降级', unavailable: '不可用', unconfigured: '未配置', disabled: '未启用', unknown: '未知'})[normalizedState(state)] || String(state || '未知');
}
function humanRuntimeState(state) {
  return ({available: '可用', healthy: '健康', error: '错误', degraded: '降级', disabled: '未启用', not_observed: '尚未探测', configured: '已配置', unavailable: '不可用', unconfigured: '未配置', source_missing: 'Source 缺失', artifact_missing: 'Artifact 缺失', artifact_unreadable: 'Artifact 不可读', support_root_missing: 'Support root 缺失', unresolved: '未解析', no_environment: '未选择环境', unknown: '未知'})[normalizedState(state)] || String(state || '未知');
}
function humanRefreshState(state) { return ({ok: '刷新正常', pending: '待刷新', error: '刷新失败'})[normalizedState(state)] || String(state || '未知'); }
function mcpReferenceVariableNames(entry) {
  const refs = entry?.transport === 'stdio' ? entry?.env_refs : entry?.header_refs; const names = new Set();
  for (const value of Object.values(refs || {})) { const text = String(value || ''); for (const match of text.matchAll(/\$\{([A-Za-z_][A-Za-z0-9_]*)/g)) names.add(match[1]); }
  return [...names].sort();
}
function setStatus(message, kind = 'normal') {
  if (statusTimer) clearTimeout(statusTimer);
  const dialogMessage = activeEditorDialog()?.querySelector('.dialog-message');
  if (dialogMessage) { dialogMessage.textContent = message; dialogMessage.hidden = kind !== 'error'; }
  elements.statusPanel.textContent = message; elements.statusPanel.dataset.kind = kind; elements.statusPanel.hidden = false;
  if (kind !== 'loading') statusTimer = setTimeout(() => { elements.statusPanel.hidden = true; statusTimer = null; }, kind === 'error' ? 7000 : 2400);
}
function initializeManagementNavigation() {
  if (!window.ADMNavigation?.createNavigation) throw new Error('management navigation helper is not ready');
  managementNavigation = window.ADMNavigation.createNavigation({
    beforeNavigate: () => !connectionSwitching && !activeEditorDialog() && elements.environmentDetailPanel.hidden,
    afterNavigate: (route) => {
      updateManagementContextVisibility(route);
      if (route === 'memory') { renderEnvironmentMemoryScope(); maybeAutoLoadGlobalMemory(); }
      if (route === 'diagnostics') renderDiagnosticsPage();
    },
  });
}

function updateManagementContextVisibility(route = currentManagementRoute()) {
  if (elements.managementContextPanel) elements.managementContextPanel.hidden = !environmentContextRoutes.has(route);
}
function renderDashboardState(state, error = '') {
  if (!window.ADMDashboard?.renderDashboard) return;
  window.ADMDashboard.renderDashboard({
    state: elements.dashboardDataState,
    lastSuccess: elements.dashboardLastSuccess,
    detail: elements.dashboardDataDetail,
    metrics: {
      workspaceCount: elements.workspaceCount,
      environmentCount: elements.environmentCount,
      execCount: elements.execCount,
      mcpCount: elements.mcpCount,
      skillCount: elements.skillCount,
      memoryCount: elements.memoryCount,
    },
  }, {state, snapshot: currentSnapshot, lastSuccessAt: lastSnapshotSuccessAt, error});
}
function captureEnvironmentScope() { return {connectionGeneration, environmentGeneration, environmentID: managementEnvironmentID}; }
function environmentScopeIsCurrent(scope) { return window.ADMDashboard?.scopeMatches(scope, {connectionGeneration, environmentGeneration, environmentID: managementEnvironmentID}) ?? false; }
function errorText(error) { return error?.message || String(error); }

function stateBadge(label, state = label) {
  const badge = document.createElement('span'); badge.className = 'resource-badge'; badge.dataset.state = normalizedState(state); badge.textContent = label || 'unknown'; return badge;
}
function createActionButton(label, action, id, kind = 'secondary') {
  const button = document.createElement('button'); button.type = 'button'; button.className = `small-button ${kind === 'danger' ? 'danger-button' : 'secondary-button'}`;
  button.textContent = label; button.dataset.action = action; button.dataset.id = id; return button;
}
function checkControl(labelText, checked, action, id, disabled = false) {
  const label = document.createElement('label'); label.className = 'check-field compact-check';
  const input = document.createElement('input'); input.type = 'checkbox'; input.checked = Boolean(checked); input.disabled = disabled; input.dataset.action = action; input.dataset.id = id;
  const text = document.createElement('span'); text.textContent = labelText; label.append(input, text); return label;
}
function currentWorkspace() { return safeArray(currentSnapshot?.workspaces).find((item) => item.workspace_id === managementWorkspaceID) || null; }
function currentEnvironment() { return safeArray(currentSnapshot?.environments).find((item) => item.environment_id === managementEnvironmentID) || null; }
function capabilityMap(report) { const map = new Map(); for (const fact of safeArray(report?.facts)) if (fact?.key) map.set(fact.key, fact); return map; }
function mcpHealthKey(mcpID) { return mcpID || ''; }
let mcpProbeRequests = new Map();
function mcpProbeFingerprint(entry) { if (!entry) return ''; const {default_include_in_environment, ...definition} = entry; return JSON.stringify(definition); }

const defaultADMBaseURL = 'http://127.0.0.1:43137';
function currentADMBaseURL() { return elements.gatewayBaseURL.value.trim() || defaultADMBaseURL; }
async function loadDesktopPreferences(showMessage = false) {
  try {
    const preferences = await desktopAdapter().GetDesktopPreferences();
    const supported = Boolean(preferences?.launch_at_login_supported);
    elements.launchAtLogin.disabled = !supported;
    elements.launchAtLogin.checked = supported && Boolean(preferences?.launch_at_login);
    elements.desktopShellHint.textContent = supported ? '可随用户登录自动启动 Desktop' : '当前平台不支持开机启动';
    if (showMessage) setStatus('Desktop 设置已刷新', 'success');
    return preferences;
  } catch (error) {
    elements.launchAtLogin.disabled = true;
    elements.desktopShellHint.textContent = `设置读取失败：${error?.message || String(error)}`;
    if (showMessage) setStatus(`Desktop 设置读取失败：${error?.message || String(error)}`, 'error');
    return null;
  }
}
async function updateLaunchAtLogin() {
  const desired = elements.launchAtLogin.checked;
  elements.launchAtLogin.disabled = true;
  elements.desktopShellHint.textContent = desired ? '正在启用开机启动…' : '正在关闭开机启动…';
  try {
    const preferences = await desktopAdapter().SetLaunchAtLogin(desired);
    window.runtime?.EventsEmit?.('desktop:preferences-changed');
    elements.launchAtLogin.checked = Boolean(preferences?.launch_at_login);
    elements.desktopShellHint.textContent = '关闭窗口后继续驻留托盘';
    setStatus(preferences?.launch_at_login ? '已启用开机启动' : '已关闭开机启动', 'success');
  } catch (error) {
    elements.launchAtLogin.checked = !desired;
    elements.desktopShellHint.textContent = `设置失败：${error?.message || String(error)}`;
    setStatus(`开机启动设置失败：${error?.message || String(error)}`, 'error');
  } finally {
    elements.launchAtLogin.disabled = false;
  }
}
function renderGatewayAccessStatus(status) {
  gatewayAccessStatus = status || null;
  const hosts = safeArray(status?.allowed_hosts);
  elements.gatewayAllowedHosts.value = hosts.join('\n');
  const hostSummary = hosts.includes('*') ? '不限制 (*)' : (hosts.length ? hosts.length + ' 个' : '未配置');
  elements.gatewayAccessSummary.textContent = `远程 Host ${hostSummary} · Admin Key ${status?.admin_api_key_configured ? '已配置' : '未配置'} · Agent Key ${status?.agent_api_key_configured ? '已配置' : '未配置'}`;
  elements.gatewayAccessClearAdminKey.disabled = !Boolean(status?.admin_api_key_configured);
  elements.gatewayAccessClearAgentKey.disabled = !Boolean(status?.agent_api_key_configured);
}
async function refreshLoggingStatus() {
  try {
    const status = await desktopAdapter().GetLoggingStatus();
    const bytes = safeNumber(status?.max_bytes);
    const sizeText = bytes ? (bytes / (1024 * 1024)).toFixed(bytes % (1024 * 1024) === 0 ? 0 : 1) + ' MiB' : '—';
    elements.loggingStatusSummary.replaceChildren(
      detailRow('Directory', status?.directory || '—'),
      detailRow('Levels', 'debug / info / warn / error'),
      detailRow('Rotate', 'UTC day change or ' + sizeText + ' per active level file'),
      detailRow('Retention', safeNumber(status?.max_files_per_level) + ' rotated files / level')
    );
    return '';
  } catch (error) {
    if (elements.loggingStatusSummary) elements.loggingStatusSummary.replaceChildren(detailRow('Status', '读取失败：' + errorText(error)));
    return 'Logging: ' + errorText(error);
  }
}
async function refreshGatewayAccessStatus() {
  try {
    renderGatewayAccessStatus(await desktopAdapter().GetGatewayAccessStatus());
    return '';
  } catch (error) {
    gatewayAccessStatus = null;
    elements.gatewayAccessSummary.textContent = '远程访问配置读取失败：' + errorText(error);
    return 'Gateway access: ' + errorText(error);
  }
}
function renderExecAuthorizationStatus(status) {
  execAuthorizationStatus = status || null;
  const full = Boolean(status?.full_authorization);
  elements.execAuthorizationMode.value = full ? 'full' : 'strict';
  elements.execAuthorizationModeLabel.textContent = full ? 'Full' : 'Strict';
}
async function refreshExecAuthorizationStatus() {
  try {
    renderExecAuthorizationStatus(await desktopAdapter().GetExecAuthorizationStatus());
    return '';
  } catch (error) {
    execAuthorizationStatus = null;
    return 'Exec authorization: ' + errorText(error);
  }
}
async function saveGatewayAllowedHosts() {
  const hosts = String(elements.gatewayAllowedHosts.value || '').split(/[\n,]+/).map((value) => value.trim()).filter(Boolean);
  const profile = activeConnectionProfile();
  if (hosts.length && (!gatewayAccessStatus?.admin_api_key_configured || !gatewayAccessStatus?.agent_api_key_configured)) {
    return setStatus('启用远程 Host 前必须先配置 Admin Key 和 Agent Key。', 'error');
  }
  if (hosts.length && gatewayAccessStatus?.admin_api_key_configured && !profile?.api_key_configured) {
    return setStatus('服务端已经配置 Admin Key，但当前 Desktop 连接还没有保存 Admin Key。请先用下方“设置 Admin Key”同步后再启用远程 Host。', 'error');
  }
  await runMutation('保存远程 Host 白名单', () => desktopAdapter().SetGatewayAllowedHosts(hosts));
}
async function setGatewayAdminAPIKey() {
  const value = String(elements.gatewayAdminAPIKey.value || '').trim();
  if (!value) return setStatus('请输入 Admin API Key。', 'error');
  await runMutation('设置 Admin API Key', async () => {
    await desktopAdapter().ConfigureGatewayAdminAPIKey(value);
    elements.gatewayAdminAPIKey.value = '';
    connectionProfiles = await desktopAdapter().GetConnectionProfiles();
    renderConnectionProfiles();
  });
}
async function clearGatewayAdminAPIKey() {
  if (!window.confirm('清除 Admin API Key 后，/admin/mcp 将无法进行远程鉴权，直到重新配置。继续？')) return;
  await runMutation('清除 Admin API Key', () => desktopAdapter().ClearGatewayAdminAPIKey());
}
async function setGatewayAgentAPIKey() {
  const value = String(elements.gatewayAgentAPIKey.value || '').trim();
  if (!value) return setStatus('请输入 Agent API Key。', 'error');
  await runMutation('设置 Agent API Key', async () => {
    await desktopAdapter().SetGatewayAgentAPIKey(value);
    elements.gatewayAgentAPIKey.value = '';
  });
}
async function clearGatewayAgentAPIKey() {
  if (!window.confirm('清除 Agent API Key 后，/mcp 将无法进行远程鉴权，直到重新配置。继续？')) return;
  await runMutation('清除 Agent API Key', () => desktopAdapter().ClearGatewayAgentAPIKey());
}
async function saveExecAuthorizationMode() {
  const full = elements.execAuthorizationMode.value === 'full';
  if (full && !window.confirm('Full 模式允许执行未列入 allowlist 的 executable；这些调用仍会记录审计。确认开启？')) {
    renderExecAuthorizationStatus(execAuthorizationStatus);
    return;
  }
  await runMutation('保存执行授权模式', () => desktopAdapter().SetExecFullAuthorization(full));
}
async function loadAboutInfo() {
  try {
    const info = await desktopAdapter().GetAboutInfo();
    elements.aboutVersion.textContent = info?.version || '—';
    elements.aboutRuntime.textContent = [info?.go_version, info?.goos && info?.goarch ? info.goos + '/' + info.goarch : ''].filter(Boolean).join(' · ') || '—';
  } catch (_) {
    elements.aboutVersion.textContent = '—';
    elements.aboutRuntime.textContent = '—';
  }
}
function renderGatewayStatus(status) {
  const state = status?.state || 'unknown'; elements.gatewayState.textContent = state; elements.gatewayState.dataset.state = state;
  const baseURL = status?.base_url || currentADMBaseURL();
  elements.gatewayBaseURL.value = baseURL; elements.gatewayHealthURL.textContent = status?.health_url || `${baseURL.replace(/\/$/, '')}/healthz`;
  elements.gatewayURL.textContent = status?.agent_mcp_url || status?.mcp_url || `${baseURL.replace(/\/$/, '')}/mcp`;
  elements.gatewayAdminURL.textContent = status?.admin_mcp_url || `${baseURL.replace(/\/$/, '')}/admin/mcp`;
  elements.gatewayProcess.textContent = `PID ${status?.pid || '—'} · Version ${status?.version || '—'}`;
  const localEligible = Boolean(status?.local_bootstrap_eligible);
  elements.gatewayDetail.textContent = status?.detail || (state === 'running' ? 'ADM health check 通过；Desktop 管理数据通过 Admin MCP 读取。' : 'ADM 未连接时 Desktop 不读取或修改本地 state。');
  elements.gatewayStartButton.disabled = !localEligible || state === 'running' || state === 'incompatible';
  elements.gatewayStopButton.disabled = !localEligible || state === 'stopped' || state === 'incompatible' || state === 'unknown';

}
async function refreshGatewayStatus(showMessage = false) {
  if (connectionProfilesLoaded && !connectionProfiles.active_id) throw new Error('请先选择 ADM 连接');
  try { const status = await desktopAdapter().ConnectADM({base_url: currentADMBaseURL()}); renderGatewayStatus(status); if (showMessage) setStatus('ADM 连接状态已刷新', 'success'); return status; }
  catch (error) { elements.gatewayState.textContent = 'unknown'; elements.gatewayState.dataset.state = 'unknown'; elements.gatewayDetail.textContent = error?.message || String(error); elements.gatewayStartButton.disabled = true; elements.gatewayStopButton.disabled = true; if (showMessage) setStatus(`ADM 连接检查失败：${error?.message || String(error)}`, 'error'); throw error; }
}
async function runGatewayAction(label, action) {
  elements.gatewayStartButton.disabled = true; elements.gatewayStopButton.disabled = true; setStatus(`${label}…`, 'loading');
  try {
    const status = await action({base_url: currentADMBaseURL()}); renderGatewayStatus(status);
    if (status?.state === 'running') await refreshSnapshot(`${label}完成`); else { clearManagementData(); setStatus(`${label}完成`, 'success'); }
  }
  catch (error) { clearManagementData(); setStatus(`${label}失败：${error?.message || String(error)}`, 'error'); try { await refreshGatewayStatus(false); } catch (_) {} }
}

function renderWorkspaceOptions(workspaces) {
  const current = elements.environmentWorkspace.value; elements.environmentWorkspace.replaceChildren();
  const creatable = safeArray(workspaces).filter((workspace) => !workspace.system_kind);
  elements.createEnvironmentButton.disabled = !creatable.length;
  if (!creatable.length) { const option = document.createElement('option'); option.value = ''; option.textContent = '先添加普通 Workspace'; elements.environmentWorkspace.append(option); elements.environmentWorkspace.disabled = true; return; }
  elements.environmentWorkspace.disabled = false;
  for (const workspace of creatable) { const option = document.createElement('option'); option.value = workspace.workspace_id || ''; option.textContent = `${workspace.name || workspace.workspace_id} · ${workspace.path || ''}`; elements.environmentWorkspace.append(option); }
  if (creatable.some((workspace) => workspace.workspace_id === current)) elements.environmentWorkspace.value = current;
}
function renderManagementEnvironmentOptions(workspaces, environments) {
  const previousWorkspace = managementWorkspaceID, previousEnvironment = managementEnvironmentID;
  const workspaceList = safeArray(workspaces), environmentList = safeArray(environments);
  const previousEnvironmentRow = environmentList.find((item) => item.environment_id === previousEnvironment);
  if (previousWorkspace && workspaceList.some((item) => item.workspace_id === previousWorkspace)) managementWorkspaceID = previousWorkspace;
  else if (previousEnvironmentRow?.workspace_id && workspaceList.some((item) => item.workspace_id === previousEnvironmentRow.workspace_id)) managementWorkspaceID = previousEnvironmentRow.workspace_id;
  else if (workspaceList.length === 1) managementWorkspaceID = workspaceList[0].workspace_id;
  else managementWorkspaceID = '';

  elements.managementWorkspace.replaceChildren();
  const workspaceNone = document.createElement('option'); workspaceNone.value = ''; workspaceNone.textContent = '不选择 Workspace（只管理全局定义）'; elements.managementWorkspace.append(workspaceNone);
  for (const workspace of workspaceList) { const option = document.createElement('option'); option.value = workspace.workspace_id || ''; option.textContent = `${workspace.name || workspace.workspace_id} · ${workspace.path || ''}`; elements.managementWorkspace.append(option); }
  elements.managementWorkspace.disabled = !workspaceList.length; elements.managementWorkspace.value = managementWorkspaceID;

  const scopedEnvironments = managementWorkspaceID ? environmentList.filter((item) => item.workspace_id === managementWorkspaceID) : [];
  elements.managementEnvironment.replaceChildren();
  const none = document.createElement('option'); none.value = ''; none.textContent = managementWorkspaceID ? '不选择 Environment（管理当前 Workspace）' : '请先选择 Workspace'; elements.managementEnvironment.append(none);
  for (const environment of scopedEnvironments) { const option = document.createElement('option'); option.value = environment.environment_id || ''; option.textContent = `${environment.name || environment.environment_id} · ${environment.root || ''}`; elements.managementEnvironment.append(option); }
  elements.managementEnvironment.disabled = !managementWorkspaceID;
  if (previousEnvironment && scopedEnvironments.some((item) => item.environment_id === previousEnvironment)) managementEnvironmentID = previousEnvironment;
  else if (scopedEnvironments.length === 1) managementEnvironmentID = scopedEnvironments[0].environment_id;
  else managementEnvironmentID = '';
  if (managementEnvironmentID !== previousEnvironment || managementWorkspaceID !== previousWorkspace) environmentGeneration++;
  elements.managementEnvironment.value = managementEnvironmentID;
  updateManagementHint();
}
function updateManagementHint() {
  const workspace = currentWorkspace(), environment = currentEnvironment();
  elements.editEnvironmentButton.disabled = !environment;
  const base = environment
    ? `当前查看 ${environment.name || environment.environment_id} · Workspace ${workspace?.name || environment.workspace_id || '—'}。Workspace 开启的 MCP / Skill 会被该 Environment 继承；Environment 开关只管理显式附加选择。`
    : workspace
      ? `当前 Workspace：${workspace.name || workspace.workspace_id}。可以直接管理 Workspace MCP / Skill 继承；选择下方 Environment 后再查看健康、Skill availability 和 Runtime。`
      : '未选择 Workspace：只管理全局定义和默认值，不显示 Workspace / Environment 选择状态。';
  elements.managementEnvironmentHint.textContent = managementContextError ? `${base} 部分状态不可用：${managementContextError}` : base;
}
function runtimeWriterOwner() {
  const environmentID = currentEnvironment()?.environment_id || '';
  return environmentID && runtimeWriterCredential.environmentID === environmentID ? runtimeWriterCredential.owner : '';
}
function clearRuntimeWriterCredential() {
  runtimeWriterCredential = {environmentID: '', owner: ''};
  if (elements.runtimeWriterOwnerInput) elements.runtimeWriterOwnerInput.value = '';
}
function generatedDesktopWriterOwner() {
  const uuid = globalThis.crypto?.randomUUID?.();
  return 'desktop-' + (uuid || (Date.now() + '-' + Math.random().toString(16).slice(2)));
}
function runtimeItem(titleText, idText, state, detailText, actions = []) {
  const item = document.createElement('article'); item.className = 'runtime-item';
  const head = document.createElement('div'); head.className = 'runtime-item-head';
  const title = document.createElement('strong'); title.textContent = titleText || idText || 'runtime item';
  head.append(title, stateBadge(state || 'unknown', state || 'unknown'));
  const id = document.createElement('code'); id.textContent = idText || '';
  const detail = document.createElement('small'); detail.textContent = detailText || '';
  item.append(head, id, detail);
  if (actions.length) { const actionRow = document.createElement('div'); actionRow.className = 'runtime-actions'; actionRow.append(...actions); item.append(actionRow); }
  return item;
}
function runtimeContainer(kind) { return ({verifiers: elements.verifierList, processes: elements.processList, runs: elements.runList})[kind]; }
function runtimeCountElement(kind) { return ({verifiers: elements.runtimeVerifierCount, processes: elements.runtimeProcessCount, runs: elements.runtimeRunCount})[kind]; }
function resetRuntimeCollections(state = 'unloaded', error = '') {
  runtimeLists = {verifiers: [], processes: [], runs: []};
  runtimeListStates = {verifiers: state, processes: state, runs: state};
  runtimeListErrors = {verifiers: error, processes: error, runs: error};
  runtimeRunsByID = new Map();
}
function syncRuntimeSubviewUI() {
  runtimeSubview = window.ADMRuntimeView?.normalizeSubview(runtimeSubview) || runtimeSubview || 'verifiers';
  for (const button of elements.runtimeSubviewTabs.querySelectorAll('[data-runtime-subview]')) {
    const active = button.dataset.runtimeSubview === runtimeSubview; button.setAttribute('aria-selected', String(active)); button.tabIndex = active ? 0 : -1;
  }
  for (const panel of document.querySelectorAll('[data-runtime-subview-panel]')) panel.hidden = panel.dataset.runtimeSubviewPanel !== runtimeSubview;
}
function clearRuntimeOutput(reason = '尚未选择资源') {
  runtimeOutputBinding = null;
  elements.runtimeOutputMeta.textContent = `有界 current-owner observation · ${reason}`;
  elements.runtimeOutput.textContent = '尚无输出';
}
function runtimeOutputBindingIsCurrent(binding = runtimeOutputBinding) {
  if (!binding) return false;
  const current = {...captureEnvironmentScope(), kind: runtimeSubview, id: binding.id, viewGeneration: runtimeSubviewGeneration};
  return window.ADMRuntimeView?.outputBindingMatches(binding, current) ?? false;
}
function validateRuntimeOutputBinding() {
  if (!runtimeOutputBinding) return;
  if (!runtimeOutputBindingIsCurrent(runtimeOutputBinding)) { clearRuntimeOutput('上下文或 Runtime 视图已变化'); return; }
  const known = window.ADMRuntimeView?.resourceStillKnown(runtimeOutputBinding, runtimeLists, runtimeListStates) ?? true;
  if (!known) clearRuntimeOutput('资源已不在最新 owner-local 列表');
}
function showRuntimeOutput(kind, id, title, stdout = '', stderr = '', meta = '', truncation = '') {
  if (runtimeSubview !== kind) return false;
  const scope = captureEnvironmentScope();
  runtimeOutputBinding = window.ADMRuntimeView?.makeOutputBinding(scope, kind, id, runtimeSubviewGeneration) || {...scope, kind, id, viewGeneration: runtimeSubviewGeneration};
  const environment = currentEnvironment(); const observedAt = new Date().toLocaleTimeString();
  elements.runtimeOutputMeta.textContent = `有界 current-owner observation · ${environment?.name || scope.environmentID || 'Environment'} · ${kind} ${id} · observed ${observedAt}${truncation ? ` · ${truncation}` : ''}`;
  const sections = [title]; if (meta) sections.push(meta); if (stdout) sections.push(`STDOUT\n${stdout}`); if (stderr) sections.push(`STDERR\n${stderr}`);
  elements.runtimeOutput.textContent = sections.filter(Boolean).join('\n\n') || '尚无输出';
  return true;
}
function renderRuntimeListState(kind, emptyText, renderItems) {
  const container = runtimeContainer(kind), count = runtimeCountElement(kind), state = runtimeListStates[kind], items = safeArray(runtimeLists[kind]);
  setMetric(count, state === 'success' ? items.length : state === 'loading' ? '…' : '—');
  if (!currentEnvironment()) return emptyMessage(container, '请选择 Environment');
  if (state === 'loading') return emptyMessage(container, `正在读取 ${kind}…`);
  if (state === 'error') return emptyMessage(container, `${kind} 状态不可用：${runtimeListErrors[kind] || 'unknown error'}`);
  if (state !== 'success') return emptyMessage(container, `${kind} 尚未加载`);
  if (!items.length) return emptyMessage(container, emptyText);
  container.replaceChildren(); container.classList.remove('empty'); renderItems(items, container);
}
function renderRuntime() {
  syncRuntimeSubviewUI();
  const environment = currentEnvironment(), writerOwner = runtimeWriterOwner(), pending = Boolean(runtimePendingActionKey);
  const activeWriter = Boolean(environment?.writer), writerAuthorized = activeWriter && Boolean(writerOwner);
  elements.runtimeHint.textContent = !environment
    ? '选择 Management Environment 后可查看当前 Gateway owner 的运行状态。'
    : activeWriter
      ? '当前 Environment 有 active Writer。服务端不会暴露 owner；只有当前 Desktop 会话已显式持有正确 owner 时，运行/停止/取消操作才可用。'
      : '当前 Environment 没有 active Writer；可查看 owner-local 状态与日志，也可以显式获取一个 Writer。';
  if (elements.runtimeWriterOwnerInput && runtimeWriterCredential.environmentID === environment?.environment_id && document.activeElement !== elements.runtimeWriterOwnerInput) elements.runtimeWriterOwnerInput.value = runtimeWriterCredential.owner || '';
  elements.runtimeWriterOwnerInput.disabled = !environment || pending;
  elements.runtimeWriterAcquireButton.disabled = !environment || pending;
  elements.runtimeWriterReleaseButton.disabled = !environment || pending || !writerAuthorized;
  elements.runtimeWriterHint.textContent = !environment ? '选择 Environment 后管理 Writer。' : writerOwner ? '当前 Desktop 会话已保存一个 Writer Owner；它只存在于内存，不写入 profile/state，也不会从服务端回显。' : activeWriter ? '检测到 active Writer，但 owner 被隐藏。要使用该 Writer，必须由人类显式输入正确 owner；错误 owner 不会获得 authority。' : '当前没有 active Writer。Owner 留空点击“获取 / 续租 Writer”会为本 Desktop 会话生成一个新 owner。';

  renderRuntimeListState('verifiers', '没有配置 verifier', (verifiers, container) => {
    for (const verifier of verifiers) {
      const run = createActionButton('运行', 'run-verifier', verifier.verifier_id); run.disabled = pending || !writerAuthorized || verifier.enabled === false;
      const args = safeArray(verifier.args).join(' '); const detail = `${verifier.kind || 'custom'} · ${verifier.executable || ''}${args ? ` ${args}` : ''}${verifier.cwd ? ` · cwd ${verifier.cwd}` : ''}`;
      container.append(runtimeItem(verifier.name || verifier.verifier_id, verifier.verifier_id, verifier.enabled === false ? 'disabled' : 'configured', detail, [run]));
    }
  });
  renderRuntimeListState('processes', '当前 Gateway owner 没有 process', (processes, container) => {
    for (const process of processes) {
      const logs = createActionButton('日志', 'process-logs', process.id); logs.disabled = pending; const actions = [logs];
      if (process.state === 'running') { const stop = createActionButton('停止', 'stop-process', process.id, 'danger'); stop.disabled = pending || !writerAuthorized; actions.push(stop); }
      const ports = safeArray(process.listening_ports); const detail = `PID ${process.pid || '—'}${ports.length ? ` · ports ${ports.join(', ')}` : ''}${process.exit_code !== undefined && process.exit_code !== null ? ` · exit ${process.exit_code}` : ''}${process.error_kind ? ` · ${process.error_kind}` : ''}`;
      container.append(runtimeItem(process.id, process.id, process.state, detail, actions));
    }
  });
  runtimeRunsByID = new Map(safeArray(runtimeLists.runs).map((run) => [run.id, run]));
  renderRuntimeListState('runs', '当前 Gateway owner 没有 generic run', (runs, container) => {
    for (const run of runs) {
      const output = createActionButton('输出', 'run-output', run.id); output.disabled = pending; const actions = [output];
      if (run.state === 'running') { const cancel = createActionButton('取消', 'cancel-run', run.id, 'danger'); cancel.disabled = pending || !writerAuthorized; actions.push(cancel); }
      const detail = `${run.executable || ''}${safeArray(run.args).length ? ` ${safeArray(run.args).join(' ')}` : ''}${run.cwd ? ` · cwd ${run.cwd}` : ''}${run.exit_code !== undefined && run.exit_code !== null ? ` · exit ${run.exit_code}` : ''}${run.error_kind ? ` · ${run.error_kind}` : ''}`;
      container.append(runtimeItem(run.id, run.id, run.state, detail, actions));
    }
  });
  validateRuntimeOutputBinding();
}
async function refreshRuntimeContext(showMessage = false, scope = captureEnvironmentScope()) {
  const environment = currentEnvironment();
  if (!environment || !scope.environmentID) {
    if (environmentScopeIsCurrent(scope)) { resetRuntimeCollections('unloaded'); renderRuntime(); }
    return {stale: false, errors: []};
  }
  if (environment.environment_id !== scope.environmentID || !environmentScopeIsCurrent(scope)) return {stale: true, errors: []};
  runtimeListStates = {verifiers: 'loading', processes: 'loading', runs: 'loading'}; runtimeListErrors = {verifiers: '', processes: '', runs: ''}; renderRuntime();
  if (showMessage) setStatus('正在刷新 Runtime 状态…', 'loading');
  const results = await Promise.allSettled([
    desktopAdapter().ListVerifiers(scope.environmentID), desktopAdapter().ListProcesses(scope.environmentID), desktopAdapter().ListRuns(scope.environmentID),
  ]);
  if (!environmentScopeIsCurrent(scope)) return {stale: true, errors: []};
  const descriptors = [
    ['verifiers', 'Verifiers', results[0]], ['processes', 'Processes', results[1]], ['runs', 'Runs', results[2]],
  ];
  const errors = [];
  for (const [kind, label, result] of descriptors) {
    if (result.status === 'fulfilled') { runtimeLists[kind] = safeArray(result.value); runtimeListStates[kind] = 'success'; runtimeListErrors[kind] = ''; }
    else { runtimeListStates[kind] = 'error'; runtimeListErrors[kind] = errorText(result.reason); errors.push(`${label}: ${runtimeListErrors[kind]}`); }
  }
  renderRuntime();
  if (showMessage) setStatus(errors.length ? `Runtime 部分状态不可用：${errors.join(' · ')}` : 'Runtime 状态已刷新', errors.length ? 'error' : 'success');
  return {stale: false, errors};
}
async function runRuntimeAction(label, actionKey, scope, action) {
  if (runtimePendingActionKey) return null;
  runtimePendingActionKey = actionKey; renderRuntime(); setStatus(`${label}…`, 'loading');
  try {
    let result;
    try { result = await action(); }
    catch (error) { if (environmentScopeIsCurrent(scope)) setStatus(`${label}失败：${error?.message || String(error)}`, 'error'); return null; }
    const refreshErrors = [];
    if (environmentScopeIsCurrent(scope)) {
      try { const refreshResult = await refreshRuntimeContext(false, scope); refreshErrors.push(...safeArray(refreshResult?.errors)); }
      catch (error) { refreshErrors.push(errorText(error)); }
    }
    if (environmentScopeIsCurrent(scope)) setStatus(refreshErrors.length ? `${label}完成；后续状态刷新部分失败：${refreshErrors.join(' · ')}` : `${label}完成`, refreshErrors.length ? 'error' : 'success');
    return result;
  } finally { runtimePendingActionKey = ''; renderRuntime(); }
}
function renderEnvironmentWorkspaceFilter(workspaces) {
  const current = elements.environmentWorkspaceFilter.value;
  elements.environmentWorkspaceFilter.replaceChildren();
  const all = document.createElement('option'); all.value = ''; all.textContent = '全部 Workspace'; elements.environmentWorkspaceFilter.append(all);
  for (const workspace of workspaces) {
    const option = document.createElement('option'); option.value = workspace.workspace_id || ''; option.textContent = workspace.name || workspace.workspace_id; elements.environmentWorkspaceFilter.append(option);
  }
  const exists = !current || workspaces.some((workspace) => workspace.workspace_id === current);
  if (!exists) { const invalid = document.createElement('option'); invalid.value = current; invalid.textContent = `已移除 · ${current}`; elements.environmentWorkspaceFilter.append(invalid); }
  elements.environmentWorkspaceFilter.value = current;
  elements.environmentFilterHint.hidden = exists;
  elements.environmentFilterHint.textContent = exists ? '' : `Workspace 筛选 ${current} 已失效。请选择“全部 Workspace”或其他 Workspace 重置筛选。`;
}
function renderWorkspaces(workspaces) {
  const model = window.ADMProjectPages?.workspaceListModel(workspaces, safeArray(currentSnapshot?.environments), elements.workspaceFilter.value) || {items: workspaces, total: workspaces.length, visible: workspaces.length, environmentCounts: {}};
  elements.workspaceBadge.textContent = String(model.total); renderWorkspaceOptions(workspaces); renderEnvironmentWorkspaceFilter(workspaces); setMetric(elements.workspaceVisibleCount, model.visible); setMetric(elements.workspaceListTotalCount, model.total);
  if (!model.total) return emptyMessage(elements.workspaceList, '暂无已登记 Workspace。添加现有目录即可；Git 不是前置条件。');
  if (!model.visible) return emptyMessage(elements.workspaceList, '没有符合当前筛选条件的 Workspace。');
  elements.workspaceList.replaceChildren(); elements.workspaceList.classList.remove('empty');
  for (const workspace of model.items) {
    const item = document.createElement('article'); item.className = 'list-item managed-item'; const content = document.createElement('div'); content.className = 'item-content';
    const title = document.createElement('strong'); title.textContent = workspace.name || workspace.workspace_id; const id = document.createElement('code'); id.textContent = workspace.workspace_id || ''; const path = document.createElement('span'); path.className = 'project-path'; path.textContent = workspace.path || '';
    const system = workspace.system_kind ? ` · System ${workspace.system_kind}` : ''; const meta = document.createElement('small'); meta.textContent = `Environments ${safeNumber(model.environmentCounts[workspace.workspace_id])} · MCP 继承 ${safeArray(workspace.enabled_mcp_ids).length} · Skill 继承 ${safeArray(workspace.enabled_skill_ids).length}${system}`; content.append(title, id, path, meta);
    const actions = document.createElement('div'); actions.className = 'item-actions'; actions.append(createActionButton('Discover projects', 'discover-workspace', workspace.workspace_id), createActionButton('查看 Environments', 'filter-environments-by-workspace', workspace.workspace_id), createActionButton('改名', 'rename-workspace', workspace.workspace_id)); if (!workspace.system_kind) actions.append(createActionButton('删除记录', 'remove-workspace', workspace.workspace_id, 'danger')); item.append(content, actions); elements.workspaceList.append(item);
  }
}
function environmentRetentionView(environment) {
  const retention = environment?.retention || {};
  const persistence = String(retention.persistence || '').trim();
  if (!persistence) return {persistence: 'unknown', label: 'Retention 未知', tone: 'configured', detail: 'Retention 尚未加载'};
  if (persistence !== 'temporary') return {persistence, label: 'Durable', tone: 'available', detail: '持久 Environment'};
  const expiresAt = retention.expires_at ? new Date(retention.expires_at) : null;
  const expired = Boolean(expiresAt && Number.isFinite(expiresAt.getTime()) && expiresAt.getTime() <= Date.now());
  return {persistence, label: expired ? 'Temporary · 已过期' : 'Temporary', tone: expired ? 'degraded' : 'selected', detail: expiresAt && Number.isFinite(expiresAt.getTime()) ? `到期 ${expiresAt.toLocaleString()}` : '无有效到期时间'};
}

function temporaryLifecycleStatus(environment) { return temporaryLifecycleByEnvironmentID.get(environment?.environment_id || '') || null; }
function temporaryLifecycleRows(environment) {
  const retention = environment?.retention || {}, status = temporaryLifecycleStatus(environment), view = environmentRetentionView(environment);
  const rows = [detailRow('Retention', view.label + ' · ' + view.detail)];
  if (view.persistence === 'temporary') {
    rows.push(detailRow('Lifecycle owner', retention.owner_id || '—'));
    if (retention.session_id) rows.push(detailRow('Session provenance', retention.session_id));
    if (retention.run_id) rows.push(detailRow('Run provenance', retention.run_id));
    rows.push(detailRow('Isolation', status ? (status.managed_worktree ? 'ADM managed worktree' : 'Existing Workspace root') : '尚未读取 lifecycle status'));
    if (status) rows.push(detailRow('Cleanup', status.cleanup_eligible ? 'Eligible' : (safeArray(status.blockers).join(', ') || status.cleanup_state || 'Blocked')));
  }
  return rows;
}

function renderTemporaryLifecycleNote(content, environment) {
  const view = environmentRetentionView(environment), status = temporaryLifecycleStatus(environment);
  const note = document.createElement('small'); note.className = 'environment-lifecycle-note';
  if (view.persistence !== 'temporary') { note.textContent = view.detail; content.append(note); return; }
  const retention = environment.retention || {}; const provenance = [retention.owner_id ? `Owner ${retention.owner_id}` : 'Owner 未知', retention.session_id ? `Session ${retention.session_id}` : '', retention.run_id ? `Run ${retention.run_id}` : ''].filter(Boolean);
  const mode = status ? (status.managed_worktree ? 'Managed worktree' : 'Existing root') : 'Mode 未读取';
  const blockers = status && !status.cleanup_eligible ? safeArray(status.blockers).join(', ') : '';
  note.textContent = `${view.detail} · ${provenance.join(' · ')} · ${mode}${blockers ? ` · Blocked: ${blockers}` : ''}`;
  content.append(note);
}

function renderEnvironmentWorkspaceDialog() {
  const environment = safeArray(currentSnapshot?.environments).find((item) => item.environment_id === environmentWorkspaceEnvironmentID) || null;
  if (elements.environmentWorkspaceDialogTitle) elements.environmentWorkspaceDialogTitle.textContent = `调整 Workspace 归属 · ${environment?.name || environmentWorkspaceEnvironmentID || 'Environment'}`;
  if (!elements.environmentWorkspaceTarget) return;
  elements.environmentWorkspaceTarget.replaceChildren();
  elements.environmentWorkspaceSaveButton.disabled = true;
  if (environmentWorkspaceLoading) {
    const option = document.createElement('option'); option.value = ''; option.textContent = '正在读取可选 Workspace…'; elements.environmentWorkspaceTarget.append(option); elements.environmentWorkspaceTarget.disabled = true;
    elements.environmentWorkspaceSummary.textContent = '正在读取包含当前 Environment root 的 Workspace。'; elements.environmentWorkspaceReason.textContent = '';
    return;
  }
  const options = environmentWorkspaceOptions;
  if (!options) {
    const option = document.createElement('option'); option.value = ''; option.textContent = '未加载'; elements.environmentWorkspaceTarget.append(option); elements.environmentWorkspaceTarget.disabled = true;
    elements.environmentWorkspaceSummary.textContent = 'Workspace 归属信息不可用。'; return;
  }
  const candidates = safeArray(options.candidates);
  for (const workspace of candidates) {
    const option = document.createElement('option'); option.value = workspace.workspace_id || ''; option.textContent = `${workspace.name || workspace.workspace_id} · ${workspace.path || ''}${workspace.workspace_id === options.current_workspace_id ? ' · 当前' : ''}${workspace.workspace_id === options.recommended_workspace_id ? ' · 推荐' : ''}`; elements.environmentWorkspaceTarget.append(option);
  }
  if (!candidates.length) {
    const option = document.createElement('option'); option.value = ''; option.textContent = '没有可选 Workspace'; elements.environmentWorkspaceTarget.append(option);
  }
  const preferred = options.recommended_workspace_id || options.current_workspace_id || '';
  elements.environmentWorkspaceTarget.value = candidates.some((item) => item.workspace_id === preferred) ? preferred : (candidates[0]?.workspace_id || '');
  elements.environmentWorkspaceTarget.disabled = !options.rebind_allowed || candidates.length === 0;
  const current = candidates.find((item) => item.workspace_id === options.current_workspace_id);
  const recommended = candidates.find((item) => item.workspace_id === options.recommended_workspace_id);
  elements.environmentWorkspaceSummary.textContent = `当前：${current?.name || options.current_workspace_id || '未知'}${recommended ? ` · 最具体建议：${recommended.name || recommended.workspace_id}` : ''}`;
  elements.environmentWorkspaceReason.textContent = options.reason || (options.rebind_allowed ? '只允许选择包含当前 Environment root 的非系统 Workspace；explicit MCP/Skill 选择会保留，继承层按新 Workspace 重算。' : '当前 Environment 不允许调整 Workspace 归属。');
  elements.environmentWorkspaceSaveButton.disabled = !options.rebind_allowed || !elements.environmentWorkspaceTarget.value || elements.environmentWorkspaceTarget.value === options.current_workspace_id;
}

async function openEnvironmentWorkspaceDialog(environment) {
  if (!environment?.environment_id) return;
  environmentWorkspaceGeneration++; const generation = environmentWorkspaceGeneration; const connection = connectionGeneration;
  environmentWorkspaceEnvironmentID = environment.environment_id; environmentWorkspaceOptions = null; environmentWorkspaceLoading = true; renderEnvironmentWorkspaceDialog(); openEditorDialog('environmentWorkspaceDialog');
  try {
    const options = await desktopAdapter().EnvironmentWorkspaceOptions(environment.environment_id);
    if (generation !== environmentWorkspaceGeneration || connection !== connectionGeneration || environmentWorkspaceEnvironmentID !== environment.environment_id) return;
    environmentWorkspaceOptions = options; environmentWorkspaceLoading = false; renderEnvironmentWorkspaceDialog();
  } catch (error) {
    if (generation !== environmentWorkspaceGeneration || connection !== connectionGeneration || environmentWorkspaceEnvironmentID !== environment.environment_id) return;
    environmentWorkspaceLoading = false; environmentWorkspaceOptions = null; renderEnvironmentWorkspaceDialog(); elements.environmentWorkspaceReason.textContent = `读取 Workspace 归属失败：${errorText(error)}`;
  }
}

async function promptRecommendedWorkspaceAfterCreate(environment, requestConnectionGeneration) {
  if (!environment?.environment_id || requestConnectionGeneration !== connectionGeneration) return;
  try {
    const options = await desktopAdapter().EnvironmentWorkspaceOptions(environment.environment_id);
    if (requestConnectionGeneration !== connectionGeneration) return;
    if (!options?.rebind_allowed || !options.recommended_workspace_id || options.recommended_workspace_id === options.current_workspace_id) return;
    environmentWorkspaceGeneration++;
    environmentWorkspaceEnvironmentID = environment.environment_id;
    environmentWorkspaceOptions = options;
    environmentWorkspaceLoading = false;
    renderEnvironmentWorkspaceDialog();
    elements.environmentWorkspaceReason.textContent = `刚创建的 Environment 当前归属不是最具体 Workspace。这里只给出建议，不会自动迁移；确认保存后才会调整归属。 ${elements.environmentWorkspaceReason.textContent}`;
    openEditorDialog('environmentWorkspaceDialog');
  } catch (error) {
    if (requestConnectionGeneration === connectionGeneration) setStatus(`Environment 已创建，但推荐 Workspace 检查失败：${errorText(error)}`, 'error');
  }
}

function renderEnvironmentWorkspaceRecommendationsDialog() {
  const items = safeArray(environmentWorkspaceRecommendations);
  elements.environmentWorkspaceRecommendationsApplyButton.disabled = environmentWorkspaceRecommendationsLoading || environmentWorkspaceRecommendationsApplying || items.length === 0;
  elements.environmentWorkspaceRecommendationsList.replaceChildren();
  if (environmentWorkspaceRecommendationsLoading) {
    elements.environmentWorkspaceRecommendationsSummary.textContent = '正在检查所有普通 Environment 的 Workspace 归属…';
    emptyMessage(elements.environmentWorkspaceRecommendationsList, '正在检查推荐归属…');
  } else if (!items.length) {
    elements.environmentWorkspaceRecommendationsSummary.textContent = '当前没有需要调整的 Environment。';
    emptyMessage(elements.environmentWorkspaceRecommendationsList, '所有普通 Environment 已绑定到最具体的可用 Workspace。');
  } else {
    elements.environmentWorkspaceRecommendationsSummary.textContent = `发现 ${items.length} 个可优化的 Environment。应用时会逐个重新校验，预览不会自动修改归属。`;
    elements.environmentWorkspaceRecommendationsList.classList.remove('empty');
    for (const item of items) {
      const row = document.createElement('article'); row.className = 'list-item managed-item';
      const content = document.createElement('div'); content.className = 'item-content';
      const title = document.createElement('strong'); title.textContent = item.environment_name || item.environment_id || 'Environment';
      const id = document.createElement('code'); id.textContent = item.environment_id || '';
      const root = document.createElement('span'); root.className = 'project-path'; root.textContent = item.root || '';
      const current = item.current_workspace || {}; const recommended = item.recommended_workspace || {};
      const detail = document.createElement('small'); detail.textContent = `${current.name || current.workspace_id || '未知'} → ${recommended.name || recommended.workspace_id || '未知'} · ${recommended.path || ''}`;
      content.append(title, id, root, detail); row.append(content); elements.environmentWorkspaceRecommendationsList.append(row);
    }
  }
  elements.environmentWorkspaceRecommendationsApplyButton.textContent = environmentWorkspaceRecommendationsApplying ? '正在应用…' : '应用全部建议';
  elements.environmentWorkspaceRecommendationsResult.textContent = environmentWorkspaceRecommendationsResult;
}

async function openEnvironmentWorkspaceRecommendationsDialog() {
  environmentWorkspaceRecommendationsGeneration++; const generation = environmentWorkspaceRecommendationsGeneration; const connection = connectionGeneration;
  environmentWorkspaceRecommendations = []; environmentWorkspaceRecommendationsLoading = true; environmentWorkspaceRecommendationsApplying = false; environmentWorkspaceRecommendationsResult = '';
  renderEnvironmentWorkspaceRecommendationsDialog(); openEditorDialog('environmentWorkspaceRecommendationsDialog');
  try {
    const items = await desktopAdapter().EnvironmentWorkspaceRecommendations();
    if (generation !== environmentWorkspaceRecommendationsGeneration || connection !== connectionGeneration) return;
    environmentWorkspaceRecommendations = safeArray(items); environmentWorkspaceRecommendationsLoading = false; renderEnvironmentWorkspaceRecommendationsDialog();
  } catch (error) {
    if (generation !== environmentWorkspaceRecommendationsGeneration || connection !== connectionGeneration) return;
    environmentWorkspaceRecommendationsLoading = false; environmentWorkspaceRecommendationsResult = `检查推荐归属失败：${errorText(error)}`; renderEnvironmentWorkspaceRecommendationsDialog();
  }
}

async function promptWorkspaceRecommendationsAfterWorkspaceAdd(workspace, requestConnectionGeneration) {
  if (!workspace?.workspace_id || requestConnectionGeneration !== connectionGeneration) return;
  try {
    const items = safeArray(await desktopAdapter().EnvironmentWorkspaceRecommendations()).filter((item) => item?.recommended_workspace?.workspace_id === workspace.workspace_id);
    if (requestConnectionGeneration !== connectionGeneration || !items.length) return;
    environmentWorkspaceRecommendationsGeneration++;
    environmentWorkspaceRecommendations = items;
    environmentWorkspaceRecommendationsLoading = false;
    environmentWorkspaceRecommendationsApplying = false;
    environmentWorkspaceRecommendationsResult = `刚添加的 Workspace ${workspace.name || workspace.workspace_id} 成为 ${items.length} 个 Environment 的更具体推荐归属。这里只预览，不会自动迁移。`;
    renderEnvironmentWorkspaceRecommendationsDialog();
    openEditorDialog('environmentWorkspaceRecommendationsDialog');
  } catch (error) {
    if (requestConnectionGeneration === connectionGeneration) setStatus(`Workspace 已添加，但 Environment 推荐归属检查失败：${errorText(error)}`, 'error');
  }
}

async function applyEnvironmentWorkspaceRecommendations() {
  if (environmentWorkspaceRecommendationsLoading || environmentWorkspaceRecommendationsApplying) return;
  const items = safeArray(environmentWorkspaceRecommendations).slice(); if (!items.length) return;
  const generation = environmentWorkspaceRecommendationsGeneration; const connection = connectionGeneration;
  environmentWorkspaceRecommendationsApplying = true; environmentWorkspaceRecommendationsResult = ''; renderEnvironmentWorkspaceRecommendationsDialog();
  let changed = 0; const failures = [];
  for (const item of items) {
    try {
      await desktopAdapter().SetEnvironmentWorkspace(item.environment_id, item.recommended_workspace?.workspace_id || ''); changed++;
    } catch (error) {
      failures.push(`${item.environment_name || item.environment_id}: ${errorText(error)}`);
    }
  }
  if (generation !== environmentWorkspaceRecommendationsGeneration || connection !== connectionGeneration) return;
  try { await refreshSnapshot(); } catch (_) {}
  if (generation !== environmentWorkspaceRecommendationsGeneration || connection !== connectionGeneration) return;
  try { environmentWorkspaceRecommendations = safeArray(await desktopAdapter().EnvironmentWorkspaceRecommendations()); }
  catch (error) { failures.push(`重新检查失败：${errorText(error)}`); }
  environmentWorkspaceRecommendationsApplying = false;
  environmentWorkspaceRecommendationsResult = failures.length ? `已调整 ${changed} 个；${failures.length} 个失败：${failures.join('；')}` : `已调整 ${changed} 个 Environment；当前剩余 ${environmentWorkspaceRecommendations.length} 个推荐项。`;
  renderEnvironmentWorkspaceRecommendationsDialog();
  setStatus(failures.length ? `Workspace 批量调整部分失败（成功 ${changed}，失败 ${failures.length}）` : `Workspace 批量调整完成（${changed}）`, failures.length ? 'error' : 'success');
}

function renderEnvironments(environments) {
  const workspaces = safeArray(currentSnapshot?.workspaces); const workspaceID = elements.environmentWorkspaceFilter.value;
  const model = window.ADMProjectPages?.environmentListModel(environments, workspaces, {query: elements.environmentFilter.value, workspaceID}) || {items: environments, total: environments.length, visible: environments.length, workspaceNames: {}, workspaceFilterValid: true};
  elements.environmentBadge.textContent = String(model.total); setMetric(elements.environmentVisibleCount, model.visible); setMetric(elements.environmentListTotalCount, model.total);
  elements.environmentFilterHint.hidden = model.workspaceFilterValid; if (model.workspaceFilterValid) elements.environmentFilterHint.textContent = '';
  if (!model.total) return emptyMessage(elements.environmentList, '暂无 Environment。可以从任意已登记 Workspace 创建，Git 不是前置条件。');
  if (!model.visible) return emptyMessage(elements.environmentList, model.workspaceFilterValid ? '没有符合当前筛选条件的 Environment。' : '当前 Workspace 筛选已失效；重置筛选后可查看现有 Environment。');
  elements.environmentList.replaceChildren(); elements.environmentList.classList.remove('empty');
  for (const environment of model.items) {
    const currentContext = environment.environment_id === managementEnvironmentID; const item = document.createElement('article'); item.className = 'list-item managed-item'; item.dataset.environmentId = environment.environment_id || ''; if (currentContext) item.classList.add('current-context');
    const content = document.createElement('div'); content.className = 'item-content'; const titleLine = document.createElement('div'); titleLine.className = 'item-title-line';
    const title = document.createElement('strong'); title.textContent = environment.name || environment.environment_id; const contextMarker = stateBadge('当前管理环境', 'selected'); contextMarker.classList.add('context-marker'); contextMarker.hidden = !currentContext; const retentionView = environmentRetentionView(environment); titleLine.append(title, contextMarker, stateBadge(retentionView.label, retentionView.tone));
    const id = document.createElement('code'); id.textContent = environment.environment_id || ''; const root = document.createElement('span'); root.className = 'project-path'; root.textContent = environment.root || '';
    const workspaceName = model.workspaceNames[environment.workspace_id] || environment.workspace_id || '—';
    const meta = document.createElement('small'); meta.textContent = `Workspace ${workspaceName}${environment.workspace_id ? ` (${environment.workspace_id})` : ''} · ${environment.state || 'unknown'} · MCP ${safeArray(environment.enabled_mcp_ids).length} · Skills ${safeArray(environment.enabled_skill_ids).length} · Private Memory ${safeNumber(environment.private_memory_count)} · ${environment.writer ? `Writer active${environment.writer.expires_at ? ` until ${new Date(environment.writer.expires_at).toLocaleTimeString()}` : ''}` : 'No writer'}`; content.append(titleLine, id, root, meta); renderTemporaryLifecycleNote(content, environment);
    const actions = document.createElement('div'); actions.className = 'item-actions'; actions.append(createActionButton('详情', 'inspect-environment', environment.environment_id), createActionButton('诊断', 'diagnose-environment', environment.environment_id));
    if (retentionView.persistence === 'temporary') {
      actions.append(createActionButton('刷新生命周期', 'refresh-temporary-environment', environment.environment_id), createActionButton('Promote durable', 'promote-temporary-environment', environment.environment_id), createActionButton('清理预览', 'preview-temporary-environment-cleanup', environment.environment_id));
      const lifecycle = temporaryLifecycleStatus(environment); if (temporaryCleanupPreviewIDs.has(environment.environment_id) && lifecycle?.cleanup_eligible) actions.append(createActionButton('确认清理', 'execute-temporary-environment-cleanup', environment.environment_id, 'danger'));
    }
    actions.append(createActionButton('调整 Workspace', 'workspace-environment', environment.environment_id), createActionButton('改名', 'rename-environment', environment.environment_id), createActionButton('删除记录', 'remove-environment', environment.environment_id, 'danger')); item.append(content, actions); elements.environmentList.append(item);
  }
}
function updateEnvironmentContextMarkers() {
  for (const item of elements.environmentList.querySelectorAll('[data-environment-id]')) {
    const current = item.dataset.environmentId === managementEnvironmentID;
    item.classList.toggle('current-context', current);
    const marker = item.querySelector('.context-marker'); if (marker) marker.hidden = !current;
  }
}
function renderExecutables(executables, denials = []) {
  executables = safeArray(executables);
  denials = safeArray(denials).slice().sort((a, b) => (safeNumber(b.count) - safeNumber(a.count)) || String(a.executable || '').localeCompare(String(b.executable || '')));
  const denialCount = denials.reduce((sum, item) => sum + safeNumber(item.count), 0);
  elements.execBadge.textContent = String(executables.length);
  setMetric(elements.execAllowedCount, executables.length);
  setMetric(elements.execBlockedCount, denialCount);
  elements.execClearAllBlockedButton.disabled = denials.length === 0;

  if (!executables.length) emptyMessage(elements.execList, '暂无 allowed executable');
  else {
    elements.execList.replaceChildren(); elements.execList.classList.remove('empty');
    for (const executable of executables) {
      const row = document.createElement('div'); row.className = 'managed-row exec-row';
      const content = document.createElement('div'); content.className = 'item-content';
      const title = document.createElement('div'); title.className = 'item-title-line';
      const code = document.createElement('code'); code.textContent = executable;
      title.append(code, stateBadge('允许', 'available'));
      const meta = document.createElement('small'); meta.textContent = 'Runtime 可以准备此 executable；真正执行仍需要对应操作权限和 writer。';
      content.append(title, meta);
      row.append(content, createActionButton('移除', 'remove-executable', executable, 'danger'));
      elements.execList.append(row);
    }
  }

  if (!denials.length) emptyMessage(elements.execBlockedList, '暂无执行授权观察');
  else {
    elements.execBlockedList.replaceChildren(); elements.execBlockedList.classList.remove('empty');
    for (const denial of denials) {
      const executable = denial.executable || '';
      const row = document.createElement('div'); row.className = 'managed-row exec-denial-row'; row.dataset.executable = executable;
      const content = document.createElement('div'); content.className = 'item-content';
      const title = document.createElement('div'); title.className = 'item-title-line';
      const code = document.createElement('code'); code.textContent = executable || 'unknown executable';
      const bypass = String(denial.last_reason || '').includes('full_authorization_bypass');
      title.append(code, stateBadge((bypass ? 'Full 旁路 × ' : '拒绝 × ') + safeNumber(denial.count, 1), bypass ? 'degraded' : 'error'));
      const meta = document.createElement('small');
      meta.textContent = [denial.last_surface || 'unknown surface', denial.last_environment_id || 'unknown Environment', formatDateTime(denial.last_blocked_at)].join(' · ');
      const reason = document.createElement('small'); reason.className = 'warning-text'; reason.textContent = denial.last_reason || 'executable was blocked by Runtime allowlist';
      content.append(title, meta, reason);
      const actions = document.createElement('div'); actions.className = 'item-actions';
      actions.append(createActionButton(bypass ? '加入白名单' : '允许', 'allow-blocked-executable', executable), createActionButton('清理', 'clear-blocked-executable', executable, 'danger'));
      row.append(content, actions);
      elements.execBlockedList.append(row);
    }
  }
}

function resourceHeader(titleText, idText, badges) {
  const header = document.createElement('div'); header.className = 'resource-title-line'; const title = document.createElement('strong'); title.textContent = titleText; header.append(title, ...badges);
  const id = document.createElement('code'); id.textContent = idText || ''; return {header, id};
}
function mcpConfigured(entry) { return entry.transport === 'stdio' ? Boolean(entry.executable) : Boolean(entry.endpoint); }
function mcpConfigurationState(entry, health) {
  if (!mcpConfigured(entry)) return 'unconfigured';
  if (health?.error_kind === 'unresolved_secret_reference') return 'unavailable';
  return 'configured';
}
function mcpRuntimeState(health) { return health?.error_kind ? 'error' : (health?.state || 'not_observed'); }
function mcpProbeStateLabel(health) {
  if (health?.error_kind === 'unresolved_secret_reference') return '受阻';
  if (health?.state === 'configured' && !health?.error_kind) return '未完成';
  return humanRuntimeState(mcpRuntimeState(health));
}
function mcpProbeRecoveryHint(health, entry) {
  const kind = health?.error_kind || '';
  if (!kind) return '';
  const refs = mcpReferenceVariableNames(entry);
  if (kind === 'unresolved_secret_reference') return `处理建议：在 ADM 服务进程环境中设置 ${refs.length ? refs.join(', ') : '对应引用变量'}，重启 ADM 后重新探测。`;
  if (kind === 'auth_failure') return `处理建议：确认 HTTP Auth 为 Secret-backed headers、Header reference mapping 正确，并确认 ${refs.length ? refs.join(', ') : '引用变量'} 已注入 ADM 服务进程；环境变量变更后重启 ADM。`;
  if (kind === 'connection_refused') return '处理建议：确认 MCP 服务已启动，Endpoint/端口正确，并且本机能够访问该地址。';
  if (kind === 'timeout') return '处理建议：确认 Endpoint 可访问；服务较慢时可提高 Probe timeout 后重试。';
  if (kind === 'tool_list_failed') return '处理建议：确认 Endpoint 指向 Streamable HTTP MCP 服务，并且服务支持 tools/list。';
  if (kind === 'activation_failed') return entry?.transport === 'stdio' ? '处理建议：检查 executable、Environment references 与执行许可。' : '处理建议：检查 Endpoint、HTTP Auth 与 Header reference mappings。';
  return '处理建议：检查 Endpoint、Transport、TLS/代理；使用 Header reference 时选择 Secret-backed headers，并确认引用环境变量存在于 ADM 服务进程中。';
}
function renderMCPManager(mcps) {
  const scrollTop = elements.mcpList.scrollTop;
  try { renderMCPManagerContents(mcps); } finally { elements.mcpList.scrollTop = scrollTop; }
}
function renderMCPManagerContents(mcps) {
  const workspace = currentWorkspace(), environment = currentEnvironment(); const selected = new Set(safeArray(environment?.enabled_mcp_ids)); const explicitSelected = new Set(Array.isArray(environment?.explicit_mcp_ids) ? environment.explicit_mcp_ids : safeArray(environment?.enabled_mcp_ids)); const workspaceSelected = new Set(safeArray(workspace?.enabled_mcp_ids));
  const configIssueStates = new Set(['unavailable', 'degraded', 'unconfigured', 'error']);
  const runtimeIssueStates = new Set(['unavailable', 'degraded', 'unconfigured', 'error']);
  const queryText = String(elements.mcpFilter?.value || '').trim(); const queryMatches = searchMatcher(queryText); const filter = elements.mcpStateFilter?.value || 'all';
  let issues = 0, visible = 0;
  elements.mcpBadge.textContent = String(mcps.length); setMetric(elements.mcpTotalCount, mcps.length); setMetric(elements.mcpListTotalCount, mcps.length); setMetric(elements.mcpDefaultCount, mcps.filter((mcp) => mcp.default_include_in_environment).length); setMetric(elements.mcpEnvironmentCount, environment ? mcps.filter((mcp) => selected.has(mcp.id)).length : 0);
  if (!mcps.length) { setMetric(elements.mcpIssueCount, 0); setMetric(elements.mcpVisibleCount, 0); emptyMessage(elements.mcpList, '暂无 MCP 定义。可以添加 typed MCP，或先预览再导入 JSON/JSONC。'); updateMCPBulkControls(); return; }
  elements.mcpList.replaceChildren(); elements.mcpList.classList.remove('empty');
  for (const entry of mcps) {
    const enabled = environment ? selected.has(entry.id) : false;
    const observation = mcpHealthByKey.get(mcpHealthKey(entry.id));
    const health = observation?.fingerprint === mcpProbeFingerprint(entry) ? observation.health : null;
    const configState = mcpConfigurationState(entry, health); const runtimeState = mcpRuntimeState(health); const configNormalized = normalizedState(configState); const runtimeNormalized = normalizedState(runtimeState);
    const issue = Boolean(health?.error_kind) || configIssueStates.has(configNormalized) || runtimeIssueStates.has(runtimeNormalized); if (issue) issues++;
    const haystack = [entry.name, entry.id, entry.endpoint, entry.executable, entry.transport].filter(Boolean).join(' ');
    const filterMatch = filter === 'all' || (filter === 'selected' && Boolean(environment && enabled)) || (filter === 'unselected' && Boolean(environment && !enabled)) || (filter === 'issues' && issue) || (filter === 'unobserved' && runtimeNormalized === 'not_observed');
    if (!queryMatches(haystack) || !filterMatch) continue;
    visible++;
    const row = document.createElement('article'); row.className = 'resource-row'; row.dataset.id = entry.id || ''; if (entry.id === editingMCPID) row.dataset.editing = 'true';
    const main = document.createElement('div'); main.className = 'resource-main';
    const badges = [stateBadge(entry.transport || 'streamable-http', 'transport'), stateBadge(`配置 · ${health?.error_kind === 'unresolved_secret_reference' ? '引用未解析' : humanConfigState(configState)}`, configState), stateBadge(`全局探测 · ${mcpProbeStateLabel(health)}`, runtimeState)];
    const {header, id} = resourceHeader(entry.name || entry.id, entry.id, badges);
    const refCount = entry.transport === 'stdio' ? Object.keys(entry.env_refs || {}).length : Object.keys(entry.header_refs || {}).length;
    const detail = document.createElement('div'); detail.className = 'resource-detail'; detail.textContent = entry.transport === 'stdio' ? `Executable: ${textOrDash(entry.executable)}${safeArray(entry.args).length ? ` · ${entry.args.length} args` : ''}${refCount ? ` · ${refCount} env refs` : ''}` : `Endpoint: ${textOrDash(entry.endpoint)} · Auth: ${entry.auth_mode || 'none'}${refCount ? ` · ${refCount} header refs` : ''}`;
    main.append(header, id, detail);
    const configMessage = health?.error_kind === 'unresolved_secret_reference' ? '配置引用尚未解析' : '';
    if (configMessage) { const note = document.createElement('small'); note.textContent = `配置：${configMessage}`; main.append(note); }
    const referenceNames = mcpReferenceVariableNames(entry);
    if (referenceNames.length && ['unavailable', 'unconfigured'].includes(configNormalized)) { const note = document.createElement('small'); note.className = 'reference-text'; note.textContent = `配置引用：${referenceNames.join(', ')}（至少一个当前未解析）`; main.append(note); }
    const runtimeMessage = health?.message || (!health ? '尚未显式探测；刷新页面不会自动连接 MCP。' : '');
    if (runtimeMessage) { const note = document.createElement('small'); note.textContent = `全局探测：${runtimeMessage}`; main.append(note); }
    const recoveryHint = mcpProbeRecoveryHint(health, entry);
    if (recoveryHint) { const note = document.createElement('small'); note.className = 'reference-text'; note.textContent = recoveryHint; main.append(note); }
    const controls = document.createElement('div'); controls.className = 'resource-actions';
    const selectControl = checkControl('选择', selectedMCPIDs.has(entry.id), 'select-mcp', entry.id, mcpBulkBusy || capabilityAssignmentBusy); selectControl.classList.add('skill-select-control');
    controls.append(selectControl, checkControl('新环境默认', entry.default_include_in_environment, 'default-mcp', entry.id));
    controls.append(checkControl(workspace ? '当前 Workspace 启用' : '选择 Workspace 后启用', workspaceSelected.has(entry.id), 'workspace-mcp', entry.id, !workspace));
    controls.append(checkControl(environment ? '当前 Environment 显式启用' : '选择 Environment 后附加启用', explicitSelected.has(entry.id), 'environment-mcp', entry.id, !environment));
    const edit = createActionButton(entry.id === editingMCPID ? '正在编辑' : '编辑', 'edit-mcp', entry.id); edit.disabled = entry.id === editingMCPID;
    const probe = createActionButton(health ? '重新探测' : '探测', 'probe-mcp', entry.id); probe.disabled = mcpProbeRequests.has(entry.id); if (probe.disabled) probe.textContent = '探测中…';
    controls.append(edit, createActionButton('复制配置', 'copy-mcp-config', entry.id), probe, createActionButton('删除全局定义', 'remove-mcp', entry.id, 'danger'));
    row.append(main, controls); elements.mcpList.append(row);
  }
  setMetric(elements.mcpIssueCount, issues); setMetric(elements.mcpVisibleCount, visible);
  if (!visible) emptyMessage(elements.mcpList, queryText || filter !== 'all' ? '没有符合当前筛选条件的 MCP。' : '暂无 MCP 定义。');
  updateMCPBulkControls();
}
function skillAvailabilityState(entry, environment, selected) {
  const availability = skillAvailabilityByID.get(entry.id); if (availability?.state) return availability.state; if (!environment) return entry.artifact_path && entry.source_root ? 'configured' : 'unconfigured'; return selected ? 'not_observed' : 'disabled';
}
function skillCatalogFingerprint(skills = safeArray(currentSnapshot?.skills)) {
  return window.ADMSkillBulk?.catalogFingerprint(skills) || skills.map((skill) => skill.id || '').filter(Boolean).sort().join('|');
}
function currentSkillProbeIdentity() {
  return {connectionGeneration, scope: 'catalog', catalogFingerprint: skillCatalogFingerprint()};
}
function explicitSkillProbeIsCurrent() {
  return Boolean(window.ADMSkillBulk?.probeMatches(explicitSkillAvailabilityProbe, currentSkillProbeIdentity()));
}
function skillAvailabilitySummary() {
  return window.ADMSkillBulk?.summarizeAvailability(safeArray(currentSnapshot?.skills), [...skillAvailabilityByID.values()]) || {available: 0, disabled: 0, unavailable: 0, unknown: 0, cleanupIDs: []};
}
function pruneSkillSelection() {
  const retained = window.ADMSkillBulk?.retainExistingSelection([...selectedSkillIDs], safeArray(currentSnapshot?.skills)) || [];
  selectedSkillIDs = new Set(retained);
}
function pruneMCPSelection() {
  const retained = window.ADMEnvironmentBulk?.retainExistingSelection([...selectedMCPIDs], safeArray(currentSnapshot?.mcps)) || [];
  selectedMCPIDs = new Set(retained);
}
function pruneAssignmentEnvironmentSelection() {
  const retained = window.ADMEnvironmentBulk?.retainExistingSelection([...selectedAssignmentEnvironmentIDs], safeArray(currentSnapshot?.environments), 'environment_id') || [];
  selectedAssignmentEnvironmentIDs = new Set(retained);
}
function selectedCapabilityCount() { return selectedMCPIDs.size + selectedSkillIDs.size; }
function missingSelectionOperations(kind) {
  const entries = kind === 'mcp' ? safeArray(currentSnapshot?.mcps) : safeArray(currentSnapshot?.skills);
  const workspaceOps = window.ADMEnvironmentBulk?.missingWorkspaceSelections(safeArray(currentSnapshot?.workspaces), entries, kind) || [];
  const environmentOps = window.ADMEnvironmentBulk?.missingSelections(safeArray(currentSnapshot?.environments), entries, kind) || [];
  return [...workspaceOps, ...environmentOps];
}
async function clearMissingEnvironmentSelections(kind) {
  const operations = missingSelectionOperations(kind);
  const label = kind === 'mcp' ? 'MCP' : 'Skill';
  if (!operations.length) return setStatus(`没有需要清理的不存在 ${label} 引用。`, 'success');
  const workspaceCount = new Set(operations.filter((operation) => operation.scope === 'workspace').map((operation) => operation.workspaceID)).size;
  const environmentCount = new Set(operations.filter((operation) => operation.scope === 'environment').map((operation) => operation.environmentID)).size;
  const resourceCount = new Set(operations.map((operation) => operation.resourceID)).size;
  if (!window.confirm(`将清除 ${resourceCount} 个不存在的 ${label} ID，共 ${operations.length} 条引用（Workspace ${workspaceCount} 个 · Environment ${environmentCount} 个）。继续？`)) return;
  const requestGeneration = connectionGeneration;
  if (kind === 'mcp') mcpBulkBusy = true; else skillBulkBusy = true;
  updateMCPBulkControls(); updateSkillBulkControls(); setStatus(`正在清理不存在的 ${label} 引用…`, 'loading');
  let changed = 0, failed = 0;
  try {
    for (const operation of operations) {
      if (requestGeneration !== connectionGeneration) break;
      try {
        if (operation.scope === 'workspace') {
          if (kind === 'mcp') await desktopAdapter().SetWorkspaceMCP(operation.workspaceID, operation.resourceID, false);
          else await desktopAdapter().SetWorkspaceSkill(operation.workspaceID, operation.resourceID, false);
        } else if (kind === 'mcp') await desktopAdapter().SetEnvironmentMCP(operation.environmentID, operation.resourceID, false);
        else await desktopAdapter().SetEnvironmentSkill(operation.environmentID, operation.resourceID, false);
        changed++;
      } catch (error) { failed++; }
    }
    if (requestGeneration === connectionGeneration) {
      await refreshSnapshot();
      setStatus(`不存在的 ${label} 引用清理完成：已清理 ${changed} · 失败 ${failed}`, failed ? 'error' : 'success');
    }
  } finally {
    if (kind === 'mcp') mcpBulkBusy = false; else skillBulkBusy = false;
    if (requestGeneration === connectionGeneration) { updateMCPBulkControls(); updateSkillBulkControls(); }
  }
}
function updateMCPBulkControls() {
  pruneMCPSelection();
  const visibleCount = visibleResourceIDs(elements.mcpList).length;
  const environment = currentEnvironment();
  const selectedCount = selectedMCPIDs.size;
  const disabled = mcpBulkBusy || capabilityAssignmentBusy || visibleCount === 0;
  elements.mcpSelectedCount.textContent = String(selectedCount);
  elements.mcpSelectVisibleButton.disabled = mcpBulkBusy || capabilityAssignmentBusy || visibleCount === 0;
  elements.mcpClearSelectionButton.disabled = mcpBulkBusy || capabilityAssignmentBusy || selectedCount === 0;
  elements.mcpAssignEnvironmentsButton.disabled = mcpBulkBusy || capabilityAssignmentBusy || selectedCapabilityCount() === 0 || !safeArray(currentSnapshot?.environments).length;
  const missing = missingSelectionOperations('mcp'); elements.mcpClearMissingButton.disabled = mcpBulkBusy || capabilityAssignmentBusy || missing.length === 0; elements.mcpClearMissingButton.textContent = missing.length ? `清除不存在引用 (${missing.length})` : '清除不存在引用';
  elements.mcpSetVisibleDefaultButton.disabled = disabled;
  elements.mcpUnsetVisibleDefaultButton.disabled = disabled;
  elements.mcpEnableVisibleButton.disabled = disabled || !environment;
  elements.mcpDisableVisibleButton.disabled = disabled || !environment;
  elements.mcpBulkHint.textContent = selectedCapabilityCount()
    ? `已选择 MCP ${selectedMCPIDs.size} · Skill ${selectedSkillIDs.size}；可批量分配到多个 Environment。当前筛选结果仍为 ${visibleCount} 项。`
    : (environment
      ? `批量操作只作用于当前筛选结果（${visibleCount} 项）；当前 Environment：${environment.name || environment.environment_id}。`
      : `先选择 MCP / Skill 后可批量分配到多个 Environment；当前筛选结果 ${visibleCount} 项。`);
}
function updateSkillBulkControls() {
  pruneSkillSelection();
  const environment = currentEnvironment(); const selectedCount = selectedSkillIDs.size; const probeCurrent = explicitSkillProbeIsCurrent(); const summary = probeCurrent ? skillAvailabilitySummary() : null; const hasSkills = Boolean(safeArray(currentSnapshot?.skills).length); const visibleCount = visibleResourceIDs(elements.skillList).length;
  elements.skillSelectedCount.textContent = String(selectedCount);
  elements.skillProbeAllButton.disabled = skillBulkBusy || capabilityAssignmentBusy || !hasSkills;
  elements.skillSelectVisibleButton.disabled = skillBulkBusy || capabilityAssignmentBusy || !safeArray(currentSnapshot?.skills).length;
  elements.skillClearSelectionButton.disabled = skillBulkBusy || capabilityAssignmentBusy || selectedCount === 0;
  elements.skillAssignEnvironmentsButton.disabled = skillBulkBusy || capabilityAssignmentBusy || selectedCapabilityCount() === 0 || !safeArray(currentSnapshot?.environments).length;
  const missing = missingSelectionOperations('skill'); elements.skillClearMissingButton.disabled = skillBulkBusy || capabilityAssignmentBusy || missing.length === 0; elements.skillClearMissingButton.textContent = missing.length ? `清除不存在引用 (${missing.length})` : '清除不存在引用';
  elements.skillDeleteSelectedButton.disabled = skillBulkBusy || capabilityAssignmentBusy || selectedCount === 0;
  elements.skillSetVisibleDefaultButton.disabled = skillBulkBusy || visibleCount === 0;
  elements.skillUnsetVisibleDefaultButton.disabled = skillBulkBusy || visibleCount === 0;
  elements.skillEnableVisibleButton.disabled = skillBulkBusy || visibleCount === 0 || !environment;
  elements.skillDisableVisibleButton.disabled = skillBulkBusy || visibleCount === 0 || !environment;
  elements.skillClearUnavailableButton.disabled = skillBulkBusy || !probeCurrent || !summary?.cleanupIDs?.length;
  elements.skillClearUnavailableButton.textContent = probeCurrent && summary?.cleanupIDs?.length ? `一键清除不可用 (${summary.cleanupIDs.length})` : '一键清除不可用';
  if (!probeCurrent) elements.skillBulkHint.textContent = '批量检查会检查全局 Skill catalog 的 source root、artifact 与 support roots；不依赖当前 Environment。';
  else elements.skillBulkHint.textContent = `最近全局检查：可用 ${summary.available} · 不可用 ${summary.unavailable} · 未知 ${summary.unknown}。清理只删除 ADM catalog metadata，不删除磁盘文件。`;
}
async function probeAllSkillAvailability() {
  const identity = currentSkillProbeIdentity();
  if (!safeArray(currentSnapshot?.skills).length) return setStatus('当前 catalog 没有 Skill 可检查。', 'success');
  skillBulkBusy = true; explicitSkillAvailabilityProbe = null; updateSkillBulkControls(); setStatus('正在批量检查全局 Skill catalog 可用性…', 'loading');
  try {
    const result = await desktopAdapter().ListSkillAvailability();
    if (identity.connectionGeneration !== connectionGeneration || identity.catalogFingerprint !== skillCatalogFingerprint()) return false;
    skillAvailabilityByID = new Map(); for (const item of safeArray(result?.skills)) if (item?.skill_id) skillAvailabilityByID.set(item.skill_id, item);
    explicitSkillAvailabilityProbe = {...identity, checkedAt: Date.now()};
    renderSkillManager(safeArray(currentSnapshot?.skills));
    const summary = skillAvailabilitySummary();
    setStatus(`Skill 全局可用性检查完成：可用 ${summary.available} · 不可用 ${summary.unavailable} · 未知 ${summary.unknown}`, summary.unavailable ? 'error' : 'success');
    return true;
  } catch (error) {
    if (identity.connectionGeneration === connectionGeneration) { explicitSkillAvailabilityProbe = null; updateSkillBulkControls(); setStatus(`Skill 全局可用性检查失败：${errorText(error)}`, 'error'); }
    return false;
  } finally { skillBulkBusy = false; renderSkillManager(safeArray(currentSnapshot?.skills)); }
}
async function runVisibleBatch(kind, label, ids, mutate, rerender) {
  const uniqueIDs = [...new Set(safeArray(ids))].filter(Boolean);
  if (!uniqueIDs.length) return setStatus('当前筛选结果为空，没有可批量操作的项目。', 'success');
  if (!window.confirm(label + '：当前筛选结果中的 ' + uniqueIDs.length + ' 项？')) return;
  const requestGeneration = connectionGeneration;
  if (kind === 'mcp') mcpBulkBusy = true;
  else skillBulkBusy = true;
  rerender();
  setStatus(label + '…', 'loading');
  const failures = []; let changed = 0;
  try {
    for (const id of uniqueIDs) {
      if (requestGeneration !== connectionGeneration) { failures.push({id, error: 'connection changed before batch completed'}); break; }
      try { await mutate(id); changed++; }
      catch (error) { failures.push({id, error: errorText(error)}); }
    }
    if (requestGeneration === connectionGeneration) await refreshSnapshot();
    const message = failures.length
      ? label + '完成：已处理 ' + changed + '/' + uniqueIDs.length + '；失败 ' + failures.length + '。' + failures.slice(0, 3).map((item) => item.id + ': ' + item.error).join(' · ')
      : label + '完成：已处理 ' + changed + ' 项。';
    setStatus(message, failures.length ? 'error' : 'success');
  } finally {
    if (kind === 'mcp') { mcpBulkBusy = false; renderMCPManager(safeArray(currentSnapshot?.mcps)); }
    else { skillBulkBusy = false; renderSkillManager(safeArray(currentSnapshot?.skills)); }
  }
}
function renderCapabilityAssignmentDialog() {
  pruneMCPSelection(); pruneSkillSelection(); pruneAssignmentEnvironmentSelection();
  const environments = safeArray(currentSnapshot?.environments), selectedEnvironmentCount = selectedAssignmentEnvironmentIDs.size, capabilityCount = selectedCapabilityCount();
  elements.capabilityAssignmentSummary.textContent = `已选择 MCP ${selectedMCPIDs.size} · Skill ${selectedSkillIDs.size} · Environment ${selectedEnvironmentCount}。这里只修改 Environment 显式选择；Workspace 继承继续生效。`;
  elements.capabilityAssignmentSelectAllButton.disabled = capabilityAssignmentBusy || environments.length === 0;
  elements.capabilityAssignmentClearButton.disabled = capabilityAssignmentBusy || selectedEnvironmentCount === 0;
  elements.capabilityAssignmentEnableButton.disabled = capabilityAssignmentBusy || capabilityCount === 0 || selectedEnvironmentCount === 0;
  elements.capabilityAssignmentDisableButton.disabled = capabilityAssignmentBusy || capabilityCount === 0 || selectedEnvironmentCount === 0;
  if (!environments.length) return emptyMessage(elements.capabilityAssignmentEnvironmentList, '暂无 Environment。');
  elements.capabilityAssignmentEnvironmentList.replaceChildren(); elements.capabilityAssignmentEnvironmentList.classList.remove('empty');
  for (const environment of environments) {
    const summary = window.ADMEnvironmentBulk?.environmentSummary(environment, [...selectedMCPIDs], [...selectedSkillIDs]) || {mcpEnabled: 0, mcpExplicit: 0, mcpInherited: 0, mcpTotal: selectedMCPIDs.size, skillEnabled: 0, skillExplicit: 0, skillInherited: 0, skillTotal: selectedSkillIDs.size};
    const row = document.createElement('label'); row.className = 'selection-row rich-selection bulk-environment-row';
    const checkbox = document.createElement('input'); checkbox.type = 'checkbox'; checkbox.checked = selectedAssignmentEnvironmentIDs.has(environment.environment_id); checkbox.disabled = capabilityAssignmentBusy; checkbox.dataset.action = 'select-assignment-environment'; checkbox.dataset.id = environment.environment_id;
    const copy = document.createElement('span'); copy.className = 'selection-copy';
    const name = document.createElement('strong'); name.textContent = environment.name || environment.environment_id;
    const id = document.createElement('code'); id.textContent = environment.environment_id || '';
    const state = document.createElement('small'); state.textContent = `有效：MCP ${summary.mcpEnabled}/${summary.mcpTotal}（显式 ${summary.mcpExplicit} · Workspace ${summary.mcpInherited}） · Skill ${summary.skillEnabled}/${summary.skillTotal}（显式 ${summary.skillExplicit} · Workspace ${summary.skillInherited}）`;
    copy.append(name, id, state);
    const mixed = (summary.mcpTotal && summary.mcpExplicit > 0 && summary.mcpExplicit < summary.mcpTotal) || (summary.skillTotal && summary.skillExplicit > 0 && summary.skillExplicit < summary.skillTotal);
    const allEnabled = (!summary.mcpTotal || summary.mcpExplicit === summary.mcpTotal) && (!summary.skillTotal || summary.skillExplicit === summary.skillTotal) && (summary.mcpTotal + summary.skillTotal > 0);
    row.append(checkbox, copy, stateBadge(mixed ? '显式混合' : allEnabled ? '全部显式开启' : '存在未显式开启', mixed ? 'degraded' : allEnabled ? 'available' : 'disabled'));
    elements.capabilityAssignmentEnvironmentList.append(row);
  }
}
function openCapabilityAssignmentDialog() {
  pruneMCPSelection(); pruneSkillSelection();
  if (!selectedCapabilityCount()) return setStatus('请先选择至少一个 MCP 或 Skill。', 'error');
  if (!safeArray(currentSnapshot?.environments).length) return setStatus('当前没有可分配的 Environment。', 'error');
  selectedAssignmentEnvironmentIDs = new Set(); capabilityAssignmentBusy = false;
  emptyMessage(elements.capabilityAssignmentResult, '尚未执行批量分配。');
  renderCapabilityAssignmentDialog(); openEditorDialog('capabilityAssignmentDialog');
}
function renderCapabilityAssignmentResult(results) {
  const list = safeArray(results), summary = window.ADMEnvironmentBulk?.summarizeResults(list) || {total: list.length, changed: 0, unchanged: 0, failed: 0, environments: 0};
  elements.capabilityAssignmentResult.replaceChildren(); elements.capabilityAssignmentResult.classList.remove('empty');
  const head = document.createElement('div'); head.className = 'preview-summary'; head.textContent = `Environment ${summary.environments} · 组合 ${summary.total} · 已修改 ${summary.changed} · 已是目标状态 ${summary.unchanged} · 失败 ${summary.failed}`; elements.capabilityAssignmentResult.append(head);
  for (const result of list) {
    const row = document.createElement('div'); row.className = 'preview-row';
    const title = document.createElement('strong'); title.textContent = `${result.environmentName || result.environmentID} · ${String(result.kind || '').toUpperCase()} · ${result.resourceID}`;
    const detail = document.createElement('small'); detail.textContent = result.status === 'failed' ? `失败：${result.error || 'unknown error'}` : result.status === 'unchanged' ? (result.inherited && !result.enabled ? '未修改：Environment 已无显式选择，Workspace 继承仍然有效' : `未修改：Environment 显式选择已经${result.enabled ? '开启' : '关闭'}`) : (result.inherited && !result.enabled ? '已关闭 Environment 显式选择；Workspace 继承仍然有效' : `Environment 显式选择已${result.enabled ? '开启' : '关闭'}`);
    row.append(title, stateBadge(result.status === 'changed' ? '已修改' : result.status === 'unchanged' ? '未变化' : '失败', result.status === 'failed' ? 'error' : result.status === 'changed' ? 'available' : 'configured'), detail); elements.capabilityAssignmentResult.append(row);
  }
}
async function runCapabilityAssignment(enabled) {
  pruneMCPSelection(); pruneSkillSelection(); pruneAssignmentEnvironmentSelection();
  const environments = safeArray(currentSnapshot?.environments), environmentIDs = [...selectedAssignmentEnvironmentIDs];
  const operations = window.ADMEnvironmentBulk?.buildOperations(environments, environmentIDs, [...selectedMCPIDs], [...selectedSkillIDs], enabled) || [];
  if (!operations.length) return setStatus('请选择至少一个资源和一个 Environment。', 'error');
  const actionLabel = enabled ? '开启' : '关闭';
  if (!window.confirm(`将在 ${environmentIDs.length} 个 Environment 中${actionLabel}显式 MCP ${selectedMCPIDs.size} 个、Skill ${selectedSkillIDs.size} 个，共 ${operations.length} 个资源组合。Workspace 继承不会被 Environment 关闭覆盖；已是目标显式状态的组合会跳过。继续？`)) return;
  const requestGeneration = connectionGeneration; capabilityAssignmentBusy = true; renderCapabilityAssignmentDialog(); updateMCPBulkControls(); updateSkillBulkControls(); setStatus(`正在批量${actionLabel} MCP / Skill…`, 'loading');
  const results = [];
  try {
    for (const operation of operations) {
      const base = {environmentID: operation.environmentID, environmentName: operation.environmentName, kind: operation.kind, resourceID: operation.resourceID, enabled: operation.enabled, inherited: Boolean(operation.inherited), effectiveEnabled: Boolean(operation.effectiveEnabled)};
      if (requestGeneration !== connectionGeneration) { results.push({...base, status: 'failed', error: 'ADM connection changed before operation completed'}); continue; }
      if (operation.noop) { results.push({...base, status: 'unchanged'}); continue; }
      try {
        if (operation.kind === 'mcp') await desktopAdapter().SetEnvironmentMCP(operation.environmentID, operation.resourceID, enabled);
        else await desktopAdapter().SetEnvironmentSkill(operation.environmentID, operation.resourceID, enabled);
        results.push({...base, status: 'changed'});
      } catch (error) { results.push({...base, status: 'failed', error: errorText(error)}); }
    }
    if (requestGeneration === connectionGeneration) await refreshSnapshot();
    const summary = window.ADMEnvironmentBulk?.summarizeResults(results) || {changed: 0, unchanged: 0, failed: 0};
    if (requestGeneration === connectionGeneration) {
      renderCapabilityAssignmentDialog(); renderCapabilityAssignmentResult(results);
      setStatus(`批量${actionLabel}完成：修改 ${summary.changed} · 未变化 ${summary.unchanged} · 失败 ${summary.failed}`, summary.failed ? 'error' : 'success');
    }
  } finally {
    capabilityAssignmentBusy = false; updateMCPBulkControls(); updateSkillBulkControls(); if (elements.capabilityAssignmentDialog.open && requestGeneration === connectionGeneration) renderCapabilityAssignmentDialog();
  }
}

async function removeSkillCatalogEntries(ids, label) {
  const entriesByID = new Map(safeArray(currentSnapshot?.skills).map((entry) => [entry.id, entry]));
  const targets = [...new Set(safeArray(ids))].map((id) => entriesByID.get(id)).filter(Boolean);
  if (!targets.length) return;
  const sourceManaged = targets.filter((entry) => entry.source_id).length; const legacy = targets.length - sourceManaged;
  const confirmation = `${label} ${targets.length} 个 Skill？\n\n只删除 ADM catalog metadata，不删除磁盘文件。删除时会同步清理 Workspace 和 Environment 中对这些 Skill ID 的选择。${sourceManaged ? `\n其中 ${sourceManaged} 个是 source-managed，之后刷新 Source 可能重新发现并恢复为新的 catalog 状态。` : ''}\nLegacy: ${legacy} · Source-managed: ${sourceManaged}`;
  if (!window.confirm(confirmation)) return;
  const requestGeneration = connectionGeneration; skillBulkBusy = true; explicitSkillAvailabilityProbe = null; updateSkillBulkControls(); setStatus(`${label}…`, 'loading');
  const failures = []; let removed = 0;
  try {
    for (const entry of targets) {
      if (requestGeneration !== connectionGeneration) { failures.push({id: entry.id, error: 'connection changed before deletion'}); break; }
      try { await desktopAdapter().RemoveSkill(entry.id); removed++; selectedSkillIDs.delete(entry.id); }
      catch (error) { failures.push({id: entry.id, error: errorText(error)}); }
    }
    if (requestGeneration === connectionGeneration) await refreshSnapshot();
    const summary = failures.length ? `${label}完成：已删除 ${removed}/${targets.length}；失败 ${failures.length}。${failures.slice(0, 3).map((item) => `${item.id}: ${item.error}`).join(' · ')}` : `${label}完成：已删除 ${removed} 个 Skill catalog 条目。`;
    setStatus(summary, failures.length ? 'error' : 'success');
  } finally { skillBulkBusy = false; renderSkillManager(safeArray(currentSnapshot?.skills)); }
}
function resetSkillSourceEditor(close = false, rerender = true) {
  editingSkillSourceID = '';
  elements.skillSourceID.value = '';
  elements.skillSourceForm.reset();
  elements.skillSourceDialogTitle.textContent = '添加 Skill source';
  elements.skillSourceSubmitButton.textContent = '添加并刷新 Source';
  if (close) closeEditorDialog('skillSourceDialog');
  if (rerender) renderSkillSources();
}
function beginSkillSourceEdit(source) {
  if (!source) return;
  editingSkillSourceID = source.skill_source_id || '';
  elements.skillSourceID.value = editingSkillSourceID;
  elements.skillSourceRoot.value = source.root || '';
  elements.skillSupportRoots.value = safeArray(source.support_roots).join('\n');
  elements.skillSourceDefault.checked = Boolean(source.default_include_in_environment);
  elements.skillSourceDialogTitle.textContent = '编辑 Skill source';
  elements.skillSourceSubmitButton.textContent = '保存 Source';
  openEditorDialog('skillSourceDialog');
}
function setSkillSubview(next) {
  skillSubview = next === 'sources' ? 'sources' : 'skills';
  syncSkillSubviewUI();
}
function syncSkillSubviewUI() {
  for (const button of elements.skillSubviewTabs?.querySelectorAll('[data-skill-subview]') || []) button.setAttribute('aria-selected', button.dataset.skillSubview === skillSubview ? 'true' : 'false');
  for (const panel of [elements.skillsPanel, elements.skillSourcesPanel]) if (panel) panel.hidden = panel.dataset.skillSubviewPanel !== skillSubview;
}
function skillSourceDisplayName(sourceID) {
  if (!sourceID) return 'Legacy';
  const source = skillSources.find((item) => item.skill_source_id === sourceID);
  if (!source) return sourceID;
  const root = source.root || source.skill_source_id || sourceID;
  return root.split(/[\\/]/).filter(Boolean).pop() || root;
}
function renderSkillSourceFilterOptions(skills) {
  if (!elements.skillSourceFilter) return;
  const current = elements.skillSourceFilter.value;
  const seen = new Set();
  const options = [new Option('全部 Source', ''), new Option('Legacy', '__legacy__')];
  for (const skill of skills) {
    const sourceID = skill.source_id || '';
    if (!sourceID || seen.has(sourceID)) continue;
    seen.add(sourceID);
    options.push(new Option(skillSourceDisplayName(sourceID), sourceID));
  }
  elements.skillSourceFilter.replaceChildren(...options);
  elements.skillSourceFilter.value = current && (current === '__legacy__' || seen.has(current)) ? current : '';
}
function appendSkillFact(parent, label, value) {
  const item = document.createElement('small');
  item.className = 'resource-fact';
  item.textContent = label + '：' + textOrDash(value);
  parent.append(item);
}
function renderSkillSources() {
  const queryText = String(elements.skillSourceFilterInput?.value || '').trim();
  const queryMatches = searchMatcher(queryText);
  if (skillSourcesState === 'loading') {
    setMetric(elements.skillSourceCount, '—'); setMetric(elements.skillSubviewSourceCount, '—'); setMetric(elements.skillSourceListTotalCount, '—'); setMetric(elements.skillSourceVisibleCount, 0);
    return emptyMessage(elements.skillSourceList, '正在读取 Skill sources…');
  }
  if (skillSourcesState === 'error') {
    setMetric(elements.skillSourceCount, '—'); setMetric(elements.skillSubviewSourceCount, '—'); setMetric(elements.skillSourceListTotalCount, '—'); setMetric(elements.skillSourceVisibleCount, 0);
    return emptyMessage(elements.skillSourceList, 'Skill source 列表不可用：' + (skillSourcesError || 'unknown error'));
  }
  if (skillSourcesState !== 'success') {
    setMetric(elements.skillSourceCount, '—'); setMetric(elements.skillSubviewSourceCount, '—'); setMetric(elements.skillSourceListTotalCount, '—'); setMetric(elements.skillSourceVisibleCount, 0);
    return emptyMessage(elements.skillSourceList, 'Skill sources 尚未加载');
  }
  setMetric(elements.skillSourceCount, skillSources.length); setMetric(elements.skillSubviewSourceCount, skillSources.length); setMetric(elements.skillSourceListTotalCount, skillSources.length);
  if (!skillSources.length) {
    setMetric(elements.skillSourceVisibleCount, 0);
    const legacyCount = safeArray(currentSnapshot?.skills).filter((skill) => !skill.source_id).length;
    return emptyMessage(elements.skillSourceList, legacyCount ? '暂无 Skill source。下方 ' + legacyCount + ' 个 Skill 是历史 legacy 条目；添加 source root 后新发现项会标记为 source-managed。' : '暂无 Skill source。添加显式 source root 后由 Core 扫描 SKILL.md。');
  }
  elements.skillSourceList.replaceChildren(); elements.skillSourceList.classList.remove('empty');
  let visible = 0;
  for (const source of skillSources) {
    const discovered = safeArray(currentSnapshot?.skills).filter((skill) => skill.source_id === source.skill_source_id);
    const haystack = [source.skill_source_id, source.root, ...safeArray(source.support_roots)].filter(Boolean).join(' ');
    if (!queryMatches(haystack)) continue;
    visible++;
    const status = source.last_refresh_status || 'pending';
    const row = document.createElement('article'); row.className = 'resource-row'; row.dataset.id = source.skill_source_id || '';
    const main = document.createElement('div'); main.className = 'resource-main';
    const {header, id} = resourceHeader(source.root || source.skill_source_id, source.skill_source_id, [stateBadge(humanRefreshState(status), status), stateBadge(discovered.length + ' skills', 'count')]);
    const facts = document.createElement('div'); facts.className = 'resource-facts';
    appendSkillFact(facts, 'Root', source.root);
    appendSkillFact(facts, 'Support roots', safeArray(source.support_roots).length);
    appendSkillFact(facts, '新环境默认', source.default_include_in_environment ? '是' : '否');
    if (source.last_refresh_at) appendSkillFact(facts, 'Last refresh', new Date(source.last_refresh_at).toLocaleString());
    const note = document.createElement('small'); note.textContent = source.last_refresh_error || '刷新只更新这个 source；失败不会污染其他 Skill。';
    main.append(header, id, facts, note);
    const controls = document.createElement('div'); controls.className = 'resource-actions';
    controls.append(createActionButton('编辑 Source', 'edit-skill-source', source.skill_source_id), createActionButton('刷新 Source', 'refresh-skill-source', source.skill_source_id), createActionButton('删除 Source', 'remove-skill-source', source.skill_source_id, 'danger'));
    row.append(main, controls); elements.skillSourceList.append(row);
  }
  setMetric(elements.skillSourceVisibleCount, visible);
  if (!visible) emptyMessage(elements.skillSourceList, queryText ? '没有符合当前筛选条件的 Skill source。' : '暂无 Skill source。');
}
function renderSkillManager(skills) {
  pruneSkillSelection();
  syncSkillSubviewUI();
  renderSkillSourceFilterOptions(skills);
  const workspace = currentWorkspace(), environment = currentEnvironment(); const selected = new Set(safeArray(environment?.enabled_skill_ids)); const explicitSelected = new Set(Array.isArray(environment?.explicit_skill_ids) ? environment.explicit_skill_ids : safeArray(environment?.enabled_skill_ids)); const workspaceSelected = new Set(safeArray(workspace?.enabled_skill_ids));
  const queryText = String(elements.skillFilter?.value || '').trim(); const queryMatches = searchMatcher(queryText); const filter = elements.skillStateFilter?.value || 'all'; const sourceFilter = elements.skillSourceFilter?.value || '';
  let issues = 0, visible = 0;
  elements.skillBadge.textContent = String(skills.length); setMetric(elements.skillTotalCount, skills.length); setMetric(elements.skillSubviewSkillCount, skills.length); setMetric(elements.skillListTotalCount, skills.length); setMetric(elements.skillEnvironmentCount, environment ? skills.filter((skill) => selected.has(skill.id)).length : 0);
  renderSkillSources();
  if (!skills.length) { setMetric(elements.skillIssueCount, 0); setMetric(elements.skillVisibleCount, 0); emptyMessage(elements.skillList, '暂无已发现 Skill。添加并刷新 Skill source 后会显示在这里。'); updateSkillBulkControls(); return; }
  elements.skillList.replaceChildren(); elements.skillList.classList.remove('empty');
  for (const entry of skills) {
    const enabled = environment ? selected.has(entry.id) : false; const availability = skillAvailabilityByID.get(entry.id); const state = skillAvailabilityState(entry, environment, enabled); const normalized = normalizedState(state); const issue = normalized === 'degraded' || Boolean(window.ADMSkillBulk?.isCleanupState(normalized)) || ['unavailable', 'unconfigured', 'error'].includes(normalized); if (issue) issues++;
    const haystack = [entry.name, entry.id, entry.relative_artifact_path, entry.artifact_path, entry.source_root, entry.source_id].filter(Boolean).join(' ');
    const sourceMatch = !sourceFilter || (sourceFilter === '__legacy__' ? !entry.source_id : entry.source_id === sourceFilter);
    const filterMatch = filter === 'all' || (filter === 'selected' && Boolean(environment && enabled)) || (filter === 'unselected' && Boolean(environment && !enabled)) || (filter === 'issues' && issue) || (filter === 'source' && Boolean(entry.source_id)) || (filter === 'legacy' && !entry.source_id);
    if (!queryMatches(haystack) || !sourceMatch || !filterMatch) continue;
    visible++;
    const fact = managementCapabilityFacts.get('skill/' + entry.id); const row = document.createElement('article'); row.className = 'resource-row'; row.dataset.id = entry.id || ''; const main = document.createElement('div'); main.className = 'resource-main';
    const {header, id} = resourceHeader(entry.name || entry.id, entry.id, [stateBadge('可用性 · ' + humanRuntimeState(state), state), entry.source_id ? stateBadge('Source 管理', 'source') : stateBadge('Legacy', 'legacy')]);
    const facts = document.createElement('div'); facts.className = 'resource-facts';
    appendSkillFact(facts, 'Source', entry.source_id ? skillSourceDisplayName(entry.source_id) + ' · ' + entry.source_id : 'Legacy metadata');
    appendSkillFact(facts, 'Artifact', entry.relative_artifact_path || entry.artifact_path);
    appendSkillFact(facts, 'Support roots', safeArray(entry.support_roots).length);
    appendSkillFact(facts, '新环境默认', entry.default_include_in_environment ? '是' : '否');
    appendSkillFact(facts, '当前 Workspace', workspace ? (workspaceSelected.has(entry.id) ? '已启用' : '未启用') : '未选择');
    appendSkillFact(facts, '当前 Environment', environment ? (workspaceSelected.has(entry.id) ? '已启用（Workspace 继承）' : explicitSelected.has(entry.id) ? '已显式启用' : '未启用') : '未选择');
    appendSkillFact(facts, '全局可用性', humanRuntimeState(state));
    const note = document.createElement('small'); const supportMissing = safeArray(availability?.missing_support_roots);
    const availabilityNote = availability?.reason || fact?.message || (supportMissing.length ? 'Missing support roots: ' + supportMissing.join(', ') : ''); note.textContent = availabilityNote ? '可用性：' + availabilityNote : '';
    main.append(header, id, facts); if (note.textContent) main.append(note);
    const controls = document.createElement('div'); controls.className = 'resource-actions';
    const selectControl = checkControl('选择', selectedSkillIDs.has(entry.id), 'select-skill', entry.id, skillBulkBusy || capabilityAssignmentBusy); selectControl.classList.add('skill-select-control');
    controls.append(selectControl, checkControl('新环境默认', entry.default_include_in_environment, 'default-skill', entry.id)); controls.append(checkControl(workspace ? '当前 Workspace 启用' : '选择 Workspace 后启用', workspaceSelected.has(entry.id), 'workspace-skill', entry.id, !workspace)); controls.append(checkControl(environment ? '当前 Environment 显式启用' : '选择 Environment 后附加启用', explicitSelected.has(entry.id), 'environment-skill', entry.id, !environment));
    if (!entry.source_id) controls.append(createActionButton('删除条目', 'remove-skill', entry.id, 'danger'));
    row.append(main, controls); elements.skillList.append(row);
  }
  setMetric(elements.skillIssueCount, issues); setMetric(elements.skillVisibleCount, visible);
  if (!visible) emptyMessage(elements.skillList, queryText || filter !== 'all' || sourceFilter ? '没有符合当前筛选条件的 Skill。' : '暂无已发现 Skill。');
  updateSkillBulkControls();
  updateMCPBulkControls();
}
function renderOperationalSettings(snapshot) {
  const settings = snapshot?.worktree_settings || {};
  if (elements.worktreeRoot && document.activeElement !== elements.worktreeRoot) elements.worktreeRoot.value = settings.root || '';
  if (elements.worktreeBranchPrefix && document.activeElement !== elements.worktreeBranchPrefix) elements.worktreeBranchPrefix.value = settings.branch_prefix || 'adm/';
  if (elements.worktreeSettingsHint) elements.worktreeSettingsHint.textContent = settings.workspace_id
    ? `System Workspace ${settings.workspace_id} · ${settings.root || '未配置'} · branch ${settings.branch_prefix || 'adm/'}`
    : `当前 root：${settings.root || 'ADM default'} · branch ${settings.branch_prefix || 'adm/'}。保存后 root 会自动登记为受保护 system Workspace。`;
  const host = snapshot?.host_environment || {};
  if (elements.hostEnvironmentSummary) {
    const refreshed = host.last_refresh_at ? new Date(host.last_refresh_at).toLocaleString() : '尚未手动刷新';
    const detail = host.last_refresh_error ? ` · ${host.last_refresh_error}` : '';
    elements.hostEnvironmentSummary.textContent = `Source ${host.source || 'unknown'} · ${host.last_refresh_status || 'unknown'} · variables ${safeNumber(host.variable_count)} · ${refreshed}${detail}`;
  }
}

function renderSnapshotBase(snapshot) {
  currentSnapshot = snapshot; const workspaces = safeArray(snapshot.workspaces), environments = safeArray(snapshot.environments), executables = safeArray(snapshot.allowed_executables), mcps = safeArray(snapshot.mcps), skills = safeArray(snapshot.skills);
  if (editingMCPID && !mcps.some((mcp) => mcp.id === editingMCPID)) resetMCPEditor(false, false);
  renderWorkspaces(workspaces); renderManagementEnvironmentOptions(workspaces, environments); renderEnvironments(environments); renderExecutables(executables, safeArray(snapshot.exec_denials)); renderMCPManager(mcps); renderSkillManager(skills); renderOperationalSettings(snapshot); renderDiagnosticsPage(); updateManagementContextVisibility();
  if (!globalMemoryLoaded) emptyMessage(elements.globalMemoryList, '尚未加载 Global Memory');
  renderDashboardState('success');
  if (selectedEnvironmentID && !environments.some((env) => env.environment_id === selectedEnvironmentID)) closeEnvironmentDetail();
}
function renderManagementUnavailable(message) {
  for (const element of [elements.workspaceBadge, elements.workspaceVisibleCount, elements.workspaceListTotalCount, elements.environmentBadge, elements.environmentVisibleCount, elements.environmentListTotalCount, elements.execBadge, elements.mcpBadge, elements.skillBadge, elements.mcpTotalCount, elements.mcpDefaultCount, elements.mcpEnvironmentCount, elements.mcpIssueCount, elements.mcpVisibleCount, elements.mcpListTotalCount, elements.skillSourceCount, elements.skillTotalCount, elements.skillEnvironmentCount, elements.skillIssueCount, elements.skillVisibleCount, elements.skillListTotalCount]) setMetric(element, '—');
  renderWorkspaceOptions([]); renderEnvironmentWorkspaceFilter([]);
  elements.managementWorkspace.replaceChildren(new Option('管理数据未加载', '')); elements.managementWorkspace.value = ''; elements.managementWorkspace.disabled = true; elements.managementEnvironment.replaceChildren(new Option('管理数据未加载', '')); elements.managementEnvironment.value = ''; elements.managementEnvironment.disabled = true; elements.editEnvironmentButton.disabled = true;
  elements.managementEnvironmentHint.textContent = message;
  emptyMessage(elements.workspaceList, message); emptyMessage(elements.environmentList, message); emptyMessage(elements.execList, message); emptyMessage(elements.execBlockedList, message); elements.execClearAllBlockedButton.disabled = true; setMetric(elements.execAllowedCount, '—'); setMetric(elements.execBlockedCount, '—'); emptyMessage(elements.mcpList, message); emptyMessage(elements.skillSourceList, message); emptyMessage(elements.skillList, message); emptyMessage(elements.globalMemoryList, message); emptyMessage(elements.environmentMemoryList, message); emptyMessage(elements.diagnosticsPageContent, message); elements.diagnosticsPageHint.textContent = message; renderEnvironmentMemoryScope(); updateManagementContextVisibility();
  elements.runtimeHint.textContent = message; resetRuntimeCollections('error', message); emptyMessage(elements.verifierList, message); emptyMessage(elements.processList, message); emptyMessage(elements.runList, message); clearRuntimeOutput(message);
  updateSkillBulkControls();
  updateMCPBulkControls();
}
function clearManagementData(message = 'ADM 未连接。连接 Admin MCP 后加载管理数据。') {
  connectionGeneration++; environmentGeneration++; detailGeneration++; workspaceDiscoveryGeneration++; environmentTreeDigestGeneration++;
  workspaceDiscoveryWorkspaceID = ''; workspaceDiscoveryLoading = false; workspaceDiscoveryReport = null; workspaceDiscoveryError = ''; workspaceDiscoveryFingerprint = '';
  if (elements.workspaceDiscoveryDialog.open) closeEditorDialog('workspaceDiscoveryDialog');
  if (elements.capabilityAssignmentDialog.open) closeEditorDialog('capabilityAssignmentDialog');
  resetMCPEditor(true, false);
  pendingMCPImport = null; elements.mcpImportApplyButton.disabled = true;
  elements.mcpImportContent.value = ''; resetMCPImportPreview();
  managementWorkspaceID = ''; managementEnvironmentID = ''; currentSnapshot = null; lastSnapshotSuccessAt = 0;
  elements.workspaceFilter.value = ''; elements.environmentFilter.value = ''; elements.environmentWorkspaceFilter.value = '';
  skillSources = []; skillSourcesState = 'unloaded'; skillSourcesError = ''; managementContextError = ''; managementSkillAvailabilityError = '';
  selectedMCPIDs = new Set(); selectedSkillIDs = new Set(); selectedAssignmentEnvironmentIDs = new Set(); capabilityAssignmentBusy = false; explicitSkillAvailabilityProbe = null; skillBulkBusy = false; mcpBulkBusy = false; editingSkillSourceID = ''; skillSubview = 'skills'; syncSkillSubviewUI();
  managementInspection = null; managementCapabilityFacts = new Map(); skillAvailabilityByID = new Map(); environmentSkillAvailabilityByID = new Map(); mcpHealthByKey = new Map(); temporaryLifecycleByEnvironmentID = new Map(); temporaryCleanupPreviewIDs = new Set(); mcpProbeRequests.clear();
  runtimeSubview = 'verifiers'; runtimeSubviewGeneration++; runtimePendingActionKey = ''; clearRuntimeWriterCredential(); resetRuntimeCollections('unloaded'); clearRuntimeOutput('管理上下文已清除'); syncRuntimeSubviewUI();
  renderDashboardState('unloaded', message); renderManagementUnavailable(message);
  globalMemoryLoaded = false; globalMemoryLoading = false; resetEnvironmentMemoryScope(message); if (!elements.environmentDetailPanel.hidden) closeEnvironmentDetail();
}
async function refreshManagementContext(scope = captureEnvironmentScope()) {
  if (!environmentScopeIsCurrent(scope)) return {stale: true, errors: []};
  managementInspection = null; managementCapabilityFacts = new Map(); environmentSkillAvailabilityByID = new Map(); managementContextError = ''; managementSkillAvailabilityError = ''; updateManagementHint();
  if (!scope.environmentID) { renderMCPManager(safeArray(currentSnapshot?.mcps)); renderSkillManager(safeArray(currentSnapshot?.skills)); resetEnvironmentMemoryScope(); resetRuntimeCollections('unloaded'); renderRuntime(); renderDiagnosticsPage(); return {stale: false, errors: [], inspection: null}; }
  const [inspectionResult, availabilityResult] = await Promise.allSettled([
    desktopAdapter().InspectEnvironment(scope.environmentID),
    desktopAdapter().ListEnvironmentSkills(scope.environmentID),
  ]);
  if (!environmentScopeIsCurrent(scope)) return {stale: true, errors: []};
  const errors = [];
  if (inspectionResult.status === 'fulfilled') { managementInspection = inspectionResult.value; managementCapabilityFacts = capabilityMap(inspectionResult.value?.capability_report); }
  else errors.push(`Environment inspection: ${errorText(inspectionResult.reason)}`);
  if (availabilityResult.status === 'fulfilled') { for (const item of safeArray(availabilityResult.value?.skills)) if (item?.skill_id) environmentSkillAvailabilityByID.set(item.skill_id, item); }
  else { managementSkillAvailabilityError = errorText(availabilityResult.reason); errors.push(`Skill availability: ${managementSkillAvailabilityError}`); }
  managementContextError = errors.join(' · '); updateManagementHint();
  renderMCPManager(safeArray(currentSnapshot?.mcps)); renderSkillManager(safeArray(currentSnapshot?.skills)); renderDiagnosticsPage();
  const runtimeResult = await refreshRuntimeContext(false, scope);
  return {stale: Boolean(runtimeResult?.stale), errors: [...errors, ...safeArray(runtimeResult?.errors)], inspection: managementInspection};
}

function currentMCPImportInput() {
  return {
    format: elements.mcpImportFormat.value,
    json_or_jsonc: elements.mcpImportContent.value,
    conflict_policy: elements.mcpImportConflict.value,
    default_include: elements.mcpImportDefault.checked,
  };
}
function mcpImportFingerprint(input) {
  return JSON.stringify({
    format: String(input?.format || ''),
    json_or_jsonc: String(input?.json_or_jsonc || ''),
    conflict_policy: String(input?.conflict_policy || ''),
    default_include: Boolean(input?.default_include),
  });
}
function resetMCPImportPreview(message = '尚未预览。导入不会修改 Environment 选择。') {
  pendingMCPImport = null;
  mcpImportBusy = false;
  elements.mcpImportApplyButton.disabled = true;
  emptyMessage(elements.mcpImportPreview, message);
}
function invalidateMCPImportPreview(message = '导入内容或选项已变化，请重新预览。') {
  if (!pendingMCPImport) return;
  resetMCPImportPreview(message);
}
function currentPendingMCPImport() {
  const pending = pendingMCPImport;
  if (!pending) return null;
  if (pending.connectionGeneration !== connectionGeneration || pending.fingerprint !== mcpImportFingerprint(currentMCPImportInput())) {
    resetMCPImportPreview('导入预览已过期，请重新预览。');
    return null;
  }
  return pending;
}
function renderImportPreview(preview) {
  const candidates = safeArray(preview?.candidates); elements.mcpImportPreview.replaceChildren(); elements.mcpImportPreview.classList.remove('empty');
  if (!candidates.length) { resetMCPImportPreview('没有可导入候选项。'); return; }
  let hasErrors = false;
  const head = document.createElement('div'); head.className = 'preview-summary'; head.textContent = `识别格式：${preview.format || 'unknown'} · ${candidates.length} 个候选项。预览不会修改 catalog 或 Environment；源配置中的 literal env/header 值不会被保存。`; elements.mcpImportPreview.append(head);
  for (const candidate of candidates) {
    const errors = safeArray(candidate.errors), warnings = safeArray(candidate.warnings), refs = safeArray(candidate.reference_requirements); if (errors.length) hasErrors = true;
    const row = document.createElement('div'); row.className = 'preview-row'; const title = document.createElement('strong'); title.textContent = candidate.name || 'unnamed';
    const badges = document.createElement('div'); badges.className = 'inline-badges'; badges.append(stateBadge(candidate.transport || 'unknown', 'transport'), stateBadge(errors.length ? 'error' : warnings.length ? 'warning' : 'ready', errors.length ? 'unavailable' : warnings.length ? 'degraded' : 'available'));
    const detail = document.createElement('small'); detail.textContent = `${candidate.endpoint_configured ? 'Endpoint configured' : candidate.executable ? `Executable ${candidate.executable}` : 'No endpoint/executable'}${refs.length ? ` · ${refs.length} 个环境变量引用待配置` : ''}`;
    row.append(title, badges, detail);
    for (const ref of refs) { const refLine = document.createElement('small'); refLine.className = 'reference-text'; refLine.textContent = `${ref.field_path || 'reference'} → ${ref.reference_name}`; row.append(refLine); }
    for (const issue of [...errors, ...warnings]) { const issueLine = document.createElement('small'); issueLine.className = errors.includes(issue) ? 'error-text' : 'warning-text'; issueLine.textContent = `${issue.field_path ? `${issue.field_path}: ` : ''}${issue.message || issue.kind}`; row.append(issueLine); }
    elements.mcpImportPreview.append(row);
  }
  elements.mcpImportApplyButton.disabled = hasErrors; if (hasErrors) pendingMCPImport = null;
}
function renderImportApplyResult(result) {
  const mutations = safeArray(result?.result?.mutations); elements.mcpImportPreview.replaceChildren(); elements.mcpImportPreview.classList.remove('empty'); const summary = document.createElement('div'); summary.className = 'preview-summary'; summary.textContent = `已应用 ${mutations.length} 个 MCP 定义；现有 Environment 选择未被自动修改。`; elements.mcpImportPreview.append(summary);
  for (const mutation of mutations) { const row = document.createElement('div'); row.className = 'preview-row'; const name = document.createElement('strong'); name.textContent = mutation.definition?.name || mutation.definition?.id || 'MCP'; const detail = document.createElement('small'); detail.textContent = `${mutation.action || 'created'} · ${mutation.definition?.transport || ''}`; row.append(name, detail); elements.mcpImportPreview.append(row); }
}

function detailRow(label, value) { const row = document.createElement('div'); row.className = 'detail-row'; const term = document.createElement('strong'); term.textContent = label; const content = document.createElement('span'); content.textContent = value || '—'; row.append(term, content); return row; }
function detailGroup(title, rows, note = '') {
  const section = document.createElement('section'); section.className = 'detail-group'; const heading = document.createElement('div'); heading.className = 'detail-group-heading'; const label = document.createElement('strong'); label.textContent = title; heading.append(label);
  if (note) { const description = document.createElement('span'); description.textContent = note; heading.append(description); }
  const grid = document.createElement('div'); grid.className = 'detail-group-grid'; grid.append(...rows); section.append(heading, grid); return section;
}
function renderSelectionList(container, entries, selectedIDs, kind, facts, availabilityMap, availabilityError = '') {
  if (!entries.length) return emptyMessage(container, `暂无 ${kind.toUpperCase()} 项`); container.replaceChildren(); container.classList.remove('empty'); const selected = new Set(safeArray(selectedIDs));
  for (const entry of entries) {
    const row = document.createElement('label'); row.className = 'selection-row rich-selection'; const configured = kind === 'mcp' ? mcpConfigured(entry) : Boolean(entry.artifact_path && entry.source_root); const enabled = selected.has(entry.id);
    const checkbox = document.createElement('input'); checkbox.type = 'checkbox'; checkbox.checked = enabled; checkbox.disabled = !configured; checkbox.dataset.kind = kind; checkbox.dataset.id = entry.id;
    const text = document.createElement('span'); text.className = 'selection-copy'; const name = document.createElement('strong'); name.textContent = entry.name || entry.id; const id = document.createElement('code'); id.textContent = entry.id || '';
    let state = enabled ? 'configured' : 'disabled', reason = '';
    if (kind === 'mcp') {
      const fact = facts.get(`mcp/${entry.id}`); state = !configured ? 'unconfigured' : !enabled ? 'disabled' : fact?.state || 'configured'; reason = fact?.message || fact?.reason_code || '';
    } else {
      const availability = availabilityMap.get(entry.id);
      if (availability?.state) { state = availability.state; reason = availability.reason || ''; }
      else if (!configured) state = 'unconfigured';
      else if (!enabled) state = 'disabled';
      else if (availabilityError) { state = 'unknown'; reason = `可用性读取失败：${availabilityError}`; }
      else state = facts.get(`skill/${entry.id}`)?.state || 'configured';
      if (!reason) reason = facts.get(`skill/${entry.id}`)?.message || '';
    }
    const meta = document.createElement('small'); meta.textContent = reason || (configured ? '配置可用' : '缺少配置'); text.append(name, id, meta); row.append(checkbox, text, stateBadge(state, state)); container.append(row);
  }
}
function syncEnvironmentDetailSubviewUI() {
  environmentDetailSubview = environmentDetailSubview === 'diagnostics' ? 'diagnostics' : 'summary';
  for (const button of elements.environmentDetailSubviewTabs.querySelectorAll('[data-environment-detail-subview]')) {
    const active = button.dataset.environmentDetailSubview === environmentDetailSubview;
    button.setAttribute('aria-selected', String(active));
    button.tabIndex = active ? 0 : -1;
  }
  elements.environmentDetail.hidden = environmentDetailSubview !== 'summary';
  elements.environmentTreeDigestSection.hidden = environmentDetailSubview !== 'summary';
  elements.environmentDiagnostics.hidden = environmentDetailSubview !== 'diagnostics';
}
function diagnosticFactText(fact) {
  const parts = [humanRuntimeState(fact.state || 'unknown')];
  if (fact.reason_code) parts.push('reason ' + fact.reason_code);
  if (fact.message) parts.push(fact.message);
  if (fact.source) parts.push('source ' + fact.source);
  if (fact.generated_at) parts.push('generated ' + new Date(fact.generated_at).toLocaleString());
  if (fact.observed_at) parts.push('observed ' + new Date(fact.observed_at).toLocaleString());
  return parts.join(' · ');
}
function diagnosticGroupLabel(kind) {
  const labels = {environment: 'Environment 基础', files: '文件访问', exec: '命令执行', verifier: 'Verifier', git: 'Git', isolation: 'Worktree / 隔离', mcp: 'MCP', skill: 'Skill', process: 'Process', run: 'Run'};
  return labels[String(kind || '').toLowerCase()] || (kind || '其他能力');
}
function diagnosticActionHint(fact) {
  const reason = String(fact?.reason_code || fact?.reason || '').toLowerCase();
  if (reason.includes('writer')) return '处理建议：等待当前 writer 到期，或由持有 owner 的会话续租/释放；管理员可使用 CLI 强制释放。';
  if (reason.includes('executable') || reason.includes('allow')) return '处理建议：检查执行许可页的 Strict/Full 模式和 executable allowlist。';
  if (reason.includes('mcp') || reason.includes('connection') || reason.includes('auth')) return '处理建议：检查 MCP Endpoint、鉴权引用、网络和全局 MCP 探测结果。';
  if (reason.includes('skill') || reason.includes('artifact') || reason.includes('source')) return '处理建议：刷新 Skill Source，并检查 artifact/support root 是否存在。';
  if (reason.includes('git') || reason.includes('worktree')) return '处理建议：检查 Git/worktree 状态；dirty 或 unpublished 工作不会被自动清理。';
  if (reason.includes('root') || reason.includes('path')) return '处理建议：检查 Environment root 与 Workspace containment。';
  return '';
}
function environmentDiagnosticGroups(inspection) {
  const environment = inspection?.environment || {}, report = inspection?.capability_report || {};
  const facts = safeArray(report.facts);
  const unresolvedMCP = safeArray(inspection?.unresolved_mcp_ids), unresolvedSkill = safeArray(inspection?.unresolved_skill_ids);
  const stateCounts = {available: 0, degraded: 0, unavailable: 0, unconfigured: 0, disabled: 0, unknown: 0};
  for (const fact of facts) {
    const state = normalizedState(fact.state);
    if (Object.prototype.hasOwnProperty.call(stateCounts, state)) stateCounts[state]++;
    else stateCounts.unknown++;
  }
  const sourceRows = [
    detailRow('Environment', (environment.name || environment.environment_id || '—') + ' · ' + (environment.root || '')),
    detailRow('诊断时间', report.generated_at ? new Date(report.generated_at).toLocaleString() : '未返回'),
    detailRow('正常', String(stateCounts.available)),
    detailRow('需关注', String(stateCounts.degraded + stateCounts.unavailable + stateCounts.unconfigured)),
    detailRow('Disabled / Unknown', String(stateCounts.disabled + stateCounts.unknown)),
  ];
  const unresolvedRows = [
    detailRow('MCP IDs', unresolvedMCP.join(', ') || 'None'),
    detailRow('Skill IDs', unresolvedSkill.join(', ') || 'None'),
  ];
  const grouped = new Map();
  for (const fact of facts) {
    const group = fact.kind || String(fact.key || 'capability').split('/')[0] || 'capability';
    if (!grouped.has(group)) grouped.set(group, []);
    grouped.get(group).push(fact);
  }
  const groups = [
    detailGroup('诊断摘要', sourceRows, '这里解释当前已返回的 capability facts；不会自动执行命令、MCP 探测、Verifier 或读取 Memory。'),
    detailGroup('未解析引用', unresolvedRows, '正常 catalog 删除会同步清理 Workspace / Environment 选择；这里显示的是历史或外部状态留下的 unresolved ID，可用“清除不存在引用”修复。'),
  ];
  if (!facts.length) {
    groups.push(detailGroup('Capability facts', [detailRow('Returned facts', 'None')], 'Unknown means no fact was returned; it is not an all-green live health claim.'));
  } else {
    for (const [group, items] of grouped) {
      const container = document.createElement('section'); container.className = 'detail-group';
      const heading = document.createElement('h3'); heading.textContent = diagnosticGroupLabel(group);
      const list = document.createElement('div'); list.className = 'diagnostic-fact-list';
      for (const fact of items) {
        const row = document.createElement('article'); row.className = 'diagnostic-fact';
        const title = document.createElement('strong'); title.textContent = fact.key || fact.kind || 'capability';
        const badge = stateBadge(humanRuntimeState(fact.state || 'unknown'), fact.state || 'unknown');
        const detail = document.createElement('small'); const actionHint = diagnosticActionHint(fact); detail.textContent = diagnosticFactText(fact) + (actionHint ? ' · ' + actionHint : '');
        row.append(title, badge, detail); list.append(row);
      }
      const note = document.createElement('small'); note.textContent = '状态来自当前诊断事实；这里只做解释和分组，不会在后台自动执行修复。';
      container.append(heading, list, note); groups.push(container);
    }
  }
  return groups;
}
function renderEnvironmentDiagnostics(inspection, container = elements.environmentDiagnostics) {
  container.replaceChildren(...environmentDiagnosticGroups(inspection));
}
function renderDiagnosticsPage() {
  const environment = currentEnvironment();
  elements.diagnosticsRefreshButton.disabled = !Boolean(environment && currentSnapshot);
  if (!currentSnapshot) {
    elements.diagnosticsPageHint.textContent = 'ADM 管理数据尚未加载。';
    return emptyMessage(elements.diagnosticsPageContent, '连接 Admin MCP 后查看 Environment 诊断。');
  }
  if (!environment) {
    elements.diagnosticsPageHint.textContent = '请选择 Management Environment 后查看诊断。';
    return emptyMessage(elements.diagnosticsPageContent, '未选择 Environment；诊断页不会自动 fan-out 到所有 Environment。');
  }
  elements.diagnosticsPageHint.textContent = '当前 Environment：' + (environment.name || environment.environment_id) + ' · 先看“诊断摘要”，再查看需关注的能力分组。';
  if (!managementInspection || managementInspection?.environment?.environment_id !== environment.environment_id) {
    return emptyMessage(elements.diagnosticsPageContent, '尚未加载当前 Environment 的诊断事实；点击“刷新诊断”或重新选择 Environment。');
  }
  elements.diagnosticsPageContent.replaceChildren(...environmentDiagnosticGroups(managementInspection));
  elements.diagnosticsPageContent.classList.remove('empty');
}
function renderEnvironmentDetailFromInspection(inspection, token = detailGeneration) {
  const environment = inspection?.environment || {}, workspace = inspection?.workspace || {}, report = inspection?.capability_report || {}, facts = capabilityMap(report);
  if (token !== detailGeneration || environment.environment_id !== selectedEnvironmentID) return false;
  elements.environmentDetailTitle.textContent = environment.name || environment.environment_id || 'Environment';
  const issueFacts = safeArray(report.facts).filter((fact) => ['unavailable', 'degraded', 'unconfigured'].includes(normalizedState(fact.state)));
  const capabilityRows = issueFacts.length
    ? issueFacts.slice(0, 8).map((fact) => detailRow(fact.key || fact.kind || 'capability', humanRuntimeState(fact.state) + (fact.reason_code ? ' · ' + fact.reason_code : '') + (fact.message ? ' · ' + fact.message : '')))
    : [detailRow('Status', '没有 unavailable / degraded / unconfigured capability fact')];
  if (managementSkillAvailabilityError) capabilityRows.unshift(detailRow('Skill availability', '读取失败 · ' + managementSkillAvailabilityError));
  if (report.generated_at) capabilityRows.push(detailRow('Generated at', new Date(report.generated_at).toLocaleString()));
  const identityRows = [
    detailRow('Environment ID', environment.environment_id),
    detailRow('Root', environment.root),
    detailRow('Workspace', (workspace.name || workspace.workspace_id || '—') + (workspace.workspace_id ? ' (' + workspace.workspace_id + ')' : '') + ' · ' + (workspace.path || '')),
    detailRow('State', environment.state),
  ];
  const authorityRows = [
    detailRow('Writer lease', environment.writer ? ('Active · expires ' + (environment.writer.expires_at ? new Date(environment.writer.expires_at).toLocaleString() : 'unknown')) : 'No active writer'),
    detailRow('Private Memory count', safeNumber(environment.private_memory_count) + ' entries'),
  ];
  const unresolvedRows = [
    detailRow('MCP IDs', safeArray(inspection?.unresolved_mcp_ids).join(', ') || 'None'),
    detailRow('Skill IDs', safeArray(inspection?.unresolved_skill_ids).join(', ') || 'None'),
  ];
  elements.environmentDetail.replaceChildren(
    detailGroup('Identity', identityRows, 'Stable identity/root/Workspace facts from the current inspection.'),
    detailGroup('Lifecycle retention', temporaryLifecycleRows(environment), 'Lifecycle owner/session/run are retention provenance only, not task hierarchy or status.'),
    detailGroup('Runtime authority', authorityRows, 'Writer is an observation only; this page never acquires or force-releases a lease.'),
    detailGroup('Capability issues', capabilityRows, issueFacts.length + ' issue fact(s); optional failures stay local.'),
    detailGroup('未解析引用', unresolvedRows, '正常 catalog 删除会同步清理选择；若这里仍有 unresolved ID，说明是历史或外部状态残留。')
  );
  renderEnvironmentDiagnostics(inspection);
  syncEnvironmentDetailSubviewUI();
  renderSelectionList(elements.environmentMCPSelections, safeArray(currentSnapshot?.mcps), environment.enabled_mcp_ids, 'mcp', facts, skillAvailabilityByID);
  renderSelectionList(elements.environmentSkillSelections, safeArray(currentSnapshot?.skills), environment.enabled_skill_ids, 'skill', facts, environmentSkillAvailabilityByID, managementSkillAvailabilityError);
  elements.environmentDetailBackdrop.hidden = false; elements.environmentDetailPanel.hidden = false; return true;
}
function closeEnvironmentDetail() {
  const wasOpen = !elements.environmentDetailPanel.hidden; const opener = environmentDetailOpener;
  detailGeneration++; selectedEnvironmentID = ''; environmentDetailOpener = null; environmentDetailSubview = 'summary'; resetEnvironmentTreeDigest('尚未读取目录摘要。'); syncEnvironmentDetailSubviewUI(); elements.environmentDetailBackdrop.hidden = true; elements.environmentDetailPanel.hidden = true; elements.environmentDetail.replaceChildren(); elements.environmentDiagnostics.replaceChildren();
  if (!wasOpen) return;
  updateEnvironmentContextMarkers();
  if (opener?.isConnected && !opener.closest('[hidden]')) opener.focus({preventScroll: true}); else document.querySelector('[data-management-page]:not([hidden]) [data-page-heading]')?.focus({preventScroll: true});
}
function refreshSelectedEnvironmentDetail() { if (!selectedEnvironmentID) return false; const token = detailGeneration, id = selectedEnvironmentID, inspection = managementInspection; if (token !== detailGeneration || id !== selectedEnvironmentID || inspection?.environment?.environment_id !== id) return false; return renderEnvironmentDetailFromInspection(inspection, token); }

function renderMemory(container, entries, scope) {
  if (!entries.length) return emptyMessage(container, '没有 Memory 条目'); container.replaceChildren(); container.classList.remove('empty');
  for (const entry of entries) { const row = document.createElement('div'); row.className = 'memory-row'; const content = document.createElement('div'); content.className = 'memory-value'; const key = document.createElement('code'); key.textContent = entry.key || ''; const value = document.createElement('pre'); value.textContent = entry.value || ''; content.append(key, value); const edit = createActionButton('编辑', 'edit-' + scope + '-memory', entry.key); edit.addEventListener('click', () => { const isGlobal = scope === 'global'; (isGlobal ? elements.globalMemoryKey : elements.environmentMemoryKey).value = entry.key || ''; (isGlobal ? elements.globalMemoryValue : elements.environmentMemoryValue).value = entry.value || ''; openEditorDialog(isGlobal ? 'globalMemoryDialog' : 'environmentMemoryDialog'); }); row.append(content, edit, createActionButton('删除', 'delete-' + scope + '-memory', entry.key, 'danger')); container.append(row); }
}
function currentManagementRoute() { return managementNavigation?.current?.() || window.ADMNavigation?.normalizeRoute?.(window.location.hash) || 'overview'; }
function currentGlobalMemoryScope() { return {connectionGeneration}; }
function globalMemoryScopeIsCurrent(scope) { return Boolean(scope && scope.connectionGeneration === connectionGeneration); }
function currentEnvironmentMemoryScope() { return {connectionGeneration, environmentGeneration, environmentID: managementEnvironmentID}; }
function environmentMemoryScopeIsCurrent(scope) { return Boolean(scope && scope.connectionGeneration === connectionGeneration && scope.environmentGeneration === environmentGeneration && scope.environmentID === managementEnvironmentID); }
function resetEnvironmentMemoryScope(message = '') {
  environmentMemoryLoaded = false;
  environmentMemoryLoading = false;
  loadedEnvironmentMemoryID = '';
  emptyMessage(elements.environmentMemoryList, message || (managementEnvironmentID ? '尚未加载当前 Environment Memory' : '请选择 Management Environment 后加载 private Memory'));
  renderEnvironmentMemoryScope();
}
function renderEnvironmentMemoryScope() {
  const environment = currentEnvironment();
  const enabled = Boolean(environment && currentSnapshot && !environmentMemoryLoading);
  elements.loadEnvironmentMemory.disabled = !enabled;
  elements.writeEnvironmentMemoryButton.disabled = !Boolean(environment && currentSnapshot);
  elements.environmentMemoryScopeHint.textContent = environment
    ? '当前 Environment：' + (environment.name || environment.environment_id) + ' · ' + environment.environment_id + '。private Memory 必须显式加载。'
    : '请选择 Management Environment 后显式加载。';
  if (!environment) emptyMessage(elements.environmentMemoryList, '请选择 Management Environment 后加载 private Memory');
}
function maybeAutoLoadGlobalMemory() {
  if (currentManagementRoute() !== 'memory' || globalMemoryLoaded || globalMemoryLoading || !currentSnapshot) return false;
  loadGlobalMemory({auto: true});
  return true;
}
async function loadGlobalMemory(options = {}) {
  if (globalMemoryLoading) return false;
  const scope = options.scope || currentGlobalMemoryScope();
  globalMemoryLoading = true;
  setStatus(options.auto ? '正在加载 Global Memory…' : '正在显式读取 Global Memory…', 'loading');
  try {
    const entries = safeArray(await desktopAdapter().ListGlobalMemory());
    if (!globalMemoryScopeIsCurrent(scope)) return false;
    renderMemory(elements.globalMemoryList, entries, 'global');
    globalMemoryLoaded = true;
    setStatus('Global Memory 已加载', 'success');
    return true;
  } catch (error) {
    if (globalMemoryScopeIsCurrent(scope)) setStatus('Global Memory 读取失败：' + (error?.message || String(error)), 'error');
    return false;
  } finally {
    if (globalMemoryScopeIsCurrent(scope)) globalMemoryLoading = false;
  }
}
async function loadEnvironmentMemory(options = {}) {
  if (environmentMemoryLoading) return false;
  const scope = options.scope || currentEnvironmentMemoryScope();
  if (!scope.environmentID) { renderEnvironmentMemoryScope(); return false; }
  environmentMemoryLoading = true;
  renderEnvironmentMemoryScope();
  setStatus('正在显式读取 Environment-private Memory…', 'loading');
  try {
    const entries = safeArray(await desktopAdapter().ListEnvironmentMemory(scope.environmentID));
    if (!environmentMemoryScopeIsCurrent(scope)) return false;
    renderMemory(elements.environmentMemoryList, entries, 'environment');
    environmentMemoryLoaded = true;
    loadedEnvironmentMemoryID = scope.environmentID;
    setStatus('Environment-private Memory 已加载', 'success');
    return true;
  } catch (error) {
    if (environmentMemoryScopeIsCurrent(scope)) setStatus('Environment-private Memory 读取失败：' + (error?.message || String(error)), 'error');
    return false;
  } finally {
    if (environmentMemoryScopeIsCurrent(scope)) { environmentMemoryLoading = false; renderEnvironmentMemoryScope(); }
  }
}

async function refreshSnapshot(successMessage = '') {
  const requestGeneration = connectionGeneration;
  elements.refreshButton.disabled = true; renderDashboardState('loading');
  setStatus('正在通过 Admin MCP 读取 ADM 状态…', 'loading');
  try {
    let snapshot;
    try { snapshot = await desktopAdapter().GetSnapshot(); }
    catch (error) {
      if (requestGeneration !== connectionGeneration) return false;
      const message = `Admin MCP 管理快照读取失败：${errorText(error)}`;
      renderDashboardState(currentSnapshot ? 'stale' : 'error', message);
      resetEnvironmentMemoryScope(message);
      if (!currentSnapshot) renderManagementUnavailable(message);
      setStatus(message, 'error'); return false;
    }
    if (requestGeneration !== connectionGeneration) return false;
    lastSnapshotSuccessAt = Date.now(); explicitSkillAvailabilityProbe = null; skillSourcesState = 'loading'; skillSourcesError = ''; renderSnapshotBase(snapshot); renderEnvironmentMemoryScope();

    let sourceError = '';
    try { skillSources = safeArray(await desktopAdapter().ListSkillSources()); skillSourcesState = 'success'; }
    catch (error) { skillSources = []; skillSourcesState = 'error'; skillSourcesError = errorText(error); sourceError = `Skill sources: ${skillSourcesError}`; }
    if (requestGeneration !== connectionGeneration) return false;
    renderSkillManager(safeArray(currentSnapshot?.skills));
    const [loggingError, gatewayAccessError, execAuthorizationError] = await Promise.all([refreshLoggingStatus(), refreshGatewayAccessStatus(), refreshExecAuthorizationStatus()]);
    if (requestGeneration !== connectionGeneration) return false;

    const contextResult = await refreshManagementContext(captureEnvironmentScope());
    if (requestGeneration !== connectionGeneration) return false;
    let detailError = '';
    if (selectedEnvironmentID) {
      try { await refreshSelectedEnvironmentDetail(); }
      catch (error) { detailError = `Environment detail: ${errorText(error)}`; }
    }
    const auxiliaryErrors = [sourceError, loggingError, gatewayAccessError, execAuthorizationError, ...safeArray(contextResult?.errors), detailError].filter(Boolean);
    if (auxiliaryErrors.length) setStatus(`基础快照已刷新；部分状态不可用：${auxiliaryErrors.join(' · ')}`, 'error');
    else setStatus(successMessage || `已刷新 · ${new Date().toLocaleTimeString()}`, 'success');
    return true;
  } finally { if (requestGeneration === connectionGeneration) elements.refreshButton.disabled = false; }
}
async function refreshConnectedADM(showConnectionMessage = false) {
  const status = await refreshGatewayStatus(showConnectionMessage);
  if (status?.state === 'running') await refreshSnapshot();
  else { clearManagementData(); if (!showConnectionMessage) setStatus('ADM 未连接；管理数据未加载', 'error'); }
  return status;
}
async function runMutation(label, action, after) { setStatus(`${label}…`, 'loading'); try { await action(); await refreshSnapshot(`${label}完成`); if (after) await after(); } catch (error) { setStatus(`${label}失败：${error?.message || String(error)}`, 'error'); } }

async function cleanupExpiredTemporaryEnvironments() {
  if (!currentSnapshot) return setStatus('请先连接 ADM 并加载管理快照。', 'error');
  const button = elements.cleanupExpiredTemporaryEnvironmentsButton;
  button.disabled = true;
  setStatus('正在检查已过期 temporary Environment…', 'loading');
  try {
    const preview = await desktopAdapter().CleanupExpiredTemporaryEnvironments(false);
    const eligible = safeArray(preview?.report?.resources).filter((item) => item.kind === 'environment' && item.cleanup_eligible);
    const blocked = safeArray(preview?.report?.resources).filter((item) => item.kind === 'environment' && !item.cleanup_eligible);
    if (!eligible.length) {
      setStatus(blocked.length ? `没有可安全清理的过期 Environment；${blocked.length} 个存在 blocker。` : '没有已过期的 temporary Environment。', blocked.length ? 'error' : 'success');
      return;
    }
    const names = eligible.slice(0, 8).map((item) => item.name || item.id).join('\n');
    const suffix = eligible.length > 8 ? `\n…以及另外 ${eligible.length - 8} 个` : '';
    if (!window.confirm(`将清理 ${eligible.length} 个已过期且通过安全检查的 temporary Environment。\n\n${names}${suffix}\n\n有 blocker 的 Environment 不会被强制删除。继续？`)) {
      setStatus('已取消清理；未修改 Environment。', 'success');
      return;
    }
    const result = await desktopAdapter().CleanupExpiredTemporaryEnvironments(true);
    const removed = safeArray(result?.removed).filter((item) => item.kind === 'environment').length;
    await refreshSnapshot();
    setStatus(`过期 temporary Environment 清理完成：移除 ${removed} 个；被阻止项保持不变。`, 'success');
  } catch (error) {
    setStatus('清理过期 temporary Environment 失败：' + errorText(error), 'error');
  } finally {
    button.disabled = false;
  }
}
function temporaryLifecycleOwner(environment) { return String(environment?.retention?.owner_id || temporaryLifecycleStatus(environment)?.retention?.owner_id || '').trim(); }
function temporaryCleanupItem(result, environmentID) { return safeArray(result?.report?.resources).find((item) => item.kind === 'environment' && item.id === environmentID) || null; }
function rerenderTemporaryLifecycle(environmentID) {
  renderEnvironments(safeArray(currentSnapshot?.environments));
  if (selectedEnvironmentID === environmentID && managementInspection?.environment?.environment_id === environmentID) refreshSelectedEnvironmentDetail();
}
async function refreshTemporaryEnvironmentLifecycle(environment, {silent = false} = {}) {
  if (!environment?.environment_id) return null;
  const requestGeneration = connectionGeneration, id = environment.environment_id;
  if (!silent) setStatus('正在刷新 temporary Environment lifecycle…', 'loading');
  try {
    const status = await desktopAdapter().GetTemporaryEnvironmentStatus(id);
    if (requestGeneration !== connectionGeneration) return null;
    temporaryLifecycleByEnvironmentID.set(id, status); temporaryCleanupPreviewIDs.delete(id); rerenderTemporaryLifecycle(id);
    if (!silent) setStatus('Temporary Environment lifecycle 已刷新', 'success');
    return status;
  } catch (error) {
    if (requestGeneration === connectionGeneration && !silent) setStatus('Temporary Environment lifecycle 刷新失败：' + errorText(error), 'error');
    return null;
  }
}
async function previewTemporaryEnvironmentCleanup(environment) {
  if (!environment?.environment_id) return;
  const ownerID = temporaryLifecycleOwner(environment); if (!ownerID) return setStatus('Temporary Environment 缺少 lifecycle owner，不能预览清理。', 'error');
  const requestGeneration = connectionGeneration, id = environment.environment_id; setStatus('正在预览 temporary Environment cleanup…', 'loading');
  try {
    const [status, result] = await Promise.all([desktopAdapter().GetTemporaryEnvironmentStatus(id), desktopAdapter().CleanupTemporaryEnvironment(id, ownerID, false)]);
    if (requestGeneration !== connectionGeneration) return;
    const item = temporaryCleanupItem(result, id); const merged = {...status};
    if (item) { merged.cleanup_state = item.cleanup_state || merged.cleanup_state; merged.cleanup_eligible = Boolean(item.cleanup_eligible); merged.blockers = safeArray(item.blockers); merged.uncertainties = safeArray(item.uncertainties); }
    temporaryLifecycleByEnvironmentID.set(id, merged);
    if (merged.cleanup_eligible) temporaryCleanupPreviewIDs.add(id); else temporaryCleanupPreviewIDs.delete(id);
    rerenderTemporaryLifecycle(id);
    const reasons = safeArray(merged.blockers).concat(safeArray(merged.uncertainties));
    setStatus(merged.cleanup_eligible ? 'Cleanup preview：当前可安全清理。请再次点击“确认清理”并确认。' : 'Cleanup preview：当前被阻止 · ' + (reasons.join(' · ') || merged.cleanup_state || 'unknown blocker'), merged.cleanup_eligible ? 'success' : 'error');
  } catch (error) { if (requestGeneration === connectionGeneration) setStatus('Cleanup preview 失败：' + errorText(error), 'error'); }
}
async function promoteTemporaryEnvironment(environment) {
  if (!environment?.environment_id) return;
  const ownerID = temporaryLifecycleOwner(environment); if (!ownerID) return setStatus('Temporary Environment 缺少 lifecycle owner，不能 promote。', 'error');
  if (!window.confirm(`将同一个 Environment 保留为 durable？不会复制 root、Memory 或创建新 Environment。\n${environment.name || environment.environment_id}\nOwner: ${ownerID}`)) return;
  const requestGeneration = connectionGeneration, id = environment.environment_id; setStatus('正在 promote temporary Environment…', 'loading');
  try {
    const status = await desktopAdapter().PromoteTemporaryEnvironment(id, ownerID);
    if (requestGeneration !== connectionGeneration) return;
    temporaryLifecycleByEnvironmentID.set(id, status); temporaryCleanupPreviewIDs.delete(id); await refreshSnapshot();
    if (requestGeneration !== connectionGeneration) return;
    setStatus('Environment 已 promote 为 durable；stable ID/root 保持不变。', 'success');
  } catch (error) { if (requestGeneration === connectionGeneration) setStatus('Promote temporary Environment 失败：' + errorText(error), 'error'); }
}
async function executeTemporaryEnvironmentCleanup(environment) {
  if (!environment?.environment_id) return;
  const id = environment.environment_id, status = temporaryLifecycleStatus(environment), ownerID = temporaryLifecycleOwner(environment);
  if (!temporaryCleanupPreviewIDs.has(id) || !status?.cleanup_eligible) return setStatus('请先执行 cleanup preview；只有最新预览明确 eligible 才能确认清理。', 'error');
  if (!ownerID) return setStatus('Temporary Environment 缺少 lifecycle owner，不能清理。', 'error');
  const managed = Boolean(status.managed_worktree); const effect = managed ? 'ADM-managed worktree root 会被移除，managed branch 会保留。' : '只移除 ADM Environment 状态；Workspace/root/project files 不会被删除。';
  if (!window.confirm(`确认清理这个 temporary Environment？\n${environment.name || id}\n${effect}\n没有 force 路径。`)) return;
  const requestGeneration = connectionGeneration; setStatus('正在执行 temporary Environment cleanup…', 'loading');
  try {
    await desktopAdapter().CleanupTemporaryEnvironment(id, ownerID, true);
    if (requestGeneration !== connectionGeneration) return;
    temporaryLifecycleByEnvironmentID.delete(id); temporaryCleanupPreviewIDs.delete(id); if (selectedEnvironmentID === id) closeEnvironmentDetail(); await refreshSnapshot();
    if (requestGeneration !== connectionGeneration) return;
    setStatus(managed ? 'Temporary Environment 已清理：managed worktree root 已移除，branch 已保留。' : 'Temporary Environment 已清理：仅移除 ADM Environment 状态，项目目录未删除。', 'success');
  } catch (error) { if (requestGeneration === connectionGeneration) { temporaryCleanupPreviewIDs.delete(id); setStatus('Temporary Environment cleanup 失败：' + errorText(error), 'error'); await refreshTemporaryEnvironmentLifecycle(environment, {silent: true}); } }
}

function syncMCPAuthForm() {
  const stdio = elements.mcpTransport.value === 'stdio';
  elements.mcpRefsField.hidden = stdio || elements.mcpAuthMode.value !== 'headers';
}
function syncMCPTransportForm() {
  const stdio = elements.mcpTransport.value === 'stdio';
  elements.mcpEndpointField.hidden = stdio; elements.mcpExecutableField.hidden = !stdio; elements.mcpArgsField.hidden = !stdio; elements.mcpAuthField.hidden = stdio; elements.mcpEnvRefsField.hidden = !stdio;
  elements.mcpEndpoint.required = !stdio; elements.mcpExecutable.required = stdio; syncMCPAuthForm();
}
function syncMCPHealthForm() {
  const healthEnabled = elements.mcpHealthEnabled.checked; const reconnectEnabled = elements.mcpAutoReconnect.checked;
  elements.mcpProbeTimeoutField.hidden = !healthEnabled; elements.mcpCheckIntervalField.hidden = !healthEnabled; elements.mcpReconnectIntervalField.hidden = !reconnectEnabled;
  elements.mcpProbeTimeout.disabled = !healthEnabled; elements.mcpCheckInterval.disabled = !healthEnabled; elements.mcpReconnectInterval.disabled = !reconnectEnabled;
}
function clearMCPObservedHealth(mcpID) {
  mcpHealthByKey.delete(mcpHealthKey(mcpID)); mcpProbeRequests.delete(mcpID);
}
function resetMCPEditor(close = false, rerender = true) {
  editingMCPID = ''; elements.mcpForm.reset(); elements.mcpTransport.value = 'streamable-http'; elements.mcpHealthEnabled.checked = true; elements.mcpAutoReconnect.checked = false; elements.mcpProbeTimeout.value = '5'; elements.mcpCheckInterval.value = '30'; elements.mcpReconnectInterval.value = '30';
  elements.mcpEditorSummary.textContent = '添加 MCP 定义'; elements.mcpEditorHint.hidden = true; elements.mcpSubmitButton.textContent = '添加全局 MCP'; elements.mcpEditCancelButton.hidden = true; syncMCPTransportForm(); syncMCPHealthForm();
  if (close) closeEditorDialog('mcpEditorFlow'); if (rerender) renderMCPManager(safeArray(currentSnapshot?.mcps));
}
function beginMCPEdit(entry) {
  if (!entry) return;
  editingMCPID = entry.id; elements.mcpName.value = entry.name || ''; elements.mcpTransport.value = entry.transport || 'streamable-http'; elements.mcpEndpoint.value = entry.endpoint || ''; elements.mcpExecutable.value = entry.executable || ''; elements.mcpArgs.value = safeArray(entry.args).join('\n'); elements.mcpAuthMode.value = entry.auth_mode || 'none';
  elements.mcpReferencePairs.value = referenceLines(entry.header_refs); elements.mcpEnvReferencePairs.value = referenceLines(entry.env_refs); syncMCPAuthModeFromReferences(); elements.mcpHealthEnabled.checked = Boolean(entry.health_policy?.health_check_enabled); elements.mcpAutoReconnect.checked = Boolean(entry.health_policy?.auto_reconnect); elements.mcpProbeTimeout.value = String(entry.health_policy?.probe_timeout_seconds || 5); elements.mcpCheckInterval.value = String(entry.health_policy?.check_interval_seconds || 30); elements.mcpReconnectInterval.value = String(entry.health_policy?.reconnect_interval_seconds || 30); elements.mcpDefault.checked = Boolean(entry.default_include_in_environment);
  elements.mcpEditorSummary.textContent = `编辑 MCP · ${entry.name || entry.id}`; elements.mcpEditorHint.hidden = false; elements.mcpSubmitButton.textContent = '保存修改'; elements.mcpEditCancelButton.hidden = false; syncMCPTransportForm(); syncMCPHealthForm(); openEditorDialog('mcpEditorFlow');
}

elements.mcpTransport.addEventListener('change', () => { syncMCPTransportForm(); syncMCPAuthModeFromReferences(); });
elements.mcpAuthMode.addEventListener('change', syncMCPAuthForm);
elements.mcpReferencePairs.addEventListener('input', syncMCPAuthModeFromReferences);
elements.mcpHealthEnabled.addEventListener('change', syncMCPHealthForm);
elements.mcpAutoReconnect.addEventListener('change', syncMCPHealthForm);
elements.mcpEditCancelButton.addEventListener('click', () => resetMCPEditor(true));
elements.workspaceFilter.addEventListener('input', () => renderWorkspaces(safeArray(currentSnapshot?.workspaces)));
elements.environmentFilter.addEventListener('input', () => renderEnvironments(safeArray(currentSnapshot?.environments)));
elements.environmentWorkspaceFilter.addEventListener('change', () => renderEnvironments(safeArray(currentSnapshot?.environments)));
elements.mcpFilter.addEventListener('input', () => renderMCPManager(safeArray(currentSnapshot?.mcps)));
elements.mcpStateFilter.addEventListener('change', () => renderMCPManager(safeArray(currentSnapshot?.mcps)));
for (const element of [elements.mcpImportFormat, elements.mcpImportConflict, elements.mcpImportDefault]) element.addEventListener('change', () => invalidateMCPImportPreview());
elements.mcpImportContent.addEventListener('input', () => invalidateMCPImportPreview());
elements.mcpSelectVisibleButton.addEventListener('click', () => { for (const row of elements.mcpList.querySelectorAll('.resource-row[data-id]')) selectedMCPIDs.add(row.dataset.id); renderMCPManager(safeArray(currentSnapshot?.mcps)); updateSkillBulkControls(); });
elements.mcpClearSelectionButton.addEventListener('click', () => { selectedMCPIDs.clear(); renderMCPManager(safeArray(currentSnapshot?.mcps)); updateSkillBulkControls(); });
elements.mcpAssignEnvironmentsButton.addEventListener('click', openCapabilityAssignmentDialog);
elements.mcpClearMissingButton.addEventListener('click', () => clearMissingEnvironmentSelections('mcp'));
elements.mcpSetVisibleDefaultButton.addEventListener('click', () => runVisibleBatch('mcp', '批量设置 MCP 新环境默认值', visibleResourceIDs(elements.mcpList), (id) => desktopAdapter().SetMCPDefault(id, true), () => renderMCPManager(safeArray(currentSnapshot?.mcps))));
elements.mcpUnsetVisibleDefaultButton.addEventListener('click', () => runVisibleBatch('mcp', '批量取消 MCP 新环境默认值', visibleResourceIDs(elements.mcpList), (id) => desktopAdapter().SetMCPDefault(id, false), () => renderMCPManager(safeArray(currentSnapshot?.mcps))));
elements.mcpEnableVisibleButton.addEventListener('click', () => { const environmentID = managementEnvironmentID; if (!environmentID) return setStatus('请先选择 Management Environment。', 'error'); return runVisibleBatch('mcp', '批量启用当前 Environment 显式 MCP', visibleResourceIDs(elements.mcpList), (id) => desktopAdapter().SetEnvironmentMCP(environmentID, id, true), () => renderMCPManager(safeArray(currentSnapshot?.mcps))); });
elements.mcpDisableVisibleButton.addEventListener('click', () => { const environmentID = managementEnvironmentID; if (!environmentID) return setStatus('请先选择 Management Environment。', 'error'); return runVisibleBatch('mcp', '批量取消当前 Environment 显式 MCP', visibleResourceIDs(elements.mcpList), (id) => desktopAdapter().SetEnvironmentMCP(environmentID, id, false), () => renderMCPManager(safeArray(currentSnapshot?.mcps))); });
elements.skillSubviewTabs.addEventListener('click', (event) => { const button = event.target.closest('[data-skill-subview]'); if (!button) return; setSkillSubview(button.dataset.skillSubview); });
elements.skillFilter.addEventListener('input', () => renderSkillManager(safeArray(currentSnapshot?.skills)));
elements.skillSourceFilter.addEventListener('change', () => renderSkillManager(safeArray(currentSnapshot?.skills)));
elements.skillSourceFilterInput.addEventListener('input', () => renderSkillSources());
elements.skillStateFilter.addEventListener('change', () => renderSkillManager(safeArray(currentSnapshot?.skills)));
elements.skillProbeAllButton.addEventListener('click', () => probeAllSkillAvailability());
elements.skillSelectVisibleButton.addEventListener('click', () => { for (const input of elements.skillList.querySelectorAll('input[data-action="select-skill"]')) selectedSkillIDs.add(input.dataset.id); renderSkillManager(safeArray(currentSnapshot?.skills)); });
elements.skillClearSelectionButton.addEventListener('click', () => { selectedSkillIDs.clear(); renderSkillManager(safeArray(currentSnapshot?.skills)); });
elements.skillAssignEnvironmentsButton.addEventListener('click', openCapabilityAssignmentDialog);
elements.skillClearMissingButton.addEventListener('click', () => clearMissingEnvironmentSelections('skill'));
elements.skillSetVisibleDefaultButton.addEventListener('click', () => runVisibleBatch('skill', '批量设置 Skill 新环境默认值', visibleResourceIDs(elements.skillList), (id) => desktopAdapter().SetSkillDefault(id, true), () => renderSkillManager(safeArray(currentSnapshot?.skills))));
elements.skillUnsetVisibleDefaultButton.addEventListener('click', () => runVisibleBatch('skill', '批量取消 Skill 新环境默认值', visibleResourceIDs(elements.skillList), (id) => desktopAdapter().SetSkillDefault(id, false), () => renderSkillManager(safeArray(currentSnapshot?.skills))));
elements.skillEnableVisibleButton.addEventListener('click', () => { const environmentID = managementEnvironmentID; if (!environmentID) return setStatus('请先选择 Management Environment。', 'error'); return runVisibleBatch('skill', '批量启用当前 Environment 显式 Skill', visibleResourceIDs(elements.skillList), (id) => desktopAdapter().SetEnvironmentSkill(environmentID, id, true), () => renderSkillManager(safeArray(currentSnapshot?.skills))); });
elements.skillDisableVisibleButton.addEventListener('click', () => { const environmentID = managementEnvironmentID; if (!environmentID) return setStatus('请先选择 Management Environment。', 'error'); return runVisibleBatch('skill', '批量取消当前 Environment 显式 Skill', visibleResourceIDs(elements.skillList), (id) => desktopAdapter().SetEnvironmentSkill(environmentID, id, false), () => renderSkillManager(safeArray(currentSnapshot?.skills))); });
elements.skillDeleteSelectedButton.addEventListener('click', () => removeSkillCatalogEntries([...selectedSkillIDs], '批量删除'));
elements.skillClearUnavailableButton.addEventListener('click', () => {
  if (!explicitSkillProbeIsCurrent()) return setStatus('当前 Skill 可用性检查结果已过期，请重新批量检查后再清理。', 'error');
  const ids = skillAvailabilitySummary().cleanupIDs;
  if (!ids.length) return setStatus('当前显式检查没有发现可清理的不可用 Skill。', 'success');
  return removeSkillCatalogEntries(ids, '一键清除不可用 Skill');
});
elements.capabilityAssignmentEnvironmentList.addEventListener('change', (event) => {
  const input = event.target.closest('input[data-action="select-assignment-environment"]'); if (!input) return;
  if (input.checked) selectedAssignmentEnvironmentIDs.add(input.dataset.id); else selectedAssignmentEnvironmentIDs.delete(input.dataset.id);
  renderCapabilityAssignmentDialog();
});
elements.capabilityAssignmentSelectAllButton.addEventListener('click', () => { selectedAssignmentEnvironmentIDs = new Set(safeArray(currentSnapshot?.environments).map((environment) => environment.environment_id).filter(Boolean)); renderCapabilityAssignmentDialog(); });
elements.capabilityAssignmentClearButton.addEventListener('click', () => { selectedAssignmentEnvironmentIDs.clear(); renderCapabilityAssignmentDialog(); });
elements.capabilityAssignmentEnableButton.addEventListener('click', () => runCapabilityAssignment(true));
elements.capabilityAssignmentDisableButton.addEventListener('click', () => runCapabilityAssignment(false));
elements.managementWorkspace.addEventListener('change', async () => {
  const nextWorkspaceID = elements.managementWorkspace.value;
  if (!elements.environmentDetailPanel.hidden) closeEnvironmentDetail();
  environmentGeneration++; managementWorkspaceID = nextWorkspaceID; managementEnvironmentID = ''; managementContextError = ''; managementSkillAvailabilityError = ''; managementInspection = null; managementCapabilityFacts = new Map(); environmentSkillAvailabilityByID = new Map(); runtimePendingActionKey = ''; clearRuntimeWriterCredential();
  renderManagementEnvironmentOptions(safeArray(currentSnapshot?.workspaces), safeArray(currentSnapshot?.environments));
  resetEnvironmentMemoryScope(managementEnvironmentID ? '尚未加载当前 Environment Memory' : '请选择 Management Environment 后加载 private Memory'); resetRuntimeCollections(managementEnvironmentID ? 'loading' : 'unloaded'); clearRuntimeOutput('Workspace 已切换'); updateSkillBulkControls(); updateEnvironmentContextMarkers(); renderRuntime();
  const scope = captureEnvironmentScope();
  try { const result = await refreshManagementContext(scope); if (result?.stale) return; setStatus(managementEnvironmentID ? 'Workspace 已切换并加载唯一 Environment' : 'Workspace 管理上下文已切换', 'success'); }
  catch (error) { if (environmentScopeIsCurrent(scope)) setStatus(`Workspace 状态读取失败：${errorText(error)}`, 'error'); }
});
elements.managementEnvironment.addEventListener('change', async () => {
  const nextEnvironmentID = elements.managementEnvironment.value; if (!elements.environmentDetailPanel.hidden) closeEnvironmentDetail();
  environmentGeneration++; managementEnvironmentID = nextEnvironmentID; managementContextError = ''; managementSkillAvailabilityError = ''; managementInspection = null; managementCapabilityFacts = new Map(); environmentSkillAvailabilityByID = new Map(); runtimePendingActionKey = ''; clearRuntimeWriterCredential(); resetEnvironmentMemoryScope(nextEnvironmentID ? '尚未加载当前 Environment Memory' : '请选择 Management Environment 后加载 private Memory'); resetRuntimeCollections(nextEnvironmentID ? 'loading' : 'unloaded'); clearRuntimeOutput('Environment 已切换'); updateManagementHint(); updateSkillBulkControls(); updateEnvironmentContextMarkers(); renderRuntime();
  const scope = captureEnvironmentScope();
  setStatus('正在加载 Environment MCP/Skill/Runtime 状态…', 'loading');
  try { const result = await refreshManagementContext(scope); if (result?.stale) return; setStatus(result?.errors?.length ? `Environment 已切换；部分状态不可用：${result.errors.join(' · ')}` : 'Environment 管理上下文已切换', result?.errors?.length ? 'error' : 'success'); }
  catch (error) { if (environmentScopeIsCurrent(scope)) setStatus(`Environment 状态读取失败：${errorText(error)}`, 'error'); }
});
elements.editEnvironmentButton.addEventListener('click', () => {
  const environment = currentEnvironment();
  if (!environment) return;
  openRenameDialog('Environment', environment.name || '', (name) => desktopAdapter().RenameEnvironment(environment.environment_id, name));
});
elements.runtimeSubviewTabs.addEventListener('click', (event) => {
  const button = event.target.closest('button[data-runtime-subview]'); if (!button) return;
  const next = window.ADMRuntimeView?.normalizeSubview(button.dataset.runtimeSubview) || button.dataset.runtimeSubview;
  if (next === runtimeSubview) return;
  runtimeSubview = next; runtimeSubviewGeneration++; clearRuntimeOutput('已切换 Runtime 视图'); renderRuntime();
});
elements.runtimeWriterAcquireButton.addEventListener('click', async () => {
  const environment = currentEnvironment(); if (!environment) return setStatus('请先选择 Management Environment。', 'error');
  let owner = String(elements.runtimeWriterOwnerInput.value || '').trim();
  if (!owner && environment.writer) return setStatus('当前已有 active Writer，owner 已隐藏；必须由人类显式输入正确 owner 才能续租。', 'error');
  if (!owner) { owner = generatedDesktopWriterOwner(); elements.runtimeWriterOwnerInput.value = owner; }
  const environmentID = environment.environment_id;
  await runMutation('获取 / 续租 Writer', async () => {
    await desktopAdapter().AcquireRuntimeWriter(environmentID, owner);
    runtimeWriterCredential = {environmentID, owner};
  });
});
elements.runtimeWriterReleaseButton.addEventListener('click', async () => {
  const environment = currentEnvironment(), owner = runtimeWriterOwner();
  if (!environment || !owner) return setStatus('当前 Desktop 会话没有可释放的 Writer Owner。', 'error');
  const environmentID = environment.environment_id;
  await runMutation('释放 Writer', async () => {
    await desktopAdapter().ReleaseRuntimeWriter(environmentID, owner);
    clearRuntimeWriterCredential();
  });
});
elements.runtimeRefreshButton.addEventListener('click', () => refreshRuntimeContext(true).catch((error) => setStatus(`Runtime 刷新失败：${error?.message || String(error)}`, 'error')));
elements.diagnosticsRefreshButton.addEventListener('click', async () => {
  const environment = currentEnvironment();
  if (!environment) return setStatus('请先选择 Management Environment。', 'error');
  const scope = captureEnvironmentScope();
  setStatus('正在刷新 Environment 诊断…', 'loading');
  try {
    const result = await refreshManagementContext(scope);
    if (result?.stale) return;
    setStatus(result?.errors?.length ? 'Environment 诊断已刷新；部分状态不可用：' + result.errors.join(' · ') : 'Environment 诊断已刷新', result?.errors?.length ? 'error' : 'success');
  } catch (error) { if (environmentScopeIsCurrent(scope)) setStatus('Environment 诊断刷新失败：' + errorText(error), 'error'); }
});
elements.verifierList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action="run-verifier"]'); if (!button) return;
  const scope = captureEnvironmentScope(), environmentID = scope.environmentID, verifierID = button.dataset.id, owner = runtimeWriterOwner(), viewGeneration = runtimeSubviewGeneration;
  if (!environmentID) return; if (!owner || !currentEnvironment()?.writer) return setStatus('当前 Desktop 会话没有可用的 active Writer authority，不能运行 verifier。', 'error');
  const result = await runRuntimeAction('运行 verifier', `verifier:${verifierID}`, scope, () => desktopAdapter().RunVerifier(environmentID, owner, verifierID));
  if (result && environmentScopeIsCurrent(scope) && runtimeSubview === 'verifiers' && runtimeSubviewGeneration === viewGeneration) {
    showRuntimeOutput('verifiers', verifierID, `Verifier ${verifierID} · ${result.status || 'unknown'}`, result.stdout || '', result.stderr || '', `${result.summary || ''}${result.exit_code !== undefined ? ` · exit ${result.exit_code}` : ''}`);
  }
});
elements.processList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return;
  const scope = captureEnvironmentScope(), environmentID = scope.environmentID, processID = button.dataset.id, viewGeneration = runtimeSubviewGeneration;
  if (!environmentID) return;
  if (button.dataset.action === 'process-logs') {
    if (runtimePendingActionKey) return; const actionKey = `process-logs:${processID}`; runtimePendingActionKey = actionKey; renderRuntime(); setStatus('正在读取 process 日志…', 'loading');
    try {
      const logs = await desktopAdapter().GetProcessLogs(environmentID, processID);
      if (!environmentScopeIsCurrent(scope) || runtimeSubview !== 'processes' || runtimeSubviewGeneration !== viewGeneration) return;
      const truncation = window.ADMRuntimeView?.truncationSummary(logs) || '';
      showRuntimeOutput('processes', processID, `Process ${processID}`, logs.stdout || '', logs.stderr || '', '', truncation); setStatus('Process 日志已读取', 'success');
    } catch (error) { if (environmentScopeIsCurrent(scope) && runtimeSubview === 'processes' && runtimeSubviewGeneration === viewGeneration) setStatus(`Process 日志读取失败：${error?.message || String(error)}`, 'error'); }
    finally { if (runtimePendingActionKey === actionKey) { runtimePendingActionKey = ''; renderRuntime(); } }
    return;
  }
  if (button.dataset.action === 'stop-process') {
    const owner = runtimeWriterOwner(); if (!owner || !currentEnvironment()?.writer) return setStatus('当前 Desktop 会话没有可用的 active Writer authority，不能停止 process。', 'error');
    await runRuntimeAction('停止 process', `stop-process:${processID}`, scope, () => desktopAdapter().StopProcess(environmentID, owner, processID));
  }
});
elements.runList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return;
  const scope = captureEnvironmentScope(), environmentID = scope.environmentID, runID = button.dataset.id, run = runtimeRunsByID.get(runID);
  if (!environmentID) return;
  if (button.dataset.action === 'run-output') {
    if (!run) return; const truncation = window.ADMRuntimeView?.truncationSummary(run) || '';
    showRuntimeOutput('runs', run.id, `Run ${run.id} · ${run.state || 'unknown'}`, run.stdout || '', run.stderr || '', `${run.message || ''}${run.exit_code !== undefined && run.exit_code !== null ? ` · exit ${run.exit_code}` : ''}`, truncation); return;
  }
  if (button.dataset.action === 'cancel-run') {
    const owner = runtimeWriterOwner(); if (!owner || !currentEnvironment()?.writer) return setStatus('当前 Desktop 会话没有可用的 active Writer authority，不能取消 run。', 'error');
    await runRuntimeAction('取消 run', `cancel-run:${runID}`, scope, () => desktopAdapter().CancelRun(environmentID, owner, runID));
  }
});

elements.mcpForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const editingID = editingMCPID;
  try {
    const transport = elements.mcpTransport.value; const authMode = effectiveMCPAuthMode(transport, elements.mcpAuthMode.value);
    const headerRefs = transport !== 'stdio' && authMode === 'headers' ? referenceMap(elements.mcpReferencePairs.value) : {};
    const envRefs = transport === 'stdio' ? referenceMap(elements.mcpEnvReferencePairs.value) : {};
    if (transport !== 'stdio' && authMode === 'headers' && Object.keys(headerRefs).length === 0) throw new Error('Secret-backed headers 需要至少一个 Header reference mapping');
    elements.mcpAuthMode.value = authMode;
    const input = {
      name: elements.mcpName.value.trim(), transport, auth_mode: authMode,
      endpoint: transport === 'stdio' ? '' : elements.mcpEndpoint.value.trim(), executable: transport === 'stdio' ? elements.mcpExecutable.value.trim() : '', args: transport === 'stdio' ? lineValues(elements.mcpArgs.value) : [],
      header_refs: transport === 'stdio' ? {} : headerRefs, env_refs: transport === 'stdio' ? envRefs : {}, default_include_in_environment: elements.mcpDefault.checked,
      health_policy: {health_check_enabled: elements.mcpHealthEnabled.checked, check_interval_seconds: Math.max(1, safeNumber(elements.mcpCheckInterval.value, 30)), probe_timeout_seconds: Math.max(1, safeNumber(elements.mcpProbeTimeout.value, 5)), auto_reconnect: elements.mcpAutoReconnect.checked, reconnect_interval_seconds: Math.max(1, safeNumber(elements.mcpReconnectInterval.value, 30))},
    };
    if (!input.name) throw new Error('MCP 名称不能为空');
    if (editingID) {
      await runMutation('更新全局 MCP', async () => { await desktopAdapter().UpdateMCP(editingID, input); clearMCPObservedHealth(editingID); resetMCPEditor(true); });
    } else {
      await runMutation('添加全局 MCP', async () => { await desktopAdapter().AddMCP(input); resetMCPEditor(true); });
    }
  } catch (error) { setStatus(`${editingID ? '更新' : '添加'} MCP 失败：${error?.message || String(error)}`, 'error'); }
});

elements.mcpImportForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (mcpImportBusy) return;
  const input = currentMCPImportInput();
  if (!input.json_or_jsonc.trim()) return;
  const fingerprint = mcpImportFingerprint(input);
  const requestGeneration = connectionGeneration;
  pendingMCPImport = null;
  elements.mcpImportApplyButton.disabled = true;
  mcpImportBusy = true;
  setStatus('正在预览 MCP 导入…', 'loading');
  try {
    const preview = await desktopAdapter().PreviewMCPImport(input);
    if (requestGeneration !== connectionGeneration || fingerprint !== mcpImportFingerprint(currentMCPImportInput())) {
      resetMCPImportPreview('导入预览已过期，请重新预览。');
      return;
    }
    pendingMCPImport = {input, fingerprint, connectionGeneration: requestGeneration};
    renderImportPreview(preview);
    if (!elements.mcpImportApplyButton.disabled) setStatus('导入预览已生成；确认后再应用', 'success');
    else setStatus('导入预览包含错误，不能应用', 'error');
  }
  catch (error) { resetMCPImportPreview('预览失败：' + errorText(error)); setStatus('MCP 导入预览失败：' + errorText(error), 'error'); }
  finally { mcpImportBusy = false; }
});
elements.mcpImportApplyButton.addEventListener('click', async () => {
  const pending = currentPendingMCPImport();
  if (!pending || mcpImportBusy) return;
  mcpImportBusy = true;
  elements.mcpImportApplyButton.disabled = true;
  pendingMCPImport = null;
  setStatus('正在应用 MCP 导入…', 'loading');
  try {
    const result = await desktopAdapter().ApplyMCPImport(pending.input);
    renderImportApplyResult(result);
    elements.mcpImportContent.value = '';
    closeFormDialog(elements.mcpImportForm);
    await refreshSnapshot('MCP 导入完成');
  }
  catch (error) { setStatus('MCP 导入失败：' + errorText(error), 'error'); resetMCPImportPreview('导入应用失败；输入已保留，请重新预览后再应用。'); }
  finally { mcpImportBusy = false; }
});
elements.skillSourceForm.addEventListener('submit', async (event) => {
  event.preventDefault(); const root = elements.skillSourceRoot.value.trim(); if (!root) return;
  const supportRoots = lineValues(elements.skillSupportRoots.value), defaultInclude = elements.skillSourceDefault.checked, editingID = editingSkillSourceID;
  if (editingID) {
    await runMutation('更新 Skill source', async () => { await desktopAdapter().UpdateSkillSource(editingID, {root, support_roots: supportRoots, default_include_in_environment: defaultInclude}); resetSkillSourceEditor(true, false); });
    return;
  }
  setStatus('正在添加 Skill source…', 'loading');
  try {
    const source = await desktopAdapter().AddSkillSource({root, support_roots: supportRoots, default_include_in_environment: defaultInclude});
    elements.skillSourceForm.reset(); closeFormDialog(elements.skillSourceForm);
    try { await desktopAdapter().RefreshSkillSource(source.skill_source_id); elements.skillSourceForm.reset(); closeFormDialog(elements.skillSourceForm); await refreshSnapshot('Skill source 已添加并刷新'); }
    catch (refreshError) { await refreshSnapshot(); setStatus('Skill source 已保存，但刷新失败：' + errorText(refreshError), 'error'); }
  } catch (error) { setStatus('添加 Skill source 失败：' + errorText(error), 'error'); }
});

elements.mcpList.addEventListener('change', async (event) => {
  const input = event.target.closest('input[data-action]'); if (!input) return;
  if (input.dataset.action === 'select-mcp') { if (input.checked) selectedMCPIDs.add(input.dataset.id); else selectedMCPIDs.delete(input.dataset.id); updateMCPBulkControls(); updateSkillBulkControls(); return; }
  if (input.dataset.action === 'default-mcp') {
    const id = input.dataset.id, checked = input.checked;
    await runMutation('更新 MCP 新环境默认值', () => desktopAdapter().SetMCPDefault(id, checked));
  }
  if (input.dataset.action === 'workspace-mcp' && managementWorkspaceID) {
    const workspaceID = managementWorkspaceID, id = input.dataset.id, checked = input.checked;
    await runMutation('更新当前 Workspace MCP 继承', () => desktopAdapter().SetWorkspaceMCP(workspaceID, id, checked));
  }
  if (input.dataset.action === 'environment-mcp' && managementEnvironmentID) {
    const environmentID = managementEnvironmentID, id = input.dataset.id, checked = input.checked;
    await runMutation('更新当前 Environment MCP 选择', () => desktopAdapter().SetEnvironmentMCP(environmentID, id, checked));
  }
});
elements.mcpList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return; const id = button.dataset.id; const entry = safeArray(currentSnapshot?.mcps).find((mcp) => mcp.id === id);
  if (button.dataset.action === 'copy-mcp-config') { try { await copyMCPConfig(entry); } catch (error) { setStatus('复制 MCP 配置失败：' + errorText(error), 'error'); } return; }
  if (button.dataset.action === 'edit-mcp') { beginMCPEdit(entry); return; }
  if (button.dataset.action === 'probe-mcp') {
    if (!entry || mcpProbeRequests.has(id)) return;
    const requestGeneration = connectionGeneration, fingerprint = mcpProbeFingerprint(entry), request = {};
    mcpProbeRequests.set(id, request);
    const isCurrent = () => requestGeneration === connectionGeneration && mcpProbeRequests.get(id) === request && fingerprint === mcpProbeFingerprint(safeArray(currentSnapshot?.mcps).find(mcp => mcp.id === id));
    renderMCPManager(safeArray(currentSnapshot?.mcps));
    setStatus('正在探测 ' + (entry.name || id) + '…', 'loading');
    try {
      const health = await desktopAdapter().ProbeMCPHealth(id);
      if (!isCurrent()) return;
      mcpHealthByKey.set(mcpHealthKey(id), {fingerprint, health});
      const runtimeState = mcpRuntimeState(health), recoveryHint = mcpProbeRecoveryHint(health, entry);
      setStatus(runtimeState === 'healthy' ? 'MCP 全局探测：健康' : `MCP 全局探测失败：${health?.message || mcpProbeStateLabel(health)}${recoveryHint ? ` · ${recoveryHint}` : ''}`, runtimeState === 'healthy' ? 'success' : 'error');
    } catch (error) {
      if (isCurrent()) {
        mcpHealthByKey.set(mcpHealthKey(id), {fingerprint, health: {state: 'error', message: '探测请求失败，请重试。'}});
        setStatus('MCP 探测失败：' + errorText(error), 'error');
      }
    } finally {
      if (mcpProbeRequests.get(id) === request) mcpProbeRequests.delete(id);
      if (requestGeneration === connectionGeneration) renderMCPManager(safeArray(currentSnapshot?.mcps));
    }
  }
  if (button.dataset.action === 'remove-mcp' && window.confirm(`这是全局删除，不是只从当前 Environment 禁用。删除时会同时从所有 Workspace / Environment 选择中清除这个 MCP ID。继续？\n${entry?.name || id}`)) await runMutation('删除全局 MCP 定义', async () => { await desktopAdapter().RemoveMCP(id); clearMCPObservedHealth(id); if (editingMCPID === id) resetMCPEditor(true, false); });
});

elements.skillSourceList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return; const id = button.dataset.id; const source = skillSources.find((item) => item.skill_source_id === id);
  if (button.dataset.action === 'edit-skill-source') { beginSkillSourceEdit(source); return; }
  if (button.dataset.action === 'refresh-skill-source') await runMutation('刷新 Skill source', () => desktopAdapter().RefreshSkillSource(id));
  if (button.dataset.action === 'remove-skill-source' && window.confirm(`删除全局 Skill source 及它发现的 Skill 条目。删除 Source 时会同时从所有 Workspace / Environment 选择中清除其已删除 Skill ID。继续？\n${source?.root || id}`)) await runMutation('删除 Skill source', () => desktopAdapter().RemoveSkillSource(id));
});
elements.skillList.addEventListener('change', async (event) => {
  const input = event.target.closest('input[data-action]'); if (!input) return;
  if (input.dataset.action === 'select-skill') { if (input.checked) selectedSkillIDs.add(input.dataset.id); else selectedSkillIDs.delete(input.dataset.id); updateSkillBulkControls(); updateMCPBulkControls(); return; }
  if (input.dataset.action === 'default-skill') await runMutation('更新 Skill 新环境默认值', () => desktopAdapter().SetSkillDefault(input.dataset.id, input.checked));
  if (input.dataset.action === 'workspace-skill' && managementWorkspaceID) await runMutation('更新当前 Workspace Skill 继承', () => desktopAdapter().SetWorkspaceSkill(managementWorkspaceID, input.dataset.id, input.checked));
  if (input.dataset.action === 'environment-skill' && managementEnvironmentID) await runMutation('更新当前 Environment Skill 选择', () => desktopAdapter().SetEnvironmentSkill(managementEnvironmentID, input.dataset.id, input.checked));
});
elements.skillList.addEventListener('click', async (event) => { const button = event.target.closest('button[data-action="remove-skill"]'); if (!button) return; if (window.confirm(`删除这个全局 Skill 条目？删除时会同时从所有 Workspace / Environment 选择中清除这个 Skill ID。Source-managed Skill 应通过 Source 管理。\n${button.dataset.id}`)) await runMutation('删除 Skill 条目', () => desktopAdapter().RemoveSkill(button.dataset.id)); });

elements.workspaceForm.addEventListener('submit', async (event) => { event.preventDefault(); const path = elements.workspacePath.value.trim(), name = elements.workspaceName.value.trim(); if (path) { const requestConnectionGeneration = connectionGeneration; let createdWorkspace = null; await runMutation('添加 Workspace', async () => { createdWorkspace = await desktopAdapter().AddWorkspace({path, name}); elements.workspaceForm.reset(); closeFormDialog(elements.workspaceForm); }, async () => promptWorkspaceRecommendationsAfterWorkspaceAdd(createdWorkspace, requestConnectionGeneration)); } });
elements.workspaceDiscoveryForm.addEventListener('submit', async (event) => { event.preventDefault(); await scanWorkspaceDiscovery(); });
elements.workspaceDiscoveryFilter.addEventListener('input', () => { if (workspaceDiscoveryReport && !workspaceDiscoveryLoading) renderWorkspaceDiscovery(); });
for (const input of [elements.workspaceDiscoveryPath, elements.workspaceDiscoveryQuery, elements.workspaceDiscoveryMaxDepth, elements.workspaceDiscoveryMaxEntries, elements.workspaceDiscoveryMaxCandidates, elements.workspaceDiscoveryMaxDigestEntries, elements.workspaceDiscoveryMaxOutputBytes]) input.addEventListener('input', () => { if (workspaceDiscoveryReport && !workspaceDiscoveryLoading) renderWorkspaceDiscovery(); });
elements.workspaceDiscoveryResult.addEventListener('click', (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return;
  const workspace = currentWorkspaceDiscoveryWorkspace(); if (!workspace) return;
  const relativeRoot = button.dataset.root || button.dataset.id || '.';
  if (button.dataset.action === 'use-discovery-root') {
    elements.environmentWorkspace.value = workspace.workspace_id;
    elements.environmentRoot.value = relativeRoot;
    closeEditorDialog('workspaceDiscoveryDialog');
    queueMicrotask(() => openEditorDialog('environmentDialog'));
    return;
  }
  if (button.dataset.action === 'show-discovery-environments') {
    elements.environmentWorkspaceFilter.value = workspace.workspace_id;
    elements.environmentFilter.value = absoluteCandidateRoot(workspace, relativeRoot);
    renderEnvironments(safeArray(currentSnapshot?.environments));
    closeEditorDialog('workspaceDiscoveryDialog');
    managementNavigation?.navigate('environments', {focus: true});
  }
});
elements.workspaceDiscoveryDialog.addEventListener('close', () => { if (elements.workspaceDiscoveryDialog.open) return; workspaceDiscoveryGeneration++; workspaceDiscoveryLoading = false; workspaceDiscoveryWorkspaceID = ''; workspaceDiscoveryReport = null; workspaceDiscoveryError = ''; workspaceDiscoveryFingerprint = ''; });
elements.environmentWorkspaceDialog.addEventListener('close', () => { environmentWorkspaceGeneration++; environmentWorkspaceEnvironmentID = ''; environmentWorkspaceOptions = null; environmentWorkspaceLoading = false; });
elements.environmentWorkspaceRecommendationsButton.addEventListener('click', () => openEnvironmentWorkspaceRecommendationsDialog());
elements.environmentWorkspaceRecommendationsApplyButton.addEventListener('click', () => applyEnvironmentWorkspaceRecommendations());
elements.environmentWorkspaceRecommendationsDialog.addEventListener('close', () => { environmentWorkspaceRecommendationsGeneration++; environmentWorkspaceRecommendations = []; environmentWorkspaceRecommendationsLoading = false; environmentWorkspaceRecommendationsApplying = false; environmentWorkspaceRecommendationsResult = ''; });
elements.environmentTreeDigestButton.addEventListener('click', () => loadEnvironmentTreeDigest());
elements.environmentForm.addEventListener('submit', async (event) => { event.preventDefault(); const workspaceID = elements.environmentWorkspace.value, name = elements.environmentName.value.trim(), root = elements.environmentRoot.value.trim(); if (workspaceID && name) { const requestConnectionGeneration = connectionGeneration; let createdEnvironment = null; await runMutation('创建 Environment', async () => { createdEnvironment = await desktopAdapter().CreateEnvironment({workspace_id: workspaceID, name, root: environmentRootForCreate(workspaceID, root)}); elements.environmentName.value = ''; elements.environmentRoot.value = ''; closeFormDialog(elements.environmentForm); }, async () => promptRecommendedWorkspaceAfterCreate(createdEnvironment, requestConnectionGeneration)); } });
elements.environmentWorkspaceTarget.addEventListener('change', () => {
  const currentWorkspaceID = environmentWorkspaceOptions?.current_workspace_id || '';
  elements.environmentWorkspaceSaveButton.disabled = environmentWorkspaceLoading || !environmentWorkspaceOptions?.rebind_allowed || !elements.environmentWorkspaceTarget.value || elements.environmentWorkspaceTarget.value === currentWorkspaceID;
});
elements.environmentWorkspaceForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const environmentID = environmentWorkspaceEnvironmentID; const workspaceID = elements.environmentWorkspaceTarget.value;
  if (!environmentID || !workspaceID || environmentWorkspaceLoading || !environmentWorkspaceOptions?.rebind_allowed) return;
  await runMutation('调整 Environment Workspace', () => desktopAdapter().SetEnvironmentWorkspace(environmentID, workspaceID), async () => {
    closeFormDialog(elements.environmentWorkspaceForm); environmentWorkspaceGeneration++; environmentWorkspaceEnvironmentID = ''; environmentWorkspaceOptions = null; environmentWorkspaceLoading = false;
  });
});
elements.execForm.addEventListener('submit', async (event) => { event.preventDefault(); const executable = elements.execExecutable.value.trim(); if (executable) await runMutation('更新 exec allowlist', async () => { await desktopAdapter().AllowExecutable(executable); elements.execForm.reset(); closeFormDialog(elements.execForm); }); });
elements.globalMemoryForm.addEventListener('submit', async (event) => {
  event.preventDefault(); const key = elements.globalMemoryKey.value.trim(), value = elements.globalMemoryValue.value; if (!key) return;
  const scope = currentGlobalMemoryScope();
  await runMutation('写入 Global Memory', () => desktopAdapter().WriteGlobalMemory(key, value), async () => {
    elements.globalMemoryForm.reset(); closeFormDialog(elements.globalMemoryForm);
    if (globalMemoryLoaded && globalMemoryScopeIsCurrent(scope)) await loadGlobalMemory({scope});
  });
});
elements.environmentMemoryForm.addEventListener('submit', async (event) => {
  event.preventDefault(); const scope = currentEnvironmentMemoryScope(); if (!scope.environmentID) return setStatus('请先选择 Management Environment。', 'error');
  const key = elements.environmentMemoryKey.value.trim(), value = elements.environmentMemoryValue.value; if (!key) return;
  await runMutation('写入 Environment-private Memory', () => desktopAdapter().WriteEnvironmentMemory(scope.environmentID, key, value), async () => {
    elements.environmentMemoryForm.reset(); closeFormDialog(elements.environmentMemoryForm);
    if (environmentMemoryLoaded && environmentMemoryScopeIsCurrent(scope)) await loadEnvironmentMemory({scope});
  });
});

elements.workspaceList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return;
  const id = button.dataset.id, workspace = safeArray(currentSnapshot?.workspaces).find((item) => item.workspace_id === id);
  if (button.dataset.action === 'discover-workspace') { if (workspace) { button.focus({preventScroll: true}); openWorkspaceDiscovery(workspace, button); } return; }
  if (button.dataset.action === 'filter-environments-by-workspace') {
    if (!workspace) return;
    elements.environmentWorkspaceFilter.value = id;
    renderEnvironments(safeArray(currentSnapshot?.environments));
    managementNavigation?.navigate('environments', {focus: true});
    return;
  }
  if (button.dataset.action === 'rename-workspace') { openRenameDialog('Workspace', workspace?.name || '', (name) => desktopAdapter().RenameWorkspace(id, name)); return; }
  if (button.dataset.action === 'remove-workspace' && window.confirm(`只移除 ADM Workspace 记录，不删除目录。继续？\n${workspace?.path || id}`)) await runMutation('移除 Workspace', () => desktopAdapter().RemoveWorkspace(id));
});
elements.environmentList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return; const id = button.dataset.id, environment = safeArray(currentSnapshot?.environments).find((item) => item.environment_id === id);
  if (button.dataset.action === 'diagnose-environment') {
    if (managementEnvironmentID !== id || managementWorkspaceID !== (environment?.workspace_id || '')) environmentGeneration++;
    managementWorkspaceID = environment?.workspace_id || ''; managementEnvironmentID = id; renderManagementEnvironmentOptions(safeArray(currentSnapshot?.workspaces), safeArray(currentSnapshot?.environments)); managementContextError = ''; managementSkillAvailabilityError = ''; updateSkillBulkControls(); updateEnvironmentContextMarkers(); resetEnvironmentMemoryScope('尚未加载当前 Environment Memory'); resetRuntimeCollections('loading'); renderRuntime();
    managementNavigation?.navigate('diagnostics', {focus: true});
    const scope = captureEnvironmentScope();
    setStatus('正在读取 Environment 诊断…', 'loading');
    try {
      const result = await refreshManagementContext(scope);
      if (result?.stale) return;
      setStatus(result?.errors?.length ? 'Environment 诊断已加载；部分状态不可用：' + result.errors.join(' · ') : 'Environment 诊断已加载', result?.errors?.length ? 'error' : 'success');
    } catch (error) { if (environmentScopeIsCurrent(scope)) setStatus('Environment 诊断读取失败：' + errorText(error), 'error'); }
    return;
  }
  if (button.dataset.action === 'inspect-environment') {
    environmentDetailSubview = 'summary';
    environmentDetailOpener = button; const token = ++detailGeneration; selectedEnvironmentID = id; resetEnvironmentTreeDigest();
    if (managementEnvironmentID !== id || managementWorkspaceID !== (environment?.workspace_id || '')) environmentGeneration++;
    managementWorkspaceID = environment?.workspace_id || ''; managementEnvironmentID = id; renderManagementEnvironmentOptions(safeArray(currentSnapshot?.workspaces), safeArray(currentSnapshot?.environments)); managementContextError = ''; managementSkillAvailabilityError = ''; updateSkillBulkControls(); updateEnvironmentContextMarkers(); setStatus('读取 Environment 详情…', 'loading');
    try {
      const contextResult = await refreshManagementContext(captureEnvironmentScope());
      if (token !== detailGeneration || selectedEnvironmentID !== id || contextResult?.stale) return;
      if (!contextResult?.inspection) throw new Error(managementContextError || 'Environment inspection 不可用');
      renderEnvironmentDetailFromInspection(contextResult.inspection, token);
      if (token !== detailGeneration || selectedEnvironmentID !== id) return;
      setStatus(contextResult?.errors?.length ? 'Environment 详情已加载；部分状态不可用：' + contextResult.errors.join(' · ') : 'Environment 详情已加载', contextResult?.errors?.length ? 'error' : 'success');
    } catch (error) { if (token === detailGeneration && selectedEnvironmentID === id) { environmentDetailOpener = null; setStatus('读取 Environment 详情失败：' + errorText(error), 'error'); } }
    return;
  }
  if (button.dataset.action === 'refresh-temporary-environment') { await refreshTemporaryEnvironmentLifecycle(environment); return; }
  if (button.dataset.action === 'promote-temporary-environment') { await promoteTemporaryEnvironment(environment); return; }
  if (button.dataset.action === 'preview-temporary-environment-cleanup') { await previewTemporaryEnvironmentCleanup(environment); return; }
  if (button.dataset.action === 'execute-temporary-environment-cleanup') { await executeTemporaryEnvironmentCleanup(environment); return; }
  if (button.dataset.action === 'workspace-environment') { if (environment) await openEnvironmentWorkspaceDialog(environment); return; }
  if (button.dataset.action === 'rename-environment') { openRenameDialog('Environment', environment?.name || '', (name) => desktopAdapter().RenameEnvironment(id, name)); }
  if (button.dataset.action === 'remove-environment' && window.confirm('只移除 ADM Environment 记录，不删除 root 或项目文件。继续？\n' + (environment?.root || id))) await runMutation('移除 Environment', () => desktopAdapter().RemoveEnvironment(id));
});
elements.execList.addEventListener('click', async (event) => { const button = event.target.closest('button[data-action="remove-executable"]'); if (button) await runMutation('移除 executable', () => desktopAdapter().RemoveExecutable(button.dataset.id)); });
elements.execBlockedList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]'); if (!button) return;
  const executable = button.dataset.id; if (!executable) return;
  if (button.dataset.action === 'allow-blocked-executable') await runMutation('允许被拦截 executable', () => desktopAdapter().AllowExecutable(executable));
  if (button.dataset.action === 'clear-blocked-executable') await runMutation('清理被拦截 executable', () => desktopAdapter().ClearExecDenial(executable));
});
elements.execClearAllBlockedButton.addEventListener('click', async () => {
  if (elements.execClearAllBlockedButton.disabled) return;
  if (!window.confirm('清理全部被拦截 executable 记录？不会修改 allowlist。')) return;
  await runMutation('清理全部被拦截 executable', () => desktopAdapter().ClearAllExecDenials());
});
async function handleSelectionChange(event, kind) {
  const checkbox = event.target.closest('input[data-kind]'); const environmentID = selectedEnvironmentID; if (!checkbox || !environmentID) return;
  const itemID = checkbox.dataset.id, enabled = checkbox.checked;
  await runMutation(`更新 Environment ${kind.toUpperCase()} 选择`, () => kind === 'mcp' ? desktopAdapter().SetEnvironmentMCP(environmentID, itemID, enabled) : desktopAdapter().SetEnvironmentSkill(environmentID, itemID, enabled));
}
elements.environmentMCPSelections.addEventListener('change', (event) => handleSelectionChange(event, 'mcp')); elements.environmentSkillSelections.addEventListener('change', (event) => handleSelectionChange(event, 'skill'));
elements.globalMemoryList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action="delete-global-memory"]'); if (!button) return;
  const scope = currentGlobalMemoryScope();
  await runMutation('删除 Global Memory', () => desktopAdapter().DeleteGlobalMemory(button.dataset.id), async () => { if (globalMemoryScopeIsCurrent(scope)) await loadGlobalMemory({scope}); });
});
elements.environmentMemoryList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action="delete-environment-memory"]'); if (!button) return;
  const scope = currentEnvironmentMemoryScope(); if (!scope.environmentID) return setStatus('请先选择 Management Environment。', 'error');
  await runMutation('删除 Environment-private Memory', () => desktopAdapter().DeleteEnvironmentMemory(scope.environmentID, button.dataset.id), async () => {
    if (environmentMemoryScopeIsCurrent(scope)) await loadEnvironmentMemory({scope});
  });
});
elements.environmentDetailSubviewTabs.addEventListener('click', (event) => {
  const button = event.target.closest('button[data-environment-detail-subview]'); if (!button) return;
  environmentDetailSubview = button.dataset.environmentDetailSubview || 'summary'; syncEnvironmentDetailSubviewUI();
});
elements.environmentDetailRoutes.addEventListener('click', (event) => {
  const button = event.target.closest('button[data-detail-route]'); if (!button) return;
  const route = button.dataset.detailRoute; closeEnvironmentDetail(); managementNavigation?.navigate(route, {focus: true});
});

elements.loadGlobalMemory.addEventListener('click', loadGlobalMemory); elements.loadEnvironmentMemory.addEventListener('click', loadEnvironmentMemory); elements.closeEnvironmentDetail.addEventListener('click', closeEnvironmentDetail); elements.environmentDetailBackdrop.addEventListener('click', closeEnvironmentDetail);
elements.worktreeSettingsForm?.addEventListener('submit', async (event) => {
  event.preventDefault();
  const root = elements.worktreeRoot.value.trim(), branchPrefix = elements.worktreeBranchPrefix.value.trim();
  await runMutation('更新 Worktree 设置', () => desktopAdapter().UpdateWorktreeSettings(root, branchPrefix));
});
elements.hostEnvironmentRefreshButton?.addEventListener('click', async () => {
  if (elements.hostEnvironmentRefreshButton.disabled) return;
  elements.hostEnvironmentRefreshButton.disabled = true; setStatus('正在刷新环境变量…', 'loading');
  try {
    await desktopAdapter().RefreshHostEnvironment();
    await refreshSnapshot('环境变量已刷新；现有 MCP session 已失效，后续运行使用新快照');
  } catch (error) {
    try { await refreshSnapshot('环境变量刷新失败'); } catch (_) {}
    setStatus(`环境变量刷新失败：${error?.message || String(error)}`, 'error');
  } finally { elements.hostEnvironmentRefreshButton.disabled = false; }
});
elements.launchAtLogin.addEventListener('change', updateLaunchAtLogin);
window.addEventListener('focus', () => { loadDesktopPreferences(false); });
window.addEventListener('keydown', (event) => { if (event.key !== 'Escape' || activeEditorDialog()) return; if (selectedEnvironmentID) closeEnvironmentDetail(); else if (editingMCPID) resetMCPEditor(true); });
elements.gatewayAccessSaveHosts.addEventListener('click', saveGatewayAllowedHosts);
elements.gatewayAccessSetAdminKey.addEventListener('click', setGatewayAdminAPIKey);
elements.gatewayAccessClearAdminKey.addEventListener('click', clearGatewayAdminAPIKey);
elements.gatewayAccessSetAgentKey.addEventListener('click', setGatewayAgentAPIKey);
elements.gatewayAccessClearAgentKey.addEventListener('click', clearGatewayAgentAPIKey);
elements.execAuthorizationSave.addEventListener('click', saveExecAuthorizationMode);
elements.cleanupExpiredTemporaryEnvironmentsButton.addEventListener('click', cleanupExpiredTemporaryEnvironments);
elements.gatewayRefreshButton.addEventListener('click', () => refreshConnectedADM(true).catch((error) => { clearManagementData(); setStatus(`ADM 连接检查失败：${error?.message || String(error)}`, 'error'); }));
elements.gatewayStartButton.addEventListener('click', () => runGatewayAction('启动本地 ADM', (input) => desktopAdapter().StartLocalADM(input)));
elements.gatewayStopButton.addEventListener('click', () => runGatewayAction('停止本地 ADM', (input) => desktopAdapter().StopLocalADM(input)));
elements.refreshButton.addEventListener('click', () => refreshConnectedADM(false).catch((error) => { clearManagementData(); setStatus(`ADM 连接检查失败：${error?.message || String(error)}`, 'error'); }));
window.addEventListener('DOMContentLoaded', () => { window.runtime?.EventsOn?.('desktop:preferences-changed', () => loadDesktopPreferences(false)); initializeManagementNavigation(); initializeConnectionProfiles(); syncMCPTransportForm(); clearManagementData(); loadDesktopPreferences(false); loadAboutInfo(); });
