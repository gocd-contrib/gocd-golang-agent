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

package protocol

import (
	"encoding/json"
	"strings"
)

const (
	TestReportFileName = "index.html"

	RunIfConfigAny    = "any"
	RunIfConfigPassed = "passed"
	ExecInput         = ""

	CommandCompose             = "compose"
	CommandCond                = "cond"
	CommandAnd                 = "and"
	CommandOr                  = "or"
	CommandExport              = "export"
	CommandTest                = "test"
	CommandExec                = "exec"
	CommandEcho                = "echo"
	CommandUploadArtifact      = "uploadArtifact"
	CommandReportCurrentStatus = "reportCurrentStatus"
	CommandReportCompleting    = "reportCompleting"
	CommandMkdirs              = "mkdirs"
	CommandCleandir            = "cleandir"
	CommandFail                = "fail"
	CommandSecret              = "secret"
	CommandDownloadFile        = "downloadFile"
	CommandDownloadDir         = "downloadDir"
	CommandGenerateTestReport  = "generateTestReport"
	CommandGenerateProperty    = "generateProperty"
)

type BuildCommand struct {
	Name             string
	Args             map[string]string
	RunIfConfig      string
	ExecInput        string
	SubCommands      []*BuildCommand
	WorkingDirectory string
	Test             *BuildCommand
	OnCancel         *BuildCommand
}

func NewBuildCommand(name string) *BuildCommand {
	return &BuildCommand{
		Name:        name,
		RunIfConfig: RunIfConfigPassed,
	}
}

func NewBuild(id, locator, locatorForDisplay,
	consoleUrl, artifactUploadBaseUrl, propertyBaseUrl string,
	commands ...*BuildCommand) *Build {
	return &Build{
		BuildId:                id,
		BuildLocator:           locator,
		BuildLocatorForDisplay: locator,
		ConsoleUrl:             consoleUrl,
		ArtifactUploadBaseUrl:  artifactUploadBaseUrl,
		PropertyBaseUrl:        propertyBaseUrl,
		BuildCommand:           ComposeCommand(commands...),
	}
}

func ComposeCommand(commands ...*BuildCommand) *BuildCommand {
	return NewBuildCommand(CommandCompose).AddCommands(commands...)
}

func CondCommand(commands ...*BuildCommand) *BuildCommand {
	return NewBuildCommand("cond").AddCommands(commands...)
}

func AndCommand(commands ...*BuildCommand) *BuildCommand {
	return NewBuildCommand("and").AddCommands(commands...)
}

func OrCommand(commands ...*BuildCommand) *BuildCommand {
	return NewBuildCommand("or").AddCommands(commands...)
}

func EchoCommand(line string) *BuildCommand {
	return NewBuildCommand(CommandEcho).AddArg("line", line)
}

func ExecCommand(args ...string) *BuildCommand {
	return NewBuildCommand(CommandExec).AddArg("command", args[0]).AddListArg("args", args[1:])
}

func ExportCommand(kvs ...string) *BuildCommand {
	args := map[string]string{"name": kvs[0]}
	if len(kvs) == 3 {
		args["value"] = kvs[1]
		args["secure"] = kvs[2]
	}
	return NewBuildCommand(CommandExport).SetArgs(args)
}

func ReportCurrentStatusCommand(jobState string) *BuildCommand {
	args := map[string]string{"status": jobState}
	return NewBuildCommand(CommandReportCurrentStatus).SetArgs(args)
}

func ReportCompletingCommand() *BuildCommand {
	return NewBuildCommand(CommandReportCompleting).RunIf("any")
}

func TestCommand(args ...string) *BuildCommand {
	argsMap := map[string]string{
		"flag": args[0],
		"left": args[1],
	}
	cmd := NewBuildCommand(CommandTest).SetArgs(argsMap)
	if len(args) > 2 {
		cmd.AddCommands(ExecCommand(args[2:]...))
	}
	return cmd
}

