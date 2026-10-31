# Changelog

本文件记录每个版本的变化，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。
发布时 GitHub Actions 会把对应版本这一节作为 Release 说明。

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.3.3] - 2026-10-31

### 新增 / Added

- 检查更新：打开程序时和之后每天一次去 GitHub 查新版本，有新版本时侧边栏“设置”挂红点；“设置 → 关于”里一键更新（下载 → 按 SHA256SUMS.txt 校验 → 替换 → 重新打开；Mac 上后台进程也一起换），“以后再说”跳过这个版本，“设置 → 通用”可关掉自动检查。命令行 `update`（`--dry` 只检查）。访问 GitHub 会用系统代理
  Update check: the app looks for a new version on GitHub when it opens and once a day after that, and puts a red dot on "Settings" when there is one; Settings → About updates in one click (download → verify against SHA256SUMS.txt → replace → reopen; on the Mac the background process is switched too), "Not now" skips that version, and automatic checks can be turned off in Settings → General. Command line: `update` (`--dry` to only check). GitHub is reached through the system proxy

## [0.3.2] - 2026-10-31

### 修复 / Fixed

- 同步途中程序被强制结束（任务管理器结束进程、崩溃、断电）后，留下的同步锁要 15 分钟才失效，这期间同步和安装 hooks 都会失败；现在发现锁文件里的进程已经不在，马上接管
  If the app was killed mid-sync (Task Manager, crash, power loss), the leftover sync lock blocked syncing and hook installation for 15 minutes; a lock whose process no longer exists is now taken over immediately
- 换了 exe 的位置后第一次打开，hooks 可能因为撞上首次同步而没改过来；现在每轮同步结束后都会检查并补上
  After moving the exe, the first launch could fail to update the hooks because the first sync held the lock; hooks are now checked and repaired after every sync

## [0.3.1] - 2026-10-31

### 变更 / Changed

- macOS 改用 DMG 安装包（`claude-code-sync-<版本>-macos-arm64.dmg`）：打开后把 App 拖到“应用程序”图标上即可，不再提供 zip。Mac 版只支持 Apple 芯片，不再支持 Intel 芯片的 Mac
  macOS now ships as a DMG installer (`claude-code-sync-<version>-macos-arm64.dmg`): open it and drag the app onto the Applications icon; the zip is gone. The Mac build is Apple silicon only, Intel Macs are no longer supported

### 修复 / Fixed

- 路径翻译漏了 Git Bash 写法：Windows 上 Claude Code 用 Bash 跑过命令后，会话里的 cwd 会记成 `/d/code/x`，以前原样拉到 Mac 上，现在会换成 Mac 的项目路径（到另一台 Windows 上换成那台的 `/e/work/x`）
  Path translation missed the Git Bash form: after Claude Code on Windows runs a Bash command the session records cwd as `/d/code/x`, which used to reach the Mac unchanged; it is now mapped to the Mac's project path (or to `/e/work/x` on another Windows machine)

### 文档 / Docs

- macOS 15 及以上首次打开未公证的 App 要在“系统设置 → 隐私与安全性”点“仍要打开”（右键 → 打开已不再放行）
  On macOS 15 and later, the first launch of the unnotarized app is allowed via System Settings → Privacy & Security → Open Anyway (right-click → Open no longer works there)

## [0.3.0] - 2026-10-31

### 新增 / Added

- 支持 macOS 13 及以上（Claude Code 和 Codex 都支持）：一个通用版 App 同时支持 Intel 和 Apple 芯片（Release 里的 `claude-code-sync-<版本>-macos-universal.zip`）。Mac 版没有菜单栏图标，关闭窗口就是退出；后台实时同步由登录时自动启动的无窗口进程 `claude-code-sync daemon` 负责（LaunchAgent `com.alfo111111.claude-code-sync.daemon`，设置页“开机自启”控制它），每小时兜底同步由 LaunchAgent `com.alfo111111.claude-code-sync.sync` 负责，都在 `~/Library/LaunchAgents`，不需要管理员权限。App 未经 Apple 公证（只有 ad-hoc 签名），第一次打开需在“访达”里右键 → 打开
  macOS 13+ support (both Claude Code and Codex): one universal app for Intel and Apple silicon (`claude-code-sync-<version>-macos-universal.zip` in the release). There is no menu bar icon on the Mac and closing the window quits the app; background sync is done by a windowless `claude-code-sync daemon` started at login (LaunchAgent `com.alfo111111.claude-code-sync.daemon`, controlled by the "Start at login" switch), and the hourly safety-net sync by the LaunchAgent `com.alfo111111.claude-code-sync.sync`, both in `~/Library/LaunchAgents` with no admin rights needed. The app is not notarized by Apple (ad-hoc signature only), so the first launch needs right-click → Open in Finder
