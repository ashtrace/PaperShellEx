package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Adaptix-Framework/axc2"
)

const (
	OS_UNKNOWN = 0
	OS_WINDOWS = 1
	OS_LINUX   = 2
	OS_MAC     = 3

	TYPE_TASK       = 1
	TYPE_BROWSER    = 2
	TYPE_JOB        = 3
	TYPE_TUNNEL     = 4
	TYPE_PROXY_DATA = 5

	MESSAGE_INFO    = 5
	MESSAGE_ERROR   = 6
	MESSAGE_SUCCESS = 7

	DOWNLOAD_STATE_RUNNING  = 1
	DOWNLOAD_STATE_STOPPED  = 2
	DOWNLOAD_STATE_FINISHED = 3
	DOWNLOAD_STATE_CANCELED = 4
)

const (
	COMMAND_CAT          = 24
	COMMAND_COPY         = 12
	COMMAND_CD           = 8
	COMMAND_DISKS        = 15
	COMMAND_DOWNLOAD     = 32
	COMMAND_EXEC_BOF     = 50
	COMMAND_EXEC_BOF_OUT = 51
	COMMAND_EXFIL        = 35
	COMMAND_GETUID       = 22
	COMMAND_JOBS_KILL    = 47
	COMMAND_JOB_LIST     = 46
	COMMAND_LINK         = 38
	COMMAND_LS           = 14
	COMMAND_MV           = 18
	COMMAND_MKDIR        = 27
	COMMAND_PIVOT_EXEC   = 37
	COMMAND_PS_LIST      = 41
	COMMAND_PS_KILL      = 42
	COMMAND_PS_RUN       = 43
	COMMAND_PROFILE      = 21
	COMMAND_PWD          = 4
	COMMAND_REV2SELF     = 23
	COMMAND_RM           = 17
	COMMAND_RUN			 = 6
	COMMAND_TERMINATE    = 10
	COMMAND_UNLINK       = 39
	COMMAND_UPLOAD       = 33

	COMMAND_TUNNEL_START_TCP = 62
	COMMAND_TUNNEL_START_UDP = 63
	COMMAND_TUNNEL_WRITE_TCP = 64
	COMMAND_TUNNEL_WRITE_UDP = 65
	COMMAND_TUNNEL_CLOSE     = 66
	COMMAND_TUNNEL_REVERSE   = 67
	COMMAND_TUNNEL_ACCEPT    = 68
	COMMAND_TUNNEL_PAUSE     = 69
	COMMAND_TUNNEL_RESUME    = 70

	COMMAND_SHELL_START  = 71
	COMMAND_SHELL_WRITE  = 72
	COMMAND_SHELL_CLOSE  = 73
	COMMAND_SHELL_ACCEPT = 74

	COMMAND_JOB        = 0x8437
	COMMAND_SAVEMEMORY = 0x2321
	COMMAND_ERROR      = 0x1111ffff
)

/// TASKS
// PackTasks converts Adaptix TaskData array into agent-consumable format.
// Called when the agent checks in to send pending tasks for execution.
func (ext *ExtenderAgent) PackTasks(agentData adaptix.AgentData, tasks []adaptix.TaskData) ([]byte, error) {

	var packData []byte

	/// START CODE HERE

	var (
		array	[]interface{}
		err		error
	)

	for _, taskData := range tasks {
		taskId, err := strconv.ParseInt(taskData.TaskId, 16, 64)
		if err != nil {
			return nil, err
		}
		array = append(array, taskData.Data)
		array = append(array, int(taskId))
	}

	packData, err = PackArray(array)
	if err != nil {
		return nil, err
	}

	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(packData)))
	packData = append(size, packData...)

	/// END CODE

	return packData, nil
}

// CreateTask converts user input from the UI into a task for the agent.
// Called when an operator executes a command in the Adaptix console.
func (ext *ExtenderAgent) CreateCommand(agentData adaptix.AgentData, args map[string]any) (adaptix.TaskData, adaptix.ConsoleMessageData, error) {
	var (
		taskData    adaptix.TaskData
		messageData adaptix.ConsoleMessageData
		err         error
	)

	command, ok := args["command"].(string)
	if !ok {
		return taskData, messageData, errors.New("'command' must be set")
	}
	// subcommand, _ := args["subcommand"].(string)

	taskData = adaptix.TaskData{
		Type: adaptix.TASK_TYPE_TASK,
		Sync: true,
	}

	messageData = adaptix.ConsoleMessageData{
		Status: adaptix.MESSAGE_INFO,
		Text:   "",
	}
	messageData.Message, _ = args["message"].(string)

	/// START CODE HERE

	var array []interface{}

	switch command {
	case "cat":
		var path string
		path, err = getStringArg(args, "path")
		if err != nil {
			goto RET
		}
		array = []interface{}{COMMAND_CAT, Ts.TsConvertUTF8toCp(path, agentData.ACP)}
	case "cd":
		var path string
		path, err = getStringArg(args, "path")
		if err != nil {
			goto RET
		}
		array = []interface{}{COMMAND_CD, Ts.TsConvertUTF8toCp(path, agentData.ACP)}
	case "ls":
		var path string
		path, _ = getStringArg(args, "path")	// If no path is provided, list current directory
		if len(path) == 0 {
			path = "."
		}
		array = []interface{}{COMMAND_LS, Ts.TsConvertUTF8toCp(path, agentData.ACP)}
	case "run":
		executable, err := getStringArg(args, "executable")
		if err != nil {
			goto RET
		}
		args, _ := args["args"].(string)
		executable	 = Ts.TsConvertUTF8toCp(executable, agentData.ACP)
		programArgs := Ts.TsConvertUTF8toCp(args, agentData.ACP)

		if programArgs != "" {
			array = []interface{}{COMMAND_RUN, executable, programArgs}
		} else {
			array = []interface{}{COMMAND_RUN, executable}
		}
	default:
		err = errors.New(fmt.Sprintf("Command '%v' not found", command))
		goto RET
	}
	
	taskData.Data, err = PackArray(array)

	/// END CODE

RET:
	return taskData, messageData, err
}

