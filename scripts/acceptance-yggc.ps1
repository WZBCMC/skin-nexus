<#
.SYNOPSIS
  SkinNexus yggc/OIDC 免密登录验收脚本（辅助）。

.DESCRIPTION
  按 analysis/oidc-client-contract-synthesis.md §4 十条自动走查：
  ① 元数据 openid_configuration_url  ② discovery 字段  ③ device_authorization
  ④ 站内登录+批准（可选：传 -Email/-Password 全自动；否则提示手批）
  ⑤ token 轮询（pending→成功）  ⑥ id_token RS256 声明  ⑦ userinfo 双路径
  ⑧ access_token 贯通 join/validate  ⑨ 轮询错误形状  ⑩ X-Authlib-Injector-API-Location
  输出 checklist（PASS/FAIL/SKIP），最后汇总。真机验收（A2）时当辅助工具。

.EXAMPLE
  ./scripts/acceptance-yggc.ps1 -BaseURL http://127.0.0.1:8080 `
    -Email tester@example.com -Password 'pass1234'
#>
param(
  [string]$BaseURL = 'http://127.0.0.1:8080',
  [string]$Email = '',          # 站内账号（可选：不填则需手批）
  [string]$Password = '',
  [string]$ClientID = '2'
)

$ErrorActionPreference = 'Stop'
$script:results = @()

function Report([string]$id, [string]$name, [string]$status, [string]$note = '') {
  $script:results += [pscustomobject]@{ Id = $id; Name = $name; Status = $status; Note = $note }
  $color = switch ($status) { 'PASS' { 'Green' } 'FAIL' { 'Red' } default { 'Yellow' } }
  Write-Host ("[{0}] {1} {2} {3}" -f $status, $id, $name, $note) -ForegroundColor $color
}

$api = "$BaseURL/api/yggdrasil"
$scope = 'openid offline_access Yggdrasil.PlayerProfiles.Select Yggdrasil.Server.Join'

# ---- ① 元数据 openid_configuration_url ----
$meta = $null
try {
  $meta = (Invoke-RestMethod "$api/" -TimeoutSec 15).meta
} catch { }
$oidcURL = $meta.feature.'openid_configuration_url'
if ($oidcURL -eq "$api/.well-known/openid-configuration") {
  Report '①' '元数据 openid_configuration_url' 'PASS' $oidcURL
} elseif ($oidcURL) {
  Report '①' '元数据 openid_configuration_url' 'FAIL' "got $oidcURL"
} else {
  Report '①' '元数据 openid_configuration_url' 'FAIL' 'missing (OIDC 未开启?)'
}

# ---- ② discovery 字段 ----
$disc = $null
try {
  $disc = Invoke-RestMethod "$api/.well-known/openid-configuration" -TimeoutSec 15
} catch { }
if (-not $disc) {
  Report '②' 'discovery 可达' 'FAIL' 'no document'
} else {
  $missing = @()
  foreach ($f in 'issuer','authorization_endpoint','token_endpoint','device_authorization_endpoint','userinfo_endpoint','jwks_uri','scopes_supported','subject_types_supported','id_token_signing_alg_values_supported') {
    if ($null -eq $disc.$f) { $missing += $f }
  }
  if ($missing.Count) { Report '②' 'discovery 字段' 'FAIL' ("missing: " + ($missing -join ',')) }
  elseif ($disc.shared_client_id -ne $ClientID) { Report '②' 'discovery 字段' 'FAIL' "shared_client_id=$($disc.shared_client_id)" }
  elseif (($disc.id_token_signing_alg_values_supported -join ',') -ne 'RS256') { Report '②' 'discovery 字段' 'FAIL' 'algs != [RS256]' }
  else { Report '②' 'discovery 字段（含 shared_client_id/RS256）' 'PASS' }
}

# ---- ③ device_authorization ----
$da = $null
try {
  $da = Invoke-RestMethod "$api/device_authorization" -Method Post -Body @{ client_id = $ClientID; scope = $scope } -TimeoutSec 15
} catch { }
if (-not $da.device_code -or $da.user_code -notmatch '^[A-Z]{4}-[A-Z]{4}$' -or $da.expires_in -ne 600) {
  Report '③' 'device_authorization 响应形状' 'FAIL' (($da | ConvertTo-Json -Compress) -as [string])
} else {
  Report '③' 'device_authorization（user_code 形状/expires_in 600）' 'PASS' $da.user_code
}