- Windows ↔ macOS 路径自动翻译：项目根路径换成本机的，后面的分隔符也一起换（`D:\code\x\src\a.go` ↔ `/Users/me/code/x/src/a.go`），适用于 Claude 会话、输入历史、MCP 路径占位和 Codex 会话
  Windows ↔ macOS path translation: the project root is replaced with the local one and the separators after it are converted too (`D:\code\x\src\a.go` ↔ `/Users/me/code/x/src/a.go`), for Claude sessions, input history, MCP path placeholders and Codex sessions
- Codex 会话同步（新类别 `codex/sessions`，默认开）：同步 `$CODEX_HOME/sessions/**/rollout-*.jsonl`，在另一台电脑上接着聊。云端按设备分开存（`tools/codex/sessions/<设备ID>/...`），每台只写自己那份；另一台接着聊时只追加多出来的行，两台各自续写时对方那份另存成新会话；追加前先拿 Codex 自己的会话写锁，Codex 正开着这个会话就本轮跳过；本机删除 / 归档的会话不再拉回；项目还没对上时先留在云端。SQLite 索引和 `config.toml` 不同步，恢复时目标电脑要有同名的模型供应商
  Codex session sync (new category `codex/sessions`, on by default): syncs `$CODEX_HOME/sessions/**/rollout-*.jsonl` so you can continue on another machine. The cloud keeps one copy per device (`tools/codex/sessions/<device-id>/...`) and each device only writes its own; when the other machine continued a session only the new lines are appended, and if both continued it differently the other copy is saved as a new session; Codex's own per-session writer lock is taken before appending, so a session open in Codex is skipped for this round; sessions deleted or archived locally are not pulled back; sessions of a project not yet matched stay in the cloud until it is. SQLite indexes and `config.toml` are not synced, so the target machine needs a model provider with the same name
- Codex 旧位置技能（新类别 `codex/legacy-skills`，默认开）：`$CODEX_HOME/skills`（Codex 仍会读取），排除 Codex 自带的 `.system`
  Codex legacy skills (new category `codex/legacy-skills`, on by default): `$CODEX_HOME/skills`, which Codex still reads, excluding Codex's built-in `.system`
- Codex 全局规则与技能：同步 `$CODEX_HOME/AGENTS.md`、`AGENTS.override.md`（默认 `~/.codex`）和 `~/.agents/skills`，云端放在 `tools/codex/files/rules/...`、`tools/codex/files/skills/...`。复用加密、全量同步、独立类别开关、三方文本合并、冲突解决、备份恢复、删除不传播、Dry、改口令和文件监听；CC Switch 只影响 Claude。Codex 的 history/session_index、SQLite/WAL、config.toml/MCP、auth、插件/hooks 不同步，项目级 AGENTS/skills 仍由 Git 管理
  Codex global rules and skills: sync `$CODEX_HOME/AGENTS.md`, `AGENTS.override.md` (`~/.codex` by default) and `~/.agents/skills` to `tools/codex/files/rules/...` and `tools/codex/files/skills/...`. Reuses encryption, full sync, independent category switches, three-way text merge, conflict resolution, backup/restore, non-propagating deletions, Dry, passphrase changes and file watching; CC Switch only affects Claude. Codex history/session_index, SQLite/WAL, config.toml/MCP, auth and plugins/hooks are not synced; project-level AGENTS/skills remain managed by Git
- 同步个人配置：全局规则 `CLAUDE.md`、`plans/`、`skills/`、`agents/`、`commands/`、`tasks/`、用户级 MCP 服务器、settings.json 的安全字段（`language`、`autoMemoryEnabled`、`includeCoAuthoredBy`）。模型、Key、hooks、权限、MCP 的密钥和环境变量不上传；设置页“同步内容”可逐类开关
  Sync personal config: global `CLAUDE.md`, `plans/`, `skills/`, `agents/`, `commands/`, `tasks/`, user-level MCP servers and safe settings.json fields (`language`, `autoMemoryEnabled`, `includeCoAuthoredBy`). Models, keys, hooks, permissions and MCP secrets/env vars are never uploaded; each category can be turned off under Settings → What to sync
