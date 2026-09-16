package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
	productversion "ai-dev-manager-v2/internal/version"
)

const defaultGatewayListen = gateway.DefaultHTTPListen

type gatewayHealth struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	Transport string `json:"transport"`
	OwnerID   string `json:"owner_id,omitempty"`
}

type incompatibleGatewayError struct{ detail string }

func (e *incompatibleGatewayError) Error() string { return e.detail }

var (
	matchesADMExecutable = sameADMExecutable
	startGatewayDetached = startDetachedHTTPGateway
)

func resolveMCPImportContent(inline string, inlineSet bool, filePath string, fileSet bool, useStdin bool, stdin io.Reader, stdinInteractive bool) (string, error) {
	sources := 0
	if inlineSet {
		sources++
	}
	if fileSet {
		sources++
	}
	if useStdin {
		sources++
	}
	if sources != 1 {
		return "", fmt.Errorf("exactly one MCP import content source is required: --json-or-jsonc, --file, or --stdin")
	}

	var content []byte
	var err error
	switch {
	case inlineSet:
		content = []byte(inline)
	case fileSet:
		path := strings.TrimSpace(filePath)
		if path == "" {
			return "", fmt.Errorf("--file requires a non-empty path")
		}
		content, err = os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read MCP import file %q: %w", path, err)
		}
	case useStdin:
		if stdinInteractive {
			return "", fmt.Errorf("--stdin requires redirected or piped input; interactive stdin is not read")
		}
		content, err = io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read MCP import stdin: %w", err)
		}
	}
	if strings.TrimSpace(string(content)) == "" {
		return "", fmt.Errorf("MCP import content is empty")
	}
	return string(content), nil
}

func stdinIsInteractive() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Run executes the ADM CLI surface for the provided command-line arguments.
func Run(args []string) error {
	return run(args)
}

func run(args []string) error {
	baseURL, args, err := parseADMTarget(args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(os.Stdout, productversion.Current())
		return nil
	case "workspace", "environment", "env", "exec", "mcp", "skill", "memory":
		return withCLIAdmin(baseURL, func(backend cliManagementBackend) error {
			switch args[0] {
			case "workspace":
				return runWorkspace(backend, args[1:])
			case "environment", "env":
				return runEnvironment(backend, args[1:])
			case "exec":
				return runExec(backend, args[1:])
			case "mcp":
				return runCatalog("mcp", backend, nil, args[1:])
			case "skill":
				return runCatalog("skill", backend, nil, args[1:])
			default:
				return runMemory(backend, args[1:])
			}
		})
	case "gateway", "doctor", "state":
		statePath, err := store.DefaultPath()
		if err != nil {
			return err
		}
		service := app.New(statePath)
		switch args[0] {
		case "gateway":
			return runGatewayForTarget(service, baseURL, args[1:])
		case "doctor":
			return runDoctor(service, statePath, args[1:])
		default:
			return runState(statePath, args[1:])
		}
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q; run adm -h for help", args[0])
	}
}

func runWorkspace(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printWorkspaceHelp()
		return nil
	}
	switch args[0] {
	case "add":
		fs := newFlagSet("workspace add", func() {
			fmt.Fprintln(os.Stdout, "用法：adm workspace add --path PATH [--name NAME]")
			fmt.Fprintln(os.Stdout, "\n登记一个现有本地目录为 Workspace；不要求 Git。")
		})
		path := fs.String("path", "", "Workspace 目录路径")
		name := fs.String("name", "", "Workspace 显示名称")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*path) == "" {
			return fmt.Errorf("缺少 --path；运行 adm workspace add -h 查看帮助")
		}
		ws, err := service.WorkspaceAdd(*path, *name)
		if err != nil {
			return err
		}
		return writeJSON(ws)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("workspace list 不接受参数")
		}
		items, err := service.WorkspaceList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "inspect":
		fs := newFlagSet("workspace inspect", func() {
			fmt.Fprintln(os.Stdout, "用法：adm workspace inspect --workspace-id WS_ID")
		})
		workspaceID := fs.String("workspace-id", "", "Workspace ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" {
			return fmt.Errorf("缺少 --workspace-id；运行 adm workspace inspect -h 查看帮助")
		}
		ws, err := service.WorkspaceInspect(*workspaceID)
		if err != nil {
			return err
		}
		return writeJSON(ws)
	case "discover":
		fs := newFlagSet("workspace discover", func() {
			fmt.Fprintln(os.Stdout, "用法：adm workspace discover --workspace-id WS_ID [--path REL] [--query TEXT] [预算选项]")
			fmt.Fprintln(os.Stdout, "\n显式执行有界 metadata-only 项目发现；0 值预算交由 Core 使用默认值，不读取文件内容，也不创建 Environment。")
		})
		workspaceID := fs.String("workspace-id", "", "Workspace ID")
		path := fs.String("path", "", "Workspace 内可选相对扫描路径")
		query := fs.String("query", "", "可选 literal 候选路径/名称查询")
		maxDepth := fs.Int("max-depth", 0, "最大目录深度；0 使用 Core 默认值")
		maxEntries := fs.Int("max-entries", 0, "最多访问的目录项；0 使用 Core 默认值")
		maxCandidates := fs.Int("max-candidates", 0, "最多返回的项目候选；0 使用 Core 默认值")
		maxDigestEntries := fs.Int("max-digest-entries", 0, "最多返回的目录摘要项；0 使用 Core 默认值")
		maxOutputBytes := fs.Int("max-output-bytes", 0, "最大 JSON 输出预算；0 使用 Core 默认值")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" {
			return fmt.Errorf("缺少 --workspace-id；运行 adm workspace discover -h 查看帮助")
		}
		discovery, ok := service.(cliWorkspaceDiscoveryBackend)
		if !ok {
			return fmt.Errorf("workspace discovery requires the connected Admin MCP backend")
		}
		report, err := discovery.WorkspaceDiscover(strings.TrimSpace(*workspaceID), model.DiscoveryRequest{
			Path: *path, Query: *query, MaxDepth: *maxDepth, MaxEntries: *maxEntries,
			MaxCandidates: *maxCandidates, MaxDigestEntries: *maxDigestEntries, MaxOutputBytes: *maxOutputBytes,
		})
		if err != nil {
			return err
		}
		return writeJSON(report)
	case "mcp", "skill":
		return runWorkspaceSelection(args[0], service, args[1:])
	case "worktree-settings":
		return runWorktreeSettings(service, args[1:])
	case "rename":
		fs := newFlagSet("workspace rename", func() {
			fmt.Fprintln(os.Stdout, "用法：adm workspace rename --workspace-id WS_ID --name NAME")
			fmt.Fprintln(os.Stdout, "\n只修改 ADM 中的显示名称，不移动或重命名项目目录。")
		})
		workspaceID := fs.String("workspace-id", "", "Workspace ID")
		name := fs.String("name", "", "新的 Workspace 显示名称")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" || strings.TrimSpace(*name) == "" {
			return fmt.Errorf("必须提供 --workspace-id 和 --name；运行 adm workspace rename -h 查看帮助")
		}
		ws, err := service.WorkspaceRename(*workspaceID, *name)
		if err != nil {
			return err
		}
		return writeJSON(ws)
	case "remove":
		fs := newFlagSet("workspace remove", func() {
			fmt.Fprintln(os.Stdout, "用法：adm workspace remove --workspace-id WS_ID")
			fmt.Fprintln(os.Stdout, "\n只删除 ADM 中的 Workspace 记录，不会删除项目目录或文件；仍有 Environment 引用时禁止删除。")
		})
		workspaceID := fs.String("workspace-id", "", "Workspace ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" {
			return fmt.Errorf("缺少 --workspace-id；运行 adm workspace remove -h 查看帮助")
		}
		removed, err := service.WorkspaceRemove(*workspaceID)
		if err != nil {
			return err
		}
		return writeJSON(map[string]any{"removed": removed})
	default:
		return fmt.Errorf("未知 workspace 命令 %q；运行 adm workspace -h 查看帮助", args[0])
	}
}

func runWorkspaceSelection(kind string, service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintf(os.Stdout, "用法：adm workspace %s enable|disable --workspace-id WS_ID --%s-id ID\n\nWorkspace 开启的 %s 会被该 Workspace 下所有 Environment 继承；Environment 显式选择是附加层，不能覆盖关闭 Workspace 继承。\n", kind, kind, strings.ToUpper(kind))
		return nil
	}
	action := args[0]
	if action != "enable" && action != "disable" {
		return fmt.Errorf("未知 workspace %s 命令 %q；运行 adm workspace %s -h 查看帮助", kind, action, kind)
	}
	fs := newFlagSet("workspace "+kind+" "+action, func() {
		fmt.Fprintf(os.Stdout, "用法：adm workspace %s %s --workspace-id WS_ID --%s-id ID\n", kind, action, kind)
	})
	workspaceID := fs.String("workspace-id", "", "Workspace ID")
	entryID := fs.String(kind+"-id", "", strings.ToUpper(kind)+" catalog ID")
	if err := fs.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" || strings.TrimSpace(*entryID) == "" {
		return fmt.Errorf("必须提供 --workspace-id 和 --%s-id；运行 adm workspace %s %s -h 查看帮助", kind, kind, action)
	}
	enabled := action == "enable"
	if kind == "mcp" {
		workspace, err := service.WorkspaceMCPSet(strings.TrimSpace(*workspaceID), strings.TrimSpace(*entryID), enabled)
		if err != nil {
			return err
		}
		return writeJSON(workspace)
	}
	workspace, err := service.WorkspaceSkillSet(strings.TrimSpace(*workspaceID), strings.TrimSpace(*entryID), enabled)
	if err != nil {
		return err
	}
	return writeJSON(workspace)
}

func runWorktreeSettings(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, "Usage: adm workspace worktree-settings show|set [--root PATH] [--branch-prefix PREFIX]")
		return nil
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("workspace worktree-settings show does not accept arguments")
		}
		settings, err := service.WorktreeSettings()
		if err != nil {
			return err
		}
		return writeJSON(settings)
	case "set":
		fs := newFlagSet("workspace worktree-settings set", func() {
			fmt.Fprintln(os.Stdout, "Usage: adm workspace worktree-settings set [--root PATH] [--branch-prefix PREFIX]")
		})
		root := fs.String("root", "", "managed worktree storage directory; empty uses ADM default")
		branchPrefix := fs.String("branch-prefix", "", "managed worktree branch prefix; empty uses adm/")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("workspace worktree-settings set does not accept positional arguments")
		}
		settings, err := service.UpdateWorktreeSettings(*root, *branchPrefix)
		if err != nil {
			return err
		}
		return writeJSON(settings)
	default:
		return fmt.Errorf("unknown workspace worktree-settings command %q", args[0])
	}
}

func runHostEnvironment(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, "Usage: adm environment host-env status|refresh")
		fmt.Fprintln(os.Stdout, "Refresh never returns variable values; on Windows it re-reads machine/user environment sources.")
		return nil
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("environment host-env status does not accept arguments")
		}
		status, err := service.HostEnvironmentStatus()
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "refresh":
		if len(args) != 1 {
			return fmt.Errorf("environment host-env refresh does not accept arguments")
		}
		status, err := service.RefreshHostEnvironment()
		if err != nil {
			return err
		}
		return writeJSON(status)
	default:
		return fmt.Errorf("unknown environment host-env command %q", args[0])
	}
}

