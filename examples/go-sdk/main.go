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
	url := os.Getenv("LICENSIFY_BASE_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	key := os.Getenv("LICENSIFY_LICENSE_KEY")
	if key == "" {
		key = "LICENSE-KEY-DEV"
	}
	c, err := licensify.New(licensify.Config{
		ServerURL: url,
		CachePath: os.TempDir() + "/licensify.token",
	})
	if err != nil {
		panic(err)
	}
	defer c.Close()
	if err := c.Activate(key); err != nil {
		panic(err)
	}
	st, _ := c.Check()
	fmt.Printf("status=%+v\n", st)
}
