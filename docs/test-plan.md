# 上线前测试方案：确保不会弄丢本地会话

> 核心思路：**先备份 → 在副本上彩排 → 再上真实环境 → 每一步都用脚本对比"同步前后"**。
> 任何一步对比结果是红色，就停下来，备份还在，什么都没丢。

## 0. 先吃颗定心丸：这个工具会对本地会话做什么

已经在代码里逐行确认过（`internal/engine/transfer.go`）：

- **从不删除** `~/.claude/projects` 下的任何文件。程序里唯一的删除操作，是清理它自己 14 天前的备份目录。
- 只有下面 4 种情况会往本地**写**文件：

| 情况 | 写入内容 | 会不会丢东西 |
|---|---|---|
| 本地没改过、云端有新版本 | 用云端新版替换 | 不会：旧版就是上次同步的内容，云端新版包含它 |
| 两边都改了同一个会话，一边是另一边的子集 | 用记录更多的那份 | 不会：多的那份包含少的那份的每一条记录 |
| 两边各自聊了不同的内容 | 本地那份不动，**另存一个新会话** | 不会：两份都保留 |
| MEMORY.md 两边都改了 | 两边的行合在一起 | 不会：取并集 |

- 其他文件（非会话）两边都改了：**本机优先**，云端那份放进 `~/.agent-sync/backup/`。
- Claude Code 自己清理掉的旧会话，不会被拉回来。
- `~/.claude/settings.json` 只会被加两条 hooks（`SessionStart`、`Stop`），你原有的配置一个字都不动。

下面的测试就是要**用你自己的真实数据亲眼确认**上面这些话。

## 1. 准备

- 两台电脑都 `git pull` 拿到最新代码，`scripts\` 目录里有两个检查脚本：
  - `snapshot.ps1`：给会话目录拍"快照"（记下每个文件的指纹）
  - `compare.ps1`：对比两次快照，**少了文件、少了对话记录都会标红**
- 下面的命令都在**项目根目录**打开 PowerShell 执行。
- 如果 PowerShell 提示"禁止运行脚本"，先执行一次：`Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`

## 2. 第一道保险：完整备份（每台电脑、每个阶段开始前都做）

先**关掉所有 Claude Code 窗口**（不然正在聊的会话文件会一直变），然后：

```powershell
$BK = "D:\ccs-backup\claude-$(Get-Date -Format yyyyMMdd-HHmm)"
robocopy "$env:USERPROFILE\.claude" $BK /E /R:1 /W:1 /NFL /NDL
"备份在: $BK"
```

**万一要恢复**（先关掉 Claude Code，并在托盘里退出 agent-sync）：

```powershell
robocopy "D:\ccs-backup\claude-xxxx" "$env:USERPROFILE\.claude" /MIR
```

`/MIR` 会让 `.claude` 变得和备份**一模一样**（同步拉下来的新文件也会被删掉，回到备份那一刻）。

## 3. 阶段 A：沙箱彩排（电脑 A，完全不碰真实会话）

用环境变量把程序指向一份**会话副本**，真实的 `~/.claude` 一个字节都不会动。沙箱模式下程序也不会安装 hooks 和定时任务。

```powershell
$S = "D:\ccs-sandbox"
robocopy "$env:USERPROFILE\.claude" "$S\claude" /E /R:1 /W:1 /NFL /NDL      # 复制一份
.\scripts\snapshot.ps1 -Dir "$S\claude\projects" -Out "$S\A-before.csv"