func runEnvironment(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printEnvironmentHelp()
		return nil
	}
	switch args[0] {
	case "create":
		fs := newFlagSet("environment create", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment create --workspace-id WS_ID --name NAME [--root PATH]")
			fmt.Fprintln(os.Stdout, "\n创建持久开发上下文；不写 --root 时默认使用整个 Workspace。")
		})
		workspaceID := fs.String("workspace-id", "", "Workspace ID")
		name := fs.String("name", "", "Environment 名称")
		root := fs.String("root", "", "Workspace 内的可选根目录；默认使用整个 Workspace")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*workspaceID) == "" || strings.TrimSpace(*name) == "" {
			return fmt.Errorf("必须提供 --workspace-id 和 --name；运行 adm environment create -h 查看帮助")
		}
		env, err := service.EnvironmentCreate(*workspaceID, *name, *root)
		if err != nil {
			return err
		}
		return writeJSON(env)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("environment list 不接受参数")
		}
		items, err := service.EnvironmentList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "inspect":
		fs := newFlagSet("environment inspect", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment inspect --environment-id ENV_ID")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment inspect -h 查看帮助")
		}
		info, err := service.EnvironmentInspect(*environmentID)
		if err != nil {
			return err
		}
		return writeJSON(info)
	case "capability-report", "capabilities":
		fs := newFlagSet("environment capability-report", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment capability-report --environment-id ENV_ID")
			fmt.Fprintln(os.Stdout, "\n输出 canonical CapabilityReport。正常 CLI 通过 Admin MCP 使用 Gateway environment_capability_report；有 runtime owner 时可包含现有 owner-local 观察，但不会主动 reconnect、probe、调用工具或运行 verifier。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment capability-report -h 查看帮助")
		}
		report, err := service.CapabilityReport(*environmentID)
		if err != nil {
			return err
		}
		return writeJSON(report)
	case "context":
		fs := newFlagSet("environment context", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment context --environment-id ENV_ID [--path REL] [--max-depth N --max-entries N --max-digest-entries N --max-output-bytes N]")
			fmt.Fprintln(os.Stdout, "\n通过 Admin MCP 返回 Phase-20 canonical bounded Environment context bundle。它不隐式选择 Environment，不读取 Memory 值/完整 Skill 内容，也不会主动 probe MCP 或执行 verifier/process/run。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		path := fs.String("path", "", "Environment 内可选相对路径")
		maxDepth := fs.Int("max-depth", 0, "最大目录深度；0 使用 Core 默认值")
		maxEntries := fs.Int("max-entries", 0, "最多访问的目录项；0 使用 Core 默认值")
		maxDigestEntries := fs.Int("max-digest-entries", 0, "最多返回的目录摘要项；0 使用 Core 默认值")
		maxOutputBytes := fs.Int("max-output-bytes", 0, "最大 JSON 输出预算；0 使用 Core 默认值")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment context -h 查看帮助")
		}
		backend, ok := service.(cliEnvironmentAgentBackend)
		if !ok {
			return fmt.Errorf("environment context requires the connected Admin MCP backend")
		}
		bundle, err := backend.EnvironmentContext(strings.TrimSpace(*environmentID), model.EnvironmentContextRequest{
			Path: *path, MaxDepth: *maxDepth, MaxEntries: *maxEntries, MaxDigestEntries: *maxDigestEntries, MaxOutputBytes: *maxOutputBytes,
		})
		if err != nil {
			return err
		}
		return writeJSON(bundle)
	case "tree-digest":
		fs := newFlagSet("environment tree-digest", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment tree-digest --environment-id ENV_ID [--path REL] [预算选项]")
			fmt.Fprintln(os.Stdout, "\n显式读取 Runtime-authorized Environment root 下的有界目录 metadata 摘要；不会扩展到 Workspace sibling，也不要求 Writer。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		path := fs.String("path", "", "Environment 内可选相对扫描路径")
		maxDepth := fs.Int("max-depth", 0, "最大目录深度；0 使用 Core 默认值")
		maxEntries := fs.Int("max-entries", 0, "最多访问的目录项；0 使用 Core 默认值")
		maxCandidates := fs.Int("max-candidates", 0, "最多返回的项目候选；0 使用 Core 默认值")
		maxDigestEntries := fs.Int("max-digest-entries", 0, "最多返回的目录摘要项；0 使用 Core 默认值")
		maxOutputBytes := fs.Int("max-output-bytes", 0, "最大 JSON 输出预算；0 使用 Core 默认值")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment tree-digest -h 查看帮助")
		}
		digest, ok := service.(cliEnvironmentTreeDigestBackend)
		if !ok {
			return fmt.Errorf("environment tree digest requires the connected Admin MCP backend")
		}
		report, err := digest.EnvironmentTreeDigest(strings.TrimSpace(*environmentID), model.DiscoveryRequest{
			Path: *path, MaxDepth: *maxDepth, MaxEntries: *maxEntries,
			MaxCandidates: *maxCandidates, MaxDigestEntries: *maxDigestEntries, MaxOutputBytes: *maxOutputBytes,
		})
		if err != nil {
			return err
		}
		return writeJSON(report)
	case "rename":
		fs := newFlagSet("environment rename", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment rename --environment-id ENV_ID --name NAME")
			fmt.Fprintln(os.Stdout, "\n只修改 ADM 中的 Environment 显示名称，不移动根目录、不修改选择或 Memory，也不触碰项目文件。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		name := fs.String("name", "", "新的 Environment 显示名称")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*name) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --name；运行 adm environment rename -h 查看帮助")
		}
		env, err := service.EnvironmentRename(*environmentID, *name)
		if err != nil {
			return err
		}
		return writeJSON(env)
	case "workspace-recommendations":
		if len(args) != 1 {
			return fmt.Errorf("environment workspace-recommendations 不接受参数")
		}
		items, err := service.EnvironmentWorkspaceRecommendations()
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "workspace-options":
		fs := newFlagSet("environment workspace-options", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment workspace-options --environment-id ENV_ID")
			fmt.Fprintln(os.Stdout, "\n列出包含当前 Environment root 的可选 Workspace，并按最具体到最宽泛排序；managed worktree 保留逻辑 source Workspace，不按物理存放目录重绑。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment workspace-options -h 查看帮助")
		}
		options, err := service.EnvironmentWorkspaceOptions(*environmentID)
		if err != nil {
			return err
		}
		return writeJSON(options)
	case "workspace-set":
		fs := newFlagSet("environment workspace-set", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment workspace-set --environment-id ENV_ID --workspace-id WS_ID")
			fmt.Fprintln(os.Stdout, "\n将普通 Environment 显式重绑到包含其 root 的非系统 Workspace。Environment explicit MCP/Skill 选择保留，Workspace 继承按新归属立即重算。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		workspaceID := fs.String("workspace-id", "", "目标 Workspace ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*workspaceID) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --workspace-id；运行 adm environment workspace-set -h 查看帮助")
		}
		env, err := service.EnvironmentWorkspaceSet(*environmentID, *workspaceID)
		if err != nil {
			return err
		}
		return writeJSON(env)
	case "remove":
		fs := newFlagSet("environment remove", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment remove --environment-id ENV_ID")
			fmt.Fprintln(os.Stdout, "\n只删除 ADM 中的 Environment 记录，不会删除根目录或项目文件；存在有效 Writer 时禁止删除。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment remove -h 查看帮助")
		}
		removed, err := service.EnvironmentRemoveResult(*environmentID)
		if err != nil {
			return err
		}
		return writeJSON(map[string]any{"removed": removed})
	case "temporary":
		return runEnvironmentTemporary(service, args[1:])
	case "host-env":
		return runHostEnvironment(service, args[1:])
	case "mcp", "skill":
		return runEnvironmentSelection(args[0], service, args[1:])
	case "verifier":
		return runEnvironmentVerifier(service, args[1:])
	case "writer":
		return runWriter(service, args[1:])
	default:
		return fmt.Errorf("未知 environment 命令 %q；运行 adm environment -h 查看帮助", args[0])
	}
}

func runEnvironmentTemporary(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printEnvironmentTemporaryHelp()
		return nil
	}
	backend, ok := service.(cliEnvironmentAgentBackend)
	if !ok {
		return fmt.Errorf("temporary Environment lifecycle requires the connected Admin MCP backend")
	}
	switch args[0] {
	case "create":
		fs := newFlagSet("environment temporary create", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment temporary create (--workspace-id WS_ID | --source-environment-id ENV_ID) --name NAME --owner-id OWNER --ttl-seconds N [--session-id ID] [--run-id ID] [--mode existing_root|managed_worktree] [--root PATH] [--base-ref REF]")
			fmt.Fprintln(os.Stdout, "\n创建一个新的 temporary Environment。existing_root 只接受 Workspace；managed_worktree 可从 Workspace 或已有 Git Environment 派生，ADM 只 fetch，不 pull/改写源 checkout。")
		})
		workspaceID := fs.String("workspace-id", "", "Workspace ID")
		sourceEnvironmentID := fs.String("source-environment-id", "", "managed_worktree 模式下已有 Git Environment ID")
		name := fs.String("name", "", "Environment 名称")
		ownerID := fs.String("owner-id", "", "稳定 lifecycle owner；不会自动推断")
		ttlSeconds := fs.Int64("ttl-seconds", 0, "正数 TTL 秒数")
		sessionID := fs.String("session-id", "", "可选 lifecycle provenance")
		runID := fs.String("run-id", "", "可选 provenance；不要求对应 Run 存在")
		mode := fs.String("mode", "", "existing_root 或 managed_worktree；空值使用服务端 existing_root 默认")
		root := fs.String("root", "", "existing_root 模式下 Workspace 内已存在目录")
		baseRef := fs.String("base-ref", "", "managed_worktree 模式下可选 Git ref")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		workspaceValue := strings.TrimSpace(*workspaceID)
		sourceEnvironmentValue := strings.TrimSpace(*sourceEnvironmentID)
		if fs.NArg() != 0 || (workspaceValue == "") == (sourceEnvironmentValue == "") || strings.TrimSpace(*name) == "" || strings.TrimSpace(*ownerID) == "" || *ttlSeconds <= 0 {
			return fmt.Errorf("必须且只能提供 --workspace-id 或 --source-environment-id 之一，并提供 --name、非空 --owner-id 和正数 --ttl-seconds；运行 adm environment temporary create -h 查看帮助")
		}
		created, err := backend.EnvironmentTemporaryCreate(model.TemporaryEnvironmentCreateRequest{
			WorkspaceID: workspaceValue, SourceEnvironmentID: sourceEnvironmentValue, Name: strings.TrimSpace(*name), OwnerID: strings.TrimSpace(*ownerID), TTLSeconds: *ttlSeconds,
			SessionID: strings.TrimSpace(*sessionID), RunID: strings.TrimSpace(*runID), Mode: strings.TrimSpace(*mode), Root: *root, BaseRef: *baseRef,
		})
		if err != nil {
			return err
		}
		return writeJSON(created)
	case "status":
		fs := newFlagSet("environment temporary status", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment temporary status --environment-id ENV_ID")
			fmt.Fprintln(os.Stdout, "\n只读返回 retention provenance、expiry 和当前 cleanup blockers；不需要 lifecycle owner，也不获取 Writer。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment temporary status -h 查看帮助")
		}
		status, err := backend.EnvironmentTemporaryStatus(strings.TrimSpace(*environmentID))
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "promote":
		fs := newFlagSet("environment temporary promote", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment temporary promote --environment-id ENV_ID --owner-id OWNER")
			fmt.Fprintln(os.Stdout, "\n要求匹配 lifecycle owner；只把 retention 改为 durable，不移动目录、不 merge/push Git，也不改变 Environment ID。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		ownerID := fs.String("owner-id", "", "匹配的 lifecycle owner")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*ownerID) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --owner-id；运行 adm environment temporary promote -h 查看帮助")
		}
		status, err := backend.EnvironmentTemporaryPromote(strings.TrimSpace(*environmentID), strings.TrimSpace(*ownerID))
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "cleanup":
		fs := newFlagSet("environment temporary cleanup", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment temporary cleanup --environment-id ENV_ID --owner-id OWNER [--execute]")
			fmt.Fprintln(os.Stdout, "\n默认只 preview。--execute 才执行服务端 fresh safety recheck 后的 targeted cleanup；没有 force 路径。ordinary existing_root cleanup 不删除项目目录或文件；managed worktree cleanup 保留生成分支。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		ownerID := fs.String("owner-id", "", "lifecycle owner；execute 时必须匹配")
		execute := fs.Bool("execute", false, "执行 targeted cleanup；默认 false 仅 preview")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*ownerID) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --owner-id；运行 adm environment temporary cleanup -h 查看帮助")
		}
		result, err := backend.EnvironmentTemporaryCleanup(strings.TrimSpace(*environmentID), strings.TrimSpace(*ownerID), *execute)
		if err != nil {
			return err
		}
		return writeJSON(result)
	default:
		return fmt.Errorf("未知 environment temporary 命令 %q；运行 adm environment temporary -h 查看帮助", args[0])
	}
}

