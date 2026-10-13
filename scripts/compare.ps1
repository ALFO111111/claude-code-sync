<#
.SYNOPSIS
  对比同步前后的两份快照（snapshot.ps1 生成），报告哪些文件被删除、修改、新增。

  判定标准：
    删除 > 0                     → 红色警告：有文件不见了（这个工具从不删本地会话，出现就是问题）
    修改的 .jsonl 会话           → 若给了 -OldDir/-NewDir，逐条核对对话记录（按 uuid），旧记录一条都不能少
    新增                         → 正常（从其他设备拉下来的会话，或分叉另存的会话）

.EXAMPLE
  .\scripts\compare.ps1 -Before D:\ccs-check\before.csv -After D:\ccs-check\after.csv

.EXAMPLE
  # 同时核对被修改会话的每条记录（OldDir 是同步前的备份，NewDir 是现在的目录）
  .\scripts\compare.ps1 -Before before.csv -After after.csv `
      -OldDir D:\ccs-backup\claude\projects -NewDir "$env:USERPROFILE\.claude\projects"
#>
param(
    [Parameter(Mandatory = $true)] [string] $Before,
    [Parameter(Mandatory = $true)] [string] $After,
    [string] $OldDir,
    [string] $NewDir
)
$ErrorActionPreference = 'Stop'

function Load([string] $csv) {
    $map = @{}   # PowerShell 的哈希表默认不区分大小写，和 Windows 路径一致
    foreach ($r in Import-Csv -LiteralPath $csv) { $map[$r.Path] = $r }
    return $map
}

# 一个 .jsonl 会话里所有记录的 uuid
function Uuids([string] $file) {
    $set = New-Object 'System.Collections.Generic.HashSet[string]'
    foreach ($line in [System.IO.File]::ReadLines($file)) {
        $m = [regex]::Match($line, '"uuid"\s*:\s*"([^"]+)"')
        if ($m.Success) { [void]$set.Add($m.Groups[1].Value) }
    }
    return , $set
}

$b = Load $Before
$a = Load $After

$deleted = @($b.Keys | Where-Object { -not $a.ContainsKey($_) } | Sort-Object)
$changed = @($b.Keys | Where-Object { $a.ContainsKey($_) -and $a[$_].Hash -ne $b[$_].Hash } | Sort-Object)
$added = @($a.Keys | Where-Object { -not $b.ContainsKey($_) } | Sort-Object)
$shrunk = @($changed | Where-Object { [int64]$a[$_].Length -lt [int64]$b[$_].Length })

Write-Host ""
Write-Host "同步前 $($b.Count) 个文件，同步后 $($a.Count) 个文件"
Write-Host "  删除 $($deleted.Count)   修改 $($changed.Count)   新增 $($added.Count)"
Write-Host ""

$bad = $false

if ($deleted.Count -gt 0) {
    $bad = $true
    Write-Host "【有文件不见了！】这个工具从不删除本地会话，请立刻停止使用并保留备份：" -ForegroundColor Red
    $deleted | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    Write-Host ""
}

if ($changed.Count -gt 0) {
    Write-Host "被修改的文件：" -ForegroundColor Yellow
    foreach ($p in $changed) {
        $note = ''
        if ($shrunk -contains $p) { $note = "（变小了：$($b[$p].Length) -> $($a[$p].Length) 字节）" }
        Write-Host "  * $p $note" -ForegroundColor Yellow
        if ($OldDir -and $NewDir -and $p -like '*.jsonl') {
            $old = Join-Path $OldDir $p
            $new = Join-Path $NewDir $p
            if ((Test-Path -LiteralPath $old) -and (Test-Path -LiteralPath $new)) {
                $newIds = Uuids $new
                $missing = @((Uuids $old) | Where-Object { -not $newIds.Contains($_) })
                if ($missing.Count -gt 0) {
                    $bad = $true
                    Write-Host "      【丢了 $($missing.Count) 条对话记录！】例如 uuid=$($missing[0])" -ForegroundColor Red
                } else {
                    Write-Host "      旧记录全部还在 ✓" -ForegroundColor Green
                }
            } else {
                Write-Host "      （找不到备份里的旧文件，跳过逐条核对）"
            }
        }
    }
    if (-not ($OldDir -and $NewDir)) {
        Write-Host "  提示：加上 -OldDir（同步前的备份）和 -NewDir（现在的目录）可以逐条核对对话记录有没有少"
    }
    Write-Host ""
}

if ($added.Count -gt 0) {
    Write-Host "新增的文件（从其他设备拉下来的会话，或分叉另存的会话，属正常）：$($added.Count) 个"
    $added | Select-Object -First 20 | ForEach-Object { Write-Host "  + $_" }
    if ($added.Count -gt 20) { Write-Host "  ……（只显示前 20 个）" }
    Write-Host ""
}

if ($bad) {
    Write-Host "结论：有问题！不要继续，保留备份，把上面的输出和 ~/.claude-code-sync/sync.log 发给开发者。" -ForegroundColor Red
    exit 1
}
Write-Host "结论：没有任何文件或对话记录丢失 ✓" -ForegroundColor Green
exit 0
