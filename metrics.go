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

import "github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/marpaia/graphite-golang"

var metric *graphite.Graphite

func initGraphite() {
	var err error
	if cfg.Graphite.Server != "" && cfg.Graphite.Port != 0 {
		metric, err = graphite.NewGraphite(cfg.Graphite.Server, cfg.Graphite.Port)
	} else {
		metric = graphite.NewGraphiteNop(cfg.Graphite.Server, cfg.Graphite.Port)
	}

	// if you couldn't connect to graphite, use a nop
	if err != nil {
		ERROR.Printf("Failed to connect to Graphite server %s on port %d. No metrics will be send! The error was: %s\n", cfg.Graphite.Server, cfg.Graphite.Port, err)
		metric = graphite.NewGraphiteNop(cfg.Graphite.Server, cfg.Graphite.Port)
	}
}