func runEnvironmentVerifier(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printEnvironmentVerifierHelp()
		return nil
	}
	switch args[0] {
	case "add":
		fs := newFlagSet("environment verifier add", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment verifier add --environment-id ENV_ID --kind test|lint|build|custom --executable NAME_OR_PATH [--name NAME] [--arg ARG ...] [--cwd RELATIVE_PATH] [--timeout-seconds N] [--enabled=true|false]")
			fmt.Fprintln(os.Stdout, "\n只声明 Environment-scoped verifier；不会把 executable 加入执行白名单。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		name := fs.String("name", "", "可选的人类可读名称")
		kind := fs.String("kind", "", "verifier 类型：test、lint、build、custom")
		executable := fs.String("executable", "", "程序名或绝对路径；运行时仍必须在全局执行白名单中")
		cwd := fs.String("cwd", "", "Environment 内的相对工作目录；空值表示 Environment root")
		timeoutSeconds := fs.Int64("timeout-seconds", 0, "超时秒数；0 使用默认值")
		enabled := fs.Bool("enabled", true, "是否启用；默认 true")
		var verifierArgs []string
		fs.Func("arg", "传给 verifier 的一个参数；可重复", func(value string) error {
			verifierArgs = append(verifierArgs, value)
			return nil
		})
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*kind) == "" || strings.TrimSpace(*executable) == "" {
			return fmt.Errorf("必须提供 --environment-id、--kind 和 --executable；运行 adm environment verifier add -h 查看帮助")
		}
		definition, err := service.VerifierAdd(*environmentID, model.VerifierDefinition{
			Name:           *name,
			Kind:           *kind,
			Enabled:        *enabled,
			Executable:     *executable,
			Args:           verifierArgs,
			Cwd:            *cwd,
			TimeoutSeconds: *timeoutSeconds,
		})
		if err != nil {
			return err
		}
		return writeJSON(definition)
	case "list":
		fs := newFlagSet("environment verifier list", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment verifier list --environment-id ENV_ID")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm environment verifier list -h 查看帮助")
		}
		items, err := service.VerifierList(*environmentID)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "remove":
		fs := newFlagSet("environment verifier remove", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment verifier remove --environment-id ENV_ID --verifier-id VF_ID")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		verifierID := fs.String("verifier-id", "", "Verifier ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*verifierID) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --verifier-id；运行 adm environment verifier remove -h 查看帮助")
		}
		removed, err := service.VerifierRemove(*environmentID, *verifierID)
		if err != nil {
			return err
		}
		return writeJSON(map[string]any{"removed": removed})
	default:
		return fmt.Errorf("未知 environment verifier 命令 %q；运行 adm environment verifier -h 查看帮助", args[0])
	}
}

func runEnvironmentSelection(kind string, service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printEnvironmentSelectionHelp(kind)
		return nil
	}
	if kind != "mcp" && kind != "skill" {
		return fmt.Errorf("unsupported Environment selection kind %q", kind)
	}
	action := args[0]
	if kind == "skill" && action == "list" {
		fs := newFlagSet("environment skill list", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment skill list --environment-id ENV_ID")
			fmt.Fprintln(os.Stdout, "\n查看该 Environment 的 Skill enabled/disabled/broken availability；不会读取 SKILL.md 内容或刷新 source。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("必须提供 --environment-id；运行 adm environment skill list -h 查看帮助")
		}
		availabilityBackend, ok := service.(cliSkillAvailabilityBackend)
		if !ok {
			return fmt.Errorf("Environment Skill availability requires the connected Admin MCP backend")
		}
		items, err := availabilityBackend.EnvironmentSkillList(strings.TrimSpace(*environmentID))
		if err != nil {
			return err
		}
		return writeJSON(items)
	}
	if kind == "skill" && action == "inspect" {
		fs := newFlagSet("environment skill inspect", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID")
			fmt.Fprintln(os.Stdout, "\n查看一个 Skill 在指定 Environment 中的 availability；不会读取 Skill 指令/支持文件内容或执行 Skill。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		skillID := fs.String("skill-id", "", "Skill catalog ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*skillID) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --skill-id；运行 adm environment skill inspect -h 查看帮助")
		}
		availabilityBackend, ok := service.(cliSkillAvailabilityBackend)
		if !ok {
			return fmt.Errorf("Environment Skill availability requires the connected Admin MCP backend")
		}
		item, err := availabilityBackend.EnvironmentSkillInspect(strings.TrimSpace(*environmentID), strings.TrimSpace(*skillID))
		if err != nil {
			return err
		}
		return writeJSON(item)
	}
	if action != "enable" && action != "disable" {
		return fmt.Errorf("未知 environment %s 命令 %q；运行 adm environment %s -h 查看帮助", kind, action, kind)
	}
	fs := newFlagSet("environment "+kind+" "+action, func() {
		fmt.Fprintf(os.Stdout, "用法：adm environment %s %s --environment-id ENV_ID --%s-id ID\n", kind, action, kind)
	})
	environmentID := fs.String("environment-id", "", "Environment ID")
	entryID := fs.String(kind+"-id", "", strings.ToUpper(kind)+" catalog ID")
	if err := fs.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*entryID) == "" {
		return fmt.Errorf("必须提供 --environment-id 和 --%s-id；运行 adm environment %s %s -h 查看帮助", kind, kind, action)
	}
	enabled := action == "enable"
	if kind == "mcp" {
		env, err := service.EnvironmentMCPSet(*environmentID, *entryID, enabled)
		if err != nil {
			return err
		}
		return writeJSON(env)
	}
	env, err := service.EnvironmentSkillSet(*environmentID, *entryID, enabled)
	if err != nil {
		return err
	}
	return writeJSON(env)
}

func runWriter(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printWriterHelp()
		return nil
	}
	switch args[0] {
	case "acquire", "heartbeat":
		action := args[0]
		fs := newFlagSet("environment writer "+action, func() {
			fmt.Fprintf(os.Stdout, "用法：adm environment writer %s --environment-id ENV_ID --owner OWNER\n", action)
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		owner := fs.String("owner", "", "稳定的 Agent/会话 Writer 标识")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || strings.TrimSpace(*owner) == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --owner；运行 adm environment writer %s -h 查看帮助", action)
		}
		var (
			env any
			err error
		)
		if action == "acquire" {
			env, err = service.WriterAcquire(*environmentID, *owner)
		} else {
			env, err = service.WriterHeartbeat(*environmentID, *owner)
		}
		if err != nil {
			return err
		}
		return writeJSON(env)
	case "release":
		fs := newFlagSet("environment writer release", func() {
			fmt.Fprintln(os.Stdout, "用法：adm environment writer release --environment-id ENV_ID (--owner OWNER | --force)")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		owner := fs.String("owner", "", "当前 Writer owner")
		force := fs.Bool("force", false, "忽略 owner，强制释放 Writer")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || (!*force && strings.TrimSpace(*owner) == "") {
			return fmt.Errorf("必须提供 --environment-id，并提供 --owner 或 --force；运行 adm environment writer release -h 查看帮助")
		}
		env, err := service.WriterRelease(*environmentID, *owner, *force)
		if err != nil {
			return err
		}
		return writeJSON(env)
	default:
		return fmt.Errorf("未知 writer 命令 %q；运行 adm environment writer -h 查看帮助", args[0])
	}
}

func runExec(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printExecHelp()
		return nil
	}
	switch args[0] {
	case "allow":
		fs := newFlagSet("exec allow", func() {
			fmt.Fprintln(os.Stdout, "用法：adm exec allow --executable NAME_OR_PATH")
		})
		executable := fs.String("executable", "", "程序名或绝对路径")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*executable) == "" {
			return fmt.Errorf("缺少 --executable；运行 adm exec allow -h 查看帮助")
		}
		items, err := service.ExecAllow(*executable)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "remove":
		fs := newFlagSet("exec remove", func() {
			fmt.Fprintln(os.Stdout, "用法：adm exec remove --executable NAME_OR_PATH")
			fmt.Fprintln(os.Stdout, "\n从执行白名单移除一个程序；后续 Environment exec 将立即按新的白名单判断。")
		})
		executable := fs.String("executable", "", "程序名或绝对路径")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*executable) == "" {
			return fmt.Errorf("缺少 --executable；运行 adm exec remove -h 查看帮助")
		}
		items, err := service.ExecRemove(*executable)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("exec list 不接受参数")
		}
		items, err := service.ExecList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	default:
		return fmt.Errorf("未知 exec 命令 %q；运行 adm exec -h 查看帮助", args[0])
	}
}