if (-not $da) { Write-Host "`n== 无 device 码，中止 ==" ; exit 1 }

# ---- ⑨a 轮询：批准前必须是 400 authorization_pending ----
try {
  Invoke-RestMethod "$api/oauth/token" -Method Post -Body @{ client_id = $ClientID; grant_type = 'urn:ietf:params:oauth:grant-type:device_code'; device_code = $da.device_code } -TimeoutSec 15 | Out-Null
  Report '⑨' 'pending 轮询形状' 'FAIL' 'expected 400 authorization_pending, got success'
} catch {
  $resp = $_.ErrorDetails.Message
  if ($_.Exception.Response.StatusCode.value__ -eq 400 -and $resp -match 'authorization_pending') {
    Report '⑨' 'pending 轮询 400 authorization_pending' 'PASS'
  } else {
    Report '⑨' 'pending 轮询 400 authorization_pending' 'FAIL' "$($_.Exception.Response.StatusCode.value__): $resp"
  }
}

# ---- ④ 批准：自动化登录 or 手批提示 ----
$approveURL = "$BaseURL/device?user_code=$($da.user_code)"
if ($Email -and $Password) {
  try {
    $sess = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    Invoke-RestMethod "$BaseURL/api/v1/login" -Method Post -Body (@{ identifier = $Email; password = $Password } | ConvertTo-Json) `
      -ContentType 'application/json' -WebSession $sess -TimeoutSec 15 | Out-Null
    $sub = Invoke-RestMethod "$BaseURL/api/v1/oidc/device/submit" -Method Post `
      -Body (@{ user_code = $da.user_code; decision = 'approve'; pid = 0 } | ConvertTo-Json) `
      -ContentType 'application/json' -WebSession $sess -TimeoutSec 15
    if ($sub.ok) { Report '④' '验证页批准（自动登录）' 'PASS' 'pid=0 将单角色自动选' }
    else { Report '④' '验证页批准（自动登录）' 'FAIL' ($sub | ConvertTo-Json -Compress) }
  } catch {
    Report '④' '验证页批准（自动登录）' 'FAIL' $_.Exception.Message
  }
} else {
  Report '④' '验证页批准' 'SKIP' "请浏览器手批：$approveURL"
  Write-Host "`n>> 在浏览器打开 $approveURL 完成登录+批准+选角后回车继续 <<"
  [void](Read-Host)
}

# ---- ⑤ token 轮询成功 ----
$tok = $null
foreach ($i in 1..12) {
  Start-Sleep -Seconds 2
  try {
    $tok = Invoke-RestMethod "$api/oauth/token" -Method Post -Body @{ client_id = $ClientID; grant_type = 'urn:ietf:params:oauth:grant-type:device_code'; device_code = $da.device_code } -TimeoutSec 15
    break
  } catch {
    if ($_.ErrorDetails.Message -notmatch 'authorization_pending|slow_down') {
      Report '⑤' 'token 轮询' 'FAIL' $_.ErrorDetails.Message; break
    }
  }
}
if ($tok) {
  $miss = @('access_token','token_type','expires_in','refresh_token','id_token') | Where-Object { $null -eq $tok.$_ }
  if ($miss) { Report '⑤' 'token 响应字段' 'FAIL' ("missing: " + ($miss -join ',')) }
  elseif ($tok.token_type -ne 'Bearer') { Report '⑤' 'token 响应字段' 'FAIL' "token_type=$($tok.token_type)" }
  else { Report '⑤' 'token 响应（Bearer/恒含 refresh+id）' 'PASS' }
} else {
  Report '⑤' 'token 轮询' 'FAIL' 'no token (批准了吗?)'
}

# ---- ⑥ id_token 声明（解码 payload；签名校验交给启动器/测试） ----
if ($tok) {
  $parts = $tok.id_token.Split('.')
  $payload = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String(($parts[1] + '==').Replace('-','+').Replace('_','/')))
  $claims = $payload | ConvertFrom-Json
  $bad = @()
  if ($claims.iss -ne $BaseURL) { $bad += "iss=$($claims.iss)" }
  if (-not $claims.sub) { $bad += 'no sub' }
  if (-not ($claims.aud -contains $ClientID -or $claims.aud -eq $ClientID)) { $bad += "aud=$($claims.aud)" }
  if (-not $claims.selectedProfile.id -or -not $claims.selectedProfile.name) { $bad += 'no selectedProfile{id,name}' }
  if ($bad.Count) { Report '⑥' 'id_token 声明' 'FAIL' ($bad -join '; ') }
  else { Report '⑥' 'id_token 声明（iss/sub/aud/selectedProfile）' 'PASS' "sub=$($claims.sub) profile=$($claims.selectedProfile.name)" }
}

