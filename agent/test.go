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
	"os"
	"path/filepath"
	"strings"
)

func CommandTest(s *BuildSession, cmd *protocol.BuildCommand) error {
	flag := cmd.Args["flag"]

	if flag == "-eq" || flag == "-neq" || flag == "-in" || flag == "-nin" {
		output, err := s.processTestCommand(cmd.SubCommands[0])
		if err != nil {
			s.debugLog("test -eq exec command error: %v", err)
		}
		expected := strings.TrimSpace(cmd.Args["left"])
		actual := strings.TrimSpace(output.String())

		if flag == "-eq" {
			if expected != actual {
				return Err("expected '%v', but was '%v'", expected, actual)
			}
		} else if flag == "-neq" {
			if expected == actual {
				return Err("expected different with '%v'", expected)
			}
		} else if flag == "-in" {
                  return strings.contains(expected, actual);
                } else if flag == "-nin" {
                  return !strings.contains(expected, actual);
                }
		return nil
	}

	targetPath := filepath.Join(s.wd, cmd.Args["left"])
	info, err := os.Stat(targetPath)
	switch flag {
	case "-d":
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		} else {
			return Err("%v is not a directory", targetPath)
		}
	case "-nd":
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return Err("%v is a directory", targetPath)
		} else {
			return nil
		}
	case "-f":
		if err != nil {
			return err
		}
		if info.IsDir() {
			return Err("%v is not a file", targetPath)
		} else {
			return nil
		}
	case "-nf":
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		} else {
			return Err("%v is a file", targetPath)
		}
	}

	return Err("unknown test flag: %v", flag)
}