func runCatalog(kind string, application cliManagementBackend, service any, args []string) error {
	label := "MCP"
	if kind == "skill" {
		label = "Skill"
	} else if kind != "mcp" {
		return fmt.Errorf("unknown catalog kind %q", kind)
	}
	if wantsHelp(args) {
		printCatalogHelp(kind)
		return nil
	}
	_ = service // retained for source compatibility with direct helper tests
	switch args[0] {
	case "add":
		if kind == "mcp" {
			fs := newFlagSet("mcp add", func() {
				fmt.Fprintln(os.Stdout, "用法：adm mcp add --name NAME [--transport streamable-http --endpoint URL | --transport stdio --executable PATH] [选项]")
				fmt.Fprintln(os.Stdout, "\n添加 typed MCP 定义。args/header/env refs 使用 JSON；secret 值必须写成环境变量引用。")
			})
			name := fs.String("name", "", "MCP 名称")
			transport := fs.String("transport", catalog.MCPTransportStreamableHTTP, "streamable-http 或 stdio")
			authMode := fs.String("auth-mode", catalog.MCPAuthNone, "none 或 headers")
			endpoint := fs.String("endpoint", "", "MCP Streamable HTTP endpoint")
			executable := fs.String("executable", "", "stdio MCP executable")
			argsJSON := fs.String("args-json", "", "stdio 参数 JSON 数组")
			headerRefsJSON := fs.String("header-refs-json", "", "HTTP header 环境引用 JSON 对象")
			envRefsJSON := fs.String("env-refs-json", "", "stdio 环境引用 JSON 对象")
			healthEnabled := fs.Bool("health-check-enabled", false, "启用定期健康检查")
			checkInterval := fs.Int64("check-interval-seconds", 0, "健康检查间隔")
			probeTimeout := fs.Int64("probe-timeout-seconds", 0, "健康探测超时")
			autoReconnect := fs.Bool("auto-reconnect", false, "启用固定间隔自动重连")
			reconnectInterval := fs.Int64("reconnect-interval-seconds", 0, "自动重连间隔")
			defaultInclude := fs.Bool("default", false, "新建 Environment 时默认启用")
			if err := fs.Parse(args[1:]); err != nil {
				return flagError(err)
			}
			if fs.NArg() != 0 || strings.TrimSpace(*name) == "" {
				return fmt.Errorf("必须提供 --name；运行 adm mcp add -h 查看帮助")
			}
			var args []string
			var headerRefs, envRefs map[string]string
			if err := decodeOptionalJSON(*argsJSON, &args, "--args-json"); err != nil {
				return err
			}
			if err := decodeOptionalJSON(*headerRefsJSON, &headerRefs, "--header-refs-json"); err != nil {
				return err
			}
			if err := decodeOptionalJSON(*envRefsJSON, &envRefs, "--env-refs-json"); err != nil {
				return err
			}
			item, err := application.MCPAddConfig(*name, catalog.MCPConfig{
				Transport:  *transport,
				AuthMode:   *authMode,
				Endpoint:   *endpoint,
				HeaderRefs: headerRefs,
				Executable: *executable,
				Args:       args,
				EnvRefs:    envRefs,
				HealthPolicy: model.MCPHealthPolicy{
					HealthCheckEnabled:       *healthEnabled,
					CheckIntervalSeconds:     *checkInterval,
					ProbeTimeoutSeconds:      *probeTimeout,
					AutoReconnect:            *autoReconnect,
					ReconnectIntervalSeconds: *reconnectInterval,
				},
				DefaultInclude: *defaultInclude,
			})
			if err != nil {
				return err
			}
			return writeJSON(item)
		}

		fs := newFlagSet("skill add", func() {
			fmt.Fprintln(os.Stdout, "用法：adm skill add --root PATH [--support-root PATH] [--default]")
			fmt.Fprintln(os.Stdout, "\n从一个显式全局 Skill root 发现真实 SKILL.md；可选 support root 只授权该 Skill 所需的共享支持文件。")
		})
		root := fs.String("root", "", "包含 Skill 目录/SKILL.md 的显式 discovery root")
		supportRoot := fs.String("support-root", "", "可选的 Skill supporting-files root")
		defaultInclude := fs.Bool("default", false, "发现的 Skill 在新建 Environment 中默认启用")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*root) == "" {
			return fmt.Errorf("缺少 --root；运行 adm skill add -h 查看帮助")
		}
		items, err := application.SkillAdd(*root, strings.TrimSpace(*supportRoot), *defaultInclude)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "import-preview":
		if kind != "mcp" {
			return fmt.Errorf("未知 %s 命令 %q；运行 adm %s -h 查看帮助", kind, args[0], kind)
		}
		fs := newFlagSet("mcp import-preview", func() {
			fmt.Fprintln(os.Stdout, "用法：adm mcp import-preview (--json-or-jsonc CONTENT | --file PATH | --stdin) [--format auto|generic-mcpservers|opencode|workbuddy|codex-plugin|claude-code|mcphub] [--source-scope SCOPE] [--default]")
			fmt.Fprintln(os.Stdout, "\n解析并脱敏预览外部 MCP JSON/JSONC；文件和 stdin 只由本地 CLI 读取，不写入 catalog，也不修改 Environment 选择。")
		})
		format := fs.String("format", app.MCPImportAuto, "导入格式；默认 auto")
		inlineContent := fs.String("json-or-jsonc", "", "内联 JSON/JSONC 内容")
		filePath := fs.String("file", "", "从本地文件读取 JSON/JSONC")
		useStdin := fs.Bool("stdin", false, "从显式重定向/管道 stdin 读取 JSON/JSONC")
		sourceScope := fs.String("source-scope", "", "Claude Code project scope 等显式来源 scope")
		defaultInclude := fs.Bool("default", false, "导入后供新建 Environment 默认选择")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("mcp import-preview 只接受 --flag 参数；运行 adm mcp import-preview -h 查看帮助")
		}
		content, err := resolveMCPImportContent(*inlineContent, flagWasSet(fs, "json-or-jsonc"), *filePath, flagWasSet(fs, "file"), *useStdin, os.Stdin, stdinIsInteractive())
		if err != nil {
			return err
		}
		if application == nil {
			return fmt.Errorf("MCP import service is not initialized")
		}
		preview, err := application.MCPImportPreview(app.MCPImportInput{Format: *format, Content: content, SourceScope: *sourceScope, DefaultInclude: *defaultInclude})
		if err != nil {
			return err
		}
		return writeJSON(preview)
	case "import-apply":
		if kind != "mcp" {
			return fmt.Errorf("未知 %s 命令 %q；运行 adm %s -h 查看帮助", kind, args[0], kind)
		}
		fs := newFlagSet("mcp import-apply", func() {
			fmt.Fprintln(os.Stdout, "用法：adm mcp import-apply (--json-or-jsonc CONTENT | --file PATH | --stdin) [--format FORMAT] [--selected-names A,B] [--conflict-policy error|skip|update_by_name] [--source-scope SCOPE] [--default]")
			fmt.Fprintln(os.Stdout, "\n重新解析并原子写入选中的全局 MCP 定义；文件和 stdin 只由本地 CLI 读取，不会启用任何已有 Environment。")
		})
		format := fs.String("format", app.MCPImportAuto, "导入格式；默认 auto")
		inlineContent := fs.String("json-or-jsonc", "", "内联 JSON/JSONC 内容")
		filePath := fs.String("file", "", "从本地文件读取 JSON/JSONC")
		useStdin := fs.Bool("stdin", false, "从显式重定向/管道 stdin 读取 JSON/JSONC")
		selectedText := fs.String("selected-names", "", "逗号分隔的 MCP 名称；空值表示全部候选")
		conflictPolicy := fs.String("conflict-policy", catalog.MCPConflictError, "error、skip 或 update_by_name")
		sourceScope := fs.String("source-scope", "", "Claude Code project scope 等显式来源 scope")
		defaultInclude := fs.Bool("default", false, "导入后供新建 Environment 默认选择")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("mcp import-apply 只接受 --flag 参数；运行 adm mcp import-apply -h 查看帮助")
		}
		content, err := resolveMCPImportContent(*inlineContent, flagWasSet(fs, "json-or-jsonc"), *filePath, flagWasSet(fs, "file"), *useStdin, os.Stdin, stdinIsInteractive())
		if err != nil {
			return err
		}
		if application == nil {
			return fmt.Errorf("MCP import service is not initialized")
		}
		selectedNames := []string{}
		for _, value := range strings.Split(*selectedText, ",") {
			if value = strings.TrimSpace(value); value != "" {
				selectedNames = append(selectedNames, value)
			}
		}
		result, err := application.MCPImportApply(app.MCPImportInput{Format: *format, Content: content, SelectedNames: selectedNames, ConflictPolicy: *conflictPolicy, SourceScope: *sourceScope, DefaultInclude: *defaultInclude})
		if err != nil {
			return err
		}
		return writeJSON(result)
	case "source-list":
		if kind != "skill" {
			return fmt.Errorf("unknown %s command %q; run adm %s -h for help", kind, args[0], kind)
		}
		if len(args) != 1 {
			return fmt.Errorf("skill source-list does not accept arguments")
		}
		sources, err := application.SkillSourceList()
		if err != nil {
			return err
		}
		return writeJSON(sources)
	case "source-add":
		if kind != "skill" {
			return fmt.Errorf("unknown %s command %q; run adm %s -h for help", kind, args[0], kind)
		}
		fs := newFlagSet("skill source-add", func() {
			fmt.Fprintln(os.Stdout, "Usage: adm skill source-add --root PATH [--support-root PATH ...] [--default]")
			fmt.Fprintln(os.Stdout, "\\nRegister one explicit Skill source without refreshing it. --support-root may be repeated.")
		})
		root := fs.String("root", "", "Skill source discovery root")
		var supportRoots []string
		fs.Func("support-root", "Optional Skill support root; may be repeated", func(value string) error {
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("--support-root requires a non-empty path")
			}
			supportRoots = append(supportRoots, value)
			return nil
		})
		defaultInclude := fs.Bool("default", false, "Default-enable Skills refreshed from this source for newly created Environments")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*root) == "" {
			return fmt.Errorf("must provide --root; run adm skill source-add -h for help")
		}
		source, err := application.SkillSourceAdd(*root, supportRoots, *defaultInclude)
		if err != nil {
			return err
		}
		return writeJSON(source)
	case "source-update":
		if kind != "skill" {
			return fmt.Errorf("unknown %s command %q; run adm %s -h for help", kind, args[0], kind)
		}
		fs := newFlagSet("skill source-update", func() {
			fmt.Fprintln(os.Stdout, "Usage: adm skill source-update --id SOURCE_ID --root PATH [--support-root PATH ...] [--default=true|false]")
			fmt.Fprintln(os.Stdout, "\\nUpdate one Skill source configuration without refreshing it. --support-root may be repeated; omitting --default preserves its current value; source-refresh remains explicit.")
		})
		id := fs.String("id", "", "Skill source ID")
		root := fs.String("root", "", "Skill source discovery root")
		var supportRoots []string
		fs.Func("support-root", "Optional Skill support root; may be repeated", func(value string) error {
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("--support-root requires a non-empty path")
			}
			supportRoots = append(supportRoots, value)
			return nil
		})
		defaultInclude := fs.Bool("default", false, "Default-enable Skills refreshed from this source for newly created Environments; omitted preserves the current value")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*id) == "" || strings.TrimSpace(*root) == "" {
			return fmt.Errorf("must provide --id and --root; run adm skill source-update -h for help")
		}
		desiredDefault := *defaultInclude
		if !flagWasSet(fs, "default") {
			sources, err := application.SkillSourceList()
			if err != nil {
				return err
			}
			found := false
			for _, source := range sources {
				if source.ID == strings.TrimSpace(*id) {
					desiredDefault = source.DefaultIncludeInEnv
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Skill source %s not found", strings.TrimSpace(*id))
			}
		}
		source, err := application.SkillSourceUpdate(*id, *root, supportRoots, desiredDefault)
		if err != nil {
			return err
		}
		return writeJSON(source)
	case "source-refresh":
		if kind != "skill" {
			return fmt.Errorf("unknown %s command %q; run adm %s -h for help", kind, args[0], kind)
		}
		fs := newFlagSet("skill source-refresh", func() {
			fmt.Fprintln(os.Stdout, "Usage: adm skill source-refresh --id SOURCE_ID")
			fmt.Fprintln(os.Stdout, "\\nAtomically refresh one Skill source snapshot.")
		})
		id := fs.String("id", "", "Skill source ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*id) == "" {
			return fmt.Errorf("must provide --id; run adm skill source-refresh -h for help")
		}
		result, err := application.SkillSourceRefresh(*id)
		if err != nil {
			return err
		}
		return writeJSON(result)
	case "source-remove":
		if kind != "skill" {
			return fmt.Errorf("unknown %s command %q; run adm %s -h for help", kind, args[0], kind)
		}
		fs := newFlagSet("skill source-remove", func() {
			fmt.Fprintln(os.Stdout, "Usage: adm skill source-remove --id SOURCE_ID")
			fmt.Fprintln(os.Stdout, "\\nRemove one Skill source and its source-owned Skills; existing Environment selections become unresolved.")
		})
		id := fs.String("id", "", "Skill source ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*id) == "" {
			return fmt.Errorf("must provide --id; run adm skill source-remove -h for help")
		}
		result, err := application.SkillSourceRemove(*id)
		if err != nil {
			return err
		}
		return writeJSON(result)
	case "availability":
		if kind != "skill" {
			return fmt.Errorf("availability is only supported for Skill")
		}
		if len(args) != 1 {
			return fmt.Errorf("skill availability does not accept arguments")
		}
		availabilityBackend, ok := application.(cliSkillAvailabilityBackend)
		if !ok {
			return fmt.Errorf("Skill availability requires the connected Admin MCP backend")
		}
		items, err := availabilityBackend.SkillAvailabilityList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("%s list 不接受参数", kind)
		}
		if kind == "mcp" {
			items, err := application.MCPList()
			if err != nil {
				return err
			}
			return writeJSON(items)
		}
		items, err := application.SkillList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "probe":
		if kind != "mcp" {
			return fmt.Errorf("probe is only supported for MCP")
		}
		fs := newFlagSet("mcp probe", func() {
			fmt.Fprintln(os.Stdout, "用法：adm mcp probe --id MCP_ID\n全局 MCP 连接探测，不需要 Environment。")
		})
		id := fs.String("id", "", "MCP ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*id) == "" {
			return fmt.Errorf("必须提供 --id")
		}
		if application == nil {
			return fmt.Errorf("MCP health service is not initialized")
		}
		status, err := application.MCPProbe(context.Background(), strings.TrimSpace(*id))
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "status":
		if kind != "mcp" {
			return fmt.Errorf("未知 %s 命令 %q；运行 adm %s -h 查看帮助", kind, args[0], kind)
		}
		fs := newFlagSet("mcp status", func() {
			fmt.Fprintln(os.Stdout, "用法：adm mcp status --id MCP_ID --environment-id ENV_ID")
			fmt.Fprintln(os.Stdout, "\n按 Environment 选择策略对一个 MCP 做即时健康检查，并输出 configured / disabled / healthy / error JSON 状态。")
		})
		id := fs.String("id", "", "MCP ID")
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		mcpID := strings.TrimSpace(*id)
		envID := strings.TrimSpace(*environmentID)
		if fs.NArg() != 0 || mcpID == "" || envID == "" {
			return fmt.Errorf("必须提供 --id 和 --environment-id；运行 adm mcp status -h 查看帮助")
		}
		if application == nil {
			return fmt.Errorf("MCP health service is not initialized")
		}
		status, err := application.MCPHealth(context.Background(), envID, mcpID)
		if err != nil {
			return err
		}
		return writeJSON(status)
	case "inspect", "refresh":
		if kind != "mcp" {
			return fmt.Errorf("%s is only supported for MCP", args[0])
		}
		action := args[0]
		fs := newFlagSet("mcp "+action, func() {
			fmt.Fprintf(os.Stdout, "用法：adm mcp %s --id MCP_ID --environment-id ENV_ID\n", action)
			if action == "inspect" {
				fmt.Fprintln(os.Stdout, "\n被动查看脱敏 desired config 与当前 Gateway owner-local observation/tool inventory；不会连接、Ping 或刷新 MCP。")
			} else {
				fmt.Fprintln(os.Stdout, "\n显式丢弃该 Environment/MCP 的 owner-local session/observation，并重新连接、Ping、刷新 bounded tool inventory；不会调用业务工具或修改 catalog/Environment 选择。")
			}
		})
		id := fs.String("id", "", "MCP ID")
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		mcpID := strings.TrimSpace(*id)
		envID := strings.TrimSpace(*environmentID)
		if fs.NArg() != 0 || mcpID == "" || envID == "" {
			return fmt.Errorf("必须提供 --id 和 --environment-id；运行 adm mcp %s -h 查看帮助", action)
		}
		runtimeBackend, ok := application.(cliMCPRuntimeBackend)
		if !ok {
			return fmt.Errorf("MCP runtime inspection/refresh requires the connected Admin MCP backend")
		}
		if action == "inspect" {
			inspection, err := runtimeBackend.MCPInspect(envID, mcpID)
			if err != nil {
				return err
			}
			return writeJSON(inspection)
		}
		observation, err := runtimeBackend.MCPRefresh(envID, mcpID)
		if err != nil {
			return err
		}
		return writeJSON(observation)
	case "remove":
		fs := newFlagSet(kind+" remove", func() {
			fmt.Fprintf(os.Stdout, "用法：adm %s remove --id ID\n", kind)
		})
		id := fs.String("id", "", label+" ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		value := strings.TrimSpace(*id)
		if fs.NArg() != 0 || value == "" {
			return fmt.Errorf("缺少 --id；运行 adm %s remove -h 查看帮助", kind)
		}
		if kind == "mcp" {
			if err := application.MCPRemove(value); err != nil {
				return err
			}
		} else if err := application.SkillRemove(value); err != nil {
			return err
		}
		return writeJSON(map[string]any{"removed": value})
	case "set-default":
		fs := newFlagSet(kind+" set-default", func() {
			fmt.Fprintf(os.Stdout, "用法：adm %s set-default --id ID --enabled true|false\n", kind)
			fmt.Fprintln(os.Stdout, "\n只影响之后新建的 Environment，不重写已有 Environment 的选择。")
		})
		id := fs.String("id", "", label+" ID")
		enabledText := fs.String("enabled", "", "是否默认启用：true 或 false")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		value := strings.TrimSpace(*id)
		if fs.NArg() != 0 || value == "" || strings.TrimSpace(*enabledText) == "" {
			return fmt.Errorf("必须提供 --id 和 --enabled；运行 adm %s set-default -h 查看帮助", kind)
		}
		enabled, err := strconv.ParseBool(strings.TrimSpace(*enabledText))
		if err != nil {
			return fmt.Errorf("--enabled 必须是 true 或 false")
		}
		if kind == "mcp" {
			item, err := application.MCPSetDefault(value, enabled)
			if err != nil {
				return err
			}
			return writeJSON(item)
		}
		item, err := application.SkillSetDefault(value, enabled)
		if err != nil {
			return err
		}
		return writeJSON(item)
	default:
		return fmt.Errorf("未知 %s 命令 %q；运行 adm %s -h 查看帮助", kind, args[0], kind)
	}
}