# ---- ⑦ userinfo 双路径 ----
if ($tok) {
  foreach ($p in '/userinfo', '/yggc/userinfo') {
    try {
      $ui = Invoke-RestMethod "$api$p" -Headers @{ Authorization = "Bearer $($tok.access_token)" } -TimeoutSec 15
      if ($ui.sub -and $null -ne $ui.selectedProfile) { Report '⑦' "userinfo $p" 'PASS' }
      else { Report '⑦' "userinfo $p" 'FAIL' ($ui | ConvertTo-Json -Compress) }
    } catch {
      Report '⑦' "userinfo $p" 'FAIL' $_.Exception.Message
    }
  }
  # 401 形状
  try {
    Invoke-RestMethod "$api/userinfo" -TimeoutSec 15 | Out-Null
    Report '⑦' 'userinfo 无票 401 形状' 'FAIL' 'expected 401'
  } catch {
    $h = $_.Exception.Response.Headers.WwwAuthenticate
    if ($_.Exception.Response.StatusCode.value__ -eq 401 -and "$h" -match 'realm="openid"') {
      Report '⑦' 'userinfo 无票 401 + WWW-Authenticate' 'PASS'
    } else {
      Report '⑦' 'userinfo 无票 401 + WWW-Authenticate' 'FAIL' "code=$($_.Exception.Response.StatusCode.value__) header=$h"
    }
  }
}

# ---- ⑧ access_token 贯通 validate（join 需游戏侧 serverId，真机 A2 验） ----
if ($tok) {
  try {
    Invoke-RestMethod "$api/authserver/validate" -Method Post -Body $tok.access_token `
      -ContentType 'text/plain' -TimeoutSec 15 | Out-Null
    Report '⑧' 'validate 接受 OAuth access_token' 'PASS'
  } catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 204 -or $_.Exception.Response.StatusCode.value__ -eq 200) {
      Report '⑧' 'validate 接受 OAuth access_token' 'PASS'
    } else {
      Report '⑧' 'validate 接受 OAuth access_token' 'FAIL' "$($_.Exception.Response.StatusCode.value__): $($_.ErrorDetails.Message)"
    }
  }
  # 吊销判据：join/validate 的 scope 闸门在 jti 台账（A2 时配合授权管理页验）
  Report '⑧' 'join scope 闸门 + 选角一致' 'SKIP' '需游戏服务器，A2 真机验'
}

# ---- ⑩ X-Authlib-Injector-API-Location ----
try {
  $r = Invoke-WebRequest "$api/" -TimeoutSec 15 -SkipHttpErrorCheck
  if ($r.Headers['X-Authlib-Injector-API-Location']) { Report '⑩' 'X-Authlib-Injector-API-Location 头' 'PASS' }
  else { Report '⑩' 'X-Authlib-Injector-API-Location 头' 'FAIL' 'missing' }
} catch {
  Report '⑩' 'X-Authlib-Injector-API-Location 头' 'FAIL' $_.Exception.Message
}

# ---- 汇总 ----
Write-Host "`n========== 验收汇总 =========="
$script:results | Format-Table -AutoSize
$pass = @($script:results | Where-Object Status -eq 'PASS').Count
$fail = @($script:results | Where-Object Status -eq 'FAIL').Count
$skip = @($script:results | Where-Object Status -eq 'SKIP').Count
Write-Host "PASS=$pass FAIL=$fail SKIP=$skip"
if ($fail -gt 0) { exit 1 }
