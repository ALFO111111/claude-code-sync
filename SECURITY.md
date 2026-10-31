# 安全策略 / Security Policy

## 报告漏洞 / Reporting a vulnerability

**请不要在公开 issue 里报告安全问题。** 请通过 GitHub 的私密渠道报告：仓库页面 → **Security** → **Report a vulnerability**。

**Please do not report security issues in public issues.** Use GitHub's private reporting instead: repository page → **Security** → **Report a vulnerability**.

请尽量附上：影响的版本、复现步骤、可能的影响。我们会尽快确认，修复后在发布说明里致谢（如果你愿意）。

Please include the affected version, steps to reproduce and the potential impact. We'll acknowledge it as soon as possible and credit you in the release notes once fixed (if you wish).

## 支持的版本 / Supported versions

只有最新发布的版本会收到安全修复。

Only the latest release receives security fixes.

## 安全设计 / Security design

- 会话内容在本机加密后才上传：口令 → Argon2id → 256 位密钥；内容 gzip 后用 AES-256-GCM 加密，每个文件随机 nonce，对象路径作为附加数据（AAD）防止文件被调包
  Sessions are encrypted locally before upload: passphrase → Argon2id → 256-bit key; content is gzip-compressed and encrypted with AES-256-GCM using a random nonce per file, with the object path as AAD to prevent swapping
- 云端（对象存储）只能看到密文和对象路径（路径里的项目部分是哈希）
  The object storage only sees ciphertext and object keys (project parts of the keys are hashed)
- 本机 `%USERPROFILE%\.agent-sync\` 下保存派生出的密钥和存储的 AccessKey（明文），依赖 Windows 账户的文件权限保护。请使用只能访问同步用 bucket 的子账号密钥
  The derived key and the storage access key (plain text) are stored under `%USERPROFILE%\.agent-sync\`, protected by Windows account file permissions. Please use a key scoped to the sync bucket only
- 程序只和用户自己配置的对象存储通信，没有遥测；例外是检查更新（向 GitHub 查最新版本号，可在设置里关掉），下载的安装包按 Release 里的 SHA256SUMS.txt 校验后才替换
  The app only talks to the object storage configured by the user; there is no telemetry. The exception is the update check (it asks GitHub for the latest version and can be turned off in Settings); a downloaded installer replaces the app only after it matches SHA256SUMS.txt from the release

## 不在范围内 / Out of scope

- 能登录你 Windows 账户的人读取本机文件（这和 Claude Code 自己的会话文件是同一个信任边界）
  Someone with access to your Windows account reading local files (the same trust boundary as Claude Code's own session files)
- 对象存储服务商自身的安全问题
  Security issues of the object storage providers themselves
