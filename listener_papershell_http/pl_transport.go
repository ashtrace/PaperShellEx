package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TransportConfig struct {
	HostBind        string `json:"host_bind"`
	PortBind        int    `json:"port_bind"`
	CallbackAddress string `json:"callback_address"`
	EncryptKey		string `json:"encrypt_key"`
}

type TransportHTTP struct {
	GinEngine *gin.Engine
	Server    *http.Server
	Config    TransportConfig
	Name      string
	Active    bool
}

type Listener struct {
	transport *TransportHTTP
}

func validConfig(config string) error {
	var conf TransportConfig

	err := json.Unmarshal([]byte(config), &conf)
	if err != nil {
		return err
	}

	if conf.HostBind == "" {
		return errors.New("HostBind is required")
	}

	if conf.PortBind < 1 || conf.PortBind > 65535 {
		return errors.New("PortBind must be in the range 1-65535")
	}

	if conf.CallbackAddress == "" {
		return errors.New("callback_address is required")
	}

	// Validate the callback address format and input

	host, portStr, err := net.SplitHostPort(conf.CallbackAddress)
	if err != nil {
		return fmt.Errorf("nvalid address (cannot split host:port): %s\n", conf.CallbackAddress)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("Invalid port: %s\n", conf.CallbackAddress)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		if len(host) == 0 || len(host) > 253 {
			return fmt.Errorf("Invalid host: %s\n", conf.CallbackAddress)
		}
		parts := strings.Split(host, ".")
		for _, part := range parts {
			if len(part) == 0 || len(part) > 63 {
				return fmt.Errorf("Invalid host: %s\n", conf.CallbackAddress)
			}
		}
	}

	match, _ := regexp.MatchString("^[0-9a-f]{32}$", conf.EncryptKey)
	if len(conf.EncryptKey) != 32 || !match {
		return errors.New("encrypt_key must be 32 hex characters")
	}

	return nil
}

func (t *TransportHTTP) Start(ts Teamserver) error {
	var err error = nil

	// Configureing Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Bind our request handler to the path
	router.POST("/api/:id/envelope", t.processRequest)

	// Initializable listener status
	t.Active = true

	// Create an HTTP Server
	t.Server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", t.Config.HostBind, t.Config.PortBind),
		Handler: router,
	}

	fmt.Printf("   Started listener: http://%s:%d\n", t.Config.HostBind, t.Config.PortBind)

	// Start HTTP server in a separate goroutine
	go func() {
		err = t.Server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Error starting HTTP server: %v\n", err)
			return
		}
		t.Active = true
	}()

	// Wait for the server to come up
	time.Sleep(500 * time.Millisecond)
	return err
}

func (t *TransportHTTP) Stop() error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
		err    error = nil
	)

	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = t.Server.Shutdown(ctx)
	return err
}

func (t *TransportHTTP) processRequest(ctx *gin.Context) {
	var (
		ExternalIP		string
		agentType		string
		agentId			string
		beat			[]byte
		bodyData		[]byte
		responseData	[]byte
		err				error
	)

	// Get the IP of the connected agent
	ExternalIP = strings.Split(ctx.Request.RemoteAddr, ":")[0]

	// Parsing data transmitted by the agent
	agentType, agentId, beat, bodyData, err = t.parseBeatAndData(ctx)
	if err != nil {
		goto ERR
	}

	if !Ts.TsAgentIsExists(agentId) {
		_, err = Ts.TsAgentCreate(agentType, agentId, beat, t.Name, ExternalIP, true)
		if err != nil {
			goto ERR
		}
	}

	_ = Ts.TsAgentSetTick(agentId, t.Name)

	_ = Ts.TsAgentProcessData(agentId, bodyData)

	responseData, err = Ts.TsAgentGetHostedAll(agentId, 0x1900000) // maxDataSize: 25 Mb

	if err != nil {
		goto ERR
	} else {
		hexEncodedResponseData := hex.EncodeToString(responseData)
		response := `{"id": "` + hexEncodedResponseData + `"}`

		// Construct the server response
		ctx.Writer.Header().Add("Content-Type", "application/json")
		_,err = ctx.Writer.Write([]byte(response))
		if err != nil {
			// If an error occurs, return a 404
			fmt.Println("Failed to write to request: " + err.Error())
			ctx.Writer.WriteHeader(http.StatusNotFound)
			return
		}
	}

	ctx.AbortWithStatus(http.StatusOK)
	return

ERR:
	// If an error occurs, return a 404
	fmt.Println("Error: " + err.Error())	// Was TBD in original project with msg: "Leave debugging for now"
	ctx.Writer.WriteHeader(http.StatusNotFound)
}