$env:AGENT_SYNC_HOME = "$S\app"     # 本工具的数据放沙箱
$env:CLAUDE_CONFIG_DIR     = "$S\claude"  # 会话目录指向副本
.\agent-sync.exe                     # 只有从这个窗口启动的才是沙箱
```

1. 走完向导（存储填正式的 bucket 就行，彩排完会清空）。完成后首页显示"同步完成"。
2. 首页会提示"沙箱模式，不安装自动同步"，这是**正常**的。
3. 托盘图标右键 → **退出**。然后对比：

```powershell
.\scripts\snapshot.ps1 -Dir "$S\claude\projects" -Out "$S\A-after.csv"
.\scripts\compare.ps1 -Before "$S\A-before.csv" -After "$S\A-after.csv"
```

**预期**：删除 0、修改 0、新增 0（第一台设备云端是空的，只会上传，本地一点都不动）。

4. 打开存储服务商的控制台（如阿里云 OSS、Cloudflare R2），看 `v1/` 下多了文件。随便下载一个用记事本打开，应该是乱码，开头是 `CCS1`（说明加密了）。

## 4. 阶段 B：双设备彩排（电脑 B，这是最关键的一步）

电脑 B 上做同样的沙箱，**用同一个口令**。这一步会把 A 的会话拉到 B 的副本里，和 B 自己的会话混在一起，正好是以后真实使用时最容易出问题的场景。

```powershell
$S = "D:\ccs-sandbox"
robocopy "$env:USERPROFILE\.claude" "$S\claude" /E /R:1 /W:1 /NFL /NDL
robocopy "$S\claude\projects" "$S\B-orig-projects" /E /R:1 /W:1 /NFL /NDL  # 再留一份原样，用来逐条核对
.\scripts\snapshot.ps1 -Dir "$S\claude\projects" -Out "$S\B-before.csv"

$env:AGENT_SYNC_HOME = "$S\app"
$env:CLAUDE_CONFIG_DIR     = "$S\claude"
.\agent-sync.exe
```

走完向导（同一个口令）→ 等首页"同步完成" → 到"项目绑定"页看看，有没有需要认领的项目（普通文件夹项目要手动认领，git 项目会自动对上）→ 认领完再点一次"立即同步" → 托盘退出。

```powershell
.\scripts\snapshot.ps1 -Dir "$S\claude\projects" -Out "$S\B-after.csv"
.\scripts\compare.ps1 -Before "$S\B-before.csv" -After "$S\B-after.csv" `
    -OldDir "$S\B-orig-projects" -NewDir "$S\claude\projects"
```

**预期**：
- 删除 = 0（**必须**）
- 新增 = A 那边拉过来的会话
- 修改：如果有，每一个都要显示"旧记录全部还在 ✓"

**可选，验证拉过来的会话能不能接着用**（还是在这个设置了环境变量的窗口里）：

```powershell
cd <某个两台电脑都有的 git 项目目录>
claude --resume
```

列表里应该能看到 A 上聊过的会话。打开一个问"我们刚才在做什么"，它应该答得上来，而且里面的文件路径是 **B 电脑上的路径**。（如果要求重新登录，就是副本里缺登录信息，登录一下就好，不影响测试。）

然后回到**电脑 A**，用阶段 A 那个沙箱再同步一次（同样先设好两个环境变量再启动），把 B 的会话拉过来，再跑一次 snapshot 加 compare，预期同样是删除 0。

## 5. 彩排收尾

1. 两台电脑托盘都退出，删掉 `D:\ccs-sandbox`。
2. **在存储服务商的控制台把 `v1/` 整个目录删掉**。彩排时上传的设备信息如果留着，正式使用时会多出两台"幽灵设备"。
3. 关掉设置过环境变量的 PowerShell 窗口（环境变量只在那个窗口里有效，关掉就没了）。

## 6. 阶段 C：真实环境，电脑 A

1. 关掉 Claude Code，**按第 2 节做备份**，记下备份目录。
2. 拍快照：
   ```powershell
   .\scripts\snapshot.ps1 -Dir "$env:USERPROFILE\.claude\projects" -Out D:\ccs-check\A-before.csv
   ```
3. **新开一个** PowerShell 窗口，或者直接双击 exe（确保没设沙箱环境变量）→ 走向导 → 等"同步完成"。
4. 首页**不应该**出现"自动同步没有就绪"。
5. 对比：
   ```powershell
   .\scripts\snapshot.ps1 -Dir "$env:USERPROFILE\.claude\projects" -Out D:\ccs-check\A-after.csv
   .\scripts\compare.ps1 -Before D:\ccs-check\A-before.csv -After D:\ccs-check\A-after.csv
   ```
   预期：删除 0、修改 0、新增 0。
6. 看一眼自动同步装上了没有：
   - `~/.claude/settings.json` 里多了两条带 `hook-event` 的命令
   - "任务计划程序"里有一个 `AgentSync` 任务

## 7. 阶段 D：真实环境，电脑 B

