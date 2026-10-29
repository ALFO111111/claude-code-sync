# Changelog

本文件记录每个版本的变化，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。
发布时 GitHub Actions 会把对应版本这一节作为 Release 说明。

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### 新增 / Added

- 同步个人配置：全局规则 `CLAUDE.md`、`plans/`、`skills/`、`agents/`、`commands/`、`tasks/`、用户级 MCP 服务器、settings.json 的安全字段（`language`、`autoMemoryEnabled`、`includeCoAuthoredBy`）。模型、Key、hooks、权限、MCP 的密钥和环境变量不上传；设置页“同步内容”可逐类开关
  Sync personal config: global `CLAUDE.md`, `plans/`, `skills/`, `agents/`, `commands/`, `tasks/`, user-level MCP servers and safe settings.json fields (`language`, `autoMemoryEnabled`, `includeCoAuthoredBy`). Models, keys, hooks, permissions and MCP secrets/env vars are never uploaded; each category can be turned off under Settings → What to sync
- 三方合并：记住上次两边一致的版本，不同行 / 不同 JSON 字段 / 不同任务 ID 的修改自动合并；改到同一处时进新的“冲突”页（保留本机 / 用另一台的 / 两份都留 / 手工合并），本机文件在此期间保持可用
  Three-way merge: changes to different lines, JSON fields or task IDs merge automatically; edits to the same spot go to the new Conflicts page (keep mine / use theirs / keep both / merge manually) while the local file stays usable
- CC Switch 共存：检测到 CC Switch 自动同步开着时，全局规则、技能、MCP 先暂停，在设置页选择由谁负责；没开就不提示
  CC Switch coexistence: when CC Switch auto sync is on, global rules, skills and MCP pause until you choose an owner in Settings; nothing is shown otherwise
- 命令行：`content`、`conflicts`、`resolve`、`restore-content`
  CLI: `content`, `conflicts`, `resolve`, `restore-content`

### 改进 / Changed

- 个人配置的删除不会传播：一台电脑删掉的文件不会删其他电脑的，本机也不会被拉回来（`restore-content` 可手动拉回）
  Deleting personal config never propagates: other machines keep the file and it is not pulled back locally (`restore-content` brings it back on purpose)
- hooks 自愈和同步改写 settings.json 时共用一把锁，并在写入前确认文件没被其他程序（如 CC Switch）刚改过
  Hook self-repair and sync share a lock when rewriting settings.json and check it was not just changed by another program (such as CC Switch)
- 文件监听：启动时还不存在的目录（如第一次建 `skills/`）创建后会自动开始监听
  File watching: folders that don't exist yet at startup (such as a first `skills/`) are picked up once created

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
