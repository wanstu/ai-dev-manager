package main

import (
	"bytes"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/desktop"
	productversion "ai-dev-manager-v2/internal/version"
)

func TestDesktopTitleUsesSharedProductVersion(t *testing.T) {
	before := productversion.Version
	t.Cleanup(func() { productversion.Version = before })
	productversion.Version = "v9.8.7"
	if got := desktopTitle(); got != "adm-desktop — v9.8.7" {
		t.Fatalf("desktopTitle()=%q", got)
	}
}

func TestDesktopClientAdapterStartsDisconnected(t *testing.T) {
	adapter := desktop.NewClientAdapter()
	if _, err := adapter.GetSnapshot(); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("disconnected Desktop snapshot error = %v", err)
	}
}

func TestProductionDesktopUsesDisconnectedAdminMCPClient(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "desktop.NewClientAdapter()") {
		t.Fatal("production Desktop must construct the Admin MCP client adapter")
	}
	if strings.Contains(text, "newDesktopAdapter(statePath)") || strings.Contains(text, "management.New(application)") {
		t.Fatal("production Desktop must not silently construct a direct state-file management adapter")
	}
}

func TestProductionDesktopUsesTrayLifecycleAndSingleInstance(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{"newTrayManager", "StartHidden", "HideWindowOnClose", "OnDomReady", "SingleInstanceLock", "OnSecondInstanceLaunch", "--autostart", "trayIcon"} {
		if !strings.Contains(text, required) {
			t.Fatalf("production Desktop missing lifecycle marker %q", required)
		}
	}
	traySource, err := os.ReadFile("tray_manager_supported.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"github.com/gogpu/systray", "runtime.LockOSThread", "systray.New", "AddCheckbox", "显示主窗口", "隐藏主窗口", "开机启动", "退出", "OnClick", "OnDoubleClick", "OnRightClick", "tray.Run"} {
		if !strings.Contains(string(traySource), required) {
			t.Fatalf("cross-platform tray manager missing %q", required)
		}
	}
}

func TestGatewayChildRejectsPositionalArgumentsBeforeStartingServer(t *testing.T) {
	if err := runGatewayChild([]string{"unexpected"}); err == nil || !strings.Contains(err.Error(), "only --listen") {
		t.Fatalf("unexpected gateway child error: %v", err)
	}
}

func TestDesktopLaunchOptionsRecogniseAutostart(t *testing.T) {
	hidden, err := desktopLaunchOptions([]string{"--autostart"})
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Fatal("--autostart must start the Desktop hidden")
	}
	if _, err := desktopLaunchOptions([]string{"unexpected"}); err == nil || !strings.Contains(err.Error(), "only --autostart") {
		t.Fatalf("unexpected Desktop launch option error: %v", err)
	}
}

