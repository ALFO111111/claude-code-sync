<div align="center">

<img src="build/appicon.png" width="96" alt="claude-code-sync" />

# claude-code-sync

**Sync your Claude Code sessions across machines. End-to-end encrypted, paths translated automatically: switch computers and `claude --resume` right where you left off.**

[![Release](https://img.shields.io/github/v/release/ALFO111111/claude-code-sync?include_prereleases)](https://github.com/ALFO111111/claude-code-sync/releases)
[![CI](https://github.com/ALFO111111/claude-code-sync/actions/workflows/ci.yml/badge.svg)](https://github.com/ALFO111111/claude-code-sync/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Platform](https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D6)

[简体中文](README.md) | English

</div>

> [!NOTE]
> This is an **unofficial** community tool and is not affiliated with Anthropic in any way. "Claude" is a trademark of Anthropic.

<p align="center"><img src="docs/images/home-en.png" width="760" alt="Home: sync status, change details, devices, cloud usage" /></p>

<details>
<summary><b>More screenshots</b> (session browser, backups & restore, dark mode)</summary>
<br />
<p align="center"><img src="docs/images/sessions-en.png" width="760" alt="Sessions: browse, search, continue in Claude Code" /></p>
<p align="center"><img src="docs/images/backups-en.png" width="760" alt="Backups: compare, then restore with one click" /></p>
<p align="center"><img src="docs/images/home-dark-en.png" width="760" alt="Dark mode" /></p>
<p align="center"><sub>Devices, projects and paths in the screenshots are demo data (<code>cd frontend && npm run demo</code>).</sub></p>
</details>

## Why

Claude Code keeps your sessions in `~/.claude/projects/` on the local machine. If you work on a desktop at home and a laptop at the office, a conversation started on one is invisible on the other.

Copying the files over doesn't work either: the same project usually lives at different paths on each machine (`D:\work\app` vs `E:\code\app`), and sessions are full of absolute paths, so `--resume` can't match them up.

claude-code-sync solves exactly that:

- **Session sync**: sessions, sub-agent transcripts and project memory (MEMORY.md) under `~/.claude/projects`, plus your input history (what ↑ brings back)
- **Automatic path translation**: `D:\work\app` on machine A becomes `E:\code\app` on machine B, so `claude --resume` just works
- **Projects match up by themselves**: git projects are matched by remote URL (ssh and https forms alike); plain folders are claimed with one click
- **End-to-end encryption**: everything is encrypted with your passphrase before upload (Argon2id + AES-256-GCM); the cloud only sees ciphertext
- **No data loss**: sessions are only ever appended to, never overwritten wholesale; if the same session was continued on two machines, both versions are kept (one is saved as a new session); any other file is backed up before it is replaced, and the Backups page lets you compare and restore with one click
- **Fully automatic**: starts at login and sits in the tray, syncing as soon as files change; syncs the current project whenever a Claude Code conversation starts or a reply finishes; plus a full sync every hour as a safety net
- **Session browser**: sessions listed by title for every project, search across all projects, and continue any session with `claude --resume` in a terminal with one click
- **Change the passphrase any time**: every cloud file is re-encrypted with the new passphrase; other machines continue once you enter it there
- **Bring your own storage**: any S3-compatible object storage: Cloudflare R2, AWS S3, Alibaba Cloud OSS, Tencent Cloud COS, MinIO (self-hosted / NAS), …
- **GUI + CLI**: a single portable exe; the interface is available in English and Simplified Chinese

## Supported systems

**Windows 10 / 11** (64-bit) only for now. macOS and Linux are planned.

The GUI uses WebView2, which ships with Windows 11 and is usually present on Windows 10; if it's missing the app will offer to install it.

## Download

Grab `claude-code-sync.exe` from [Releases](https://github.com/ALFO111111/claude-code-sync/releases) and put it in a **permanent location**, e.g. `%LOCALAPPDATA%\Programs\claude-code-sync\`. Automatic sync remembers where the exe is; if you move it later, just open the app once and the path is fixed for you.

> [!IMPORTANT]
> Code signing is being applied for (see the [code signing policy](#code-signing-policy)). Until it's approved, released executables are unsigned, and the first launch may show "Windows protected your PC". Click **More info → Run anyway**.
> You can verify the download against `SHA256SUMS.txt` in the release, or [build it from source](CONTRIBUTING.md).

## Get started in 5 minutes

> [!WARNING]
> **Back up your sessions before the first sync.** The tool never deletes your session files, but it's your data, so a backup doesn't hurt:
> ```powershell
> robocopy "$env:USERPROFILE\.claude" "D:\claude-backup" /E
> ```

1. **Get a bucket and an access key** by following the [cloud storage setup guide](#cloud-storage-setup) below (about 3 minutes).
2. **First computer**: double-click `claude-code-sync.exe`. The wizard has 4 steps: device name → storage provider → bucket and keys (there's a "Test connection" button) → sync passphrase. After "Finish", automatic sync is enabled and a first sync runs right away.
3. **Second computer**: download and run it the same way, using **the same bucket** and **the same passphrase**. The sessions from the first computer will be pulled down.
4. **Match up projects**:
   - git projects (with a remote URL) match automatically
   - plain folders: open the "Projects" page and click "Claim" under "To claim", then pick the local folder
   - conversations started from your home folder (no project) map to the other machine's home folder automatically
5. **Continue on the other machine**: run `claude --resume` in the project folder and the sessions from the other computer are there; or find the session on the app's Sessions page and click "Continue".

> [!TIP]
> Sessions are synced, **your code is not**: use git for that. Commit and push before switching machines, otherwise the conversation and the code won't match.

<a id="cloud-storage-setup"></a>

## Cloud storage setup

It's the same idea for every provider. You need just two things:

1. a **private** bucket (no public access)
2. an access key that can **only access that bucket**. Don't use your root account keys.

Every computer must use **the same bucket and the same path prefix** (default `v1/`). That's their meeting point.

<details>
<summary><b>Cloudflare R2</b> (10 GB free, no egress fees)</summary>

1. Cloudflare dashboard → R2 → Create bucket.
2. R2 → Manage R2 API Tokens → Create API token with **Object Read & Write** permission, scoped to this bucket.
3. Save the Access Key ID, the Secret Access Key, and the S3 endpoint `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`.
4. In the app choose **Cloudflare R2**, fill in that endpoint and leave the region empty.

</details>

<details>
<summary><b>AWS S3</b></summary>

1. Create a bucket and keep "Block all public access" on.
2. IAM → create a user, attach this inline policy (replace `your-bucket`), then create an access key for it:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       { "Effect": "Allow", "Action": "s3:ListBucket", "Resource": "arn:aws:s3:::your-bucket" },
       { "Effect": "Allow", "Action": ["s3:GetObject", "s3:PutObject"], "Resource": "arn:aws:s3:::your-bucket/*" }
     ]
   }
   ```
3. In the app choose **AWS S3** and enter the bucket's region (e.g. `us-east-1`).

</details>

<details>
<summary><b>Alibaba Cloud OSS</b></summary>

1. [OSS console](https://oss.console.aliyun.com/) → create a bucket with **Private** ACL and note its region (e.g. `cn-shanghai`).
2. [RAM console](https://ram.console.aliyun.com/) → create a user with a permanent AccessKey.
3. Create a custom policy (replace `your-bucket`) and grant it to the user:

   ```json
   {
     "Version": "1",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": ["oss:ListObjects", "oss:GetObject", "oss:PutObject"],
         "Resource": ["acs:oss:*:*:your-bucket", "acs:oss:*:*:your-bucket/*"]
       }
     ]
   }
   ```
4. In the app choose **Alibaba Cloud OSS**, enter the region and leave the endpoint empty.

</details>

<details>
<summary><b>Tencent Cloud COS</b></summary>

1. [COS console](https://console.cloud.tencent.com/cos) → create a bucket with **Private** access; note its full name (like `mybucket-1250000000`) and region (like `ap-shanghai`).
2. [CAM](https://console.cloud.tencent.com/cam) → create a sub-user with programmatic access and save the SecretId / SecretKey.
3. Back in the bucket → Permission Management → Bucket ACL → add the sub-user with **read and write data** permissions.
4. In the app choose **Tencent Cloud COS**, enter the region and the full bucket name; SecretId / SecretKey go into AccessKeyId / AccessKeySecret.

</details>

<details>
<summary><b>MinIO (self-hosted / NAS) and other S3-compatible services</b></summary>

- **MinIO**: endpoint `http://your-server:9000`, and tick "Use path-style URLs". Ideally create a user that can only read/write this bucket.
- **Others** (Backblaze B2, Wasabi, …): choose "Other S3-compatible service" and fill in the endpoint and region from the provider's docs.

</details>

## How it works

```
Computer A                     Your object storage (ciphertext only)          Computer B
~/.claude/projects  ──encrypt──▶  v1/files/...  v1/devices/...  ──decrypt──▶  ~/.claude/projects
D:\work\app                       v1/history.jsonl                           translated to E:\code\app
```

- **Project identity**: each project gets an ID that's stable across machines: the git remote URL for git projects, or a one-time claim for plain folders. Paths are then replaced exactly according to that mapping, with no guessing.
- **Only translate what should be translated**: only fields whose whole value is a path (`cwd`, `file_path`, …) are rewritten; paths mentioned inside the conversation text are left untouched.
- **Store raw, translate locally**: the cloud keeps the original content; each machine translates it to its own paths when downloading.
- **Conflicts**:

  | Situation | What happens |
  |---|---|
  | Only one side changed | Normal upload or download |
  | Both changed the same session, one is a prefix of the other | The more complete one wins: only the missing records are appended to the local file, never a full rewrite (so a line Claude Code is writing right now can't be lost) |
  | Both continued the session differently | Both are kept: the other one becomes a new session |
  | MEMORY.md changed on both sides | Lines are merged |
  | Any other file changed on both sides | Local wins; the other machine's version is backed up |
  | Any other file changed only on the other machine | Downloaded; the previous local version is backed up first |
  | Old sessions cleaned up by Claude Code itself | Never pulled back |

  Backups live in `~/.claude-code-sync/backup/` for 14 days. The Backups page in the app lets you view them, compare them line by line with the current file, and restore with one click (the version being replaced is backed up too, so you can switch back).

- **Four triggers**: Claude Code hooks (`SessionStart` / `Stop`) do most of the work; the app starts at login and watches for file changes from the tray (start-at-login can be turned off in Settings); a Windows scheduled task does a full sync every hour; and you can always click "Sync now".

## Command line

The same exe works as a CLI when given arguments:

```powershell
claude-code-sync init                          # set up from the terminal (same as the wizard)
claude-code-sync sync                          # two-way sync once
claude-code-sync sync --dry                    # show what would happen, change nothing
claude-code-sync status                        # bindings and projects waiting to be claimed
claude-code-sync claim <project-id> <path>     # claim a project from another machine
claude-code-sync ignore <project-id>           # ignore / stop syncing a project (no files deleted)
claude-code-sync unignore <project-id>         # stop ignoring
claude-code-sync passwd                        # change the sync passphrase; after another machine changed it, enter the new one here
claude-code-sync uninstall                     # remove automatic sync (hooks + scheduled task + start-at-login)
claude-code-sync version
```

## FAQ

<details>
<summary><b>Can several computers share one bucket?</b></summary>

Not only can they, they **must**. The computers never talk to each other directly; the shared bucket is how they meet. If the bucket or prefix differs, nothing is synced. Different cloud accounts or keys are fine as long as they point to the same bucket.
</details>

<details>
<summary><b>How do I change the sync passphrase?</b></summary>

Go to Settings → Sync passphrase → "Change passphrase" (or run `claude-code-sync passwd`). Every file in the cloud is re-encrypted with the new passphrase, which takes a few minutes with many files; if it gets interrupted, just run it again with the same passphrases.

Afterwards your other machines pause syncing (they won't keep uploading with the old passphrase); the app's home page asks for the new passphrase, and syncing continues once it's entered.
</details>

<details>
<summary><b>I forgot my passphrase.</b></summary>

It can't be recovered: the cloud data is encrypted with it. Re-run the setup on every machine with a new passphrase and empty the `v1/` folder in the bucket to start over. Local sessions are not affected.
</details>

<details>
<summary><b>Does it work alongside CC Switch?</b></summary>

Yes. When CC Switch switches providers it may rewrite `~/.claude/settings.json` and drop this tool's hooks. They are restored within seconds while the app is running, or at the next hourly sync otherwise.
</details>

<details>
<summary><b>Where is my data? Is the access key safe?</b></summary>

- Config: `%USERPROFILE%\.claude-code-sync\config.json`. The access key is stored there in plain text (inside your user profile, readable only by your Windows account and administrators by default), so please use a key that can **only access this one bucket**.
- Encryption key: the `key` file next to it (derived from your passphrase)
- Log: `sync.log` in the same folder, also shown on the app's "Logs" page
- The cloud only stores encrypted data; your provider can't read your sessions.
</details>

<details>
<summary><b>How do I uninstall?</b></summary>

1. Run `claude-code-sync uninstall` to remove the Claude Code hooks, the scheduled task and start-at-login (sessions, config and cloud data stay)
2. Right-click the tray icon → Quit
3. Delete the exe and the `%USERPROFILE%\.claude-code-sync` folder
4. Cloud data: delete the `v1/` folder in your bucket from the provider's console
</details>

## Security and privacy

- Sessions are encrypted locally before upload: the passphrase is stretched with Argon2id into a 256-bit key; content is gzip-compressed and encrypted with AES-256-GCM, with the object path as associated data to prevent swapping.
- The app only talks to **the object storage you configured**. No telemetry, analytics or any other network requests.
- Found a security issue? Please see [SECURITY.md](SECURITY.md) instead of opening a public issue.

## Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org). (Application in progress; releases are unsigned until it's approved.)

- Committers and reviewers: [ALFO111111](https://github.com/ALFO111111)
- Approvers: [ALFO111111](https://github.com/ALFO111111)

Privacy policy: This program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it. It only sends data to the object storage configured by the user.

## Contributing

Issues and pull requests are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup, project layout and release process.

## License

[MIT](LICENSE) © ALFO111111