- 三方合并：记住上次两边一致的版本，不同行 / 不同 JSON 字段 / 不同任务 ID 的修改自动合并；改到同一处时进新的“冲突”页（保留本机 / 用另一台的 / 两份都留 / 手工合并），本机文件在此期间保持可用
  Three-way merge: changes to different lines, JSON fields or task IDs merge automatically; edits to the same spot go to the new Conflicts page (keep mine / use theirs / keep both / merge manually) while the local file stays usable
- CC Switch 共存：检测到 CC Switch 自动同步开着时，全局规则、技能、MCP 先暂停，在设置页选择由谁负责；没开就不提示
  CC Switch coexistence: when CC Switch auto sync is on, global rules, skills and MCP pause until you choose an owner in Settings; nothing is shown otherwise
- 命令行：`content`、`conflicts`、`resolve`、`restore-content`；新增 `content enable|disable codex <类别>`（`rules` / `skills` / `legacy-skills` / `sessions`）和 `restore-content codex <类别> <相对路径>`，旧的 Claude 裸类别写法保留
  CLI: `content`, `conflicts`, `resolve`, `restore-content`; adds `content enable|disable codex <category>` (`rules` / `skills` / `legacy-skills` / `sessions`) and `restore-content codex <category> <relative-path>`, while keeping the legacy bare-category forms for Claude
- 发布：Release 同时提供 Windows（exe + zip）和 macOS（universal zip），`SHA256SUMS.txt` 包含全部文件
  Releases: Windows (exe + zip) and macOS (universal zip) are published together, with every file listed in `SHA256SUMS.txt`

### 改进 / Changed

- 个人配置的删除不会传播：一台电脑删掉的文件不会删其他电脑的，本机也不会被拉回来（`restore-content` 可手动拉回）
  Deleting personal config never propagates: other machines keep the file and it is not pulled back locally (`restore-content` brings it back on purpose)
- hooks 自愈和同步改写 settings.json 时共用一把锁，并在写入前确认文件没被其他程序（如 CC Switch）刚改过
  Hook self-repair and sync share a lock when rewriting settings.json and check it was not just changed by another program (such as CC Switch)
- 文件监听：启动时还不存在的目录（如第一次建 `skills/`）创建后会自动开始监听
  File watching: folders that don't exist yet at startup (such as a first `skills/`) are picked up once created
