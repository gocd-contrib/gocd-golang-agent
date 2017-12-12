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
	"os/exec"
	"strings"
)

func CommandExec(s *BuildSession, cmd *protocol.BuildCommand) error {
	args, err := cmd.ListArg("args")
	if err != nil {
		return err
	}
	execCmd := exec.Command(cmd.Args["command"], args...)
	execCmd.Env = s.Env()
	execCmd.Stdout = s.secrets
	execCmd.Stderr = s.secrets
	execCmd.Dir = s.wd
	execCmd.Stdin = strings.NewReader(cmd.ExecInput)
	done := make(chan error)
	if err := execCmd.Start(); err != nil {
		return err
	}
	go func() {
		done <- execCmd.Wait()
	}()

	select {
	case <-s.cancel:
		s.debugLog("received cancel signal")
		LogInfo("kill process(%v) %v", execCmd.Process, cmd.Args)
		if err := execCmd.Process.Kill(); err != nil {
			LogInfo("Kill command %v failed, error: %v\n", cmd.Args, err)
		} else {
			LogInfo("process %v is killed", execCmd.Process)
		}
		return Err("%v is canceled", cmd.Args)
	case err := <-done:
		return err
	}
}