func TestWailsProjectConfigLivesWithDesktopCommand(t *testing.T) {
	config, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read desktop wails.json: %v", err)
	}
	text := string(config)
	for _, required := range []string{`"name": "adm-desktop"`, `"outputfilename": "adm-desktop"`, `"frontend:dir": "frontend"`, `"frontend:build": "go run ../../../tools/prepare-desktop-assets"`, `"wailsjsdir": "frontend/wailsjs"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("desktop wails.json missing %s", required)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", "wails.json")); !os.IsNotExist(err) {
		t.Fatalf("root wails.json must not exist; Wails must run from the desktop command directory: %v", err)
	}
}

func TestDesktopIconAssetsAreWired(t *testing.T) {
	for _, source := range []string{
		filepath.Join("..", "..", "assets", "icons", "ai-dev-manager-app.png"),
		filepath.Join("..", "..", "assets", "icons", "ai-dev-manager-window.png"),
		filepath.Join("..", "..", "assets", "icons", "ai-dev-manager-tray.png"),
	} {
		if info, err := os.Stat(source); err != nil || info.Size() == 0 {
			t.Fatalf("missing Desktop icon source %s: %v", source, err)
		}
	}

	tray, err := os.ReadFile(filepath.Join("assets", "tray.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tray) < 8 || !bytes.Equal(tray[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		t.Fatal("embedded tray icon is not a PNG asset")
	}
	sourceTray, err := os.ReadFile(filepath.Join("..", "..", "assets", "icons", "ai-dev-manager-tray.png"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(tray, sourceTray) {
		t.Fatal("embedded tray icon must be the fitted tray asset, not the padded source PNG")
	}
	if fill := pngVisibleFill(t, tray); fill < 0.90 {
		t.Fatalf("fitted tray icon visible bounds fill %.1f%%; want at least 90%%", fill*100)
	}
	if sourceFill, fittedFill := pngVisibleFill(t, sourceTray), pngVisibleFill(t, tray); fittedFill <= sourceFill {
		t.Fatalf("fitted tray icon must improve visible fill: source %.1f%% fitted %.1f%%", sourceFill*100, fittedFill*100)
	}

	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	brand, err := fs.ReadFile(assets, "assets/ai-dev-manager-window.png")
	if err != nil {
		t.Fatal(err)
	}
	sourceBrand, err := os.ReadFile(filepath.Join("..", "..", "assets", "icons", "ai-dev-manager-window.png"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(brand, sourceBrand) {
		t.Fatal("embedded Desktop brand mark must match assets/icons/ai-dev-manager-window.png")
	}

	prepareScript, err := os.ReadFile(filepath.Join("..", "..", "scripts", "prepare-desktop-icons.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"Export-FittedTransparentPng", "ai-dev-manager-app.png", "ai-dev-manager-window.png", "ai-dev-manager-tray.png", "assets\\tray.png", "appicon.png", "windows\\icon.ico", "Remove-Item"} {
		if !strings.Contains(string(prepareScript), required) {
			t.Fatalf("Desktop icon preparation script missing marker %q", required)
		}
	}
}

func pngVisibleFill(t *testing.T, data []byte) float64 {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X-1, bounds.Min.Y-1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha > 0x0800 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		t.Fatal("PNG has no visible pixels")
	}
	width := maxX - minX + 1
	height := maxY - minY + 1
	canvas := bounds.Dx()
	if bounds.Dy() > canvas {
		canvas = bounds.Dy()
	}
	fill := width
	if height > fill {
		fill = height
	}
	return float64(fill) / float64(canvas)
}

func TestEmbeddedFrontendUsesDesktopManagementAndGatewayBindings(t *testing.T) {
	assets, err := frontendAssets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"workspaceCount", "environmentCount", "execCount", "mcpCount", "skillCount", "memoryCount", "refreshButton", "launchAtLogin", "desktopShellHint", "app-brand-mark", "ai-dev-manager-window.png", "navigation.js", "dashboard.js", "project-pages.js", "runtime-view.js", "skill-bulk.js", "dashboardDataState", "dashboardLastSuccess", "management-sidebar", "data-management-page=\"overview\"", "data-route-link=\"settings\"",
		"gatewayState", "gatewayBaseURL", "gatewayHealthURL", "gatewayURL", "gatewayAdminURL", "gatewayRefreshButton", "gatewayStartButton", "gatewayStopButton", "gatewayHostState", "gatewayAdminKeyState", "gatewayAdminKeyHint", "gatewayAgentKeyState", "gatewayAgentKeyHint", "connectionAPIKeyState",
		"workspaceForm", "workspaceFilter", "workspaceVisibleCount", "workspaceListTotalCount", "connectionStartOnDesktopLaunch", "environmentForm", "environmentFilter", "environmentWorkspaceFilter", "environmentVisibleCount", "environmentListTotalCount", "environmentFilterHint", "environmentDetailPanel", "environmentDetailRoutes", "aria-modal",
		"execAllowedCount", "execBlockedCount", "execBlockedList", "execClearAllBlockedButton", "Authorization observations", "execForm", "managementEnvironment", "mcpEditorFlow", "mcpEditorSummary", "mcpEditorHint", "mcpForm", "mcpTransport", "mcpEndpoint", "mcpExecutable", "mcpReconnectInterval", "mcpEditCancelButton", "mcpSubmitButton", "mcpImportForm", "mcpImportApplyButton", "generic-mcpservers", "mcpFilter", "mcpStateFilter", "mcpVisibleCount", "mcpSetVisibleDefaultButton", "mcpUnsetVisibleDefaultButton", "mcpEnableVisibleButton", "mcpDisableVisibleButton", "mcpBulkHint", "status-legend", "skillSourceForm", "skillSourceID", "skillSourceRoot", "skillSupportRoots", "skillSourceSubmitButton", "skillSubviewTabs", "skillSubviewSkillCount", "skillSubviewSourceCount", "skillsPanel", "skillSourcesPanel", "skillSourceList", "skillList", "skillFilter", "skillSourceFilter", "skillStateFilter", "skillSourceFilterInput", "skillSourceVisibleCount", "skillSourceListTotalCount", "skillVisibleCount", "skillProbeAllButton", "skillSelectVisibleButton", "skillSetVisibleDefaultButton", "skillUnsetVisibleDefaultButton", "skillEnableVisibleButton", "skillDisableVisibleButton", "skillSelectedCount", "skillDeleteSelectedButton", "skillClearUnavailableButton", "skillBulkHint", `value="unselected"`, `正则用 /pattern/i`, "loadGlobalMemory", "globalMemoryForm", "writeEnvironmentMemoryButton", "environmentMemoryScopeHint", "不自动注入 Agent",
		"environmentMCPSelections", "environmentSkillSelections", "environmentDetailSubviewTabs", "environmentDiagnostics", "environmentDiagnosticsPage", "diagnosticsPageHint", "diagnosticsPageContent", "diagnostics", "loadEnvironmentMemory", "environmentMemoryForm",
		"runtimeRefreshButton", "runtimeHint", "runtimeWriterOwnerInput", "runtimeWriterAcquireButton", "runtimeWriterReleaseButton", "runtimeWriterHint", "runtimeSubviewTabs", "runtimeVerifierCount", "runtimeProcessCount", "runtimeRunCount", "verifierList", "processList", "runList", "runtimeOutputMeta", "runtimeOutput", "loggingStatusSummary",
	} {
		if !strings.Contains(string(index), required) {
			t.Fatalf("desktop index missing %q", required)
		}
	}
	javascript, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"window.go?.desktop?.Adapter", "GetSnapshot", "refreshSnapshot",
		"GetDesktopPreferences", "SetLaunchAtLogin", "loadDesktopPreferences", "updateLaunchAtLogin",
		"ConnectADM", "StartLocalADM", "StopLocalADM", "refreshConnectedADM", "initializeConnectionProfiles",
		"AddWorkspace", "RenameWorkspace", "RemoveWorkspace",
		"CreateEnvironment", "RenameEnvironment", "RemoveEnvironment", "InspectEnvironment", "GetTemporaryEnvironmentStatus", "PromoteTemporaryEnvironment", "CleanupTemporaryEnvironment",
		"AllowExecutable", "RemoveExecutable", "ClearExecDenial", "ClearAllExecDenials", "allow-blocked-executable", "clear-blocked-executable", "exec_denials",
		"AddMCP", "UpdateMCP", "PreviewMCPImport", "ApplyMCPImport", "ProbeMCPHealth", "currentMCPImportInput", "mcpImportFingerprint", "invalidateMCPImportPreview", "currentPendingMCPImport", "SetMCPDefault", "RemoveMCP", "beginMCPEdit", "resetMCPEditor", "mcpReconnectInterval", "mcpReferenceVariableNames", "配置引用：", "配置 ·", "全局探测 ·", "尚未探测", "可用性 ·", "mcpStateFilter", "mcpVisibleCount", "reference-text", "reference_name", "没有符合当前筛选条件的 MCP",
		"AddSkillSource", "UpdateSkillSource", "ListSkillSources", "RefreshSkillSource", "RemoveSkillSource", "ListSkillAvailability", "ListEnvironmentSkills", "SetSkillDefault", "RemoveSkill", "setSkillSubview", "renderSkillSourceFilterOptions", "skillSourceDisplayName", "appendSkillFact", "resource-facts", "全局可用性", "searchMatcher", "visibleResourceIDs", "runVisibleBatch", "mcpSetVisibleDefaultButton", "skillSetVisibleDefaultButton", "skillStateFilter", "skillVisibleCount", "没有符合当前筛选条件的 Skill", "endpoint", "artifact_path", "source_root", "unconfigured",
		"SetEnvironmentMCP", "SetEnvironmentSkill",
		"ListGlobalMemory", "WriteGlobalMemory", "DeleteGlobalMemory",
		"currentEnvironmentMemoryScope", "environmentMemoryScopeIsCurrent", "resetEnvironmentMemoryScope", "renderEnvironmentMemoryScope", "ListEnvironmentMemory", "WriteEnvironmentMemory", "DeleteEnvironmentMemory",
		"AcquireRuntimeWriter", "ReleaseRuntimeWriter", "ListVerifiers", "RunVerifier", "ListProcesses", "GetProcessLogs", "StopProcess", "ListRuns", "CancelRun", "refreshRuntimeContext", "GetLoggingStatus", "refreshLoggingStatus",
		"正在显式读取 Global Memory", "正在显式读取 Environment-private Memory",
		"environmentDetailBackdrop", "statusPanel.hidden = false", "statusPanel.hidden = true", "Lifecycle retention", "preview-temporary-environment-cleanup", "execute-temporary-environment-cleanup", "没有 force 路径",
		"只移除 ADM Workspace 记录，不删除目录", "只移除 ADM Environment 记录，不删除 root 或项目文件",
		"这是全局删除，不是只从当前 Environment 禁用", "删除全局 Skill source", "预览不会修改 catalog 或 Environment", "renderDiagnosticsPage", "renderEnvironmentDiagnostics", "这里解释当前已返回的 capability facts", "不会自动执行命令、MCP 探测、Verifier 或读取 Memory", "environmentContextRoutes",
	} {
		if !strings.Contains(string(javascript), required) {
			t.Fatalf("desktop app.js missing %q", required)
		}
	}
	if strings.Contains(string(javascript), "stateBadge(config") {
		t.Fatal("desktop app.js still contains the undefined MCP config badge variable")
	}
	connections, err := fs.ReadFile(assets, "connections.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"GetConnectionProfiles", "SaveConnectionProfile", "SelectConnectionProfile", "DeleteConnectionProfile", "DisconnectADM", "withConnectionTransition", "desktopStartup", "start_service_on_desktop_launch", "connectionStartOnDesktopLaunch", "desktopPendingRequests", "desktopRequestQueue"} {
		if !strings.Contains(string(connections), required) {
			t.Fatalf("desktop connections.js missing %q", required)
		}
	}
	navigation, err := fs.ReadFile(assets, "navigation.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"overview", "workspaces", "environments", "runtime", "mcp", "skills", "memory", "gateway", "exec-allowlist", "settings", "aria-current", "popstate", "hashchange"} {
		if !strings.Contains(string(navigation), required) {
			t.Fatalf("desktop navigation.js missing %q", required)
		}
	}
	dashboard, err := fs.ReadFile(assets, "dashboard.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"dashboardModel", "renderDashboard", "workspaceCount", "memoryCount", "global_memory_count", "未加载", "数据过期"} {
		if !strings.Contains(string(dashboard), required) {
			t.Fatalf("desktop dashboard.js missing %q", required)
		}
	}
	projectPages, err := fs.ReadFile(assets, "project-pages.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"workspaceListModel", "environmentListModel", "workspaceEnvironmentCounts", "workspaceFilterValid"} {
		if !strings.Contains(string(projectPages), required) {
			t.Fatalf("desktop project-pages.js missing %q", required)
		}
	}
	runtimeView, err := fs.ReadFile(assets, "runtime-view.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"SUBVIEWS", "makeOutputBinding", "outputBindingMatches", "resourceStillKnown", "truncationSummary"} {
		if !strings.Contains(string(runtimeView), required) {
			t.Fatalf("desktop runtime-view.js missing %q", required)
		}
	}
	skillBulk, err := fs.ReadFile(assets, "skill-bulk.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"CLEANUP_STATES", "disabled", "catalogFingerprint", "summarizeAvailability", "retainExistingSelection", "probeMatches", "scope"} {
		if !strings.Contains(string(skillBulk), required) {
			t.Fatalf("desktop skill-bulk.js missing %q", required)
		}
	}
	dialogs, err := fs.ReadFile(assets, "dialogs.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"showModal", "preventScroll", "data-dialog-open", "data-dialog-close", "activeEditorDialog", "skillSourceDialog", "resetSkillSourceEditor"} {
		if !strings.Contains(string(dialogs), required) {
			t.Fatalf("desktop dialogs.js missing %q", required)
		}
	}
	styles, err := fs.ReadFile(assets, "styles.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{".topbar-brand", ".app-brand-mark", ".topbar-actions", ".desktop-toggle", ".management-layout", ".management-sidebar", ".nav-link[aria-current", "[data-management-page][hidden]", ".list-toolbar", ".project-filter-hint", ".filtered-project-list", ".managed-item.current-context", ".project-path", ".detail-group", ".detail-route-actions", ".runtime-subview-tabs", ".runtime-subview-panel", ".runtime-output-panel", ".status-legend", ".editor-hint", ".filtered-resource-list", ".resource-row[data-editing", ".resource-actions .check-field", ".editor-dialog", ".dialog-message"} {
		if !strings.Contains(string(styles), required) {
			t.Fatalf("desktop styles.css missing %q", required)
		}
	}
	if strings.Contains(string(styles), "鈥?") {
		t.Fatal("desktop styles.css contains the previously broken manager-flow marker encoding")
	}
}
