param([string]$BinDir = (Join-Path $env:LOCALAPPDATA 'Programs\Perplex Media Converter'))
$ErrorActionPreference = 'Stop'
$ff = Join-Path $BinDir 'ffmpeg.exe'
$fp = Join-Path $BinDir 'ffprobe.exe'
$manifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'FFMPEG-MANIFEST.json') -Raw | ConvertFrom-Json
foreach ($name in @('ffmpeg','ffprobe')) {
    $path = Join-Path $BinDir ($name + '.exe')
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash.ToLowerInvariant()
    if ($actual -ne $manifest.$name.sha256) { throw "Unexpected $name binary. Use the 1.1.6 release tools." }
}
function Invoke-Media([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Media command failed with exit code $LASTEXITCODE" }
}
$fixtures = Join-Path $PSScriptRoot 'internal\engine\testdata'
$out = Join-Path ([IO.Path]::GetTempPath()) ('perplex-smoke-' + [Guid]::NewGuid().ToString('N'))
[void](New-Item -ItemType Directory -Path $out)
try {
    Invoke-Media $ff @('-version')
    Invoke-Media $fp @('-version')
    $pairs = @(@('mkv','mp4'),@('mp4','mkv'),@('mp4','mp3'),@('avi','mp4'),@('mov','mp4'),@('webm','mp4'),@('flac','mp3'),@('wav','mp3'),@('m4a','mp3'),@('mp3','wav'))
    foreach ($pair in $pairs) {
        $inputPath = Join-Path $fixtures ('source.' + $pair[0])
        $target = Join-Path $out ($pair[0] + '-to-' + $pair[1] + '.' + $pair[1])
        $a = @('-v','error','-y','-i',$inputPath)
        if ($pair[1] -eq 'mp3') { $a += @('-vn','-c:a','libmp3lame','-b:a','192k') }
        elseif ($pair[1] -eq 'wav') { $a += @('-vn','-c:a','pcm_s16le') }
        elseif ($pair[1] -eq 'mkv') { $a += @('-c','copy') }
        else { $a += @('-c:v','libx264','-preset','fast','-crf','23','-pix_fmt','yuv420p','-c:a','aac') }
        Invoke-Media $ff ($a + @($target))
        $raw = & $fp '-v' 'error' '-show_streams' '-of' 'json' $target
        if ($LASTEXITCODE -ne 0) { throw "FFprobe failed for $target" }
        $info = ($raw -join "`n") | ConvertFrom-Json
        if (@($info.streams).Count -eq 0) { throw "No media streams in $target" }
        Write-Host ('PASS: ' + $pair[0] + ' -> ' + $pair[1])
    }
    Write-Host 'All ten conversions passed. Also test GUI preview, audio selection and uninstall manually.'
} finally {
    Remove-Item -LiteralPath $out -Recurse -Force
}
