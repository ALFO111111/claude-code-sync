# 参与开发 / Contributing

[中文](#中文) | [English](#english)

---

## 中文

谢谢你愿意帮忙！提 issue、改文档、修 bug、加功能都欢迎。

### 最重要的原则：不能弄丢用户的会话

这个工具直接读写用户的 `~/.claude`，以及 Codex 的会话、全局规则和技能，用户最怕的就是数据丢了。所以：

- 改同步逻辑（`internal/engine`）前后都要想清楚“最坏情况下会不会丢数据”，并补上测试
- 测试一律用 `testutil.NewDevice(t, id)` 在临时目录里搭假设备，**绝不碰真实数据**
- 手动调试 GUI 时用沙箱（见下面），不要直接对着自己的真实会话跑开发版

### 开发环境

| 工具 | 版本 |
|---|---|
| 系统 | Windows 10 / 11，或 macOS 13+（Mac 需要 Xcode 命令行工具：`xcode-select --install`） |
| Go | 1.27.2（以 `go.mod` 为准；本机 Go 版本更低时会自动下载这个版本的工具链） |
| Node.js | 22+（CI 用 24） |
| Wails CLI | v2.16.0：`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0` |

### 常用命令

```powershell
go test ./...                    # 全部测试（单元 + 双设备集成，全在临时目录，不需要任何密钥）
cd frontend; npm ci; npm run check-i18n; npm test; cd ..   # 前端文字检查 + 单元测试
cd frontend; npm run demo                          # 浏览器里用假数据跑界面（不需要 Go 后端，改界面、截图用）
wails build                      # Windows：打包单个 exe → build\bin\claude-code-sync.exe
.\打包.bat                        # 同上，双击运行：先关掉正在运行的程序；PATH 里没有 Go 时会到常见安装位置找，打包后还原 frontend/package.json.md5
```

Mac 上打通用版 App（同时支持 Intel 和 Apple 芯片）：

```bash
wails build -platform darwin/universal   # → build/bin/claude-code-sync.app
```

**gofmt 要用 `go.mod` 指定版本的**：不同 Go 版本的 gofmt 对中文字符的对齐算法不一样，用错版本会和 CI 对不上。用 `go fmt ./...`，或者：

```bash
"$(go env GOROOT)/bin/gofmt" -l .        # go env GOROOT 会跟着自动切换的工具链走
```

**用沙箱调试**：环境变量把程序的数据和工具目录都指到别处，真实的 `~/.claude`、`~/.codex` 和 `~/.agents/skills` 不会被碰，也不会安装 hooks 和定时任务：

```powershell
$env:CLAUDE_CODE_SYNC_HOME = "D:\ccs-sandbox\app"          # 本工具的数据（配置、状态、日志）
$env:CLAUDE_CONFIG_DIR     = "D:\ccs-sandbox\claude"       # Claude 会话目录（可以先复制一份真实的进来）
$env:CODEX_HOME            = "D:\ccs-sandbox\home\.codex"  # Codex 全局规则
$env:USERPROFILE           = "D:\ccs-sandbox\home"         # 让 ~/.agents/skills 也落到沙箱
wails dev                                                  # 或运行打包好的 exe
```

只调试 Claude 时，前两个变量都要设：只设第一个的话，会话目录还是真实的 `~/.claude`。涉及 Codex 时四个都要设：`CODEX_HOME` 移动规则、会话和旧位置技能，`~/.agents/skills` 的位置仍由用户目录决定。

Mac 上用户目录由 `HOME` 决定（代替 `USERPROFILE`）：

```bash
export CLAUDE_CODE_SYNC_HOME=/tmp/ccs-sandbox/app
export CLAUDE_CONFIG_DIR=/tmp/ccs-sandbox/claude
export CODEX_HOME=/tmp/ccs-sandbox/home/.codex
export HOME=/tmp/ccs-sandbox/home
wails dev
```

沙箱模式不会写 `~/Library/LaunchAgents`。

### 项目结构

```
main.go                 唯一入口：无参数 → 图形界面，有参数 → 命令行（含隐藏命令 daemon）
internal/
  cli/                  命令行：只负责收参数、展示结果，不写业务判断
  gui/                  图形界面，同上；tray_windows.go 是 Windows 托盘，tray_other.go 是 macOS 的空实现
                        （托盘库和 Wails 在 macOS 上抢同一个主循环，Mac 版没有菜单栏图标，关窗即退出）
  service/              两种界面共用的业务操作；watchers.go：NewWatchers（要盯的文件，Windows 托盘常驻和
                        Mac 的 daemon 共用）、RunDaemon（Mac 上无窗口的后台同步）
  engine/               同步引擎：比对、合并、分叉、路径翻译（含 Windows ↔ macOS）、项目绑定、备份、改口令；
                        Claude 个人配置（规则、技能、MCP…）和 Codex 全局规则 / 技能的三方合并、冲突记录与解决；
                        codex_sessions.go：Codex 会话同步（按设备分开存、只追加、分叉另存、拿 Codex 会话写锁）
  claudeconfig/         settings.json 白名单、MCP 去秘密的提取与回填
  ccswitch/             只读检测 CC Switch 自动同步是否开着（负责人选择只影响 Claude）
  state/                state.json（上次同步时的样子）+ sync.lock
  project/              项目身份证（pid）
  storage/              云端接口：S3 兼容（minio-go）+ 本地文件夹（测试用）
  crypt/                Argon2id + gzip + AES-256-GCM
  i18n/                 多语言（中文原文当 key，en.go 是英文对照表）
  platform/             系统相关：Windows 定时任务 / 开机自启（schedule_windows.go、desktop_windows.go），
                        macOS 的 LaunchAgent（launchd_darwin.go：读写 plist + launchctl，每小时兜底的 .sync）、
                        开机自启（登录启动 daemon 的 .daemon）和打开文件夹 / 终端 / 深色模式（desktop_darwin.go）
  fsutil/               原子写文件等；lock_windows.go / lock_unix.go：不等待的整文件独占锁
                        （LockFileEx / flock，和 Codex 的会话写锁互斥）
  config/ logging/ watch/ icon/ testutil/
frontend/src/           Vue 3：api/ 调后端，i18n/ 界面文字，views/ 页面
```

**依赖只能从上往下**：`main → cli/gui → service → engine → 底层包`。加新功能的套路：业务写进 service（同步算法写进 engine）→ cli、gui 各包一层 → 前端用 `import { api } from '@/api'` 调。

**Codex 同步边界**：
- 规则和技能：`$CODEX_HOME/AGENTS.md`、`AGENTS.override.md`（`rules`）、`~/.agents/skills`（`skills`）、`$CODEX_HOME/skills`（`legacy-skills`，排除 Codex 自带的 `.system`），云端键是 `tools/codex/files/<类别>/...`，复用全量同步和三方文本合并。
- 会话（`sessions`）：只同步 `$CODEX_HOME/sessions/**/rollout-*.jsonl`，云端键是 `tools/codex/sessions/<设备ID>/<相对路径>`，每台设备只写自己那份。拉取规则：本机没有 → 新建；本机是对方的开头 → 只追加；对方是本机的开头 → 不动；各写各的 → 另存成分叉会话（ID 由原 ID + 对方设备决定）。追加前必须拿到 Codex 的 `thread-writer-locks/<ID>.lock`，拿不到就跳过；本机删除 / 归档的不再拉回；项目没对上的先不拉。
- 不碰：SQLite/WAL（`state_*.sqlite`、`thread_history_*.sqlite` 等，Codex 能自己重建）、history/session_index、config.toml/MCP、auth、插件/hooks；项目级 AGENTS/skills 仍归 Git。
- CC Switch 的负责人逻辑只能作用于 Claude，新增共享代码时不要把 Codex 类别误纳进去。

**云端相对路径的安全检查**：出现 `\`、`:` 或以 `/` 开头的一律拒绝（在 macOS 上 `\` 是合法文件名字符，不能指望系统帮你挡）。新增类别时沿用同一套检查。

### 多语言

- **Go**：代码里直接写中文原文，用 `i18n.T("…")` 包起来（错误用 `fmt.Errorf(i18n.T("…: %w"), err)`；日志的 `Printf` 会自动翻译，不用包），然后在 `internal/i18n/en.go` 加英文。`go test ./internal/i18n` 会扫描全部源码，漏了、多了、占位符对不上都会报错。
- **前端**：文字放在 `frontend/src/i18n/zh-CN.js` 和 `en.js`，两边 key 要一致，`npm run check-i18n` 会检查。

### 提交

- 提交信息格式：`类型-简短描述`，类型用 `feat` / `fix` / `docs` / `chore` / `refactor` / `test`，例如 `fix-项目会话目录不存在误报同步失败`
- PR 提交前请确认：`go test ./...` 通过、`"$(go env GOROOT)/bin/gofmt" -l .` 没有输出、`npm run check-i18n` 和 `npm test` 通过（CI 会在 Windows 和 macOS 上各跑一遍）

### 发布流程（维护者）

1. 在 `CHANGELOG.md` 把 `## [Unreleased]` 改成 `## [0.x.y] - 日期`，在上面再留一个空的 `## [Unreleased]`，提交
2. `git tag v0.x.y && git push origin v0.x.y`
3. GitHub Actions 自动测试、构建（Windows：exe + zip；macOS：universal zip），然后发布到 Releases，`SHA256SUMS.txt` 包含全部文件，说明取自 CHANGELOG 对应的那一节

---

## English

Thanks for helping out! Issues, docs, bug fixes and features are all welcome.

### The one rule: never lose a user's sessions

This tool reads and writes the user's `~/.claude` plus Codex sessions, global rules and skills directly, and losing data is what users fear most. So:

- When changing sync logic (`internal/engine`), think through the worst case before and after, and add tests
- Tests always use `testutil.NewDevice(t, id)` to set up fake devices in temp folders. **Never touch real data**
- Debug the GUI in a sandbox (see below), never against your real sessions

### Requirements

| Tool | Version |
|---|---|
| OS | Windows 10 / 11, or macOS 13+ (the Mac needs the Xcode command line tools: `xcode-select --install`) |
| Go | 1.27.2 (as pinned in `go.mod`; an older local Go downloads this toolchain automatically) |
| Node.js | 22+ (CI uses 24) |
| Wails CLI | v2.16.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0` |

### Common commands

```powershell
go test ./...                    # all tests (unit + two-device integration, all in temp dirs, no keys needed)
cd frontend; npm ci; npm run check-i18n; npm test; cd ..   # UI text check + unit tests
cd frontend; npm run demo                          # run the UI in a browser with fake data (no Go backend; for UI work and screenshots)
wails build                      # Windows: build the single exe → build\bin\claude-code-sync.exe
.\打包.bat                        # same, by double-click: closes the running app first; finds Go in common install locations if it's not on PATH, and restores frontend/package.json.md5 afterwards
```

On the Mac, build the universal app (Intel + Apple silicon):

```bash
wails build -platform darwin/universal   # → build/bin/claude-code-sync.app
```

**Use the gofmt from the Go version in `go.mod`**: gofmt aligns CJK characters differently across Go versions, so the wrong one disagrees with CI. Use `go fmt ./...`, or:

```bash
"$(go env GOROOT)/bin/gofmt" -l .        # go env GOROOT follows the auto-selected toolchain
```

**Sandbox**: environment variables redirect the app's data and tool folders, so your real `~/.claude`, `~/.codex` and `~/.agents/skills` are never touched and no hooks or scheduled tasks are installed:

```powershell
$env:CLAUDE_CODE_SYNC_HOME = "D:\ccs-sandbox\app"          # this tool's data (config, state, log)
$env:CLAUDE_CONFIG_DIR     = "D:\ccs-sandbox\claude"       # Claude session folder (copy your real one in if you like)
$env:CODEX_HOME            = "D:\ccs-sandbox\home\.codex"  # Codex global rules
$env:USERPROFILE           = "D:\ccs-sandbox\home"         # also puts ~/.agents/skills in the sandbox
wails dev                                                  # or run a built exe
```

For Claude-only work, set both of the first two. With only the first one, the session folder is still your real `~/.claude`. For Codex work, set all four: `CODEX_HOME` moves the rules, sessions and legacy skills, while `~/.agents/skills` still follows the user profile.

On the Mac the user folder comes from `HOME` (instead of `USERPROFILE`):

```bash
export CLAUDE_CODE_SYNC_HOME=/tmp/ccs-sandbox/app
export CLAUDE_CONFIG_DIR=/tmp/ccs-sandbox/claude
export CODEX_HOME=/tmp/ccs-sandbox/home/.codex
export HOME=/tmp/ccs-sandbox/home
wails dev
```

Sandbox mode never writes to `~/Library/LaunchAgents`.

### Layout

```
main.go                 single entry: no args → GUI, args → CLI (including the hidden daemon command)
internal/
  cli/                  terminal front end: collect input, show results, no business logic
  gui/                  window front end, same rule; tray_windows.go is the Windows tray, tray_other.go a no-op for macOS
                        (the tray library and Wails fight over the same main loop on macOS, so the Mac has no menu bar icon
                        and closing the window quits)
  service/              operations shared by both front ends; watchers.go: NewWatchers (what to watch, shared by the
                        Windows tray and the Mac daemon) and RunDaemon (windowless background sync on the Mac)
  engine/               sync engine: diff, merge, fork, path translation (incl. Windows ↔ macOS), project binding, backups,
                        passphrase change; three-way merge, conflict records and resolution for Claude personal config
                        (rules, skills, MCP…) and Codex global rules / skills;
                        codex_sessions.go: Codex session sync (per-device copies, append-only, forks, Codex writer lock)
  claudeconfig/         extract/apply the settings.json allow-list and secret-free MCP config
  ccswitch/             read-only detection of CC Switch auto sync (owner selection only affects Claude)
  state/                state.json (what things looked like at the last sync) + sync.lock
  project/              project identity (pid)
  storage/              cloud interface: S3-compatible (minio-go) + local folder (tests)
  crypt/                Argon2id + gzip + AES-256-GCM
  i18n/                 translations (Chinese source strings as keys, en.go holds English)
  platform/             OS specifics: Windows scheduled task / start-at-login (schedule_windows.go, desktop_windows.go);
                        macOS LaunchAgents (launchd_darwin.go: plist + launchctl, the hourly .sync agent),
                        start-at-login (the .daemon agent) and open folder / terminal / dark mode (desktop_darwin.go)
  fsutil/               atomic writes etc.; lock_windows.go / lock_unix.go: non-blocking whole-file exclusive lock
                        (LockFileEx / flock, mutually exclusive with Codex's session writer lock)
  config/ logging/ watch/ icon/ testutil/
frontend/src/           Vue 3: api/ calls the backend, i18n/ UI text, views/ pages
```

**Dependencies only point downwards**: `main → cli/gui → service → engine → low-level packages`. To add a feature: put the logic in service (sync algorithms in engine) → a thin wrapper in cli and gui → call it from the frontend via `import { api } from '@/api'`.

**Codex sync boundary**:
- Rules and skills: `$CODEX_HOME/AGENTS.md`, `AGENTS.override.md` (`rules`), `~/.agents/skills` (`skills`), `$CODEX_HOME/skills` (`legacy-skills`, excluding Codex's built-in `.system`), stored under `tools/codex/files/<category>/...` and handled by full sync and three-way text merge.
- Sessions (`sessions`): only `$CODEX_HOME/sessions/**/rollout-*.jsonl`, stored under `tools/codex/sessions/<device-id>/<relative-path>`; each device writes only its own copy. Pull rules: missing locally → create; local is a prefix of theirs → append only; theirs is a prefix of local → leave it; diverged → save as a forked session (ID derived from the original ID + the other device). Codex's `thread-writer-locks/<ID>.lock` must be held before appending, otherwise skip; sessions deleted / archived locally are never pulled back; sessions of unmatched projects are not pulled yet.
- Never touched: SQLite/WAL (`state_*.sqlite`, `thread_history_*.sqlite`, …, which Codex rebuilds itself), history/session_index, config.toml/MCP, auth, plugins/hooks; project-level AGENTS/skills remain managed by Git.
- CC Switch owner logic must only affect Claude, so do not accidentally include Codex categories when extending shared code.

**Safety check for cloud relative paths**: anything containing `\` or `:`, or starting with `/`, is rejected (`\` is a valid file name character on macOS, so the OS won't catch it for you). Reuse the same check for new categories.

### Translations

- **Go**: source strings are written in Chinese and wrapped in `i18n.T("…")` (errors: `fmt.Errorf(i18n.T("…: %w"), err)`; logger `Printf` translates automatically), with the English text added to `internal/i18n/en.go`. `go test ./internal/i18n` scans all sources and fails on missing, unused or mismatched-placeholder entries.
- **Frontend**: texts live in `frontend/src/i18n/zh-CN.js` and `en.js` with identical keys, checked by `npm run check-i18n`.

### Commits and PRs

- Commit messages: `type-short description`, where type is `feat` / `fix` / `docs` / `chore` / `refactor` / `test`
- Before opening a PR: `go test ./...` passes, `"$(go env GOROOT)/bin/gofmt" -l .` prints nothing, `npm run check-i18n` and `npm test` pass (CI runs on both Windows and macOS)

### Releasing (maintainers)

1. In `CHANGELOG.md`, rename `## [Unreleased]` to `## [0.x.y] - date`, add a fresh empty `## [Unreleased]` above it, commit
2. `git tag v0.x.y && git push origin v0.x.y`
3. GitHub Actions tests, builds (Windows: exe + zip; macOS: universal zip) and publishes the release with every file in `SHA256SUMS.txt`, using that CHANGELOG section as the notes