type ResultData struct {
	Path		string		`json:"path"`
	Command		int			`json:"command"`
	TaskId		int			`json:"taskId"`

	// cat
	Content		[]byte		`json:"content,omitempty"`

	// cd
	NewPath		string		`json:"new_path,omitempty"`

	// ls
	Files		[]FileInfo	`json:"files,omitempty"`

	// run
	Executable	string		`json:"executable,omitempty"`
	Args		string		`json:"args,omitempty"`
	Stdout		string		`json:"stdout,omitempty"`
	Stderr		string		`json:"stderr,omitempty"`
	ExitCode	int			`json:"exitCode,omitempty"`
}

type FileInfo struct {
	Name			string		`json:"Name"`
	FullName		string		`json:"FullName"`
	IsDirectory		bool		`json:"IsDirectory"`
	Length			*int64		`json:"Length,omitempty"`
	LastWriteTime	string		`json:"LastWriteTime"`
}

// ProcessData parses agent task responses and displays formatted output.
// Called when agent sends back task execution results.
func (ext *ExtenderAgent) ProcessData(agentData adaptix.AgentData, decryptedData []byte) error {
	var outTasks []adaptix.TaskData

	taskData := adaptix.TaskData{
		Type:        adaptix.TASK_TYPE_TASK,
		AgentId:     agentData.Id,
		FinishDate:  time.Now().Unix(),
		MessageType: adaptix.MESSAGE_SUCCESS,
		Completed:   true,
		Sync:        true,
	}

	/// START CODE

	var resultData []ResultData

	err := json.Unmarshal(decryptedData, &resultData)
	if err != nil {
		goto HANDLER
	}

	for _, taskResult := range resultData {
		task := taskData
		task.TaskId = fmt.Sprintf("%08x", taskResult.TaskId)

		command := taskResult.Command

		switch command {
		case COMMAND_CAT:
			path := Ts.TsConvertCpToUTF8(taskResult.Path, agentData.ACP)
			task.Message = fmt.Sprintf("'%v' file content:", path)
			task.ClearText = string(taskResult.Content)
		case COMMAND_CD:
			newPath := Ts.TsConvertCpToUTF8(taskResult.NewPath, agentData.ACP)
			task.Message = fmt.Sprintf("Changed directory to: %s", newPath)
		case COMMAND_LS:
			path := Ts.TsConvertCpToUTF8(taskResult.Path, agentData.ACP)
			task.Message = fmt.Sprintf("Directory listing for %s", path)

			var output strings.Builder
			
			for _, file := range taskResult.Files {
				if file.IsDirectory {
					output.WriteString(fmt.Sprintf("[DIR]  %s\n", file.Name))
				} else {
					size := "0"
					if file.Length != nil {
						size = fmt.Sprintf("%d", *file.Length)
					}
					output.WriteString(fmt.Sprintf("[FILE] %s (%s bytes)\n", file.Name, size))
				}
			}
			task.ClearText = output.String()
		case COMMAND_RUN:
			executable	:= Ts.TsConvertCpToUTF8(taskResult.Executable, agentData.ACP)
			args		:= Ts.TsConvertCpToUTF8(taskResult.Args, agentData.ACP)
			stdout		:= Ts.TsConvertCpToUTF8(taskResult.Stdout, agentData.ACP)
			stderr		:= Ts.TsConvertCpToUTF8(taskResult.Stderr, agentData.ACP)

			task.Message = fmt.Sprintf("Command executed: %s %s (Exit code: %d)\n", executable, args, taskResult.ExitCode)

			var output strings.Builder
			
			if stdout != "" {
				output.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
			}

			if stderr != "" {
				output.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
			}

			task.ClearText = output.String()
		default:
			continue
		}

		outTasks = append(outTasks, task)
	}

HANDLER:

	/// END CODE

	for _, task := range outTasks {
		Ts.TsTaskUpdate(agentData.Id, task)
	}

	return nil
}

func (ext *ExtenderAgent) Encrypt(data []byte, key []byte) ([]byte, error) {
	/// START CODE
	// return data, nil
	return RC4Crypt(data, key)
	/// END CODE
}

func (ext *ExtenderAgent) Decrypt(data []byte, key []byte) ([]byte, error) {
	/// START CODE
	// return data, nil
	return RC4Crypt(data, key)
	/// END CODE
}

func (e *ExtenderAgent) PivotPackData(pivotId string, data []byte) (adaptix.TaskData, error) {
	return adaptix.TaskData{}, fmt.Errorf("PivotPackData not implemented")
}

func (e *ExtenderAgent) TunnelCallbacks() adaptix.TunnelCallbacks {
	return adaptix.TunnelCallbacks{}
}

func (e *ExtenderAgent) TerminalCallbacks() adaptix.TerminalCallbacks {
	return adaptix.TerminalCallbacks{}
}