// Package cli 命令行模式：参数解析、各子命令、终端输出。
// 业务逻辑都在 service 包，这里只负责"问参数、打印结果"。
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/service"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

const appName = config.AppName

// errUsage 参数用法错误（退出码 2）。
var errUsage = errors.New("usage")

// Run 执行一条命令，返回进程退出码。args 不含程序名，args[0] 是子命令。
func Run(args []string, version string) int {
	service.ApplyLanguage()
	if len(args) == 0 {
		usage()
		return 2
	}
	cmd, rest := args[0], args[1:]
	opts, err := parseFlags(rest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	err = dispatch(context.Background(), cmd, opts, version)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errUsage):
		return 2
	default:
		fmt.Fprintln(os.Stderr, i18n.T("错误:"), err)
		return 1
	}
}

func dispatch(ctx context.Context, cmd string, o flags, version string) error {
	switch cmd {
	case "init":
		return cmdInit(ctx)
	case "sync":
		if o.quiet {
			// -q 是定时任务的调用方式：GUI 没开时，靠这里每小时检查一次 hooks 有没有被别的工具抹掉
			service.RepairHooks()
		}
		_, err := service.Sync(ctx, service.SyncRequest{Quiet: o.quiet, OnlyCwd: o.cwd, Dry: o.dry, WaitLock: 5 * time.Second})
		return err
	case "status":
		return cmdStatus(ctx)
	case "claim":
		if len(o.positional) != 2 {
			return fmt.Errorf(i18n.T("用法: %s claim <项目ID> <本机路径>"), appName)
		}
		_, err := service.Sync(ctx, service.SyncRequest{ClaimPID: o.positional[0], ClaimRoot: o.positional[1]})
		return err
	case "ignore":
		if len(o.positional) != 1 {
			return fmt.Errorf(i18n.T("用法: %s ignore <项目ID>"), appName)
		}
		_, err := service.Sync(ctx, service.SyncRequest{IgnorePID: o.positional[0]})
		return err
	case "unignore":
		if len(o.positional) != 1 {
			return fmt.Errorf(i18n.T("用法: %s unignore <项目ID>"), appName)
		}
		_, err := service.Sync(ctx, service.SyncRequest{UnignorePID: o.positional[0]})
		return err
	case "passwd":
		return cmdPasswd(ctx)
	case "uninstall":
		if err := service.RemoveAutoSync(); err != nil {
			return err
		}
		fmt.Println(i18n.T("已删除自动同步（Claude Code hooks、定时任务和开机自启）。会话文件、配置和云端数据都没动。"))
		fmt.Println(i18n.T("注意：再次打开图形界面会自动重新安装。"))
		return nil
	case "hook-event": // 隐藏命令：被 Claude Code hooks 调用
		cmdHookEvent(ctx)
		return nil
	case "version":
		fmt.Println(appName, version)
		return nil
	default:
		usage()
		return errUsage
	}
}

type flags struct {
	positional []string
	quiet      bool
	dry        bool
	cwd        string
}

func parseFlags(argv []string) (flags, error) {
	var f flags
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		switch {
		case s == "-q" || s == "--quiet":
			f.quiet = true
		case s == "--dry" || s == "--dry-run":
			f.dry = true
		case s == "-v" || s == "--verbose":
			// 兼容旧参数，无作用
		case s == "--cwd":
			i++
			if i >= len(argv) {
				return f, errors.New(i18n.T("--cwd 需要参数"))
			}
			f.cwd = argv[i]
		case strings.HasPrefix(s, "-"):
			return f, fmt.Errorf(i18n.T("不认识的参数 %s"), s)
		default:
			f.positional = append(f.positional, s)
		}
	}
	return f, nil
}

func usage() {
	fmt.Print(strings.ReplaceAll(i18n.T(`{app} - 把 Claude Code 的会话在多台设备之间同步

用法：
  {app}                      （不带参数）打开图形界面
  {app} init                 初始化（设置口令、云端存储、本机信息），自动装好自动同步
  {app} sync                 双向同步（最常用）
  {app} sync --dry           只显示会做什么，不真正改动
  {app} status               查看本机项目绑定和云端待认领的项目
  {app} claim <项目ID> <本机路径>   认领一个云端项目到本机目录
  {app} ignore <项目ID>      忽略一个项目（已绑定的停止同步，文件都不删）
  {app} unignore <项目ID>    取消忽略
  {app} passwd               修改同步口令（所有设备一起换）；别的设备改过口令后，在本机输入新口令
  {app} uninstall            删除自动同步（hooks、定时任务、开机自启），不动会话和云端数据
  {app} version              显示版本

选项：
  -q, --quiet    静默，日志照样写进 sync.log
  --dry          只显示会做什么，不真正改动
  --cwd <目录>   只同步这个工作目录对应的项目
`), "{app}", appName))
}

