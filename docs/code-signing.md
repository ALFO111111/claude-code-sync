# 代码签名（SignPath 免费开源签名）

> 写给维护者。没签名的 exe 第一次运行会被 Windows SmartScreen 拦一下（“Windows 已保护你的电脑”），
> 有的杀毒软件也更容易误报（本程序会建定时任务、改 `~/.claude/settings.json`）。
> [SignPath Foundation](https://signpath.org) 给开源项目免费提供代码签名证书，签名在 SignPath 的服务器上完成，私钥不会经过我们手里。

## 一、申请前要满足的条件

SignPath Foundation 的要求（以 <https://signpath.org/terms> 为准）和本项目的现状：

| 要求 | 现状 |
|---|---|
| OSI 认可的开源许可证，不能有商业双许可 | ✅ MIT |
| 不含恶意软件、不是“潜在有害程序” | ✅ |
| 项目在积极维护 | 持续提交、及时回复 issue |
| **已经公开发布过**（要签名的那种形式） | ⬜ 先用 GitHub Actions 发一个未签名的 `v0.1.0` |
| 下载页说明了软件的功能 | ✅ README |
| 主页有 **Code signing policy** 小节：致谢语、团队角色、隐私声明 | ✅ 两份 README 都已加上 |
| 团队成员在 GitHub 和 SignPath 都开启**两步验证（MFA）** | ⬜ GitHub → Settings → Password and authentication 开启 |
| 构建在 GitHub 托管的 runner 上完成 | ✅ `release.yml` 用 `windows-latest` |
| 文件元数据（产品名、版本号）正确 | ✅ `wails.json` 的 `info`，版本号由 CI 写入 |
| 提供卸载方式 | ✅ `claude-code-sync uninstall` + README 卸载说明 |

## 二、申请步骤

1. **先发一个未签名版本**：按 CONTRIBUTING.md 的发布流程打 `v0.1.0` 标签，确认 Releases 页面有 exe。
2. **开启 GitHub 两步验证。**
3. 打开 <https://signpath.org/apply>，用 GitHub 账号填申请表：仓库地址、项目简介、下载页（Releases）、代码签名策略所在页（README 的 “代码签名策略 / Code signing policy” 小节）。
4. 等审核（通常几天到几周）。通过后会收到 SignPath 组织的邀请。

## 三、通过后在 SignPath 里配置

1. 登录 <https://app.signpath.io>，开启账户的两步验证。
2. **Trusted Build Systems**：把预置的 **GitHub.com** 连接到项目，并按提示安装 SignPath 的 GitHub App 到本仓库。
3. **Project**：新建项目（slug 例如 `claude-code-sync`），关联仓库。
4. **Artifact Configuration**：粘贴下面的配置。GitHub 上传的构建产物是 zip，exe 在 zip 根目录：

   ```xml
   <?xml version="1.0" encoding="utf-8"?>
   <artifact-configuration xmlns="http://signpath.io/artifact-configuration/v1">
     <zip-file>
       <pe-file path="claude-code-sync.exe" product-name="claude-code-sync">
         <authenticode-sign />
       </pe-file>
     </zip-file>
   </artifact-configuration>
   ```

5. **Signing Policies**：一般会有 `test-signing`（测试证书，自动通过）和 `release-signing`（正式证书，每次要 Approver 手动批准）。先用 `test-signing` 跑通，再换成 `release-signing`。
6. **API Token**：为一个 CI 用户创建 API token，给它这个项目签名策略的 *Submitter* 权限。

## 四、在 GitHub 仓库里配置

仓库 → Settings → Secrets and variables → Actions：

| 类型 | 名称 | 值 |
|---|---|---|
| Secret | `SIGNPATH_API_TOKEN` | 上一步的 API token |
| Variable | `SIGNPATH_ORGANIZATION_ID` | SignPath 组织 ID（组织设置页能看到） |
| Variable | `SIGNPATH_PROJECT_SLUG` | 例如 `claude-code-sync` |
| Variable | `SIGNPATH_POLICY_SLUG` | `test-signing` 或 `release-signing` |

配好之后，`release.yml` 会自动走签名步骤（没配 `SIGNPATH_ORGANIZATION_ID` 时跳过签名，照常发布未签名版本）：

1. 构建出 `claude-code-sync.exe`，作为构建产物上传
2. 提交签名请求，等待完成（`release-signing` 要去 SignPath 网页上点批准，最多等 1 小时）
3. 下载签好名的 exe，打包发布

## 五、确认签名成功

下载发布的 exe → 右键 → 属性 → **数字签名** 页能看到 SignPath Foundation 的签名；或者：

```powershell
Get-AuthenticodeSignature .\claude-code-sync.exe | Format-List Status, SignerCertificate
```

签名生效后，把两份 README 里“正在申请中，获批前发布的 exe 没有签名”那句话删掉。
