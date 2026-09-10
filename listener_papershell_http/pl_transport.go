package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
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

	Ssl				bool	`json:"ssl"`
	SslCert			[]byte	`json:"ssl_cert"`
	SslKey			[]byte	`json:"ssl_key"`
	SslCertPath		string	`json:"ssl_cert_path"`
	SslKeyPath		string	`json:"ssl_key_path"`

	Protocol		string	`json:"protocol"`
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
		return fmt.Errorf("Invalid address (cannot split host:port): %s\n", conf.CallbackAddress)
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

	if t.Config.Ssl {
		fmt.Printf("   Started listener '%s': https://%s:%d\n", t.Name, t.Config.HostBind, t.Config.PortBind)

		listenerPath := ListenerDataDir + "/" + t.Name
		_, err = os.Stat(listenerPath)
		if os.IsNotExist(err) {
			err = os.Mkdir(listenerPath, os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create %s folder: %s", listenerPath, err.Error())
			}
		}

		t.Config.SslCertPath	= listenerPath + "/listener.crt"
		t.Config.SslKeyPath		= listenerPath + "/listener.key"

		if len(t.Config.SslCert) == 0 || len(t.Config.SslKey) == 0 {
			err = t.generateSelfSignedCert(t.Config.SslCertPath, t.Config.SslKeyPath)
			if err != nil {
				t.Active = false
				fmt.Println("Error generating self-signed certificate: ", err);
				return err
			}
		} else {
			err = os.WriteFile(t.Config.SslCertPath, t.Config.SslCert, 0600)
			if err != nil {
				return err
			}
			err = os.WriteFile(t.Config.SslKeyPath, t.Config.SslKey, 0600)
			if err != nil {
				return err
			}
		}

		cert, err := tls.LoadX509KeyPair(t.Config.SslCertPath, t.Config.SslKeyPath)
		if err != nil {
			t.Active = false
			return fmt.Errorf("failed to load certificate: %v", err)
		}

		t.Server.TLSConfig = &tls.Config{
			Certificates:	[]tls.Certificate{cert},
			MinVersion:		tls.VersionTLS10,
			MaxVersion:		tls.VersionTLS13,
			CipherSuites:	[]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}

		go func() {
			err = t.Server.ListenAndServeTLS("", "")
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Printf("Error starting HTTPS server: %v\n", err)
				return
			}
			t.Active = true
		}()
	
	} else {
	fmt.Printf("   Started listener '%s': http://%s:%d\n", t.Name, t.Config.HostBind, t.Config.PortBind)

	// Start HTTP server in a separate goroutine
	go func() {
		err = t.Server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Error starting HTTP server: %v\n", err)
			return
		}
		t.Active = true
	}()
	}

	// Wait for the server to come up
	time.Sleep(500 * time.Millisecond)
	return err
}

func (t *TransportHTTP) Stop() error {
	var (
		ctx    			context.Context
		cancel 			context.CancelFunc
		err    			error	= nil
		listenerPath			= ListenerDataDir + "/" + t.Name
	)

	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = os.Stat(listenerPath)
	if err == nil {
		err = os.RemoveAll(listenerPath)
		if err != nil {
			return fmt.Errorf("failed to remove %s folder: %s", listenerPath, err.Error())
		}
	}

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

func (t *TransportHTTP) generateSelfSignedCert(certFile, keyFile string) error {
	var (
		certData		[]byte
		keyData			[]byte
		certBuffer		bytes.Buffer
		keyBuffer		bytes.Buffer
		privateKey		*rsa.PrivateKey
		err				error
	)

	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	serialNumberLimit	:= new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err	:= rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber:			serialNumber,
		NotBefore:				time.Now(),
		NotAfter:				time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:				x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:			[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid:	true,
	}

	hostBind := strings.TrimSpace(t.Config.HostBind)
	if hostBind == "" || hostBind == "0.0.0.0" || hostBind == "::" {
		template.DNSNames		= []string{"localhost"}
		template.IPAddresses	= []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	} else if ip := net.ParseIP(hostBind); ip != nil {
		template.IPAddresses	= []net.IP{ip}
	} else {
		template.DNSNames		= []string{hostBind}
	}

	certData, err = x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	err = pem.Encode(&certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: certData})
	if err != nil {
		return fmt.Errorf("failed to write certificate: %v", err)
	}

	t.Config.SslCert = certBuffer.Bytes()
	err = os.WriteFile(certFile, t.Config.SslCert, 0644)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %v", err)
	}

	keyData = x509.MarshalPKCS1PrivateKey(privateKey)
	err = pem.Encode(&keyBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyData})
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	t.Config.SslKey = keyBuffer.Bytes()
	err = os.WriteFile(keyFile, t.Config.SslKey, 0644)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}

	return nil
}