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
	"net/http"
	"os/exec"
	"strings"
)

func (cg *ChefGuard) executeChecks() (int, error) {
	if cfg.Tests.Foodcritic != "" {
		if errCode, err := runFoodcritic(cg.Organization, cg.CookbookPath); err != nil {
			if errCode == http.StatusBadGateway || cg.continueAfterFailedCheck("foodcritic") == false {
				return errCode, err
			}
		}
	}
	if cfg.Tests.Rubocop != "" {
		if errCode, err := runRubocop(cg.CookbookPath); err != nil {
			if errCode == http.StatusBadGateway || cg.continueAfterFailedCheck("rubocop") == false {
				return errCode, err
			}
		}
	}
	return 0, nil
}

func (cg *ChefGuard) continueAfterFailedCheck(check string) bool {
	WARNING.Printf("%s errors when uploading cookbook '%s' for '%s'\n", strings.Title(check), cg.Cookbook.Name, cg.User)
	if getEffectiveConfig("Mode", cg.Organization).(string) == "permissive" && cg.ForcedUpload == true {
		go metric.SimpleSend(fmt.Sprintf("chef-guard.failed.%s.forced_bypass", check), "1")
	} else {
		go metric.SimpleSend(fmt.Sprintf("chef-guard.failed.%s.no_bypass", check), "1")
		return false
	}
	return true
}

func runFoodcritic(org, cookbookPath string) (int, error) {
	args := getFoodcriticArgs(org, cookbookPath)
	cmd := exec.Command(cfg.Tests.Foodcritic, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadGateway, fmt.Errorf("Failed to execute foodcritic tests: %s - %s", output, err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return http.StatusPreconditionFailed, fmt.Errorf("\n=== Foodcritic errors found ===\n%s\n===============================\n", strings.TrimSpace(string(output)))
	}
	return 0, nil
}

func getFoodcriticArgs(org, cookbookPath string) []string {
	excludes := cfg.Default.ExcludeFCs
	custExcludes := getEffectiveConfig("ExcludeFCs", org)
	if excludes != custExcludes {
		excludes = fmt.Sprintf("%s,%s", excludes, custExcludes)
	}
	excls := strings.Split(excludes, ",")
	args := []string{}
	for _, excl := range excls {
		args = append(args, fmt.Sprintf("-t ~%s", strings.TrimSpace(excl)))
	}
	return append(args, "-B", cookbookPath)
}

func runRubocop(cookbookPath string) (int, error) {
	cmd := exec.Command(cfg.Tests.Rubocop, cookbookPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return http.StatusBadGateway, fmt.Errorf("Failed to execute rubocop tests: %s - %s", output, err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return http.StatusPreconditionFailed, fmt.Errorf("\n=== Rubocop errors found ===\n%s\n============================\n", strings.TrimSpace(string(output)))
	}
	return 0, nil
}
