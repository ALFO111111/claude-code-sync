# Changelog

本文件记录每个版本的变化，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。
发布时 GitHub Actions 会把对应版本这一节作为 Release 说明。

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

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
