# PaperShell agent

Simple Powershell agent for Adaptix C2

Based upon [ArturLukianov's](https://github.com/ArturLukianov/PaperShell) original papershell, this version has been made compatible with Adaptix's new (v1.2) code structure.

It includes the commands of `cat`, `cd`, `ls`, and `run` from original PaperShell.

Further implemented
- [x] beat encryption
- [x] payload encryption
- [ ] HTTPS

Installation:

```
cd listener_papershell_http
make
```
Copy dist to `AdaptixC2/dist/extenders/listener_papershell_http`

```
cd papershell_agent
make
```
Copy dist to `AdaptixC2/dist/extenders/papershell_agent`

Add new extenders to AdaptixC2 profile.json:

```json
  extenders:
    - "extenders/beacon_listener_http/config.yaml"
    - "extenders/beacon_listener_smb/config.yaml"
    - "extenders/beacon_listener_tcp/config.yaml"
    - "extenders/beacon_listener_dns/config.yaml"
    - "extenders/beacon_agent/config.yaml"
    - "extenders/gopher_listener_tcp/config.yaml"
    - "extenders/listener_papershell_http/config.yaml"
    - "extenders/papershell_agent/config.yaml"
```
