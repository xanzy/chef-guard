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

package main

import (
	"fmt"
	"log"
	"os"
)

var (
	INFO    *log.Logger
	WARNING *log.Logger
	ERROR   *log.Logger
)

func initLogging() error {
	l, err := os.OpenFile(cfg.Default.Logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("Failed to open log file %s: %s", cfg.Default.Logfile, err)
	}
	INFO = log.New(l, "INFO:    ", log.Ldate|log.Ltime)
	WARNING = log.New(l, "WARNING: ", log.Ldate|log.Ltime)
	ERROR = log.New(l, "ERROR:   ", log.Ldate|log.Ltime)
	return nil
}