1. 关掉 Claude Code，备份（第 2 节），拍快照 `B-before.csv`。
2. 双击 exe → 向导（同一个口令）→ 等"同步完成" → "项目绑定"页认领需要的项目 → 再同步一次。
3. 对比（`-OldDir` 填刚才备份里的 `projects` 目录）：
   ```powershell
   .\scripts\snapshot.ps1 -Dir "$env:USERPROFILE\.claude\projects" -Out D:\ccs-check\B-after.csv
   .\scripts\compare.ps1 -Before D:\ccs-check\B-before.csv -After D:\ccs-check\B-after.csv `
       -OldDir "D:\ccs-backup\claude-xxxx\projects" -NewDir "$env:USERPROFILE\.claude\projects"
   ```
   预期：删除 0；修改的文件都"旧记录全部还在 ✓"。
4. `claude --resume` 打开一个 A 上聊过的会话，确认能接着聊。
5. 回到电脑 A：先备份，再点"立即同步"把 B 的会话拉过来，再对比一次。

## 7.5 阶段 E：个人配置（规则、技能、MCP 等）

前提：两台电脑都已按上面跑通会话同步。开始前同样先备份（第 2 节），并另外复制一份 `~/.claude.json`（MCP 在里面）。

1. **下载**：在 A 上新建 `~/.claude/plans/test-a.md`，A 点"立即同步"，B 点"立即同步" → B 上出现同名文件、内容一致。
2. **自动合并**：两台都同步好之后，A 改 `test-a.md` 第一行、B 改最后一行，各自同步两轮 → 两台都同时有两处修改。
3. **冲突**：A、B 都改 `test-a.md` 的同一行成不同内容，各自同步 → 后同步的那台侧边栏"冲突"出现 1，本机文件内容没变；在冲突页选"手工合并"保存 → 两台最终一致，"备份"页有被替换掉的那份。
4. **删除不传播**：A 删掉 `test-a.md` 并同步 → B 上的文件还在；A 再同步几轮也不会把它拉回来；`agent-sync restore-content plans test-a.md` 能手动拉回。
5. **settings 安全字段**：A 把 `settings.json` 的 `language` 改掉 → B 同步后只有 `language` 变了；B 的 `model`、`env`、`hooks`、`permissions` 都原样（用对比工具看一眼整份文件）。
6. **MCP**：A 加一个用户级 MCP（带 env 密钥）→ B 同步后出现同名服务器，但**没有** A 的 env；B 自己补上 env 后再同步，A 的 env 也不受影响。
7. **CC Switch**：如果 CC Switch 的 WebDAV/S3 自动同步开着，设置页"同步内容"里规则、技能、MCP 会显示"由本工具同步 / 交给 CC Switch"按钮，选之前这三类不会同步；CC Switch 自动同步关着则不出现任何提示。

## 7.6 阶段 F：Codex 全局规则与技能（隔离双设备验收）

目标：验收 Codex **全局规则和技能**（会话见阶段 G）。全程使用假目录、假配置和独立云端前缀，不碰真实的 `~/.codex`、`~/.agents`、`~/.claude`，也不往正式使用的 `v1/` 里写测试数据。

1. **准备隔离环境**：准备一个空 bucket 或空前缀（例如 `codex-acceptance/`），A、B 填同一个，**不要填正式环境的 `v1/`**。两台电脑各自新开一个 PowerShell，在项目根目录执行：

   ```powershell
   $RealHome = $env:USERPROFILE
   $S = "D:\ccs-codex-sandbox"
   $env:AGENT_SYNC_HOME = "$S\app"
   $env:CLAUDE_CONFIG_DIR     = "$S\claude"
   $env:CODEX_HOME            = "$S\home\.codex"
   $env:USERPROFILE           = "$S\home"  # ~/.agents/skills 和 CC Switch 检测也进沙箱
   New-Item -ItemType Directory -Force "$env:CODEX_HOME", "$S\home\.agents\skills\demo", "$S\claude", "$S\home\.cc-switch" | Out-Null
   '{"webdavSync":{"enabled":true,"autoSync":true}}' | Set-Content "$S\home\.cc-switch\settings.json" -Encoding utf8
   ```

   最后一行是在沙箱里伪造“CC Switch 自动同步已开启”，用来确认它只影响 Claude。之后所有 Codex 命令都必须在各自这个窗口里运行；先确认 `$env:USERPROFILE` 是沙箱路径、不等于 `$RealHome`。沙箱模式不会安装 hooks、定时任务或开机自启。

2. **首轮上传、下载和范围检查**：在 A 创建测试文件：

   ```powershell
   'A 的全局规则' | Set-Content "$env:CODEX_HOME\AGENTS.md" -Encoding utf8
   'A 的覆盖规则' | Set-Content "$env:CODEX_HOME\AGENTS.override.md" -Encoding utf8
   'A 的技能' | Set-Content "$S\home\.agents\skills\demo\SKILL.md" -Encoding utf8
   New-Item -ItemType Directory -Force "$env:CODEX_HOME\sessions", "$S\project\skills\demo", "$env:CODEX_HOME\skills\old", "$env:CODEX_HOME\skills\.system\builtin" | Out-Null
   'A 的旧位置技能' | Set-Content "$env:CODEX_HOME\skills\old\SKILL.md" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\skills\.system\builtin\SKILL.md" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\sessions\notes.jsonl" -Encoding utf8   # 不是 rollout-*.jsonl，不算会话
   '不应上传' | Set-Content "$env:CODEX_HOME\history.jsonl" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\session_index.jsonl" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\state.sqlite" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\state.sqlite-wal" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\config.toml" -Encoding utf8
   '不应上传' | Set-Content "$env:CODEX_HOME\auth.json" -Encoding utf8
   '项目级文件仍归 Git' | Set-Content "$S\project\AGENTS.md" -Encoding utf8
   '项目级文件仍归 Git' | Set-Content "$S\project\skills\demo\SKILL.md" -Encoding utf8
   ```

   A 运行 `.\agent-sync.exe` 走完向导并等首次同步完成，然后托盘退出；B 同样初始化（同一独立前缀、同一测试口令）并再点一次“立即同步”。预期：
   - B 的 `$CODEX_HOME` 下有内容一致的 `AGENTS.md`、`AGENTS.override.md`、`skills/old/SKILL.md`，`~/.agents/skills/demo/SKILL.md` 也已下载；这里的 `~` 都是沙箱目录。
   - B 没有 A 创建的 `sessions\notes.jsonl`、history/session_index、SQLite/WAL、config.toml、auth、`skills\.system` 或项目级测试文件。
   - 云端 Codex 对象只有 `tools/codex/files/rules/AGENTS.md`、`tools/codex/files/rules/AGENTS.override.md`、`tools/codex/files/skills/demo/SKILL.md`、`tools/codex/files/legacy-skills/old/SKILL.md`（加上你选择的独立前缀）；下载后能看到 `CCS1` 密文头。
   - 设置页里 Claude 的规则、技能、MCP 可以因伪造的 CC Switch 状态等待负责人选择，但 Codex 的 rules、skills 仍正常同步，没有“交给 CC Switch”的要求。

3. **GUI 文件监听**：A 从沙箱窗口运行 `.\agent-sync.exe` 打开 GUI，分别修改 `AGENTS.md` 和技能文件，不手动点同步；等首页显示自动同步完成后，B 运行 `.\agent-sync.exe sync`。预期 B 得到两处修改。测完 A 托盘退出。（在 Mac 上做这一步时，窗口开着就在监听，测完直接关窗。）

4. **三方文本合并**：两端先同步到一致。A 改 `AGENTS.md` 第一行，B 改最后一行，各自保存；A、B、A、B 依次运行 `sync`。预期两台都同时保留两处修改，没有冲突。

5. **冲突与备份恢复**：两端再同时把 `AGENTS.md` 的同一行改成不同内容；A 先 `sync`，B 后 `sync`。预期 B 的“冲突”页出现 Codex 条目，本机文件保持 B 的版本。B 在冲突页选“手工合并”保存，再同步到两端一致。随后 A 修改 `AGENTS.override.md` 并同步，B 同步下载；B 的“备份”页应出现下载前的 Codex 旧版本，先逐行对比再恢复，确认文件写回沙箱 `$CODEX_HOME`，且恢复前的版本也有备份。

6. **删除不传播与手动恢复**：A 删除 `~/.agents/skills/demo/SKILL.md` 并同步，B 再同步。预期 B 的文件仍在；A 连续同步几轮也不会自动拉回。A 运行下面的命令后，文件和内容应恢复：

   ```powershell
   .\agent-sync.exe restore-content codex skills demo/SKILL.md
   ```

7. **独立类别开关**：B 运行 `content disable codex skills`；A 修改技能文件和 `AGENTS.md` 并同步，B 再同步。预期 B 的规则更新、技能不变；B 运行 `content enable codex skills` 后再同步，技能才更新。再运行旧写法 `content disable skills`，确认它只关闭 Claude skills，不改变 Codex skills 的开关。

8. **Dry 与改口令**：两端先同步到一致。A 修改 `AGENTS.override.md`，运行 `sync --dry`；B 随后正常 `sync`，应仍是旧内容，证明 Dry 没上传。A 再正常同步、B 再同步，B 才得到新内容。最后 A 运行 `passwd` 修改**测试口令**，B 按首页提示输入新口令（或运行 `passwd`），再修改一处 Codex 规则并双向同步，预期改口令后仍能正常合并和下载。

9. **收尾**：两台电脑都在托盘退出 GUI，关闭设置过环境变量的 PowerShell，删除各自的 `D:\ccs-codex-sandbox`，再在存储控制台删除本次独立前缀。不要在真实环境运行 `uninstall` 来清理本次测试；沙箱本来就没有安装自动同步。真实的 `$RealHome\.codex`、`$RealHome\.agents` 和 `$RealHome\.claude` 应保持原样。

## 7.7 阶段 G：Codex 会话（隔离双设备验收）

目标：验收 `codex/sessions`：接着聊、各自续写分叉、Codex 打开会话时跳过、删除不拉回、待认领项目。和阶段 F 一样用沙箱和**独立云端前缀**（例如 `codex-sessions-acceptance/`），不碰真实的 `~/.codex`。

1. **准备**：两台电脑按阶段 F 第 1 步新开沙箱窗口（可以不伪造 CC Switch），并额外：
   - 把真实的 `auth.json` 和 `config.toml` 复制进沙箱 `$env:CODEX_HOME`，让沙箱里的 Codex 能登录、用同一个模型供应商（这两个文件本工具不会上传，第 2 步会顺便确认）。**两台的模型供应商名字要一样**，否则恢复会话时 Codex 找不到供应商。
   - 两台都克隆同一个 git 测试仓库，**故意放在不同路径**（例如 A 在 `D:\work\demo`，B 在 `E:\code\demo`），另外在 A 上建一个普通文件夹 `D:\plain-demo`（不是 git 仓库）。
   - 之后所有 `codex` 命令都在这个沙箱窗口里运行（`CODEX_HOME` 指向沙箱）。

2. **接着聊**：A 在 git 仓库目录运行 `codex`，说“记住暗号：蓝鲸”，等回复后退出。A `sync`，B `sync`。预期：
   - B 的 `$env:CODEX_HOME\sessions\<年>\<月>\<日>\` 下出现同名的 `rollout-*.jsonl`；用记事本看第一行 `session_meta` 里的 `cwd`，是 **B 的路径**（`E:\code\demo`），不是 A 的。
   - 云端只有 `tools/codex/sessions/<A 的设备ID>/...` 下的 rollout 文件，没有任何 `.sqlite`、`config.toml`、`auth.json`。
   - B 运行 `codex resume <会话ID>`（ID 就是文件名末尾那段），问“暗号是什么”，答得出“蓝鲸”。不带 ID 的 `codex resume` 选择器可能看不到这个会话（它只查本机数据库），属于已知限制；装了 VS Code 插件或 Codex 桌面版的话，打开一次后会话列表里应该能看到。
   - B 接着聊一句后退出，B `sync`，A `sync`。A 的文件**只在末尾多出几行**：先把 A 同步前的文件复制一份，同步后用 `fc.exe /b` 或对比工具确认前面的内容一字不差。云端多了 `tools/codex/sessions/<B 的设备ID>/...`。

3. **各自续写 → 分叉**：两端先同步到一致。A、B 都 `codex resume <同一个ID>`，分别说不同的话（A：“分支 A”；B：“分支 B”），都退出；A `sync`、B `sync`、A `sync`。预期：
   - 两台各自原来的文件都只包含自己那条续写，**没有被改写**。
   - 两台都多出一个新的 rollout 文件（ID 不同，排在原会话附近），里面是对方的续写；`codex resume <新ID>` 能打开。
   - B 再 `codex resume` 原会话续写一句并同步，A 再同步：内容**追加进 A 上已有的那份分叉**，不会再多出第三个文件。

4. **Codex 正开着会话时跳过**：两端同步到一致。A `codex resume <ID>` 后**不要退出**；B 续写这个会话并 `sync`；A 另开一个沙箱窗口运行 `sync`。预期 A 的会话文件没被追加，`sync.log` 里记录这个会话本轮跳过（不算失败）。A 退出 Codex 后再 `sync`，B 的续写才追加进来（如果 A 在打开期间也说了话，则按第 3 步分叉另存）。

5. **删除不拉回**：B 删除（或在 Codex 里归档）一个已同步的会话文件，B 连续 `sync` 几轮：文件不会回来。A `sync`：A 上的那份还在（删除不传播）。

6. **待认领项目**：A 在 `D:\plain-demo` 里运行 `codex` 聊一句后退出，A `sync`，B `sync`。预期 B **没有**这个会话，`status` 或“项目绑定”页的“待认领”里出现这个项目。B 认领到本机一个空文件夹后再 `sync`：会话被拉下来，第一行的 `cwd` 是 B 认领的路径。

7. **开关**：B 运行 `content disable codex sessions`，A 新聊一个会话并同步，B 同步：B 不会拉到；`content enable codex sessions` 后再同步才拉到。

8. **收尾**：同阶段 F 第 9 步，删除沙箱目录和本次独立前缀。

## 7.8 阶段 H：macOS

前提：一台 macOS 13 及以上、Apple 芯片的 Mac。`scripts\` 里的 PowerShell 脚本 Mac 上默认没有，下面用 `cp`、`diff` 代替。下文 `$APP` 指 `/Applications/agent-sync.app/Contents/MacOS/agent-sync`，可以先 `APP=/Applications/agent-sync.app/Contents/MacOS/agent-sync`。

1. **安装与 Gatekeeper**：从 Releases 下载 `agent-sync-<版本>-macos-arm64.dmg`，用 `shasum -a 256` 和 `SHA256SUMS.txt` 核对，双击打开后把 App 拖到“应用程序”图标上。预期：
   - DMG 窗口里有 App 和“应用程序”快捷方式两个图标。
   - 直接双击会被 Gatekeeper 拦下；macOS 15 及以上在“系统设置 → 隐私与安全性”点“仍要打开”后能启动（macOS 13/14 用右键 → 打开），之后双击不再拦。
   - 换一台（或删掉重装）验证另一种方式：`xattr -dr com.apple.quarantine /Applications/agent-sync.app` 后直接双击能打开。
   - `lipo -archs "$APP"` 输出 `arm64`。

2. **命令行**：`"$APP" version` 显示版本号；`"$APP" status`、`"$APP" sync --dry` 能正常输出（还没初始化时提示先初始化）。

3. **沙箱彩排**（不碰真实会话，不装 LaunchAgent）：

   ```bash
   S=/tmp/ccs-sandbox
   cp -Rp ~/.claude "$S/claude"
   cp -Rp "$S/claude/projects" "$S/orig-projects"      # 留一份原样用来核对
   export AGENT_SYNC_HOME="$S/app" CLAUDE_CONFIG_DIR="$S/claude"
   "$APP"                                              # 不带参数 → 图形界面；只有从这个终端启动的才是沙箱
   ```

   走完向导（用独立前缀）→ 等“同步完成”→ 关窗。核对没有文件被删：

   ```bash
   diff -rq "$S/orig-projects" "$S/claude/projects" | grep "^Only in $S/orig-projects"   # 必须没有输出
   ```

   `ls ~/Library/LaunchAgents | grep agent-sync` 应该没有输出（沙箱模式不装自动同步）。

4. **真实环境：两个 LaunchAgent**：先 `cp -Rp ~/.claude ~/ccs-backup-claude`（用 Codex 的话 `~/.codex` 也备份），再从“应用程序”正常打开 App 走向导。预期：
   - `ls ~/Library/LaunchAgents` 有 `com.alfo111111.agent-sync.sync.plist` 和 `com.alfo111111.agent-sync.daemon.plist`，plist 里的程序路径是 `/Applications/agent-sync.app/Contents/MacOS/agent-sync`。
   - `launchctl print gui/$(id -u)/com.alfo111111.agent-sync.daemon` 显示在运行；`pgrep -fl "agent-sync daemon"` 能看到后台进程。
   - `~/.claude/settings.json` 里多了两条带 `hook-event` 的 hooks。
   - 设置页关掉“开机自启”：`.daemon.plist` 消失、后台进程退出，`.sync.plist` 还在；再打开，两者都回来。
   - 注销再登录（或重启）后，不打开 App，`pgrep` 也能看到 daemon。

5. **关窗即退出**：屏幕顶部菜单栏**没有**本工具的图标；关掉 App 窗口后 `pgrep -fl agent-sync` 只剩 daemon。此时在 Claude Code 里聊一句，几秒后 `~/.agent-sync/sync.log` 有新的同步记录（hooks 和 daemon 在工作）。

6. **Windows ↔ Mac 路径翻译**：Windows 电脑 A 和 Mac B 用同一个 bucket、同一个口令，同一个 git 仓库分别克隆在 `D:\code\demo` 和 `~/code/demo`。
   - **Claude 会话**：A 上在仓库里聊一个会话，让它读写 `src` 下的某个文件；两边同步后，B 在 `~/code/demo` 里 `claude --resume` 能看到并接着聊。用文本编辑器打开 B 上的会话 jsonl，`cwd`、`file_path` 等字段是 `/Users/<你>/code/demo/src/...`，分隔符全是 `/`，没有残留的 `\`；聊天正文里提到的路径原样保留。
   - **输入历史**：B 在 Claude Code 里按 ↑ 能翻到 A 上输入过的内容，`~/.claude/history.jsonl` 里对应的项目路径是 B 的路径。
   - **反方向**：B 上续聊后同步，A 上 `claude --resume` 接着聊，路径都是 `D:\code\demo\...`、分隔符是 `\`。
   - **MCP**：A 加一个用户级 MCP，参数里用用户目录下的路径（如 `C:\Users\<你>\tools\server.js`）→ B 同步后参数变成 `/Users/<你>/tools/server.js`。A 再加一个**新的** MCP 服务器，参数指向任何项目和用户目录之外的路径（如 `E:\other\tool.exe`）→ 只要有一个路径转换不了，**这整个服务器**都不上传：B 上不出现它，A 上也不会被删；已经同步过的其他服务器不受影响。
   - **Codex 会话**：按阶段 G 第 2 步在 A 上聊一个 Codex 会话，B 同步后第一行的 `cwd` 是 `/Users/<你>/code/demo`；反过来 B 上续聊，A 上追加的行里路径是 Windows 写法。

7. **卸载**：`"$APP" uninstall`。预期两个 plist 都被删除，`pgrep -fl "agent-sync daemon"` 没有输出，`settings.json` 里的 hooks 被去掉，会话和配置都还在。注意：再打开 App 会重新装上。

## 8. 第一周的日常观察

- 每天看一眼首页的"失败"数字，有问题去"日志"页看原因。
- **备份至少留一周**再删。
- 两台电脑都接着聊了**同一个**会话的话，会多出一份"分叉"的会话，这是故意的（两份都保留，宁可多不可少）。

## 9. 出问题怎么办

1. **急停**：托盘退出，然后运行 `.\agent-sync.exe uninstall`。它会删掉 hooks 和定时任务，之后不会再自动同步。Mac 上关掉窗口后运行 `/Applications/agent-sync.app/Contents/MacOS/agent-sync uninstall`，会删掉 hooks 和两个 LaunchAgent，后台进程随之停止。注意：再打开图形界面会重新装上，排查清楚之前先别打开。
2. **看现场**：`~/.agent-sync/sync.log` 是同步日志；`~/.agent-sync/backup/` 是被覆盖掉的云端版本。
3. **恢复**：按第 2 节用 `robocopy ... /MIR` 从备份恢复。
4. 把 `compare.ps1` 的输出和 `sync.log` 发给我。
