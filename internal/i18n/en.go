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
	"不是 agent-sync 加密格式":                              "Not in agent-sync encrypted format",
	"解密失败（口令不对或文件被篡改）":                                "Decryption failed (wrong passphrase or the file was tampered with)",
	"另一个同步正在进行":                                       "Another sync is in progress",
	"本地存储需要 path":                                     "Local storage requires a path",
	"不支持的存储类型: %q":                                    "Unsupported storage type: %q",
	"S3 配置不完整（需要 Bucket、AccessKeyId、AccessKeySecret）": "Incomplete S3 settings (Bucket, AccessKeyId and AccessKeySecret are required)",
	"S3 配置不完整：请填写 Endpoint（阿里云、腾讯云、AWS 也可以只填地域）": "Incomplete S3 settings: please fill in the Endpoint (for Aliyun, Tencent Cloud and AWS the region alone is enough)",
	"Endpoint 格式不对，应该像 https://s3.example.com":   "Invalid Endpoint, it should look like https://s3.example.com",

	// platform
	"安装定时任务 %s 失败: %v\n%s": "Failed to install scheduled task %s: %v\n%s",
	"删除定时任务 %s 失败：删除后仍然存在": "Failed to remove scheduled task %s: it still exists after deletion",

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
	"项目 %s 从 %s 搬到了 %s":       "Project %s moved from %s to %s",
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
	"全局内容同步失败: %v":                      "Global content sync failed: %v",
	"保存 CC Switch 检测状态失败: %v":           "Failed to save CC Switch detection state: %v",
	"全局文件 %s 正在被修改，本轮先跳过":               "Global file %s is being modified; skipped this round",
	"全局文件同步失败 %s/%s: %v":                "Global file sync failed %s/%s: %v",
	"列出云端全局文件失败: %w":                    "Failed to list global cloud files: %w",
	"遍历全局内容 %s 失败（本类别本轮跳过）: %v":         "Failed to scan global content %s (category skipped this round): %v",
	"列出云端结构化配置失败: %w":                   "Failed to list structured cloud config: %w",
	"将合并全局文件 %s/%s":                     "Will merge global file %s/%s",
	"全局文件 %s/%s 需要人工处理冲突":               "Global file %s/%s needs manual conflict resolution",
	"全局文件 %s/%s 两边都有不同修改，已放入冲突中心":       "Global file %s/%s changed differently on both sides; added to the conflict center",
	"将下载全局文件 %s/%s":                     "Will download global file %s/%s",
	"将上传全局文件 %s/%s":                     "Will upload global file %s/%s",
	"云端全局文件 %s/%s 被其他设备并发覆盖，下轮重新合并":     "Global cloud file %s/%s was overwritten by another device; will merge again next round",
	"清理已解决冲突记录失败 %s/%s: %v":             "Failed to clear resolved conflict record %s/%s: %v",
	"未知的全局内容类别":                         "Unknown global content category",
	"非法冲突 ID":                           "Invalid conflict ID",
	"本机文件在冲突产生后又改过，请先重新同步再处理":           "The local file changed after the conflict was recorded; sync again before resolving it",
	"云端文件在冲突产生后又改过，请先重新同步再处理":           "The cloud file changed after the conflict was recorded; sync again before resolving it",
	"手工合并的 settings 不符合格式: %w":          "Manually merged settings are invalid: %w",
	"手工合并的 MCP 不符合格式: %w":               "Manually merged MCP config is invalid: %w",
	"这个文件不能保留两份，请手工合并":                  "This file cannot be kept as two copies; please merge manually",
	"未知的冲突解决方式":                         "Unknown conflict resolution",
	"云端刚被其他设备覆盖，冲突尚未解决，请重试":             "The cloud copy was just overwritten by another device; the conflict is still open, please retry",
	"保留两份用的副本文件已经存在，请先重命名":              "The copy needed to keep both versions already exists; please rename it first",
	"非法全局内容路径":                          "Invalid global content path",
	"本机文件已经存在，请先同步或处理冲突":                "The local file already exists; sync or resolve its conflict first",
	"配置顶层必须是 JSON 对象":                   "The top-level config must be a JSON object",
	"配置 JSON 后面有多余内容":                   "Unexpected content after the config JSON",
	"settings 同步内容缺少 settings 对象":       "Synced settings content is missing the settings object",
	"MCP 同步内容缺少 mcpServers 对象":          "Synced MCP content is missing the mcpServers object",
	"%s 中有 %d 项含秘密或无法映射的本机路径，已保留在本机：%s": "%s: %d entries contain secrets or unmappable local paths and were kept on this machine: %s",

	// watch
	"文件监听出错: %v": "File watcher error: %v",

	// service
	"安装 Claude Code hooks 失败: %w": "Failed to install Claude Code hooks: %w",
	"登录时自动同步的任务没装上（权限不够），每小时一次的定时同步不受影响": "Could not install the sync-at-logon task (insufficient permissions); the hourly scheduled sync is not affected",
	"自动同步安装失败: %v": "Failed to set up automatic sync: %v",
	"删除改名前的定时任务/开机自启失败（不影响同步，下次再试）: %v":                               "Failed to remove the scheduled task / autostart left from before the rename (sync is not affected, will retry): %v",
	"补回 Claude Code hooks 失败: %v":                                     "Failed to restore Claude Code hooks: %v",
	"Claude Code hooks 不见了（可能是 CC Switch 等工具改写了 settings.json），已自动补回": "Claude Code hooks were missing (settings.json was probably rewritten by a tool like CC Switch); restored automatically",
	"settings.json 解析失败 %s: %w":                                       "Failed to parse settings.json %s: %w",
	"settings.json 刚被其他程序修改，本轮先不重写":                                   "settings.json was just changed by another program; it will not be rewritten this round",
	"未知的同步内容类别":                                                       "Unknown synced content category",
	"这个类别可能也在由 CC Switch 同步，请先选择由谁负责":                                 "CC Switch may also sync this category; choose which tool owns it first",
	"这个类别不需要选择 CC Switch 负责人":                                         "This category does not need a CC Switch owner",
	"未知的同步负责人":                                                        "Unknown sync owner",
	"请填写设备名":                                                          "Please enter a device name",
	"口令不能为空":                                                          "Passphrase cannot be empty",
	"存储类型不能为空":                                                        "Storage type cannot be empty",
	"连不上云端，请检查配置和网络: %w":                                              "Cannot connect to the cloud, please check the settings and network: %w",
	"口令与云端不一致！云端已有其他设备用别的口令加密的数据，请输入相同的口令（或清空云端重新开始）": "Passphrase does not match the cloud! Another device already stored data encrypted with a different passphrase. Enter the same passphrase (or empty the cloud storage and start over)",
	"连接失败: %w":        "Connection failed: %w",
	"非法路径":            "Invalid path",
	"不认识的工具: %s":      "Unknown tool: %s",
	"这个工具还不支持浏览会话":    "Browsing sessions is not supported for this tool yet",
	"找不到这个会话（可能已被删除）": "Session not found (it may have been deleted)",
	"设备名不能为空":         "Device name cannot be empty",
	"沙箱模式（设置了 AGENT_SYNC_HOME），不安装自动同步，只能手动同步": "Sandbox mode (AGENT_SYNC_HOME is set): automatic sync is not installed, sync manually",
	"Claude Code hooks 未安装": "Claude Code hooks are not installed",
	"[agent-sync] 此项目的会话已从其他设备同步。记录中的路径 %s 即本机的 %s，请按本机路径理解。": "[agent-sync] Sessions of this project were synced from another device. The path %s in the history corresponds to %s on this machine; please interpret paths accordingly.",

	// gui
	"正在同步中，请等这一轮结束再试":  "A sync is in progress, please try again when it finishes",
	"请先选择本机目录":         "Please choose a local folder first",
	"界面还没就绪":           "The window is not ready yet",
	"选择本项目在本机的目录":      "Choose this project's folder on this machine",
	"立即同步":             "Sync now",
	"同步所有项目":           "Sync all projects",
	"打开主窗口":            "Open main window",
	"退出":               "Quit",
	"退出 agent-sync":    "Quit agent-sync",
	"还没设置，打开主窗口按向导完成":  "Not set up yet, open the main window to finish the setup wizard",
	"上次同步有失败，打开主窗口看日志": "The last sync had failures, open the main window to see the log",
	"还没同步过":            "Never synced",
	"上次同步 %s":          "Last sync %s",
	"同步中…":             "Syncing…",
	"同步失败：%s":          "Sync failed: %s",

	// cli
	"错误:": "Error:",
	"用法: %s claim <项目ID> <本机路径>": "Usage: %s claim <project-id> <local-path>",
	"用法: %s ignore <项目ID>":       "Usage: %s ignore <project-id>",
	"用法: %s unignore <项目ID>":     "Usage: %s unignore <project-id>",
	"已删除自动同步（Claude Code hooks、定时任务和开机自启）。会话文件、配置和云端数据都没动。": "Automatic sync removed (Claude Code hooks, scheduled task and start-at-login). Session files, config and cloud data were not touched.",
	"注意：再次打开图形界面会自动重新安装。":                                   "Note: opening the app again will reinstall it automatically.",
	"--cwd 需要参数": "--cwd requires an argument",
	"不认识的参数 %s":  "Unknown option %s",
	"以下类别可能也在由 CC Switch 同步，本轮已暂停，请用 content owner 选择负责人:": "These categories may also be synced by CC Switch and were paused this round; choose an owner with content owner:",
	"用法: %s restore-content [工具] <类别> <相对路径>":              "Usage: %s restore-content [tool] <category> <relative-path>",
	"CC Switch Auto Sync:": "CC Switch Auto Sync:",
	"关闭":                   "off",
	"开启":                   "on",
	"待选择负责人（本轮暂停）":         "owner needed (paused this round)",
	"用法: %s content [enable|disable [工具] <类别> | owner <类别> sync|ccswitch]": "Usage: %s content [enable|disable [tool] <category> | owner <category> sync|ccswitch]",
	"没有待解决的冲突": "No unresolved conflicts",
	"用 resolve 命令选择保留哪一边；手工合并请在图形界面操作。":                                    "Use resolve to choose a side; use the app for manual merging.",
	"用法: %s resolve <冲突ID> local|remote|both，或 resolve <冲突ID> merged <文件>": "Usage: %s resolve <conflict-id> local|remote|both, or resolve <conflict-id> merged <file>",
	"merged 需要提供一个手工合并后的文件":                                                "merged requires a manually merged file",
	`{app} - 把 Claude Code 的会话在多台设备之间同步

用法：
  {app}                      （不带参数）打开图形界面
  {app} init                 初始化（设置口令、云端存储、本机信息），自动装好自动同步
  {app} sync                 双向同步（最常用）
  {app} sync --dry           只显示会做什么，不真正改动
  {app} status               查看本机项目绑定和云端待认领的项目
  {app} content              查看全局同步内容开关
  {app} content enable|disable [工具] <类别>   开关一个全局内容类别（工具：claude / codex）
  {app} content owner <类别> sync|ccswitch   选择可能重复的类别由谁同步
  {app} conflicts            列出待解决的冲突
  {app} resolve <冲突ID> local|remote|both   解决冲突
  {app} resolve <冲突ID> merged <文件>       使用手工合并后的文件解决
  {app} restore-content [工具] <类别> <相对路径>    从云端手动恢复本机已删除的全局文件
  {app} claim <项目ID> <本机路径>   认领一个云端项目到本机目录
  {app} ignore <项目ID>      忽略一个项目（已绑定的停止同步，文件都不删）
  {app} unignore <项目ID>    取消忽略
  {app} passwd               修改同步口令（所有设备一起换）；别的设备改过口令后，在本机输入新口令
  {app} uninstall            删除自动同步（hooks、定时任务、开机自启），不动会话和云端数据
  {app} version              显示版本
  {app} update               检查更新，有新版本就下载安装（--dry 只检查）

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
  {app} content              show which global content is synced
  {app} content enable|disable [tool] <category>   turn a global content category on or off (tool: claude / codex)
  {app} content owner <category> sync|ccswitch   choose which tool syncs a possibly duplicated category
  {app} conflicts            list unresolved conflicts
  {app} resolve <conflict-id> local|remote|both   resolve a conflict
  {app} resolve <conflict-id> merged <file>       resolve with a manually merged file
  {app} restore-content [tool] <category> <relative-path>   restore a locally deleted global file from the cloud
  {app} claim <project-id> <local-path>   bind a cloud project to a local folder
  {app} ignore <project-id>  ignore a project (stops syncing a bound one; no files are deleted)
  {app} unignore <project-id>  stop ignoring a project
  {app} passwd               change the sync passphrase (for all devices); after another device changed it, enter the new one here
  {app} uninstall            remove automatic sync (hooks, scheduled task, start-at-login); sessions and cloud data stay
  {app} version              show version
  {app} update               check for updates and install a newer version (--dry: only check)

Options:
  -q, --quiet    quiet; the log is still written to sync.log
  --dry          show what would happen without changing anything
  --cwd <dir>    only sync the project of this working directory
`,
	"阿里云 OSS":                      "Alibaba Cloud OSS",
	"腾讯云 COS":                      "Tencent Cloud COS",
	"MinIO（自建）":                    "MinIO (self-hosted)",
	"其他 S3 兼容服务":                   "Other S3-compatible service",
	"=== agent-sync 初始化 ===":       "=== agent-sync setup ===",
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
	"口令与云端一致，可以和其他设备互相同步":                           "Passphrase matches the cloud, ready to sync with your other devices",
	"沙箱模式（设置了 AGENT_SYNC_HOME）：不安装自动同步，请手动运行 sync":  "Sandbox mode (AGENT_SYNC_HOME is set): automatic sync is not installed, run sync manually",
	"自动同步安装失败（可以先手动 sync，稍后打开图形界面会再试一次）:":           "Failed to set up automatic sync (you can run sync manually; the app will retry when opened):",
	"已装好自动同步：Claude Code 每次对话自动同步当前项目，另有每小时一次的全量兜底": "Automatic sync is set up: the current project syncs on every Claude Code conversation, plus a full sync every hour as a safety net",
	"初始化完成！设备: %s（ID %s）":                           "Setup complete! Device: %s (ID %s)",
	"建议先运行 %s sync --dry 看看会做什么，再运行 %s sync":        "Tip: run %s sync --dry first to see what will happen, then run %s sync",
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

	// 2026-10：只追加拉取、备份、修改口令、开机自启、会话搜索
	"本机文件刚被修改": "The local file was just modified",
	"%s/%s 正在被修改，本轮先跳过，下次同步再处理":   "%s/%s is being modified; skipped this round, will retry next sync",
	"找不到这份备份（可能已超过保留期被清理）":        "Backup not found (it may have been cleaned up after the retention period)",
	"这条记录没有备份文件":                  "This entry has no backup file",
	"已从备份恢复 %s/%s":                "Restored %s/%s from backup",
	"同步口令已在其他设备上修改，请在本机输入新口令后再同步": "The sync passphrase was changed on another device. Enter the new passphrase on this device to continue syncing",
	"口令不对": "Wrong passphrase",
	"读取云端口令校验文件失败: %w":                    "Failed to read the passphrase check file from the cloud: %w",
	"云端的同步口令已在其他设备上修改，本机暂停同步，等待输入新口令":     "The sync passphrase was changed on another device; syncing is paused until the new passphrase is entered",
	"新口令和原来的一样":                           "The new passphrase is the same as the current one",
	"改口令：%s 用原口令也解不开，跳过":                  "Passphrase change: %s can't be decrypted with the old passphrase either, skipped",
	"改口令：%s 重新加密失败: %v":                   "Passphrase change: failed to re-encrypt %s: %v",
	"有 %d 个文件没能重新加密（网络问题？），口令还没改成功，请再试一次": "%d files could not be re-encrypted (network problem?). The passphrase has not been changed yet, please try again",
	"同步口令已修改：%d 个云端文件已用新口令重新加密":           "Sync passphrase changed: %d cloud files were re-encrypted with the new passphrase",
	"有 %d 个云端文件原来就解不开，没有处理":               "%d cloud files could not be decrypted before the change and were left as they were",
	"已换上新的同步口令":                           "Switched to the new sync passphrase",
	"沙箱模式下不能设置开机自启":                       "Start-at-login can't be changed in sandbox mode",
	"设置开机自启失败: %w":                        "Failed to set up start-at-login: %w",
	"只有顶层会话能继续":                           "Only top-level sessions can be continued",
	"项目目录不存在: %s":                         "Project folder does not exist: %s",
	"[agent-sync] 会话同步已暂停：同步口令在其他设备上改过了，请打开 agent-sync 输入新口令。": "[agent-sync] Session sync is paused: the sync passphrase was changed on another device. Please open agent-sync and enter the new passphrase.",
	"同步已暂停：口令在其他设备上改了，打开主窗口输入新口令":                              "Sync paused: the passphrase was changed on another device. Open the main window to enter the new one",
	"同步口令已在其他设备上修改。请输入新口令（和那台设备上改的一样）：":                        "The sync passphrase was changed on another device. Enter the new passphrase (the same one set there):",
	"新口令: ": "New passphrase: ",
	"口令已更新，可以继续同步了": "Passphrase updated, syncing can continue",
	"修改同步口令：云端的全部文件会用新口令重新加密，其他设备之后要输入新口令才能继续同步。": "Change the sync passphrase: all cloud files will be re-encrypted with the new passphrase, and other devices will need the new passphrase to keep syncing.",
	"当前口令: ":         "Current passphrase: ",
	"重新加密云端文件 %d/%d": "Re-encrypting cloud files %d/%d",
	"口令已修改。请在其他设备上运行 %s passwd（或打开图形界面）输入新口令。": "Passphrase changed. On your other devices, run %s passwd (or open the app) and enter the new passphrase.",

	// 2026-10：Codex 会话
	"发现 Codex 项目 %s（%s）":                        "Found Codex project %s (%s)",
	"列出云端 Codex 会话失败: %w":                       "Failed to list Codex sessions in the cloud: %w",
	"遍历 Codex 会话目录失败（本轮跳过 Codex 会话）: %w":        "Failed to scan the Codex sessions folder (Codex sessions skipped this round): %w",
	"Codex 会话上传失败 %s: %v":                       "Failed to upload Codex session %s: %v",
	"Codex 会话 %s 正在使用，本轮先跳过":                    "Codex session %s is in use; skipped this round",
	"Codex 会话下载失败 %s: %v":                       "Failed to download Codex session %s: %v",
	"将上传 Codex 会话 %s":                           "Would upload Codex session %s",
	"不是有效的 Codex 会话文件: %s":                      "Not a valid Codex session file: %s",
	"Codex 正在使用这个会话":                            "Codex is using this session",
	"将比对 Codex 会话 %s（来自 %s）":                    "Would compare Codex session %s (from %s)",
	"Codex 会话分叉 %s 和 %s 上的版本对不上，对方的版本已放进备份":     "Codex session fork %s does not match the version on %s; that version was saved to backups",
	"会话所在的项目在本机还没有对上":                           "The session's project is not linked on this device yet",
	"Codex 会话 %s 在本机和 %s 上各自继续了，对方那份已另存为新会话 %s": "Codex session %s was continued both here and on %s; the other copy was saved as new session %s",
	"Codex 会话同步失败: %v":                          "Codex session sync failed: %v",
	// 2026-10：macOS
	"在多台电脑之间同步 Claude Code 和 Codex":                     "Sync Claude Code and Codex across your computers",
	"这个系统还不支持自动安装定时同步，请用 cron: */%d * * * * %s sync -q": "Automatic scheduled sync is not supported on this system yet; use cron: */%d * * * * %s sync -q",
	"launchctl 加载 %s 失败（下次登录时生效）: %v %s":                "launchctl could not load %s (it will take effect at next login): %v %s",
	"定时同步任务未安装":                                         "Scheduled sync task is not installed",
	"文件监听启动失败: %v":                                      "Failed to start file watcher: %v",
	"后台同步已启动":                                           "Background sync started",
	// 2026-10：检查更新
	"连不上 GitHub: %w": "Could not reach GitHub: %w",
	"GitHub 返回 %s":   "GitHub returned %s",
	"这个系统没有可以自动安装的版本，请到发布页手动下载":      "There is no automatically installable build for this system; please download it from the release page",
	"发布里没有 SHA256SUMS.txt，没法校验下载的文件": "The release has no SHA256SUMS.txt, so the download cannot be verified",
	"下载的文件校验不通过（可能下载不完整），请重试":        "The downloaded file failed verification (it may be incomplete); please try again",
	"下载失败: %w":              "Download failed: %w",
	"下载失败: %s":              "Download failed: %s",
	"SHA256SUMS.txt 里没有 %s": "SHA256SUMS.txt does not list %s",
	"没有权限写 %s，没法自动更新，请到发布页手动下载": "No permission to write to %s, so the app cannot update itself; please download it from the release page",
	"不是从 App 里运行的，没法自动更新":       "Not running from an app bundle, so it cannot update itself",
	"打开安装包失败: %v %s":            "Could not open the installer: %v %s",
	"安装包里没有找到 App":              "No app found in the installer",
	"已是最新版本":                    "You're up to date",
	"请先完成初始化":                   "Please finish setup first",
	"检查更新失败: %v":                "Update check failed: %v",
	"发现新版本 %s（当前 %s）":           "New version %s available (current %s)",
	"正在更新，请稍候":                  "Updating, please wait",
	"请先检查更新":                    "Please check for updates first",
	"更新失败: %v":                  "Update failed: %v",
	"已更新到 %s，正在重新打开":            "Updated to %s, reopening",
	"当前是开发版，不自动更新。最新正式版: %s":    "This is a development build and does not update itself. Latest release: %s",
	"已是最新版本（%s）":                "You're up to date (%s)",
	"发现新版本 %s（当前 %s）: %s":       "New version %s available (current %s): %s",
	"下载中":                       "Downloading",
	"已更新到 %s。正在运行的图形界面要关掉重新打开才是新版本。": "Updated to %s. A running app window must be closed and reopened to use the new version.",
}