func runMemory(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printMemoryHelp()
		return nil
	}
	switch args[0] {
	case "global":
		return runGlobalMemory(service, args[1:])
	case "environment":
		return runEnvironmentMemory(service, args[1:])
	default:
		return fmt.Errorf("未知 memory 命令 %q；运行 adm memory -h 查看帮助", args[0])
	}
}

func runGlobalMemory(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printGlobalMemoryHelp()
		return nil
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("memory global list 不接受参数")
		}
		items, err := service.GlobalMemoryList()
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "read":
		fs := newFlagSet("memory global read", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory global read --key KEY")
		})
		key := fs.String("key", "", "Global Memory key")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		value := strings.TrimSpace(*key)
		if fs.NArg() != 0 || value == "" {
			return fmt.Errorf("缺少 --key；运行 adm memory global read -h 查看帮助")
		}
		item, err := service.GlobalMemoryRead(value)
		if err != nil {
			return err
		}
		return writeJSON(item)
	case "write":
		fs := newFlagSet("memory global write", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory global write --key KEY --value VALUE")
			fmt.Fprintln(os.Stdout, "\n显式写入 Global Memory；VALUE 可以是空字符串，但必须提供 --value。")
		})
		key := fs.String("key", "", "Global Memory key")
		value := fs.String("value", "", "Global Memory value")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		keyValue := strings.TrimSpace(*key)
		if fs.NArg() != 0 || keyValue == "" || !flagWasSet(fs, "value") {
			return fmt.Errorf("必须提供 --key 和 --value；运行 adm memory global write -h 查看帮助")
		}
		if err := service.GlobalMemoryWrite(keyValue, *value); err != nil {
			return err
		}
		item, err := service.GlobalMemoryRead(keyValue)
		if err != nil {
			return err
		}
		return writeJSON(item)
	case "delete":
		fs := newFlagSet("memory global delete", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory global delete --key KEY")
		})
		key := fs.String("key", "", "Global Memory key")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		value := strings.TrimSpace(*key)
		if fs.NArg() != 0 || value == "" {
			return fmt.Errorf("缺少 --key；运行 adm memory global delete -h 查看帮助")
		}
		if err := service.GlobalMemoryDelete(value); err != nil {
			return err
		}
		return writeJSON(map[string]any{"deleted": value})
	default:
		return fmt.Errorf("未知 memory global 命令 %q；运行 adm memory global -h 查看帮助", args[0])
	}
}

