/*
 * Copyright 2016 ThoughtWorks, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"crypto/tls"
)

type Config struct {
	Hostname           string
	SendMessageTimeout time.Duration
	ServerUrl          *url.URL
	ServerHostAndPort  string
	ContextPath        string
	WebSocketPath      string
	RegistrationPath   string
	TokenPath          string
	WorkingDir         string
	LogDir             string
	ConfigDir          string
	IpAddress          string

	AgentAutoRegisterKey             string
	AgentAutoRegisterResources       string
	AgentAutoRegisterEnvironments    string
	AgentAutoRegisterElasticAgentId  string
	AgentAutoRegisterElasticPluginId string

	GoServerCAFile      string
	AgentPrivateKeyFile string
	AgentCertFile       string
	AgentIdFile         string
	AgentTokenFile      string
	string
	OutputDebugLog      bool
}

func LoadConfig() *Config {
	gocdServerURL := readEnv("GOCD_SERVER_URL", "https://localhost:8154/go")
	os.Setenv("GO_SERVER_URL", gocdServerURL)
	serverUrl, err := url.Parse(gocdServerURL)
	if err != nil {
		panic(err)
	}
	serverUrl.Scheme = "https"
	hostname, _ := os.Hostname()
	wd, err := filepath.Abs(os.Getenv("GOCD_AGENT_WORKING_DIR"))
	if err != nil {
		panic(Sprintf("GOCD_AGENT_WORKING_DIR is invalid: %v", err))
	}
	wd = filepath.Clean(wd)
	configDir := filepath.Join(wd, readEnv("GOCD_AGENT_CONFIG_DIR", "config"))
	return &Config{
		Hostname:                         hostname,
		SendMessageTimeout:               120 * time.Second,
		ServerUrl:                        serverUrl,
		ServerHostAndPort:                serverUrl.Host,
		WorkingDir:                       wd,
		LogDir:                           os.Getenv("GOCD_AGENT_LOG_DIR"),
		ConfigDir:                        configDir,
		GoServerCAFile:                   filepath.Join(configDir, "go-server-ca.pem"),
		AgentPrivateKeyFile:              filepath.Join(configDir, "agent-private-key.pem"),
		AgentCertFile:                    filepath.Join(configDir, "agent-cert.pem"),
		AgentIdFile:                      filepath.Join(configDir, "agent-id"),
		AgentTokenFile:                   filepath.Join(configDir, "token"),
		AgentAutoRegisterKey:             os.Getenv("GOCD_AGENT_AUTO_REGISTER_KEY"),
		AgentAutoRegisterResources:       os.Getenv("GOCD_AGENT_AUTO_REGISTER_RESOURCES"),
		AgentAutoRegisterEnvironments:    os.Getenv("GOCD_AGENT_AUTO_REGISTER_ENVIRONMENTS"),
		AgentAutoRegisterElasticAgentId:  os.Getenv("GOCD_AGENT_AUTO_REGISTER_ELASTIC_AGENT_ID"),
		AgentAutoRegisterElasticPluginId: os.Getenv("GOCD_AGENT_AUTO_REGISTER_ELASTIC_PLUGIN_ID"),
		OutputDebugLog:                   os.Getenv("DEBUG") != "",
		WebSocketPath:                    readEnv("GOCD_SERVER_WEB_SOCKET_PATH", "/agent-websocket"),
		RegistrationPath:                 readEnv("GOCD_SERVER_REGISTRATION_PATH", "/admin/agent"),
		TokenPath:                        readEnv( "GOCD_SERVER_TOKEN_PATH", "/admin/agent/token"),
		IpAddress:                        lookupIpAddress(serverUrl.Host),
	}
}

func lookupIpAddress(host string) string {
	conn, err := tls.Dial("tcp", host, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return checkAllInterfaces()
	}
	ipAddress := strings.Split(conn.LocalAddr().String(), ":")[0]
	conn.Close()
	return ipAddress
}

func checkAllInterfaces() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func (c *Config) HttpsServerURL() string {
	return c.ServerUrl.String()
}

func (c *Config) WssServerURL() string {
	u, _ := url.Parse(c.HttpsServerURL())
	u.Scheme = "wss"
	return Join("/", u.String(), c.WebSocketPath)
}

func (c *Config) RegistrationURL() (*url.URL, error) {
	return c.MakeFullServerURL(c.RegistrationPath)
}

func (c *Config) TokenURL(agentID string) (*url.URL, error) {
	return c.MakeFullServerURL(c.TokenPath + "?uuid=" + agentID)
}

func (c *Config) MakeFullServerURL(u string) (*url.URL, error) {
	if strings.HasPrefix(u, "/") {
		return url.Parse(Join("/", c.HttpsServerURL(), u))
	} else {
		return url.Parse(u)
	}
}

func (c *Config) IsElasticAgent() bool {
	return config.AgentAutoRegisterElasticPluginId == ""
}

func readEnv(varname string, defaultVal string) string {
	val := os.Getenv(varname)
	if val == "" {
		return defaultVal
	} else {
		return val
	}
}