func SecretCommand(vs ...string) *BuildCommand {
	args := map[string]string{"value": vs[0]}
	if len(vs) == 2 {
		args["substitution"] = vs[1]
	}
	return NewBuildCommand(CommandSecret).SetArgs(args)
}

func FailCommand(msg string) *BuildCommand {
	args := map[string]string{"message": msg}
	return NewBuildCommand(CommandFail).SetArgs(args)
}

func MkdirsCommand(path string) *BuildCommand {
	args := map[string]string{"path": path}
	return NewBuildCommand(CommandMkdirs).SetArgs(args)
}

func CleandirCommand(path string, allows ...string) *BuildCommand {
	return NewBuildCommand(CommandCleandir).AddArg("path", path).AddListArg("allowed", allows)
}

func UploadArtifactCommand(src, dest, ignoreUnmatchError string) *BuildCommand {
	args := map[string]string{
		"src":                src,
		"dest":               dest,
		"ignoreUnmatchError": ignoreUnmatchError,
	}
	return NewBuildCommand(CommandUploadArtifact).SetArgs(args)
}

func DownloadFileCommand(src, url, dest, checksumUrl, checksumPath string) *BuildCommand {
	return DownloadCommand(CommandDownloadFile, src, url, dest, checksumUrl, checksumPath)
}

func DownloadDirCommand(src, url, dest, checksumUrl, checksumPath string) *BuildCommand {
	return DownloadCommand(CommandDownloadDir, src, url, dest, checksumUrl, checksumPath)
}

func DownloadCommand(file_or_dir, src, url, dest, checksumUrl, checksumPath string) *BuildCommand {
	args := map[string]string{
		"src":          src,
		"url":          url,
		"dest":         dest,
		"checksumUrl":  checksumUrl,
		"checksumFile": checksumPath,
	}
	return NewBuildCommand(file_or_dir).SetArgs(args)
}

func GenerateTestReportCommand(args ...string) *BuildCommand {
	return NewBuildCommand(CommandGenerateTestReport).AddArg("uploadPath", args[0]).AddListArg("srcs", args[1:])
}

func (cmd *BuildCommand) RunIfAny() bool {
	return strings.EqualFold(RunIfConfigAny, cmd.RunIfConfig)
}

func (cmd *BuildCommand) RunIfMatch(buildStatus string) bool {
	return strings.EqualFold(cmd.RunIfConfig, buildStatus)
}

func (cmd *BuildCommand) AddCommands(commands ...*BuildCommand) *BuildCommand {
	cmd.SubCommands = append(cmd.SubCommands, commands...)
	return cmd
}

func (cmd *BuildCommand) SetArgs(args map[string]string) *BuildCommand {
	cmd.Args = args
	return cmd
}

func (cmd *BuildCommand) AddArg(name, value string) *BuildCommand {
	if cmd.Args == nil {
		cmd.Args = make(map[string]string)
	}
	cmd.Args[name] = value
	return cmd
}

func (cmd *BuildCommand) AddListArg(name string, list []string) *BuildCommand {
	bs, err := json.Marshal(list)
	if err != nil {
		panic(err)
	}
	return cmd.AddArg(name, string(bs))
}

func (cmd *BuildCommand) SetTest(test *BuildCommand) *BuildCommand {
	cmd.Test = test
	return cmd
}

func (cmd *BuildCommand) Setwd(wd string) *BuildCommand {
	cmd.WorkingDirectory = wd
	return cmd
}

func (cmd *BuildCommand) SetExecInput(input string) *BuildCommand {
	cmd.ExecInput = input
	return cmd
}

func (cmd *BuildCommand) RunIf(c string) *BuildCommand {
	cmd.RunIfConfig = c
	return cmd
}

func (cmd *BuildCommand) SetOnCancel(c *BuildCommand) *BuildCommand {
	cmd.OnCancel = c
	return cmd
}

func (cmd *BuildCommand) ListArg(name string) (list []string, err error) {
	err = json.Unmarshal([]byte(cmd.Args[name]), &list)
	return
}
