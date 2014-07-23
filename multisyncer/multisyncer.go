//
// Copyright 2014, Sander van Harmelen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package multisyncer

type syncer chan commandData

type commandData struct {
	action commandAction
	key    string
	result chan<- interface{}
}

type commandAction int

const (
	getToken commandAction = iota
	returnToken
	end
)

type MultiSyncer interface {
	GetToken(string) <-chan bool
	ReturnToken(string) chan<- bool
}

func New() MultiSyncer {
	s := make(syncer) // type syncer chan commandData
	go s.run()
	return s
}

func (s syncer) run() {
	store := make(map[string]chan bool)
	for command := range s {
		switch command.action {
		case getToken:
			if c, exists := store[command.key]; exists {
				command.result <- c
			} else {
				c := make(chan bool, 1)
				c <- true
				store[command.key] = c
				command.result <- c
			}
		case returnToken:
			if c, exists := store[command.key]; exists {
				command.result <- c
			} else {
				c := make(chan bool, 1)
				command.result <- c
			}
		}
	}
}

func (s syncer) GetToken(key string) <-chan bool {
	reply := make(chan interface{})
	s <- commandData{action: getToken, key: key, result: reply}
	return (<-reply).(chan bool)
}

func (s syncer) ReturnToken(key string) chan<- bool {
	reply := make(chan interface{})
	s <- commandData{action: returnToken, key: key, result: reply}
	return (<-reply).(chan bool)
}
