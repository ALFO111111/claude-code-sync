package i18n

// en 中文原文 → 英文。新增中文字符串时在这里补一条（i18n_test.go 会检查有没有漏、有没有多）。
// 按所在包分组，方便找。
var en = map[string]string{
	// 通用标点（拼接列表用）
	"，": ", ",
	"、": ", ",

	// config / crypt / state / storage
	"还没有初始化，请先运行: %s init（或双击打开图形界面）": "Not initialized yet. Run: %s init (or double-click to open the app)",
	"配置文件损坏 %s: %w":                                   "Config file is corrupted %s: %w",
	"读取密钥失败，请重新运行 init: %w":                           "Failed to read the key, please run init again: %w",
	"密钥文件损坏，请重新运行 init":                               "Key file is corrupted, please run init again",
	"不是 claude-code-sync 加密格式":                        "Not in claude-code-sync encrypted format",
	"解密失败（口令不对或文件被篡改）":                                "Decryption failed (wrong passphrase or the file was tampered with)",
	"另一个同步正在进行":                                       "Another sync is in progress",
	"本地存储需要 path":                                     "Local storage requires a path",
	"不支持的存储类型: %q":                                    "Unsupported storage type: %q",
	"S3 配置不完整（需要 Bucket、AccessKeyId、AccessKeySecret）": "Incomplete S3 settings (Bucket, AccessKeyId and AccessKeySecret are required)",
	"S3 配置不完整：请填写 Endpoint（阿里云、腾讯云、AWS 也可以只填地域）": "Incomplete S3 settings: please fill in the Endpoint (for Aliyun, Tencent Cloud and AWS the region alone is enough)",
	"Endpoint 格式不对，应该像 https://s3.example.com":   "Invalid Endpoint, it should look like https://s3.example.com",

	// platform
	"目前只支持 Windows 定时任务，其他系统请用 cron: */%d * * * * %s sync -q": "Only Windows scheduled tasks are supported for now. On other systems use cron: */%d * * * * %s sync -q",
	"安装定时任务 %s 失败: %v\n%s":                                    "Failed to install scheduled task %s: %v\n%s",
	"非 Windows 系统请手动删除 cron 里的同步条目":                           "On non-Windows systems, please remove the sync entry from cron manually",
	"删除定时任务 %s 失败：删除后仍然存在":                                    "Failed to remove scheduled task %s: it still exists after deletion",

	// engine（日志 + 错误）
	"%s 和 %s 是同一个仓库，后者作为独立项目同步": "%s and %s are the same repository; the latter is synced as a separate project",
	"发现项目 %s（%s）": "Found project %s (%s)",
	"项目 %s 在本机有 %d 个克隆，请用 %s claim %s <路径> 指定一个": "Project %s has %d clones on this machine; pick one with: %s claim %s <path>",
	"自动对上项目 %s → %s":          "Matched project %s → %s",
	"跳过 %s：本机目录 %s 已属于 %s":    "Skipped %s: local folder %s already belongs to %s",
	"已停止同步项目 %s（本机和云端的文件都没动）": "Stopped syncing project %s (no local or cloud files were touched)",
	"已忽略项目 %s":                "Ignored project %s",
	"已取消忽略项目 %s":              "Un-ignored project %s",
	"已认领 %s → %s":             "Claimed %s → %s",
	"读取云端设备信息失败: %w":          "Failed to read device info from the cloud: %w",
	"上传设备信息失败: %v":            "Failed to upload device info: %v",
	"输入历史同步失败: %v":            "Failed to sync input history: %v",
	"预览完成（未做任何改动）":            "Preview finished (nothing was changed)",
	"同步完成：上传 %d，下载 %d，合并 %d，分叉 %d，失败 %d，输入历史补进 %d 条，用时 %.1fs": "Sync finished: %d uploaded, %d downloaded, %d merged, %d forked, %d failed, %d input-history entries added, took %.1fs",
	"%d 个文件同步失败，详见日志 %s":                "%d files failed to sync, see the log %s",
	"列出云端输入历史失败: %w":                    "Failed to list input history in the cloud: %w",
	"将同步输入历史：拉下 %d 条，本地有新内容=%v":         "Will sync input history: pulling %d entries, local has new entries=%v",
	"输入历史正在被写入，本次跳过":                    "Input history is being written, skipped this time",
	"输入历史：从其他设备补进 %d 条":                 "Input history: added %d entries from other devices",
	"用户目录（未打开项目）":                       "Home folder (no project opened)",
	"列出云端文件失败: %w":                      "Failed to list cloud files: %w",
	"遍历 %s 失败（本次跳过该项目）: %v":             "Failed to scan %s (project skipped this time): %v",
	"失败 %s/%s: %v":                      "Failed %s/%s: %v",
	"将上传 %s/%s":                         "Will upload %s/%s",
	"云端 %s/%s 被其他设备并发覆盖，本次保留本地，下次同步时合并": "Cloud copy of %s/%s was overwritten by another device at the same time; kept the local copy, will merge on next sync",
	"将下载 %s/%s":                         "Will download %s/%s",
	"将比对合并 %s/%s":                       "Will compare and merge %s/%s",
	"会话 %s 在两台设备上各自继续了，另一份已另存为新会话 %s":   "Session %s was continued separately on two devices; the other copy was saved as new session %s",
	"备份失败 %s: %v":                       "Backup failed %s: %v",

	// watch
	"文件监听出错: %v": "File watcher error: %v",

	// service
	"安装 Claude Code hooks 失败: %w": "Failed to install Claude Code hooks: %w",
	"登录时自动同步的任务没装上（权限不够），每小时一次的定时同步不受影响": "Could not install the sync-at-logon task (insufficient permissions); the hourly scheduled sync is not affected",
	"自动同步安装失败: %v":                "Failed to set up automatic sync: %v",
	"补回 Claude Code hooks 失败: %v": "Failed to restore Claude Code hooks: %v",
	"Claude Code hooks 不见了（可能是 CC Switch 等工具改写了 settings.json），已自动补回": "Claude Code hooks were missing (settings.json was probably rewritten by a tool like CC Switch); restored automatically",
	"settings.json 解析失败 %s: %w": "Failed to parse settings.json %s: %w",
	"请填写设备名":                    "Please enter a device name",
	"口令不能为空":                    "Passphrase cannot be empty",
	"存储类型不能为空":                  "Storage type cannot be empty",
	"连不上云端，请检查配置和网络: %w":        "Cannot connect to the cloud, please check the settings and network: %w",
	"口令与云端不一致！云端已有其他设备用别的口令加密的数据，请输入相同的口令（或清空云端重新开始）": "Passphrase does not match the cloud! Another device already stored data encrypted with a different passphrase. Enter the same passphrase (or empty the cloud storage and start over)",
	"连接失败: %w":  "Connection failed: %w",
	"非法路径":      "Invalid path",
	"项目未绑定: %s": "Project is not bound: %s",
	"设备名不能为空":   "Device name cannot be empty",
	"沙箱模式（设置了 CLAUDE_CODE_SYNC_HOME），不安装自动同步，只能手动同步": "Sandbox mode (CLAUDE_CODE_SYNC_HOME is set): automatic sync is not installed, sync manually",
	"Claude Code hooks 未安装": "Claude Code hooks are not installed",
	"Windows 定时任务未安装":       "Windows scheduled task is not installed",
	"[claude-code-sync] 此项目的会话已从其他设备同步。记录中的路径 %s 即本机的 %s，请按本机路径理解。": "[claude-code-sync] Sessions of this project were synced from another device. The path %s in the history corresponds to %s on this machine; please interpret paths accordingly.",

	// gui
	"正在同步中，请等这一轮结束再试":     "A sync is in progress, please try again when it finishes",
	"请先选择本机目录":            "Please choose a local folder first",
	"界面还没就绪":              "The window is not ready yet",
	"选择本项目在本机的目录":         "Choose this project's folder on this machine",
	"立即同步":                "Sync now",
	"同步所有项目":              "Sync all projects",
	"打开主窗口":               "Open main window",
	"退出":                  "Quit",
	"退出 claude-code-sync": "Quit claude-code-sync",
	"还没设置，打开主窗口按向导完成":     "Not set up yet, open the main window to finish the setup wizard",
	"上次同步有失败，打开主窗口看日志":    "The last sync had failures, open the main window to see the log",
	"还没同步过":               "Never synced",
	"上次同步 %s":             "Last sync %s",
	"同步中…":                "Syncing…",
	"同步失败：%s":             "Sync failed: %s",

	// cli
	"错误:": "Error:",
	"用法: %s claim <项目ID> <本机路径>": "Usage: %s claim <project-id> <local-path>",
	"用法: %s ignore <项目ID>":       "Usage: %s ignore <project-id>",
	"用法: %s unignore <项目ID>":     "Usage: %s unignore <project-id>",
	"已删除自动同步（Claude Code hooks 和定时任务）。会话文件、配置和云端数据都没动。": "Automatic sync removed (Claude Code hooks and scheduled task). Session files, config and cloud data were not touched.",
	"注意：再次打开图形界面会自动重新安装。":                               "Note: opening the app again will reinstall it automatically.",
	"--cwd 需要参数": "--cwd requires an argument",
	"不认识的参数 %s":  "Unknown option %s",
	`{app} - 把 Claude Code 的会话在多台设备之间同步

用法：
  {app}                      （不带参数）打开图形界面
  {app} init                 初始化（设置口令、云端存储、本机信息），自动装好自动同步
  {app} sync                 双向同步（最常用）
  {app} sync --dry           只显示会做什么，不真正改动
  {app} status               查看本机项目绑定和云端待认领的项目
  {app} claim <项目ID> <本机路径>   认领一个云端项目到本机目录
  {app} ignore <项目ID>      忽略一个项目（已绑定的停止同步，文件都不删）
  {app} unignore <项目ID>    取消忽略
  {app} uninstall            删除自动同步（hooks 和定时任务），不动会话和云端数据
  {app} version              显示版本

选项：
  -q, --quiet    静默，日志照样写进 sync.log
  --dry          只显示会做什么，不真正改动
  --cwd <目录>   只同步这个工作目录对应的项目
`: `{app} - sync Claude Code sessions across multiple machines

Usage:
  {app}                      (no arguments) open the app
  {app} init                 set up (passphrase, cloud storage, this device) and enable automatic sync
  {app} sync                 two-way sync (the one you'll use most)
  {app} sync --dry           show what would happen without changing anything
  {app} status               show local project bindings and cloud projects waiting to be claimed
  {app} claim <project-id> <local-path>   bind a cloud project to a local folder
  {app} ignore <project-id>  ignore a project (stops syncing a bound one; no files are deleted)
  {app} unignore <project-id>  stop ignoring a project
  {app} uninstall            remove automatic sync (hooks and scheduled task); sessions and cloud data stay
  {app} version              show version

Options:
  -q, --quiet    quiet; the log is still written to sync.log
  --dry          show what would happen without changing anything
  --cwd <dir>    only sync the project of this working directory
`,
	"阿里云 OSS":                      "Alibaba Cloud OSS",
	"腾讯云 COS":                      "Tencent Cloud COS",
	"MinIO（自建）":                    "MinIO (self-hosted)",
	"其他 S3 兼容服务":                   "Other S3-compatible service",
	"=== claude-code-sync 初始化 ===": "=== claude-code-sync setup ===",
	"给这台设备起个名字 [%s]: ":             "Name this device [%s]: ",
	"给这台设备起个名字（如 home-computer）: ": "Name this device (e.g. home-computer): ",
	"云端存储（S3 兼容对象存储），选择服务商：":       "Cloud storage (S3-compatible object storage), choose a provider:",
	"请选择 [1]: ":                    "Choose [1]: ",
	"不认识的选择 %q":                    "Unknown choice %q",
	"地域 [%s]: ":                    "Region [%s]: ",
	"Endpoint（形如 https://<账户ID>.r2.cloudflarestorage.com）: ": "Endpoint (like https://<account-id>.r2.cloudflarestorage.com): ",
	"Endpoint（如 http://192.168.1.10:9000）: ":                 "Endpoint (e.g. http://192.168.1.10:9000): ",
	"地域（不知道就直接回车）: ":                                         "Region (press Enter if unsure): ",
	"使用路径风格 URL？MinIO 等自建服务通常需要 [%s]: ":                      "Use path-style URLs? Usually needed for self-hosted services like MinIO [%s]: ",
	"Bucket 名称: ": "Bucket name: ",
	"设置一个同步口令：每台设备输入同一个口令，会话内容用它加密后才上传，云端看到的是密文。": "Set a sync passphrase. Enter the same passphrase on every device; sessions are encrypted with it before upload, so the cloud only sees ciphertext.",
	"口令: ":        "Passphrase: ",
	"再输一次确认: ":    "Confirm passphrase: ",
	"两次输入不一致":     "The two entries do not match",
	"测试云端连接... ":  "Testing cloud connection... ",
	"失败":          "failed",
	"成功":          "OK",
	"已写入云端口令校验文件": "Wrote the passphrase check file to the cloud",
	"口令与云端一致，可以和其他设备互相同步":                                "Passphrase matches the cloud, ready to sync with your other devices",
	"沙箱模式（设置了 CLAUDE_CODE_SYNC_HOME）：不安装自动同步，请手动运行 sync": "Sandbox mode (CLAUDE_CODE_SYNC_HOME is set): automatic sync is not installed, run sync manually",
	"自动同步安装失败（可以先手动 sync，稍后打开图形界面会再试一次）:":                "Failed to set up automatic sync (you can run sync manually; the app will retry when opened):",
	"已装好自动同步：Claude Code 每次对话自动同步当前项目，另有每小时一次的全量兜底":      "Automatic sync is set up: the current project syncs on every Claude Code conversation, plus a full sync every hour as a safety net",
	"初始化完成！设备: %s（ID %s）":                                "Setup complete! Device: %s (ID %s)",
	"建议先运行 %s sync --dry 看看会做什么，再运行 %s sync":             "Tip: run %s sync --dry first to see what will happen, then run %s sync",
	"本机: %s（ID %s）": "This device: %s (ID %s)",
	"自动同步未就绪: %s":   "Automatic sync is not ready: %s",
	"已绑定的项目：":       "Bound projects:",
	"  （还没有，运行 sync 后会自动发现本机项目）":                        "  (none yet; local projects are discovered automatically when you run sync)",
	"  [%s] %s\n      本机路径: %s":                         "  [%s] %s\n      local path: %s",
	"读取云端设备信息失败: %s":                                    "Failed to read device info from the cloud: %s",
	"其他设备：":                                             "Other devices:",
	"  （还没有其他设备上报）":                                     "  (no other devices have reported yet)",
	"  %s（ID %s，%d 个项目）":                                "  %s (ID %s, %d projects)",
	"    待认领: %s\n      对方路径: %s (%s)":                  "    to claim: %s\n      their path: %s (%s)",
	"用 %s claim <项目ID> <本机路径> 认领，或 %s ignore <项目ID> 忽略": "Claim with %s claim <project-id> <local-path>, or ignore with %s ignore <project-id>",
}
