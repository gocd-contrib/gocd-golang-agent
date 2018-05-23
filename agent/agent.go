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
	"github.com/gocd-contrib/gocd-golang-agent/protocol"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var (
	buildSession *BuildSession
	logger       *Logger
	config       *Config
	AgentId      string
)

func LogDebug(format string, v ...interface{}) {
	logger.Debug.Printf(format, v...)
}

func LogInfo(format string, v ...interface{}) {
	logger.Info.Printf(format, v...)
}

func GetConfig() *Config {
	return config
}

func Initialize() {
	config = LoadConfig()
	logger = MakeLogger(config.LogDir, "gocd-golang-agent.log", config.OutputDebugLog)
	LogInfo(">>>>>>> go >>>>>>>")
	LogInfo("working directory: %v", config.WorkingDir)
	if _, err := os.Stat(config.WorkingDir); err != nil {
		logger.Error.Fatal(err)
	}

	if err := Mkdirs(config.ConfigDir); err != nil {
		logger.Error.Fatal(err)
	}

	if _, err := os.Stat(config.AgentIdFile); err == nil {
		data, err2 := ioutil.ReadFile(config.AgentIdFile)
		if err2 != nil {
			logger.Error.Printf("failed to read uuid file(%v): %v", config.AgentIdFile, err2)
		} else {
			AgentId = string(data)
		}
	}
	if AgentId == "" {
		AgentId = uuid.Must(uuid.NewV4()).String()
		ioutil.WriteFile(config.AgentIdFile, []byte(AgentId), 0644)
	}
}

func Start() error {
	err := Register()
	if err != nil {
		return err
	}

	httpClient, err := GoServerRemoteClient(true)
	if err != nil {
		return err
	}

	conn, err := MakeWebsocketConnection(config.WssServerURL(), config.HttpsServerURL())
	if err != nil {
		return err
	}
	defer conn.Close()
	defer closeBuildSession()

	pingTick := time.NewTicker(10 * time.Second)
	ping(conn.Send)
	for {
		select {
		case <-pingTick.C:
			ping(conn.Send)
		case msg, ok := <-conn.Received:
			if !ok {
				return Err("Websocket connection is closed")
			}
			err := processMessage(msg, httpClient, conn.Send)
			if err != nil {
				return err
			}
		}
	}
}

func processMessage(msg *protocol.Message, httpClient *http.Client, send chan *protocol.Message) error {
	switch msg.Action {
	case protocol.SetCookieAction:
		SetState("cookie", msg.DataString())
	case protocol.CancelBuildAction:
		closeBuildSession()
	case protocol.ReregisterAction:
		CleanRegistration()
		return Err("received reregister message")
	case protocol.BuildAction:
		closeBuildSession()
		build := msg.DataBuild()
		SetState("buildLocator", build.BuildLocator)
		SetState("buildLocatorForDisplay", build.BuildLocatorForDisplay)
		curl, err := config.MakeFullServerURL(build.ConsoleUrl)
		if err != nil {
			return err
		}
		aurl, err := config.MakeFullServerURL(build.ArtifactUploadBaseUrl)
		if err != nil {
			return err
		}
		buildSession = MakeBuildSession(
			build.BuildId,
			build.BuildCommand,
			MakeBuildConsole(httpClient, curl),
			&Artifacts{httpClient: httpClient},
			aurl,
			send,
			config.WorkingDir,
		)
		buildSession.ReplaceEcho("${agent.location}", config.WorkingDir)
		buildSession.ReplaceEcho("${agent.hostname}", config.Hostname)
		buildSession.ReplaceEcho("${date}", func() string { return time.Now().Format("2006-01-02 15:04:05 PDT") })
		go processBuild(send, buildSession)
	default:
		panic(Sprintf("Unknown message action: %+v", msg))
	}
	return nil
}

func processBuild(send chan *protocol.Message, buildSession *BuildSession) {
	defer func() {
		SetState("runtimeStatus", "Idle")
		ping(send)
		logger.Debug.Printf("! exit goroutine: process build command message")
	}()
	SetState("runtimeStatus", "Building")
	ping(send)
	buildSession.Run()
	LogInfo("done")
}

func ping(send chan *protocol.Message) {
	send <- protocol.PingMessage(GetAgentRuntimeInfo())
}

func closeBuildSession() {
	if buildSession != nil {
		buildSession.Close()
		buildSession = nil
	}
}