- 安全加固：云端相对路径里出现 `\`、`:` 或以 `/` 开头的一律拒绝（防止在 macOS 上被当成普通文件名放行）；MCP 配置里的 Windows 绝对路径在 Mac 上也能识别，转换不了的留在本机、不上传
  Hardening: cloud relative paths containing `\` or `:`, or starting with `/`, are always rejected (so they can't slip through as plain file names on macOS); Windows absolute paths in MCP config are recognized on the Mac too, and ones that can't be translated stay local and are not uploaded

### 修复 / Fixed

- GitHub CI 从 v0.2.0 之后一直失败：不同 Go 版本的 gofmt 对中文字符的对齐算法不一样。现在 `go.mod` 统一为 go 1.27.2，CI 用 `go.mod` 指定版本的 gofmt，并同时在 Windows 和 macOS 上运行
  GitHub CI had been failing since v0.2.0 because gofmt aligns CJK characters differently across Go versions. `go.mod` now pins go 1.27.2, CI uses the gofmt of that version, and CI runs on both Windows and macOS
- 双击 `打包.bat` 时找不到 Go 就失败：现在会到常见安装位置查找（`GOROOT`、`C:\Program Files\Go`、`%LOCALAPPDATA%\Programs\Go`、`%USERPROFILE%\sdk\go*` 等），并允许 Go 自动下载 `go.mod` 要求的 1.27.2；打包后自动还原 `frontend/package.json.md5`
  `打包.bat` failed when Go was not on PATH: it now looks in common install locations (`GOROOT`, `C:\Program Files\Go`, `%LOCALAPPDATA%\Programs\Go`, `%USERPROFILE%\sdk\go*`, …) and lets Go download the 1.27.2 toolchain required by `go.mod`; `frontend/package.json.md5` is restored after the build

## [0.2.0] - 2026-10-19

### 新增 / Added

- 备份页：同步时被换下来的文件（冲突时另一台的版本、下载覆盖前本机的旧版本）和会话分叉都列在这里，可以和本机当前版本逐行对比、一键恢复（恢复前的版本也会备份）
  Backups page: replaced files (the other machine's version on conflict, the local version before a download overwrote it) and session forks are listed with a line-by-line comparison and one-click restore (the version being replaced is backed up too)
- 修改同步口令（设置页或 `passwd` 命令）：云端全部文件用新口令重新加密，中断后可重来；其他设备发现口令变了会暂停同步，输入新口令后继续
  Change the sync passphrase (Settings or `passwd`): all cloud files are re-encrypted, resumable if interrupted; other devices pause until the new passphrase is entered
- 开机自启：登录后在托盘里后台运行（默认开，设置页可关）；只允许运行一个实例
  Start at login in the tray (on by default, can be turned off in Settings); only one instance runs at a time
- 会话页：显示会话标题、标出分叉副本、跨项目搜索聊天内容、一键在终端里继续（`claude --resume`）或复制命令
  Sessions page: session titles, forked copies marked, search across all projects, continue in a terminal with one click or copy the command
- 首页：同步进度、本轮改动明细、其他设备最近活跃时间、云端空间占用
  Home: sync progress, per-sync change details, last-active time of other devices, cloud storage usage
- 深色模式（跟随系统 / 浅色 / 深色）；日志页可以只看问题、搜索、实时刷新、按轻重上色
  Dark mode (system / light / dark); the log page can filter to problems, search, refresh live and color lines by severity

### 改进 / Changed

- 拉取会话改成只往本机文件末尾追加缺的记录，不再整份替换：Claude Code 正在写的那一句不会被吞掉；其他文件替换前会确认没被刚刚改过，并先备份本机旧版本
  Pulling a session now only appends the missing records instead of replacing the whole file, so a line Claude Code is writing can't be lost; other files are checked for concurrent edits and backed up before being replaced
- 上传后的并发校验改用 HEAD 请求比对 etag，不再把整份文件下载回来，流量减半
  Post-upload verification compares the etag via a HEAD request instead of downloading the whole file again, halving traffic
- 会话最后一行还没写完时这一轮先跳过，不再误判成分叉
  A session whose last line is still being written is skipped for this round instead of being mistaken for a fork

### 修复 / Fixed

- 通过短文件名路径（如 `C:\Users\ABCDEF~1\...`）打开仓库里的子文件夹时，项目 ID 会丢掉子目录，导致和其他设备对不上
  Opening a repository subfolder through a short (8.3) path no longer drops the subfolder from the project ID, which kept it from matching other devices

## [0.1.0] - 2026-10-13

第一个公开版本。 / First public release.

### 新增 / Added

- 在多台 Windows 电脑之间同步 Claude Code 会话（`~/.claude/projects`）、子代理记录、项目记忆（MEMORY.md）和输入历史（`history.jsonl`）
  Sync Claude Code sessions, sub-agent transcripts, project memory and input history across Windows machines
- 项目身份证：git 项目按远程地址自动匹配，普通文件夹手动认领；拉到本机时自动翻译路径
  Project identity: git projects match by remote URL, plain folders are claimed once; paths are translated on download
- 端到端加密（Argon2id + AES-256-GCM）
  End-to-end encryption (Argon2id + AES-256-GCM)
- 冲突处理：子集取多、各自续聊则分叉另存、MEMORY.md 按行合并、其他文件本机优先并备份
  Conflict handling: keep the superset, fork diverged sessions, merge MEMORY.md by line, local-wins with backup for other files
- 自动同步：Claude Code hooks + 文件监听 + 每小时定时任务；hooks 被 CC Switch 等工具抹掉后自动补回
  Automatic sync: Claude Code hooks + file watcher + hourly scheduled task; hooks are restored if a tool like CC Switch removes them
- 云端存储支持所有 S3 兼容服务：阿里云 OSS、腾讯云 COS、Cloudflare R2、AWS S3、MinIO 等
  Any S3-compatible storage: Alibaba Cloud OSS, Tencent Cloud COS, Cloudflare R2, AWS S3, MinIO, …
- 图形界面（首页、会话浏览、项目绑定、设置、日志、托盘）+ 命令行
  GUI (home, session viewer, project bindings, settings, logs, tray) + CLI
- 界面、命令行、日志支持简体中文和 English
  Simplified Chinese and English for the GUI, CLI and logs
