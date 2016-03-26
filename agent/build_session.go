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
	"bytes"
	"encoding/json"
	"github.com/bmatcuk/doublestar"
	"github.com/gocd-contrib/gocd-golang-agent/protocal"
	"github.com/gocd-contrib/gocd-golang-agent/stream"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	DefaultSecretMask           = "********"
	DefaultCancelCommandTimeout = 25 * time.Second
	CancelCommandTimeout        = DefaultCancelCommandTimeout
	CancelBuildTimeout          = 30 * time.Second
)

type BuildSession struct {
	send      chan *protocal.Message
	console   *BuildConsole
	artifacts *Uploader
	command   *protocal.BuildCommand
	envs      map[string]string
	cancel    chan bool
	done      chan bool
	echo      *stream.SubstituteWriter
	secrets   *stream.SubstituteWriter

	buildId     string
	buildStatus string
}

func MakeBuildSession(buildId string,
	command *protocal.BuildCommand,
	console *BuildConsole,
	artifacts *Uploader,
	send chan *protocal.Message) *BuildSession {

	secrets := stream.NewSubstituteWriter(console)
	return &BuildSession{
		buildId:     buildId,
		buildStatus: protocal.BuildPassed,
		console:     console,
		artifacts:   artifacts,
		command:     command,
		send:        send,
		envs:        make(map[string]string),
		cancel:      make(chan bool),
		done:        make(chan bool),
		secrets:     secrets,
		echo:        stream.NewSubstituteWriter(secrets),
	}
}

func (s *BuildSession) Close() {
	close(s.cancel)
	select {
	case <-s.done:
	case <-time.After(CancelBuildTimeout):
		logger.Error.Printf("cancel build timeout(%v)", CancelBuildTimeout)
	}
}

func (s *BuildSession) isCanceled() bool {
	select {
	case <-s.cancel:
		return true
	default:
		return false
	}
}

func (s *BuildSession) Process() *protocal.Report {
	defer func() {
		close(s.done)
	}()

	LogInfo("start process build command:")
	LogInfo(s.command.String())
	s.process(s.command)
	return s.Report("")
}

func (s *BuildSession) process(cmd *protocal.BuildCommand) (err error) {
	defer s.onCancel(cmd)

	if s.isCanceled() {
		LogDebug("Ignored command %v, because build is canceled", cmd.Name)
		return nil
	}

	LogDebug("procssing build command: %v\n", cmd)
	if !cmd.RunIfAny() && !cmd.RunIfMatch(s.buildStatus) {
		//skip, no failure
		return nil
	}
	if cmd.Test != nil {
		if cmd.Test.Command.Name != protocal.CommandTest {
			panic("a BuildCommand test should be test command")
		}
		err := s.process(cmd.Test.Command)
		success := err == nil
		if success != cmd.Test.Expectation {
			LogDebug("test failed: %v", err)
			return nil
		}
	}

	switch cmd.Name {
	case protocal.CommandCompose:
		return s.processCompose(cmd)
	case protocal.CommandTest:
		return s.processTest(cmd)
	case protocal.CommandExport:
		s.processExport(cmd)
	case protocal.CommandEcho:
		s.processEcho(cmd)
	case protocal.CommandSecret:
		s.processSecret(cmd)
	case protocal.CommandReportCurrentStatus, protocal.CommandReportCompleting:
		jobState := cmd.Args["jobState"]
		s.send <- protocal.ReportMessage(cmd.Name, s.Report(jobState))
	case protocal.CommandExec:
		err = s.processExec(cmd, s.secrets)
	case protocal.CommandMkdirs:
		err = s.processMkdirs(cmd)
	case protocal.CommandCleandir:
		err = s.processCleandir(cmd)
	case protocal.CommandUploadArtifact:
		err = s.processUploadArtifact(cmd)
	case protocal.CommandFail:
		err = Err(cmd.Args["0"])
	default:
		s.ConsoleLog("TBI command: %v\n", cmd.Name)
	}

	if s.isCanceled() {
		LogInfo("Build canceled")
		s.buildStatus = protocal.BuildCanceled
	} else if err != nil {
		LogInfo("Build failed: %v", err)
		s.buildStatus = protocal.BuildFailed
		s.ConsoleLog(err.Error())
		s.ConsoleLog("\n")
	}

	return
}

func (s *BuildSession) onCancel(cmd *protocal.BuildCommand) {
	if cmd.OnCancel == nil || !s.isCanceled() {
		return
	}
	cancelSession := MakeBuildSession(s.buildId, cmd.OnCancel, s.console, s.artifacts, s.send)
	go func() {
		cancelSession.Process()
	}()
	select {
	case <-cancelSession.done:
	case <-time.After(CancelCommandTimeout):
		s.ConsoleLog("WARN: Kill cancel task because it did not finish in %v.\n", CancelCommandTimeout)
		cancelSession.Close()
	}
}

func (s *BuildSession) processSecret(cmd *protocal.BuildCommand) {
	value := cmd.Args["value"]
	substitution := cmd.Args["substitution"]
	if substitution == "" {
		substitution = DefaultSecretMask
	}
	s.secrets.Substitutions[value] = substitution
}

func (s *BuildSession) processCleandir(cmd *protocal.BuildCommand) (err error) {
	path := cmd.Args["path"]
	wd, err := filepath.Abs(cmd.WorkingDirectory)
	if err != nil {
		return
	}
	var allows []string
	err = json.Unmarshal([]byte(cmd.Args["allowed"]), &allows)
	if err != nil {
		return
	}
	return Cleandir(filepath.Join(wd, path), allows...)
}