func runEnvironmentMemory(service cliManagementBackend, args []string) error {
	if wantsHelp(args) {
		printEnvironmentMemoryHelp()
		return nil
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("memory environment list", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory environment list --environment-id ENV_ID")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" {
			return fmt.Errorf("缺少 --environment-id；运行 adm memory environment list -h 查看帮助")
		}
		items, err := service.EnvironmentMemoryList(*environmentID)
		if err != nil {
			return err
		}
		return writeJSON(items)
	case "read":
		fs := newFlagSet("memory environment read", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory environment read --environment-id ENV_ID --key KEY")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		key := fs.String("key", "", "Environment-private Memory key")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		keyValue := strings.TrimSpace(*key)
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || keyValue == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --key；运行 adm memory environment read -h 查看帮助")
		}
		item, err := service.EnvironmentMemoryRead(*environmentID, keyValue)
		if err != nil {
			return err
		}
		return writeJSON(item)
	case "write":
		fs := newFlagSet("memory environment write", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory environment write --environment-id ENV_ID --key KEY --value VALUE")
			fmt.Fprintln(os.Stdout, "\n显式写入指定 Environment 的 private Memory；VALUE 可以是空字符串，但必须提供 --value。")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		key := fs.String("key", "", "Environment-private Memory key")
		value := fs.String("value", "", "Environment-private Memory value")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		keyValue := strings.TrimSpace(*key)
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || keyValue == "" || !flagWasSet(fs, "value") {
			return fmt.Errorf("必须提供 --environment-id、--key 和 --value；运行 adm memory environment write -h 查看帮助")
		}
		if err := service.EnvironmentMemoryWrite(*environmentID, keyValue, *value); err != nil {
			return err
		}
		item, err := service.EnvironmentMemoryRead(*environmentID, keyValue)
		if err != nil {
			return err
		}
		return writeJSON(item)
	case "delete":
		fs := newFlagSet("memory environment delete", func() {
			fmt.Fprintln(os.Stdout, "用法：adm memory environment delete --environment-id ENV_ID --key KEY")
		})
		environmentID := fs.String("environment-id", "", "Environment ID")
		key := fs.String("key", "", "Environment-private Memory key")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		keyValue := strings.TrimSpace(*key)
		if fs.NArg() != 0 || strings.TrimSpace(*environmentID) == "" || keyValue == "" {
			return fmt.Errorf("必须提供 --environment-id 和 --key；运行 adm memory environment delete -h 查看帮助")
		}
		if err := service.EnvironmentMemoryDelete(*environmentID, keyValue); err != nil {
			return err
		}
		return writeJSON(map[string]any{"environment_id": *environmentID, "deleted": keyValue})
	default:
		return fmt.Errorf("未知 memory environment 命令 %q；运行 adm memory environment -h 查看帮助", args[0])
	}
}

func runGateway(service *app.Service, args []string) error {
	return runGatewayForTarget(service, defaultADMBaseURL(), args)
}

func runGatewayForTarget(service *app.Service, baseURL string, args []string) error {
	if wantsHelp(args) {
		printGatewayHelp()
		return nil
	}

	resolveListen := func(fs *flag.FlagSet, listen string) (string, error) {
		if flagWasSet(fs, "listen") {
			return strings.TrimSpace(listen), nil
		}
		return localGatewayListenFromBaseURL(baseURL)
	}

	switch args[0] {
	case "start", "http":
		fs := newFlagSet("gateway start", func() {
			fmt.Fprintln(os.Stdout, "用法：adm [--adm-url URL] gateway start [--listen HOST:PORT] [-d|--detach]")
			fmt.Fprintln(os.Stdout, "\n未显式提供 --listen 时，从当前 ADM Base URL 派生本机回环监听地址。")
			fmt.Fprintln(os.Stdout, "在当前终端前台启动 HTTP MCP Gateway；按 Ctrl+C 停止。加 -d 或 --detach 可后台运行。")
		})
		listen := fs.String("listen", "", "本机回环监听地址；显式值优先于 --adm-url/ADM_V2_URL")
		var detach bool
		fs.BoolVar(&detach, "detach", false, "脱离当前终端运行，并在健康检查通过后返回")
		fs.BoolVar(&detach, "d", false, "--detach 的简写")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("gateway start 只接受 --flag 参数；运行 adm gateway start -h 查看帮助")
		}
		targetListen, err := resolveListen(fs, *listen)
		if err != nil {
			return err
		}
		if detach {
			return startGatewayDetached(targetListen)
		}
		return startHTTPGateway(service, targetListen)
	case "status":
		fs := newFlagSet("gateway status", func() {
			fmt.Fprintln(os.Stdout, "用法：adm [--adm-url URL] gateway status [--listen HOST:PORT]")
			fmt.Fprintln(os.Stdout, "\n未显式提供 --listen 时检查当前 ADM Base URL；因此也可查看自定义端口或远端 health。")
		})
		listen := fs.String("listen", "", "Gateway 监听地址；显式值优先于 --adm-url/ADM_V2_URL")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("gateway status 只接受 --flag 参数")
		}
		if flagWasSet(fs, "listen") {
			return printGatewayStatus(strings.TrimSpace(*listen))
		}
		return printGatewayStatusBaseURL(baseURL)
	case "stop":
		fs := newFlagSet("gateway stop", func() {
			fmt.Fprintln(os.Stdout, "用法：adm [--adm-url URL] gateway stop [--listen HOST:PORT]")
			fmt.Fprintln(os.Stdout, "\n未显式提供 --listen 时，从当前 ADM Base URL 派生本机 Gateway；远端地址不会被停止。")
		})
		listen := fs.String("listen", "", "Gateway 监听地址；显式值优先于 --adm-url/ADM_V2_URL")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("gateway stop 只接受 --flag 参数")
		}
		targetListen, err := resolveListen(fs, *listen)
		if err != nil {
			return err
		}
		return stopHTTPGateway(targetListen)
	case "restart":
		fs := newFlagSet("gateway restart", func() {
			fmt.Fprintln(os.Stdout, "用法：adm [--adm-url URL] gateway restart [--listen HOST:PORT]")
			fmt.Fprintln(os.Stdout, "\n未显式提供 --listen 时，从当前 ADM Base URL 派生本机 Gateway。")
		})
		listen := fs.String("listen", "", "Gateway 监听地址；显式值优先于 --adm-url/ADM_V2_URL")
		if err := fs.Parse(args[1:]); err != nil {
			return flagError(err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("gateway restart 只接受 --flag 参数")
		}
		targetListen, err := resolveListen(fs, *listen)
		if err != nil {
			return err
		}
		if err := stopHTTPGateway(targetListen); err != nil {
			return err
		}
		return startHTTPGateway(service, targetListen)
	case "stdio":
		if len(args) != 1 {
			return fmt.Errorf("gateway stdio 不接受参数；运行 adm gateway -h 查看帮助")
		}
		if stdinIsTerminal() {
			return fmt.Errorf("gateway stdio 是给 MCP 客户端使用的协议通道，不是人工终端命令；人工启动 HTTP Gateway 请运行 adm gateway start")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return gateway.RunStdio(ctx, service)
	default:
		return fmt.Errorf("未知 gateway 命令 %q；运行 adm gateway -h 查看帮助", args[0])
	}
}

func runDoctor(service *app.Service, statePath string, args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(os.Stdout, "用法：adm doctor")
		fmt.Fprintln(os.Stdout, "\n一次查看当前 ADM 程序、状态文件、Gateway、Workspace、Environment、Writer 和执行白名单。")
		return nil
	}
	if len(args) != 0 {
		return fmt.Errorf("doctor 不接受参数")
	}

	workspaces, err := service.Workspaces.List()
	if err != nil {
		return fmt.Errorf("读取 Workspace 失败: %w", err)
	}
	environments, err := service.Environments.List()
	if err != nil {
		return fmt.Errorf("读取 Environment 失败: %w", err)
	}
	allowed, err := service.AllowedExecutables()
	if err != nil {
		return fmt.Errorf("读取执行白名单失败: %w", err)
	}
	executable, executableErr := os.Executable()
	if executableErr != nil {
		executable = "（无法获取：" + executableErr.Error() + "）"
	}

	fmt.Println("ADM V2 诊断")
	fmt.Println("当前程序：", executable)
	fmt.Println("状态文件：", statePath)
	fmt.Println()

	health, running, healthErr := fetchGatewayHealth(defaultGatewayListen)
	fmt.Println("Gateway")
	switch {
	case healthErr != nil:
		var incompatible *incompatibleGatewayError
		if errors.As(healthErr, &incompatible) {
			fmt.Println("  状态：    版本不兼容")
			fmt.Println("  MCP 地址：http://127.0.0.1:43137/mcp")
			fmt.Println("  详情：   ", incompatible)
			fmt.Println("  处理：    adm gateway restart")
		} else {
			fmt.Println("  状态：    未知")
			fmt.Println("  MCP 地址：http://127.0.0.1:43137/mcp")
			fmt.Println("  详情：   ", healthErr)
		}
	case running:
		fmt.Println("  状态：    运行中")
		fmt.Println("  MCP 地址：http://127.0.0.1:43137/mcp")
		if health.PID > 0 {
			fmt.Println("  PID：    ", health.PID)
		} else {
			fmt.Println("  PID：     旧版 Gateway 未提供")
		}
		fmt.Println("  版本：   ", health.Version)
		if health.OwnerID != "" {
			fmt.Println("  Runtime Owner：", health.OwnerID)
		}
	default:
		fmt.Println("  状态：    已停止")
		fmt.Println("  MCP 地址：http://127.0.0.1:43137/mcp")
		fmt.Println("  启动：    adm gateway start")
	}
	fmt.Println()

	fmt.Printf("Workspace（%d）\n", len(workspaces))
	if len(workspaces) == 0 {
		fmt.Println("  暂无；添加：adm workspace add --path PATH --name NAME")
	} else {
		for _, ws := range workspaces {
			fmt.Printf("  %s  %s  %s\n", ws.ID, ws.Name, ws.Path)
		}
	}
	fmt.Println()

	fmt.Printf("Environment（%d）\n", len(environments))
	if len(environments) == 0 {
		fmt.Println("  暂无；创建：adm environment create --workspace-id WS_ID --name NAME")
	} else {
		for _, env := range environments {
			writer := "无 Writer"
			if env.Writer != nil {
				writer = "Writer=" + env.Writer.Owner
			}
			fmt.Printf("  %s  %s  root=%s  %s\n", env.ID, env.Name, env.Root, writer)
		}
	}
	fmt.Println()

	fmt.Printf("执行白名单（%d）\n", len(allowed))
	if len(allowed) == 0 {
		fmt.Println("  暂无")
	} else {
		for _, executable := range allowed {
			fmt.Println(" ", executable)
		}
	}
	return nil
}

func runState(statePath string, args []string) error {
	if wantsHelp(args) {
		fmt.Fprintln(os.Stdout, "用法：adm state path")
		fmt.Fprintln(os.Stdout, "\n打印 ADM V2 持久状态文件路径。")
		return nil
	}
	if len(args) == 1 && args[0] == "path" {
		fmt.Println(statePath)
		return nil
	}
	return fmt.Errorf("未知 state 命令；运行 adm state -h 查看帮助")
}

func startHTTPGateway(service *app.Service, listen string) error {
	listen = strings.TrimSpace(listen)
	baseURL, err := gatewayBaseURL(listen)
	if err != nil {
		return err
	}
	fmt.Println("ADM V2 HTTP Gateway")
	fmt.Println("状态：    正在启动")
	fmt.Println("MCP 地址：", baseURL+"/mcp")
	fmt.Println("健康检查：", baseURL+"/healthz")
	fmt.Println("停止方式：当前终端按 Ctrl+C，或另开终端运行 adm gateway stop")
	fmt.Println()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := gateway.RunHTTP(ctx, service, listen); err != nil {
		return fmt.Errorf("在 %s 启动 HTTP Gateway 失败: %w；监听地址可能已被占用或被操作系统保留，可改用 --listen 并检查系统端口排除范围", listen, err)
	}
	return nil
}

func checkGatewayListenAvailable(listen string) error {
	if err := gateway.CheckHTTPListenAvailable(listen); err != nil {
		return fmt.Errorf("%w；请改用 --listen 或检查系统端口排除范围", err)
	}
	return nil
}