// ---------- 子命令 ----------

// cliProvider 命令行初始化时可选的服务商：要问哪些参数、默认值是什么。
type cliProvider struct {
	id            string
	label         func() string
	defaultRegion string // 非空：问地域，endpoint 自动生成
	askEndpoint   bool   // 要手填 endpoint
	pathStyle     bool   // 默认用路径风格
}

var cliProviders = []cliProvider{
	{id: storage.ProviderAliyun, label: func() string { return i18n.T("阿里云 OSS") }, defaultRegion: "cn-shanghai"},
	{id: storage.ProviderTencent, label: func() string { return i18n.T("腾讯云 COS") }, defaultRegion: "ap-shanghai"},
	{id: storage.ProviderR2, label: func() string { return "Cloudflare R2" }, askEndpoint: true},
	{id: storage.ProviderAWS, label: func() string { return "AWS S3" }, defaultRegion: "us-east-1"},
	{id: storage.ProviderMinIO, label: func() string { return i18n.T("MinIO（自建）") }, askEndpoint: true, pathStyle: true},
	{id: storage.ProviderOther, label: func() string { return i18n.T("其他 S3 兼容服务") }, askEndpoint: true},
}

func cmdInit(ctx context.Context) error {
	fmt.Println(i18n.T("=== claude-code-sync 初始化 ==="))
	fmt.Println()

	p := service.InitParams{}
	if def := service.DefaultDeviceName(); def != "" {
		p.DeviceName = orDefault(prompt(fmt.Sprintf(i18n.T("给这台设备起个名字 [%s]: "), def)), def)
	} else {
		for p.DeviceName == "" {
			p.DeviceName = prompt(i18n.T("给这台设备起个名字（如 home-computer）: "))
		}
	}

	fmt.Println()
	fmt.Println(i18n.T("云端存储（S3 兼容对象存储），选择服务商："))
	for i, pv := range cliProviders {
		fmt.Printf("  %d) %s\n", i+1, pv.label())
	}
	choice := orDefault(prompt(i18n.T("请选择 [1]: ")), "1")
	var pv *cliProvider
	for i := range cliProviders {
		if choice == fmt.Sprint(i+1) {
			pv = &cliProviders[i]
		}
	}
	if pv == nil {
		return fmt.Errorf(i18n.T("不认识的选择 %q"), choice)
	}
	sc := storage.Config{Type: storage.TypeS3, Provider: pv.id, PathStyle: pv.pathStyle}
	switch {
	case pv.defaultRegion != "":
		sc.Region = orDefault(prompt(fmt.Sprintf(i18n.T("地域 [%s]: "), pv.defaultRegion)), pv.defaultRegion)
	case pv.id == storage.ProviderR2:
		sc.Endpoint = prompt(i18n.T("Endpoint（形如 https://<账户ID>.r2.cloudflarestorage.com）: "))
	default:
		sc.Endpoint = prompt(i18n.T("Endpoint（如 http://192.168.1.10:9000）: "))
		sc.Region = prompt(i18n.T("地域（不知道就直接回车）: "))
		def := "y/N"
		if pv.pathStyle {
			def = "Y/n"
		}
		if ans := strings.ToLower(prompt(fmt.Sprintf(i18n.T("使用路径风格 URL？MinIO 等自建服务通常需要 [%s]: "), def))); ans != "" {
			sc.PathStyle = ans == "y" || ans == "yes"
		}
	}
	sc.Bucket = prompt(i18n.T("Bucket 名称: "))
	sc.AccessKeyID = prompt("AccessKeyId: ")
	sc.AccessKeySecret = promptSecret("AccessKeySecret: ")
	p.Storage = sc

	fmt.Println()
	fmt.Println(i18n.T("设置一个同步口令：每台设备输入同一个口令，会话内容用它加密后才上传，云端看到的是密文。"))
	p.Passphrase = promptSecret(i18n.T("口令: "))
	if pass2 := promptSecret(i18n.T("再输一次确认: ")); p.Passphrase != pass2 {
		return errors.New(i18n.T("两次输入不一致"))
	}

	fmt.Print(i18n.T("测试云端连接... "))
	res, err := service.Initialize(ctx, p)
	if err != nil {
		fmt.Println(i18n.T("失败"))
		return err
	}
	fmt.Println(i18n.T("成功"))
	if res.Keycheck == "created" {
		fmt.Println(i18n.T("已写入云端口令校验文件"))
	} else {
		fmt.Println(i18n.T("口令与云端一致，可以和其他设备互相同步"))
	}

	switch err := service.EnsureAutoSync(); {
	case config.Sandboxed():
		fmt.Println(i18n.T("沙箱模式（设置了 CLAUDE_CODE_SYNC_HOME）：不安装自动同步，请手动运行 sync"))
	case err != nil:
		fmt.Println(i18n.T("自动同步安装失败（可以先手动 sync，稍后打开图形界面会再试一次）:"), err)
	default:
		fmt.Println(i18n.T("已装好自动同步：Claude Code 每次对话自动同步当前项目，另有每小时一次的全量兜底"))
	}

	fmt.Println()
	fmt.Println(i18n.T("初始化完成！设备: %s（ID %s）", res.Config.DeviceName, res.Config.DeviceID))
	fmt.Println(i18n.T("建议先运行 %s sync --dry 看看会做什么，再运行 %s sync", appName, appName))
	return nil
}

