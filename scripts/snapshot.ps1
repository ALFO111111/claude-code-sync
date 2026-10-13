<#
.SYNOPSIS
  给一个目录拍"快照"：记下每个文件的相对路径、大小、SHA256 指纹，存成 CSV。
  同步前拍一次、同步后拍一次，再用 compare.ps1 对比，就知道到底动了哪些文件。

.EXAMPLE
  .\scripts\snapshot.ps1 -Dir "$env:USERPROFILE\.claude\projects" -Out D:\ccs-check\before.csv
#>
param(
    [Parameter(Mandatory = $true)] [string] $Dir,
    [Parameter(Mandatory = $true)] [string] $Out
)
$ErrorActionPreference = 'Stop'

$root = (Resolve-Path -LiteralPath $Dir).Path.TrimEnd('\')
$files = @(Get-ChildItem -LiteralPath $root -Recurse -File -Force)
$i = 0
$rows = foreach ($f in $files) {
    $i++
    if ($i % 200 -eq 0) { Write-Progress -Activity "拍快照" -Status "$i / $($files.Count)" -PercentComplete ($i * 100 / $files.Count) }
    [pscustomobject]@{
        Path   = $f.FullName.Substring($root.Length + 1)
        Length = $f.Length
        Hash   = (Get-FileHash -LiteralPath $f.FullName -Algorithm SHA256).Hash
    }
}
Write-Progress -Activity "拍快照" -Completed

$outDir = Split-Path -Parent $Out
if ($outDir) { New-Item -ItemType Directory -Force -Path $outDir | Out-Null }
@($rows) | Export-Csv -LiteralPath $Out -NoTypeInformation -Encoding UTF8
Write-Host "已记录 $(@($rows).Count) 个文件 -> $Out" -ForegroundColor Green
