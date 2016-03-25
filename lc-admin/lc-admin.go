/*
 * Copyright 2014-2015 Jason Woods.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/driskell/log-courier/lc-lib/admin"
	"github.com/driskell/log-courier/lc-lib/config"
	"github.com/driskell/log-courier/lc-lib/core"
)

type commandProcessor interface {
	ProcessCommand(string) bool
}

type lcAdmin struct {
	quiet        bool
	watch        bool
	legacy       bool
	adminConnect string
	configFile   string

	client *admin.Client
}

func main() {
	(&lcAdmin{}).Run()
}

func (a *lcAdmin) printHelp() {
	fmt.Printf("Available commands:\n")
	fmt.Printf("  prospector [status | files [id]]\n")
	fmt.Printf("    Get information on prospector state and running harvesters\n")
	fmt.Printf("  publisher [status | endpoints [id]]\n")
	fmt.Printf("    Get information on connectivity and endpoints\n")
	fmt.Printf("  version\n")
	fmt.Printf("    Get the remote version\n")
	fmt.Printf("  exit\n")
	fmt.Printf("    Exit\n")
}

func (a *lcAdmin) startUp() {
	var version bool

	flag.BoolVar(&version, "version", false, "display the Log Courier client version")
	flag.BoolVar(&a.quiet, "quiet", false, "quietly execute the command line argument and output only the result")
	flag.BoolVar(&a.watch, "watch", false, "repeat the command specified on the command line every second")
	flag.BoolVar(&a.legacy, "legacy", false, "connect to version 1.x Log Courier instances")
	flag.StringVar(&a.adminConnect, "connect", "", "the Log Courier instance to connect to")
	flag.StringVar(&a.configFile, "config", config.DefaultConfigurationFile, "read the Log Courier connection address from the given configuration file (ignored if connect specified)")

	flag.Parse()

	if version {
		fmt.Printf("Log Courier version %s\n", core.LogCourierVersion)
		os.Exit(0)
	}

	if !a.quiet {
		fmt.Printf("Log Courier version %s client\n\n", core.LogCourierVersion)
	}
}

func (a *lcAdmin) loadConfig() {
	if a.configFile == "" && a.adminConnect == "" {
		if config.DefaultGeneralAdminBind == "" {
			fmt.Printf("Either connect or config parameter must be specified\n")
			flag.PrintDefaults()
			os.Exit(1)
		} else {
			a.adminConnect = config.DefaultGeneralAdminBind
		}
	}

	if a.adminConnect == "" {
		// Load admin connect address from the configuration file
		config := config.NewConfig()
		if err := config.Load(a.configFile, false); err != nil {
			fmt.Printf("Configuration error: %s\n", err)
			os.Exit(1)
		}

		a.adminConnect = config.Get("admin").(*admin.Config).Bind
	}
}

func (a *lcAdmin) Run() {
	a.startUp()

	admin, err := a.newCommandProcessor()
	if err != nil {
		fmt.Printf("Failed to initialise: %s\n", err)
		os.Exit(1)
		return
	}

	prompt := &prompt{commandProcessor: admin}
	args := flag.Args()

	if len(args) != 0 {
		if prompt.argsCommand(args, a.watch) {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if a.quiet {
		fmt.Printf("No command specified on the command line for quiet execution\n")
		os.Exit(1)
	}

	if a.watch {
		fmt.Printf("No command specified on the command line to watch\n")
		os.Exit(1)
	}

	prompt.run()
}

func (a *lcAdmin) newCommandProcessor() (commandProcessor, error) {
	if a.legacy {
		// Create the old V1 legacy processor
		return newV1LcAdmin(a.quiet, a.adminConnect)
	}

	if !a.quiet {
		fmt.Printf("Attempting connection to %s...\n", a.adminConnect)
	}

	client, err := admin.NewClient(a.adminConnect)
	if err != nil {
		return nil, err
	}

	a.client = client

	if !a.quiet {
		fmt.Printf("Connected to Log Courier version %s\n\n", client.RemoteVersion())
	}

	return a, nil
}

func (a *lcAdmin) ProcessCommand(command string) bool {
	if command == "help" {
		a.printHelp()
		return true
	}

	command = url.QueryEscape(command)

	path := strings.Map(func(r rune) rune {
		if r == '+' {
			return '/'
		}
		return r
	}, command)

	resp, err := a.client.Request(path)
	if err != nil {
		if clientErr, ok := err.(admin.ClientError); ok {
			fmt.Printf("Log Courier returned an error: %s\n", clientErr)
			return false
		}
		fmt.Printf("The API request failed: %s\n", err)
		return false
	}

	fmt.Println(resp)

	return true
}
