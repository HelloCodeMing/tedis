// Copyright 2013 Docker, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.
//
// The following only applies to changes made to this file as part of ELEME development.
//
// Portions Copyright (c) 2019 ELEME, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.  You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied.  See the License for the specific language governing permissions and limitations
// under the License.
package redis

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

func parseRequest(c *Conn) (*Request, error) {
	r := c.r
	// first line of redis request should be:
	// *<number of arguments>CRLF
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	// note that this line also protects us from negative integers
	var argsCount int

	// Multiline request:
	if line[0] == '*' {
		if _, err := fmt.Sscanf(line, "*%d\r", &argsCount); err != nil {
			return nil, malformed("*<numberOfArguments>", line)
		}
		// All next lines are pairs of:
		//$<number of bytes of argument 1> CR LF
		//<argument data> CR LF
		// first argument is a command name, so just convert
		firstArg, err := readArgument(r)
		if err != nil {
			return nil, err
		}

		args := make([][]byte, argsCount-1)
		for i := 0; i < argsCount-1; i += 1 {
			if args[i], err = readArgument(r); err != nil {
				return nil, err
			}
		}

		return &Request{
			Name: strings.ToLower(string(firstArg)),
			Args: args,
		}, nil
	}

	// Inline request:
	fields := strings.Split(strings.Trim(line, "\r\n"), " ")

	var args [][]byte
	if len(fields) > 1 {
		for _, arg := range fields[1:] {
			args = append(args, []byte(arg))
		}
	}
	return &Request{
		Name: strings.ToLower(string(fields[0])),
		Args: args,
	}, nil

}

func readArgument(r *bufio.Reader) ([]byte, error) {

	line, err := r.ReadString('\n')
	if err != nil {
		return nil, malformed("$<argumentLength>", line)
	}
	var argSize int
	if _, err := fmt.Sscanf(line, "$%d\r", &argSize); err != nil {
		return nil, malformed("$<argumentSize>", line)
	}

	// I think int is safe here as the max length of request
	// should be less then max int value?
	data, err := ioutil.ReadAll(io.LimitReader(r, int64(argSize)))
	if err != nil {
		return nil, err
	}

	if len(data) != argSize {
		return nil, malformedLength(argSize, len(data))
	}

	// Now check for trailing CR
	if b, err := r.ReadByte(); err != nil || b != '\r' {
		return nil, malformedMissingCRLF()
	}

	// And LF
	if b, err := r.ReadByte(); err != nil || b != '\n' {
		return nil, malformedMissingCRLF()
	}

	return data, nil
}

func malformed(expected string, got string) error {
	Debugf("Mailformed request:'%s does not match %s\\r\\n'", got, expected)
	return fmt.Errorf("Mailformed request:'%s does not match %s\\r\\n'", got, expected)
}

func malformedLength(expected int, got int) error {
	return fmt.Errorf(
		"Mailformed request: argument length '%d does not match %d\\r\\n'",
		got, expected)
}

func malformedMissingCRLF() error {
	return fmt.Errorf("Mailformed request: line should end with \\r\\n")
}