func startDetachedHTTPGateway(listen string) error {
	listen = strings.TrimSpace(listen)
	baseURL, err := gatewayBaseURL(listen)
	if err != nil {
		return err
	}
	health, running, err := fetchGatewayHealth(listen)
	if err != nil {
		return err
	}
	if running {
		fmt.Println("ADM V2 HTTP Gateway")
		fmt.Println("状态：    已在运行")
		fmt.Println("MCP 地址：", baseURL+"/mcp")
		fmt.Println("PID：    ", health.PID)
		return nil
	}
	if err := checkGatewayListenAvailable(listen); err != nil {
		return err
	}

	process, err := startDetachedGatewayProcess(listen)
	if err != nil {
		return fmt.Errorf("后台启动 Gateway 失败: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		health, running, healthErr := fetchGatewayHealth(listen)
		if healthErr != nil {
			_ = process.Kill()
			_ = process.Release()
			return fmt.Errorf("后台 Gateway 启动失败: %w", healthErr)
		}
		if running {
			_ = process.Release()
			fmt.Println("ADM V2 HTTP Gateway")
			fmt.Println("状态：    已在后台运行")
			fmt.Println("MCP 地址：", baseURL+"/mcp")
			fmt.Println("PID：    ", health.PID)
			fmt.Println("停止：    adm gateway stop")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = process.Kill()
	_ = process.Release()
	return fmt.Errorf("后台 Gateway 未能在 5 秒内通过健康检查: %s/healthz", baseURL)
}
func printGatewayStatusBaseURL(rawBaseURL string) error {
	baseURL, err := normalizeCLIADMBaseURL(rawBaseURL)
	if err != nil {
		return err
	}
	if listen, localErr := localGatewayListenFromBaseURL(baseURL); localErr == nil {
		return printGatewayStatus(listen)
	}
	status, err := gateway.InspectHTTPBaseURL(baseURL)
	if err != nil {
		return err
	}
	fmt.Println("ADM V2 HTTP Gateway")
	fmt.Println("MCP 地址：", status.MCPURL)
	switch status.State {
	case gateway.HTTPStateRunning:
		fmt.Println("状态：    运行中")
		fmt.Println("PID：    ", status.PID)
		fmt.Println("版本：   ", status.Version)
		if status.OwnerID != "" {
			fmt.Println("Runtime Owner：", status.OwnerID)
		}
	case gateway.HTTPStateIncompatible:
		fmt.Println("状态：    版本不兼容")
		fmt.Println("详情：   ", status.Detail)
	default:
		fmt.Println("状态：    已停止或不可达")
	}
	return nil
}

func printGatewayStatus(listen string) error {
	baseURL, err := gatewayBaseURL(listen)
	if err != nil {
		return err
	}
	health, running, err := fetchGatewayHealth(listen)
	if err != nil {
		fmt.Println("ADM V2 HTTP Gateway")
		fmt.Println("MCP 地址：", baseURL+"/mcp")
		var incompatible *incompatibleGatewayError
		if errors.As(err, &incompatible) {
			fmt.Println("状态：    版本不兼容")
			fmt.Println("详情：   ", incompatible)
			fmt.Println("处理：    运行 adm gateway restart；新版 CLI 会尝试安全停止同一路径的旧版 ADM Gateway")
			return nil
		}
		fmt.Println("状态：    未知")
		return err
	}
	fmt.Println("ADM V2 HTTP Gateway")
	if !running {
		fmt.Println("状态：    已停止")
		fmt.Println("MCP 地址：", baseURL+"/mcp")
		fmt.Println("启动：    adm gateway start")
		return nil
	}
	fmt.Println("状态：    运行中")
	fmt.Println("MCP 地址：", baseURL+"/mcp")
	fmt.Println("PID：    ", health.PID)
	fmt.Println("版本：   ", health.Version)
	if health.OwnerID != "" {
		fmt.Println("Runtime Owner：", health.OwnerID)
	}
	fmt.Println("停止：    adm gateway stop")
	return nil
}

func stopHTTPGateway(listen string) error {
	baseURL, err := gatewayBaseURL(listen)
	if err != nil {
		return err
	}
	health, running, err := fetchGatewayHealth(listen)
	if err != nil {
		var incompatible *incompatibleGatewayError
		if errors.As(err, &incompatible) {
			pid, processPath, lookupErr := findListeningProcess(listen)
			if lookupErr != nil {
				return fmt.Errorf("检测到旧版或不兼容的 Gateway，但无法自动定位监听进程: %w", lookupErr)
			}
			currentExecutable, executableErr := os.Executable()
			if executableErr != nil {
				return fmt.Errorf("检测到监听进程 PID %d，但无法确认当前 ADM 可执行文件路径: %w", pid, executableErr)
			}
			if !matchesADMExecutable(processPath, currentExecutable) {
				return fmt.Errorf("%s 被其他程序占用（PID %d，%s）；为了避免误杀，ADM 不会自动停止它", listen, pid, processPath)
			}
			fmt.Printf("检测到旧版 ADM V2 Gateway（PID %d），正在停止以完成升级重启。\n", pid)
			return terminateGatewayProcess(pid, listen, baseURL)
		}
		return err
	}
	if !running {
		fmt.Println("ADM V2 HTTP Gateway 已经停止：", baseURL+"/mcp")
		return nil
	}
	if health.PID <= 0 {
		return fmt.Errorf("Gateway %s 没有提供可用 PID，无法自动停止", baseURL)
	}
	if health.OwnerID != "" {
		if _, err := gateway.StopHTTP(listen); err != nil {
			return err
		}
		fmt.Printf("ADM V2 HTTP Gateway 已停止（PID %d）。\n", health.PID)
		return nil
	}
	return terminateGatewayProcess(health.PID, listen, baseURL)
}

func sameADMExecutable(targetPath, currentPath string) bool {
	if !strings.EqualFold(filepath.Clean(filepath.Dir(targetPath)), filepath.Clean(filepath.Dir(currentPath))) {
		return false
	}
	targetName := strings.ToLower(filepath.Base(targetPath))
	currentName := strings.ToLower(filepath.Base(currentPath))
	legacyName := "ai-dev-manager-v2.exe"
	legacyPrefix := "ai-dev-manager-v2."
	if targetName == "adm.exe" {
		return currentName == "adm.exe" || strings.HasPrefix(currentName, "adm.") || currentName == legacyName || strings.HasPrefix(currentName, legacyPrefix)
	}
	if targetName == legacyName {
		return currentName == legacyName || strings.HasPrefix(currentName, legacyPrefix) || currentName == "adm.exe" || strings.HasPrefix(currentName, "adm.")
	}
	return false
}

func terminateGatewayProcess(pid int, listen, _ string) error {
	if err := gateway.TerminateHTTPProcess(pid, listen); err != nil {
		return err
	}
	fmt.Printf("ADM V2 HTTP Gateway 已停止（PID %d）。\n", pid)
	return nil
}

func fetchGatewayHealth(listen string) (gatewayHealth, bool, error) {
	status, err := gateway.InspectHTTP(listen)
	if err != nil {
		return gatewayHealth{}, false, err
	}
	switch status.State {
	case gateway.HTTPStateStopped:
		return gatewayHealth{}, false, nil
	case gateway.HTTPStateIncompatible:
		return gatewayHealth{}, false, &incompatibleGatewayError{detail: status.Detail}
	case gateway.HTTPStateRunning:
		return gatewayHealth{
			Name:      "adm",
			Version:   status.Version,
			Status:    "ok",
			PID:       status.PID,
			Transport: "http",
			OwnerID:   status.OwnerID,
		}, true, nil
	default:
		return gatewayHealth{}, false, fmt.Errorf("unknown Gateway state %q", status.State)
	}
}

func gatewayBaseURL(listen string) (string, error) {
	return gateway.HTTPBaseURL(listen)
}
func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func newFlagSet(name string, usage func()) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = usage
	return fs
}

func decodeOptionalJSON(value string, target any, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("%s 必须是有效 JSON: %w", flagName, err)
	}
	return nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(item *flag.Flag) {
		if item.Name == name {
			found = true
		}
	})
	return found
}

func flagError(err error) error {
	if err == flag.ErrHelp {
		return nil
	}
	return err
}

