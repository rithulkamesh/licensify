// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: examples/go-sdk/main.go — Minimal Go SDK usage example.

package main

import (
	"fmt"
	"os"

	"github.com/rithulkamesh/licensify/sdk/go/licensify"
)

func main() {
	c, err := licensify.New(licensify.Config{
		ServerURL: "http://localhost:8080",
		CachePath: os.TempDir() + "/licensify.token",
	})
	if err != nil {
		panic(err)
	}
	defer c.Close()
	if err := c.Activate("LICENSE-KEY-DEV"); err != nil {
		panic(err)
	}
	st, _ := c.Check()
	fmt.Printf("status=%+v\n", st)
}