func (s *BuildSession) processMkdirs(cmd *protocal.BuildCommand) error {
	path := cmd.Args["path"]
	wd, err := filepath.Abs(cmd.WorkingDirectory)
	if err != nil {
		return err
	}
	return Mkdirs(filepath.Join(wd, path))
}

func (s *BuildSession) processUploadArtifact(cmd *protocal.BuildCommand) error {
	src := cmd.Args["src"]
	destDir := cmd.Args["dest"]

	wd, err := filepath.Abs(cmd.WorkingDirectory)
	if err != nil {
		return err
	}
	absSrc := filepath.Join(wd, src)
	return s.uploadArtifacts(absSrc, strings.Replace(destDir, "\\", "/", -1))
}

func (s *BuildSession) uploadArtifacts(source, destDir string) (err error) {
	if strings.Contains(source, "*") {
		matches, err := doublestar.Glob(source)
		sort.Strings(matches)
		if err != nil {
			return err
		}
		base := BaseDirOfPathWithWildcard(source)
		baseLen := len(base)
		for _, file := range matches {
			fileDir, _ := filepath.Split(file)
			dest := Join("/", destDir, fileDir[baseLen:len(fileDir)-1])
			err = s.uploadArtifacts(file, dest)
			if err != nil {
				return err
			}
		}
		return nil
	}

	srcInfo, err := os.Stat(source)
	if err != nil {
		return
	}
	s.ConsoleLog("Uploading artifacts from %v to %v\n", source, destDescription(destDir))

	var destPath string
	if destDir != "" {
		destPath = Join("/", destDir, srcInfo.Name())
	} else {
		destPath = srcInfo.Name()
	}
	destURL := AppendUrlParam(AppendUrlPath(s.artifacts.BaseURL, destDir),
		"buildId", s.buildId)
	return s.artifacts.Upload(source, destPath, destURL)
}

func (s *BuildSession) processExec(cmd *protocal.BuildCommand, output io.Writer) error {
	args := cmd.ExtractArgList(len(cmd.Args))
	execCmd := exec.Command(args[0], args[1:]...)
	execCmd.Stdout = output
	execCmd.Stderr = output
	execCmd.Dir = cmd.WorkingDirectory
	done := make(chan error)
	go func() {
		done <- execCmd.Run()
	}()

	select {
	case <-s.cancel:
		LogDebug("received cancel signal")
		LogInfo("killing process(%v) %v", execCmd.Process, cmd.Args)
		if err := execCmd.Process.Kill(); err != nil {
			s.ConsoleLog("kill command %v failed, error: %v\n", cmd.Args, err)
		} else {
			LogInfo("Process %v is killed", execCmd.Process)
		}
		return Err("%v is canceled", cmd.Args)
	case err := <-done:
		return err
	}
}

func (s *BuildSession) processTest(cmd *protocal.BuildCommand) error {
	flag := cmd.Args["flag"]

	switch flag {
	case "-d":
		targetPath := cmd.Args["left"]
		info, err := os.Stat(targetPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		} else {
			return Err("%v is not a directory", targetPath)
		}
	case "-f":
		targetPath := cmd.Args["left"]
		info, err := os.Stat(targetPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return Err("%v is not a file", targetPath)
		} else {
			return nil
		}
	case "-eq":
		if len(cmd.SubCommands) == 0 || cmd.SubCommands[0].Name != "exec" {
			panic("test -eq should carry with an exec command")
		}
		var buf bytes.Buffer
		exec := cmd.SubCommands[0]
		err := s.processExec(exec, &buf)
		if err != nil {
			LogDebug("test -eq exec command error: %v", err)
		}
		expected := strings.TrimSpace(cmd.Args["left"])
		actual := strings.TrimSpace(buf.String())
		if expected != actual {
			return Err("expected '%v', but was '%v'", expected, actual)
		}
		return nil
	}

	return Err("unknown test flag")
}

func (s *BuildSession) Report(jobState string) *protocal.Report {
	return &protocal.Report{
		AgentRuntimeInfo: GetAgentRuntimeInfo(),
		BuildId:          s.buildId,
		JobState:         jobState,
		Result:           s.buildStatus,
	}
}

func (s *BuildSession) ConsoleLog(format string, a ...interface{}) {
	s.console.Write([]byte(Sprintf(format, a...)))
}

func (s *BuildSession) ReplaceEcho(name string, value interface{}) {
	s.echo.Substitutions[name] = value
}

func (s *BuildSession) processEcho(cmd *protocal.BuildCommand) {
	for _, line := range cmd.ExtractArgList(len(cmd.Args)) {
		s.echo.Write([]byte(line))
		s.echo.Write([]byte{'\n'})
	}
}

func (s *BuildSession) processExport(cmd *protocal.BuildCommand) {
	msg := "setting environment variable '%v' to value '%v'"
	name := cmd.Args["name"]
	value, ok := cmd.Args["value"]
	if !ok {
		s.ConsoleLog(msg, name, os.Getenv(name))
		s.ConsoleLog("\n")
		return
	}
	secure := cmd.Args["secure"]
	displayValue := value
	if secure == "true" {
		displayValue = DefaultSecretMask
	}
	_, override := s.envs[name]
	if override || os.Getenv(name) != "" {
		msg = "overriding environment variable '%v' with value '%v'"
	}
	s.envs[name] = value
	s.ConsoleLog(msg, name, displayValue)
	s.ConsoleLog("\n")
}

func (s *BuildSession) processCompose(cmd *protocal.BuildCommand) error {
	var err error
	for _, sub := range cmd.SubCommands {
		if err != nil {
			s.process(sub)
		} else {
			err = s.process(sub)
		}
	}
	return err
}

func destDescription(path string) string {
	if path == "" {
		return "[defaultRoot]"
	} else {
		return path
	}
}