func cmdStatus(ctx context.Context) error {
	s := service.LoadStatus()
	if !s.Initialized {
		return config.ErrNotInitialized
	}
	ov, err := service.LoadProjects(ctx)
	if err != nil {
		return err
	}
	fmt.Println(i18n.T("本机: %s（ID %s）", s.DeviceName, s.DeviceID))
	if !s.AutoSync {
		fmt.Println(i18n.T("自动同步未就绪: %s", s.AutoSyncIssue))
	}
	fmt.Println()
	fmt.Println(i18n.T("已绑定的项目："))
	if len(ov.Bound) == 0 {
		fmt.Println(i18n.T("  （还没有，运行 sync 后会自动发现本机项目）"))
	}
	for _, b := range ov.Bound {
		fmt.Println(i18n.T("  [%s] %s\n      本机路径: %s", b.Kind, b.PID, b.Root))
	}
	fmt.Println()
	if ov.RemoteError != "" {
		return fmt.Errorf(i18n.T("读取云端设备信息失败: %s"), ov.RemoteError)
	}
	fmt.Println(i18n.T("其他设备："))
	if len(ov.Devices) == 0 {
		fmt.Println(i18n.T("  （还没有其他设备上报）"))
	}
	for _, d := range ov.Devices {
		fmt.Println(i18n.T("  %s（ID %s，%d 个项目）", d.Name, d.ID, d.Projects))
		for _, c := range ov.Claimable {
			if c.DeviceID == d.ID {
				fmt.Println(i18n.T("    待认领: %s\n      对方路径: %s (%s)", c.PID, c.Root, c.Display))
			}
		}
	}
	if len(ov.Claimable) > 0 {
		fmt.Println()
		fmt.Println(i18n.T("用 %s claim <项目ID> <本机路径> 认领，或 %s ignore <项目ID> 忽略", appName, appName))
	}
	return nil
}

// cmdHookEvent 由 Claude Code 的 hook 调用。hook 会把一段 JSON 通过 stdin 传进来，
// 里面有 cwd（当前工作目录）。这样就不依赖任何 shell 的变量语法，Windows 下也能用。
// stdout 的内容会被 Claude Code 当作附加上下文，所以只在"真的拉到了新内容"时才输出一句提示。
func cmdHookEvent(ctx context.Context) {
	var input struct {
		Cwd string `json:"cwd"`
	}
	data, _ := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	json.Unmarshal(data, &input)
	if input.Cwd == "" {
		if home, err := os.UserHomeDir(); err == nil {
			input.Cwd = home
		}
	}
	if hint := service.HookSync(ctx, input.Cwd); hint != "" {
		fmt.Println(hint)
	}
}

// cmdPasswd 修改同步口令。本机口令已经和云端对不上（别的设备改过）时，改为输入新口令。
func cmdPasswd(ctx context.Context) error {
	ok, err := service.PassphraseMatches(ctx)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println(i18n.T("同步口令已在其他设备上修改。请输入新口令（和那台设备上改的一样）："))
		if err := service.AdoptPassphrase(ctx, promptSecret(i18n.T("新口令: "))); err != nil {
			return err
		}
		fmt.Println(i18n.T("口令已更新，可以继续同步了"))
		return nil
	}
	fmt.Println(i18n.T("修改同步口令：云端的全部文件会用新口令重新加密，其他设备之后要输入新口令才能继续同步。"))
	oldPass := promptSecret(i18n.T("当前口令: "))
	newPass := promptSecret(i18n.T("新口令: "))
	if promptSecret(i18n.T("再输一次确认: ")) != newPass {
		return errors.New(i18n.T("两次输入不一致"))
	}
	err = service.ChangePassphrase(ctx, oldPass, newPass, func(done, total int) {
		fmt.Printf("\r"+i18n.T("重新加密云端文件 %d/%d"), done, total)
	})
	fmt.Println()
	if err != nil {
		return err
	}
	fmt.Println(i18n.T("口令已修改。请在其他设备上运行 %s passwd（或打开图形界面）输入新口令。", appName))
	return nil
}
