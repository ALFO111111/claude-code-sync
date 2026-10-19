# 参与开发 / Contributing

[中文](#中文) | [English](#english)

---

## 中文

谢谢你愿意帮忙！提 issue、改文档、修 bug、加功能都欢迎。

### 最重要的原则：不能弄丢用户的会话

这个工具直接读写用户的 `~/.claude`，用户最怕的就是会话丢了。所以：

- 改同步逻辑（`internal/engine`）前后都要想清楚“最坏情况下会不会丢数据”，并补上测试
- 测试一律用 `testutil.NewDevice(t, id)` 在临时目录里搭假设备，**绝不碰真实数据**
- 手动调试 GUI 时用沙箱（见下面），不要直接对着自己的真实会话跑开发版

### 开发环境

| 工具 | 版本 |
|---|---|
| Windows | 10 / 11（目前只支持 Windows） |
| Go | 1.26+ |
| Node.js | 22+（CI 用 24） |
| Wails CLI | v2.16.0：`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0` |

### 常用命令

```powershell
go test ./...                    # 全部测试（单元 + 双设备集成，全在临时目录，不需要任何密钥）
cd frontend; npm ci; npm run check-i18n; npm test; cd ..   # 前端文字检查 + 单元测试
cd frontend; npm run demo                          # 浏览器里用假数据跑界面（不需要 Go 后端，改界面、截图用）
wails build                      # 打包单个 exe → build\bin\claude-code-sync.exe
.\打包.bat                        # 同上，双击运行，会先关掉正在运行的程序
```

**用沙箱调试**：两个环境变量把程序的数据和会话目录都指到别处，真实的 `~/.claude` 不会被碰，也不会安装 hooks 和定时任务：

```powershell
$env:CLAUDE_CODE_SYNC_HOME = "D:\ccs-sandbox\app"     # 本工具的数据（配置、状态、日志）
$env:CLAUDE_CONFIG_DIR     = "D:\ccs-sandbox\claude"  # 会话目录（可以先复制一份真实的进来）
wails dev                                             # 或运行打包好的 exe
```

两个都要设：只设第一个的话，会话目录还是真实的 `~/.claude`。

### 项目结构

```
main.go                 唯一入口：无参数 → 图形界面，有参数 → 命令行
internal/
  cli/  gui/            命令行 / 图形界面：只负责收参数、展示结果，不写业务判断
  service/              两种界面共用的业务操作
  engine/               同步引擎：比对、合并、分叉、路径翻译、项目绑定、备份、改口令
  state/                state.json（上次同步时的样子）+ sync.lock
  project/              项目身份证（pid）
  storage/              云端接口：S3 兼容（minio-go）+ 本地文件夹（测试用）
  crypt/                Argon2id + gzip + AES-256-GCM
  i18n/                 多语言（中文原文当 key，en.go 是英文对照表）
  config/ logging/ platform/ watch/ icon/ fsutil/ testutil/
frontend/src/           Vue 3：api/ 调后端，i18n/ 界面文字，views/ 页面
```

**依赖只能从上往下**：`main → cli/gui → service → engine → 底层包`。加新功能的套路：业务写进 service（同步算法写进 engine）→ cli、gui 各包一层 → 前端用 `import { api } from '@/api'` 调。

### 多语言

- **Go**：代码里直接写中文原文，用 `i18n.T("…")` 包起来（错误用 `fmt.Errorf(i18n.T("…: %w"), err)`；日志的 `Printf` 会自动翻译，不用包），然后在 `internal/i18n/en.go` 加英文。`go test ./internal/i18n` 会扫描全部源码，漏了、多了、占位符对不上都会报错。
- **前端**：文字放在 `frontend/src/i18n/zh-CN.js` 和 `en.js`，两边 key 要一致，`npm run check-i18n` 会检查。

### 提交

- 提交信息格式：`类型-简短描述`，类型用 `feat` / `fix` / `docs` / `chore` / `refactor` / `test`，例如 `fix-项目会话目录不存在误报同步失败`
- PR 提交前请确认：`go test ./...` 通过、`gofmt -l .` 没有输出、`npm run check-i18n` 和 `npm test` 通过

### 发布流程（维护者）

1. 在 `CHANGELOG.md` 把 `## [Unreleased]` 改成 `## [0.x.y] - 日期`，在上面再留一个空的 `## [Unreleased]`，提交
2. `git tag v0.x.y && git push origin v0.x.y`
3. GitHub Actions 自动测试、构建、（配置了的话）签名，然后发布到 Releases，说明取自 CHANGELOG 对应的那一节

代码签名的申请和配置见 [docs/code-signing.md](docs/code-signing.md)。

---

## English

Thanks for helping out! Issues, docs, bug fixes and features are all welcome.

### The one rule: never lose a user's sessions

This tool reads and writes the user's `~/.claude` directly, and losing conversations is what users fear most. So:

- When changing sync logic (`internal/engine`), think through the worst case before and after, and add tests
- Tests always use `testutil.NewDevice(t, id)` to set up fake devices in temp folders. **Never touch real data**
- Debug the GUI in a sandbox (see below), never against your real sessions

### Requirements

| Tool | Version |
|---|---|
| Windows | 10 / 11 (Windows only for now) |
| Go | 1.26+ |
| Node.js | 22+ (CI uses 24) |
| Wails CLI | v2.16.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0` |

### Common commands

```powershell
go test ./...                    # all tests (unit + two-device integration, all in temp dirs, no keys needed)
cd frontend; npm ci; npm run check-i18n; npm test; cd ..   # UI text check + unit tests
cd frontend; npm run demo                          # run the UI in a browser with fake data (no Go backend; for UI work and screenshots)
wails build                      # build the single exe → build\bin\claude-code-sync.exe
```

**Sandbox**: two environment variables redirect the app's data and the session folder, so your real `~/.claude` is never touched and no hooks or scheduled tasks are installed:

```powershell
$env:CLAUDE_CODE_SYNC_HOME = "D:\ccs-sandbox\app"     # this tool's data (config, state, log)
$env:CLAUDE_CONFIG_DIR     = "D:\ccs-sandbox\claude"  # session folder (copy your real one in if you like)
wails dev                                             # or run a built exe
```

Set both. With only the first one, the session folder is still your real `~/.claude`.

### Layout

```
main.go                 single entry: no args → GUI, args → CLI
internal/
  cli/  gui/            terminal / window front ends: collect input, show results, no business logic
  service/              operations shared by both front ends
  engine/               sync engine: diff, merge, fork, path translation, project binding, backups, passphrase change
  state/                state.json (what things looked like at the last sync) + sync.lock
  project/              project identity (pid)
  storage/              cloud interface: S3-compatible (minio-go) + local folder (tests)
  crypt/                Argon2id + gzip + AES-256-GCM
  i18n/                 translations (Chinese source strings as keys, en.go holds English)
  config/ logging/ platform/ watch/ icon/ fsutil/ testutil/
frontend/src/           Vue 3: api/ calls the backend, i18n/ UI text, views/ pages
```

**Dependencies only point downwards**: `main → cli/gui → service → engine → low-level packages`. To add a feature: put the logic in service (sync algorithms in engine) → a thin wrapper in cli and gui → call it from the frontend via `import { api } from '@/api'`.

### Translations

- **Go**: source strings are written in Chinese and wrapped in `i18n.T("…")` (errors: `fmt.Errorf(i18n.T("…: %w"), err)`; logger `Printf` translates automatically), with the English text added to `internal/i18n/en.go`. `go test ./internal/i18n` scans all sources and fails on missing, unused or mismatched-placeholder entries.
- **Frontend**: texts live in `frontend/src/i18n/zh-CN.js` and `en.js` with identical keys, checked by `npm run check-i18n`.

### Commits and PRs

- Commit messages: `type-short description`, where type is `feat` / `fix` / `docs` / `chore` / `refactor` / `test`
- Before opening a PR: `go test ./...` passes, `gofmt -l .` prints nothing, `npm run check-i18n` and `npm test` pass

### Releasing (maintainers)

1. In `CHANGELOG.md`, rename `## [Unreleased]` to `## [0.x.y] - date`, add a fresh empty `## [Unreleased]` above it, commit
2. `git tag v0.x.y && git push origin v0.x.y`
3. GitHub Actions tests, builds, signs (if configured) and publishes the release, using that CHANGELOG section as the notes

Code signing setup: see [docs/code-signing.md](docs/code-signing.md).