func wantsHelp(args []string) bool {
	return len(args) == 0 || (len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help"))
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printUsage() {
	fmt.Fprintln(os.Stdout, `adm

给 AI Agent 和管理员使用的开发控制面。
正常 workspace/environment/exec/mcp/skill/memory 管理统一通过 Admin MCP，不直接读写 state.json。

快速开始（本地 HTTP ADM）：
  adm gateway start --detach
  adm workspace add --path D:\projects --name projects
  adm environment create --workspace-id WS_ID --name main
  adm gateway status

管理连接：
  默认 ADM Base URL: http://127.0.0.1:43137
  adm --adm-url URL workspace list
  ADM_V2_URL=URL adm workspace list
  启动时会读取当前可执行文件同级目录的 .env；已有进程环境变量优先。
  --adm-url / ADM_V2_URL 只选择管理目标；连接失败不会回退到本地 state.json。

主要命令：
  workspace      登记、查看、重命名、移除允许 ADM 使用的本地目录
  environment    创建、查看、检查、删除开发上下文（也可以简写为 env）
  exec           管理 Agent 可以执行的程序白名单
  mcp            管理全局 MCP catalog
  skill          管理全局 Skill catalog
  memory         管理显式作用域的持久 Memory
  gateway        本机启动、查看、停止、重启 MCP Gateway
  doctor         本机离线/恢复诊断入口
  state          查看本机 ADM 状态文件位置

Gateway 常用命令：
  gateway start      启动本机 HTTP Gateway；未写 --listen 时使用当前 --adm-url / ADM_V2_URL（默认 127.0.0.1:43137）
  gateway status     查看当前 ADM Base URL，或用 --listen 显式检查一个本机监听地址
  gateway stop       停止当前本机 ADM Base URL 对应的 Gateway；远端 URL 不会被停止
  gateway restart    重启当前本机 ADM Base URL 对应的 Gateway
  gateway stdio      仅供 MCP 客户端使用；不要在普通终端里手动运行

查看子命令帮助：
  adm workspace -h
  adm environment -h
  adm environment writer -h
  adm exec -h
  adm mcp -h
  adm skill -h
  adm memory -h
  adm gateway -h
  adm doctor
  adm version`)
}

func printWorkspaceHelp() {
	fmt.Fprintln(os.Stdout, `Workspace = ADM 被允许操作的本地目录。它不是服务，也不要求 Git。

命令：
  adm workspace add --path PATH [--name NAME]
      登记一个本地目录。

  adm workspace list
      查看所有 Workspace。

  adm workspace inspect --workspace-id WS_ID
      按稳定 ID 查看一个 Workspace。

  adm workspace discover --workspace-id WS_ID [--path REL] [--query TEXT] [--max-depth N --max-entries N --max-candidates N --max-digest-entries N --max-output-bytes N]
      显式执行有界 metadata-only 项目发现；不读取文件内容，不创建 Environment，也不要求 Writer/Git/MCP/Skill。

  adm workspace mcp enable|disable --workspace-id WS_ID --mcp-id MCP_ID
      管理 Workspace 级 MCP 继承；开启后该 Workspace 下所有 Environment 都可使用。

  adm workspace skill enable|disable --workspace-id WS_ID --skill-id SKILL_ID
      管理 Workspace 级 Skill 继承；Environment 显式选择只是附加层。

  adm workspace rename --workspace-id WS_ID --name NAME
      只修改显示名称，不移动或重命名项目目录。

  adm workspace remove --workspace-id WS_ID
      只移除 ADM 记录，不删除项目目录或文件；仍有 Environment 引用时拒绝移除。`)
}

func printEnvironmentHelp() {
	fmt.Fprintln(os.Stdout, `Environment = 位于 Workspace 内的持久开发上下文。它不是运行中的服务。

命令：
  adm environment create --workspace-id WS_ID --name NAME [--root PATH]
      创建 Environment；不写 --root 时默认使用整个 Workspace。

  adm environment list
      查看所有 Environment。

  adm environment inspect --environment-id ENV_ID
      查看 Workspace 关系、结构化能力事实、已解析/未解析 MCP/Skill 选择和 private Memory 条目数；不展开 Memory 值。

  adm environment capability-report --environment-id ENV_ID
      输出 canonical CapabilityReport；正常 CLI 通过 Admin MCP 使用 Gateway 路径，可包含已有 owner-local 观察但不会主动 probe/执行。

  adm environment context --environment-id ENV_ID [--path REL] [--max-depth N --max-entries N --max-digest-entries N --max-output-bytes N]
      输出 Phase-20 canonical bounded context bundle；不隐式选 Environment，不读取 Memory 值/完整 Skill 内容，不主动 probe/执行。

  adm environment tree-digest --environment-id ENV_ID [--path REL] [--max-depth N --max-entries N --max-candidates N --max-digest-entries N --max-output-bytes N]
      显式读取当前 Runtime-authorized root 的有界目录 metadata 摘要；不会读取 Workspace sibling，也不要求 Writer。

  adm environment rename --environment-id ENV_ID --name NAME
      只修改显示名称，不移动根目录、不修改选择或 Memory，也不触碰项目文件。

  adm environment workspace-recommendations
      只读列出当前归属比最具体可选 Workspace 更宽泛的普通 Environment；不会自动迁移。
  adm environment workspace-options --environment-id ENV_ID
      列出包含 Environment root 的可选 Workspace，并标记最具体推荐归属。

  adm environment workspace-set --environment-id ENV_ID --workspace-id WS_ID
      显式重绑普通 Environment；root 与 explicit MCP/Skill 不变，Workspace 继承按新归属重算。

  adm environment remove --environment-id ENV_ID
      只删除 ADM 中的 Environment 记录，不会删除项目目录或文件。

  adm environment temporary -h
      Phase-22 temporary Environment create/status/promote/cleanup；cleanup 默认 preview 且没有 force 路径。

  adm environment mcp -h
      管理这个 Environment 启用的全局 MCP ID。

  adm environment skill -h
      管理这个 Environment 启用的全局 Skill ID。

  adm environment verifier -h
      声明、查看、删除这个 Environment 的 structured verifier 定义。

  adm environment writer -h
      查看 Writer 租约相关命令。`)
}

func printEnvironmentTemporaryHelp() {
	fmt.Fprintln(os.Stdout, `Temporary Environment = Phase-22 显式 lifecycle/retention 投影；仍是普通稳定 env_，不是 ADM task。

命令：
  adm environment temporary create (--workspace-id WS_ID | --source-environment-id ENV_ID) --name NAME --owner-id OWNER --ttl-seconds N [--session-id ID] [--run-id ID] [--mode existing_root|managed_worktree] [--root PATH] [--base-ref REF]
      owner/TTL 必须显式提供；existing_root 只接受 Workspace。managed_worktree 可从 Workspace 或已有 Git Environment 派生；ADM fetch 后从指定 ref 或 upstream 最新提交创建，不 pull 源 checkout。

  adm environment temporary status --environment-id ENV_ID
      只读查看 retention、expiry 与当前 cleanup blockers；无需 owner，也不获取 Writer。

  adm environment temporary promote --environment-id ENV_ID --owner-id OWNER
      匹配 lifecycle owner 后仅改为 durable retention；不移动文件，不 merge/push Git，不改变 Environment ID。

  adm environment temporary cleanup --environment-id ENV_ID --owner-id OWNER [--execute]
      默认 preview；--execute 才在 fresh safety recheck 后执行 targeted cleanup。没有 force 路径；ordinary root 不删除项目目录或文件，managed-worktree cleanup 保留生成分支。`)
}

func printEnvironmentSelectionHelp(kind string) {
	label := strings.ToUpper(kind)
	fmt.Fprintf(os.Stdout, `Environment %s selection = 只修改一个 Environment 启用的全局 %s ID，不修改 catalog 默认值或其他 Environment。

命令：
  adm environment %s enable --environment-id ENV_ID --%s-id ID
      为一个 Environment 启用全局 %s。

  adm environment %s disable --environment-id ENV_ID --%s-id ID
      为一个 Environment 禁用全局 %s。
`, label, label, kind, kind, label, kind, kind, label)
	if kind == "skill" {
		fmt.Fprintln(os.Stdout, `
  adm environment skill list --environment-id ENV_ID
      查看该 Environment 的 Skill availability；不读取 Skill 指令内容。

  adm environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID
      查看一个 Skill 在该 Environment 中的 enabled/disabled/broken availability。`)
	}
}

func printEnvironmentVerifierHelp() {
	fmt.Fprintln(os.Stdout, `Environment verifier = 这个 Environment 的结构化 test/lint/build/custom 验证定义。定义本身不会授予执行权限；运行时仍受全局 exec 白名单和 Environment cwd 约束。

命令：
  adm environment verifier add --environment-id ENV_ID --kind test|lint|build|custom --executable NAME_OR_PATH [--name NAME] [--arg ARG ...] [--cwd RELATIVE_PATH] [--timeout-seconds N] [--enabled=true|false]
      添加一个 Environment-scoped verifier 定义；--arg 可重复。

  adm environment verifier list --environment-id ENV_ID
      查看这个 Environment 的 verifier 定义。

  adm environment verifier remove --environment-id ENV_ID --verifier-id VF_ID
      删除一个 verifier 定义。`)
}

func printWriterHelp() {
	fmt.Fprintln(os.Stdout, `Writer = 对同一个物理目录进行修改时使用的单写入租约。

命令：
  adm environment writer acquire --environment-id ENV_ID --owner OWNER
      获取或续租 Writer。

  adm environment writer heartbeat --environment-id ENV_ID --owner OWNER
      只续租，不执行文件修改。

  adm environment writer release --environment-id ENV_ID --owner OWNER
      正常释放自己的 Writer。

  adm environment writer release --environment-id ENV_ID --force
      强制释放，用于人工恢复。`)
}

func printExecHelp() {
	fmt.Fprintln(os.Stdout, `Exec 白名单决定 Agent 可以运行哪些本地程序。

命令：
  adm exec allow --executable NAME_OR_PATH
      加入一个允许执行的程序。

  adm exec remove --executable NAME_OR_PATH
      从白名单移除一个程序；后续 exec 立即按新的白名单判断。

  adm exec list
      查看当前白名单。`)
}

func printCatalogHelp(kind string) {
	if kind == "skill" {
		fmt.Fprintln(os.Stdout, `Skill catalog = 从显式配置的全局 Skill root 发现真实 SKILL.md；Environment 只保存启用的稳定 Skill ID。

命令：
  adm skill add --root PATH [--support-root PATH] [--default]
      扫描一个显式 discovery root。support root 只用于授权 Skill 需要读取的共享支持文件。

  adm skill source-list
      查看显式 Skill sources；source 配置与 refresh 分离。

  adm skill source-add --root PATH [--support-root PATH ...] [--default]
      登记一个 source，不自动 refresh；--support-root 可重复。

  adm skill source-update --id SOURCE_ID --root PATH [--support-root PATH ...] [--default=true|false]
      更新 source 配置但不自动 refresh；refresh 仍需显式执行。

  adm skill source-refresh --id SOURCE_ID
      显式原子刷新一个 source snapshot。

  adm skill source-remove --id SOURCE_ID
      删除一个 source 及其 source-owned Skill 条目。

  adm skill list
      查看 catalog inventory；这不是 availability。

  adm skill availability
      查看全局 catalog structural availability，与任何 Environment enablement 独立。

  adm skill remove --id ID
      删除一个 catalog 条目；已有 Environment 中的 ID 引用不会被静默改写。

  adm skill set-default --id ID --enabled true|false
      修改新建 Environment 的默认选择，不重写已有 Environment。`)
		return
	}
	fmt.Fprintln(os.Stdout, `MCP catalog = 全局定义；Environment 只保存启用的 ID。

命令：
	  adm mcp add --name NAME --transport streamable-http --endpoint URL [--auth-mode headers --header-refs-json JSON] [--default]
	  adm mcp add --name NAME --transport stdio --executable PATH [--args-json JSON] [--env-refs-json JSON] [--default]
	      添加 typed Streamable HTTP 或 stdio MCP 定义；secret 值使用环境变量引用。

  adm mcp list
      查看所有全局条目。

  adm mcp import-preview (--json-or-jsonc CONTENT | --file PATH | --stdin) [--format FORMAT] [--source-scope SCOPE]
      脱敏预览 OpenCode / WorkBuddy / Codex plugin / Claude Code / MCPHub JSON/JSONC，不写入 catalog；文件/stdin 由本地 CLI 读取。

  adm mcp import-apply (--json-or-jsonc CONTENT | --file PATH | --stdin) [--selected-names A,B] [--conflict-policy error|skip|update_by_name]
      原子写入选中的全局 MCP 定义；不会修改已有 Environment 选择。

  adm mcp probe --id MCP_ID
      全局 transient 配置/连接探测，不需要 Environment。

  adm mcp status --id MCP_ID --environment-id ENV_ID
      对该 Environment 的选中状态做一次即时 configured / disabled / healthy / error 探测。

  adm mcp inspect --id MCP_ID --environment-id ENV_ID
      被动读取脱敏 desired config 与 owner-local observation/inventory；不连接、不 Ping、不刷新。

  adm mcp refresh --id MCP_ID --environment-id ENV_ID
      显式 reconnect/Ping/刷新 bounded inventory；不调用业务工具、不修改 catalog 或 Environment 选择。

  adm mcp remove --id ID
      删除一个全局条目；已有 Environment 中的 ID 引用不会被静默改写。

  adm mcp set-default --id ID --enabled true|false
      修改新建 Environment 的默认选择，不重写已有 Environment。`)
}

func printMemoryHelp() {
	fmt.Fprintln(os.Stdout, `Memory = ADM 持久开发上下文。写入时必须显式选择作用域。

命令：
  adm memory global -h
      管理跨 Environment 共享的 Global Memory。

  adm memory environment -h
      按显式 Environment ID 管理 Environment-private Memory。`)
}

func printGlobalMemoryHelp() {
	fmt.Fprintln(os.Stdout, `Global Memory = 跨 Environment 共享的持久上下文。

命令：
  adm memory global list
      查看所有 Global Memory 条目。

  adm memory global read --key KEY
      读取一个条目。

  adm memory global write --key KEY --value VALUE
      显式写入 Global Memory。

  adm memory global delete --key KEY
      删除一个 Global Memory 条目。`)
}

func printEnvironmentMemoryHelp() {
	fmt.Fprintln(os.Stdout, `Environment-private Memory = 只属于一个显式 Environment 的持久上下文。

命令：
  adm memory environment list --environment-id ENV_ID
      查看一个 Environment 的 private Memory。

  adm memory environment read --environment-id ENV_ID --key KEY
      读取一个 Environment-private Memory 条目。

  adm memory environment write --environment-id ENV_ID --key KEY --value VALUE
      显式写入一个 Environment 的 private Memory。

  adm memory environment delete --environment-id ENV_ID --key KEY
      删除一个 Environment-private Memory 条目。`)
}

func printGatewayHelp() {
	fmt.Fprintln(os.Stdout, `Gateway = 真正运行中的 MCP 服务进程。

连接目标：
  默认 ADM Base URL 是 http://127.0.0.1:43137。
  可用 --adm-url URL 或 ADM_V2_URL 覆盖；--adm-url 可写在 gateway 命令前或后。
  start/stop/restart 只允许本机 loopback HTTP URL；status 可以检查自定义端口或远端 health。
  显式 --listen HOST:PORT 时，它优先于 ADM Base URL。

人工使用的 HTTP Gateway：
  adm [--adm-url URL] gateway start [--listen HOST:PORT] [-d|--detach]
      在当前终端前台启动。终端会被占用，按 Ctrl+C 停止。
      加 -d 或 --detach 后脱离当前终端运行，健康检查通过后命令立即返回。

  adm [--adm-url URL] gateway status [--listen HOST:PORT]
      查看当前 ADM Base URL 或显式监听地址的运行状态、MCP 地址、PID 和版本。

  adm [--adm-url URL] gateway stop [--listen HOST:PORT]
      停止本机 Gateway。不会通过远端 URL 发送停止操作。

  adm [--adm-url URL] gateway restart [--listen HOST:PORT]
      停止本机旧 Gateway，然后在当前终端启动新 Gateway。

仅供 MCP 客户端使用：
  adm gateway stdio
      stdin/stdout 是 MCP 协议通道。通常由 MCP 客户端自动启动，人不要手动运行。`)
}
