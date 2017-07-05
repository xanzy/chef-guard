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
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func (cg *ChefGuard) executeChecks() (int, error) {
	if cfg.Tests.Foodcritic != "" {
		if errCode, err := runFoodcritic(cg.ChefOrg, cg.CookbookPath); err != nil {
			if errCode == http.StatusInternalServerError || !cg.continueAfterFailedCheck("foodcritic") {
				return errCode, err
			}
		}
	}
	if cfg.Tests.Rubocop != "" {
		if errCode, err := runRubocop(cg.CookbookPath); err != nil {
			if errCode == http.StatusInternalServerError || !cg.continueAfterFailedCheck("rubocop") {
				return errCode, err
			}
		}
	}
	return 0, nil
}

func (cg *ChefGuard) continueAfterFailedCheck(check string) bool {
	WARNING.Printf("%s errors when uploading cookbook '%s' for '%s'\n", strings.Title(check), cg.Cookbook.Name, cg.User)
	if getEffectiveConfig("Mode", cg.ChefOrg).(string) == "permissive" && cg.ForcedUpload {
		return true
	}
	return false
}

func runFoodcritic(org, cookbookPath string) (int, error) {
	args := getFoodcriticArgs(org, cookbookPath)
	cmd := exec.Command(cfg.Tests.Foodcritic, args...)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RUBY_THREAD_VM_STACK_SIZE=2097152")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.Sys().(syscall.WaitStatus).ExitStatus() == 3 {
				errText := strings.TrimSpace(strings.Replace(string(output), fmt.Sprintf("%s/", cookbookPath), "", -1))
				return http.StatusPreconditionFailed, fmt.Errorf("\n=== Foodcritic errors found ===\n%s\n===============================\n", errText)
			}
		}

		return http.StatusInternalServerError, fmt.Errorf("Failed to execute \"foodcritic %s\": %s - %s", strings.Join(cmd.Args, " "), output, err)
	}

	// This is still needed for Foodcritic > v9.x.x
	if strings.TrimSpace(string(output)) != "" {
		errText := strings.TrimSpace(strings.Replace(string(output), fmt.Sprintf("%s/", cookbookPath), "", -1))
		return http.StatusPreconditionFailed, fmt.Errorf("\n=== Foodcritic errors found ===\n%s\n===============================\n", errText)
	}

	return 0, nil
}

func getFoodcriticArgs(org, cookbookPath string) []string {
	excludes := cfg.Default.ExcludeFCs
	custExcludes := getEffectiveConfig("ExcludeFCs", org)
	if excludes != custExcludes {
		excludes = fmt.Sprintf("%s,%s", excludes, custExcludes)
	}
	args := []string{}
	if excludes != "" {
		args = append(args, "--tags", "~"+strings.Replace(excludes, ",", ",~", -1))
	}
	if cfg.Default.IncludeFCs != "" {
		args = append(args, "--include", cfg.Default.IncludeFCs)
	}
	return append(args, "--no-progress", "--cookbook-path", cookbookPath)
}

func runRubocop(cookbookPath string) (int, error) {
	cmd := exec.Command(cfg.Tests.Rubocop, cookbookPath)
	cmd.Env = []string{"HOME=" + cfg.Default.Tempdir}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "offense") {
			errText := strings.TrimSpace(strings.Replace(string(output), fmt.Sprintf("%s/", cookbookPath), "", -1))
			return http.StatusPreconditionFailed, fmt.Errorf("\n=== Rubocop errors found ===\n%s\n============================\n", errText)
		}
		return http.StatusInternalServerError, fmt.Errorf("Failed to execute \"rubocop %s\": %s - %s", cookbookPath, output, err)
	}
	return 0, nil
}