type BeatLine struct {
	EventId string `json:"event_id"`
}

type DataLine struct {
	Spans []struct {
		Description string `json:"description"`
	} `json:"spans"`
}

func (t *TransportHTTP) parseBeatAndData(ctx *gin.Context) (string, string, []byte, []byte, error) {
	var (
		agentType	uint
		agentId		uint
		agentInfo	[]byte
		bodyData	[]byte
		firstLine	[]byte
		thirdLine	[]byte
		agentData	[]byte
		err			error
	)

	bodyData, err = io.ReadAll(ctx.Request.Body)

	if err != nil {
		return "", "", nil, nil, errors.New("Missing POST data")
	}

	lines := bytes.Split(bodyData, []byte{'\n'})

	if len(lines) < 3 {
		return "", "", nil, nil, errors.New("Missing data - less than 3 lines")
	}
	firstLine = lines[0]
	thirdLine = lines[2]

	// Parse beat (first line of json -> event_id)
	var beatLine BeatLine

	err = json.Unmarshal(firstLine, &beatLine)
	if err != nil {
		return "", "", nil, nil, errors.New("Failed to decode beat")
	}

	agentInfoEncoded := beatLine.EventId

	strFields := strings.Fields(agentInfoEncoded)

	var agentInfoEncrypted []byte

	for _, field := range strFields {
		b, err := strconv.ParseUint(field, 10, 8)
		if err != nil {
			return "", "", nil, nil, errors.New("Failed to decode agentInfoEncoded")
		}
		agentInfoEncrypted = append(agentInfoEncrypted, byte(b))
	}

	encryptKey, err := hex.DecodeString(t.Config.EncryptKey)
	if err != nil {
		return "", "", nil, nil, errors.New("Failed to decode encryptKey")
	}

	agentInfoDecrypted, err := RC4Crypt(agentInfoEncrypted, encryptKey)
	if err != nil {
		return "", "", nil, nil, errors.New("Failed to decyrpt agentInfo")
	}

	agentInfoHex := string(agentInfoDecrypted)

	agentInfo, err = hex.DecodeString(agentInfoHex)
	if err != nil {
		return "", "", nil, nil, errors.New("Failed to decode agentInfoEncoded")
	}

	agentType	= uint(binary.LittleEndian.Uint32(agentInfo[:4]))
	agentInfo	= agentInfo[4:]
	agentId		= uint(binary.LittleEndian.Uint32(agentInfo[:4]))
	agentInfo	= agentInfo[4:]

	// Parse beat (third line of json)
	var dataLine DataLine
	
	err = json.Unmarshal(thirdLine, &dataLine)
	if err != nil {
		return "", "", nil, nil, errors.New("Failed to decode data")
	}

	if len(dataLine.Spans) == 0 {
		return "", "", nil, nil, errors.New("Failed to decode data - no spans")
	}

	agentDataEncoded := dataLine.Spans[0].Description
	agentData, err = hex.DecodeString(agentDataEncoded)
	if err != nil {
		return "", "", nil, nil, errors.New("Failed to decode agentData")
	}

	return fmt.Sprintf("%08x", agentType), fmt.Sprintf("%08x", agentId), agentInfo, agentData, nil
} 
