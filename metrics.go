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
	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/marpaia/graphite-golang"
	"gopkg.in/mgo.v2"
)

var metric *graphite.Graphite

func initGraphite() {
	var err error
	if cfg.Graphite.Server != "" && cfg.Graphite.Port != 0 {
		if metric, err = graphite.NewGraphite(cfg.Graphite.Server, cfg.Graphite.Port); err != nil {
			ERROR.Printf("Failed to connect to Graphite server %s on port %d. No metrics will be send! The error was: %s\n", cfg.Graphite.Server, cfg.Graphite.Port, err)
		}
	}
	if metric == nil {
		metric = graphite.NewGraphiteNop(cfg.Graphite.Server, cfg.Graphite.Port)
	}
}

// NOTE: We should get rid of the Graphite stuff in favour of the Chef Metrics

var chefmetrics *mgo.Collection

func initChefMetrics() (*mgo.Session, error) {
	d := &mgo.DialInfo{
		Addrs:    []string{cfg.MongoDB.Server},
		Database: cfg.MongoDB.Database,
		Username: cfg.MongoDB.User,
		Password: cfg.MongoDB.Password,
	}
	session, err := mgo.DialWithInfo(d)
	if err != nil {
		return nil, err
	}

	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)

	// Assign the collection the a global var (maybe not the best way, to review later)
	chefmetrics = session.DB(cfg.MongoDB.Database).C(cfg.MongoDB.Collection)

	return session, nil
}
