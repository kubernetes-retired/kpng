$ErrorActionPrefernce = "Stop"

Write-Host "Importing hns.pms1"
Import-Module .\hns.psm1

$NetworkName = "Calico"
Write-Host "Waiting for network '$NetworkName' to be available..."
while (-Not (Get-HnsNetwork | ? Name -EQ $NetworkName)) {
    Write-Debug "Still waiting for HNS network..."
    Start-Sleep 5
}
Write-Host "Found HNS network '$NetworkName'"

# TODO: add node-ip and cluster-cidr here once those values are exposed as flags
$argList = @(`
    "local", `
    "to-winkernel", `
    "-v=4"`
#    "--enable-dsr=true" TODO expose this as a flag..
)

Write-Host "Getting source vip"

$network = (Get-HnsNetwork | ? Name -EQ $NetworkName)
if ($network.Type -EQ "Overlay") {
    Write-Host "Overlay / VXLAN network detected... waiting for host endpoint to be created..."
    while (-not (Get-HnsEndpoint | ? Name -EQ "${NetworkName}_ep")) {
        Start-Sleep 1
    }
    $sourceVip = (Get-HnsEndpoint | ? Name -EQ "${NetworkName}_ep").IpAddress
    Write-Host "Host endpoint found. Source VIP: $sourceVip"
    # $argList += "--source-vip=$sourceVip" -- right now this is hard-coded :'(
}

Start-Sleep 5 # wait for kube to-api to start?

$env:KUBE_NETWORK=$NetworkName
Invoke-Expression "c:\hpc\kpng.exe $argList"