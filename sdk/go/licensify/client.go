// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/go/licensify/client.go — Go SDK client wrapper around the stable C ABI.

package licensify

/*
#cgo CFLAGS: -I../../../client/include
#include <stdlib.h>
#include "licensify.h"
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type Client struct {
	ptr *C.licensify_client_t
	log Logger
}

func New(cfg Config) (*Client, error) {
	if cfg.ServerURL == "" {
		return nil, &InitializationError{Message: "ServerURL is required"}
	}
	if cfg.CachePath == "" {
		return nil, &InitializationError{Message: "CachePath is required"}
	}
	url := C.CString(cfg.ServerURL)
	cache := C.CString(cfg.CachePath)
	defer C.free(unsafe.Pointer(url))
	defer C.free(unsafe.Pointer(cache))
	ccfg := C.licensify_config_t{server_url: url, cache_path: cache}
	ptr := C.licensify_new(&ccfg)
	if ptr == nil {
		return nil, &InitializationError{Message: "failed to create licensify client (native returned NULL)"}
	}
	return &Client{ptr: ptr, log: cfg.Logger}, nil
}

func (c *Client) Activate(key string) error {
	if c.ptr == nil {
		return &ActivationError{Code: ErrInitialization, Message: "client is closed"}
	}
	if key == "" {
		return &ActivationError{Code: ErrInvalidArgument, Message: "license key is required"}
	}
	start := time.Now()
	k := C.CString(key)
	defer C.free(unsafe.Pointer(k))
	code := C.licensify_activate_code(c.ptr, k)
	if code != C.LICENSIFY_OK {
		msg := c.lastError()
		if c.log != nil {
			c.log.Error("licensify.activate failed", map[string]any{"code": int(code), "message": msg})
		}
		return &ActivationError{Code: ErrorCode(code), Message: msg}
	}
	if c.log != nil {
		c.log.Info("licensify.activate ok", map[string]any{"duration_ms": time.Since(start).Milliseconds()})
	}
	return nil
}

func (c *Client) Check() (Status, error) {
	if c.ptr == nil {
		return Status{}, &CheckError{Code: ErrInitialization, Message: "client is closed"}
	}
	start := time.Now()
	var status C.int
	code := C.licensify_check_code(c.ptr, &status)
	if code != C.LICENSIFY_OK {
		msg := c.lastError()
		if c.log != nil {
			c.log.Error("licensify.check failed", map[string]any{"code": int(code), "message": msg})
		}
		return Status{}, &CheckError{Code: ErrorCode(code), Message: msg}
	}
	if c.log != nil {
		c.log.Debug("licensify.check ok", map[string]any{"duration_ms": time.Since(start).Milliseconds(), "status_code": int(status)})
	}
	return Status{Code: int(status)}, nil
}

func (c *Client) HasFeature(feature string) bool {
	if c.ptr == nil || feature == "" {
		return false
	}
	f := C.CString(feature)
	defer C.free(unsafe.Pointer(f))
	return bool(C.licensify_has_feature(c.ptr, f))
}

func (c *Client) Close() {
	if c.ptr != nil {
		C.licensify_free(c.ptr)
		c.ptr = nil
	}
}

func (c *Client) lastError() string {
	if c.ptr == nil {
		return "client is closed"
	}
	s := C.licensify_last_error(c.ptr)
	if s == nil {
		return fmt.Sprintf("native error (code only)")
	}
	return C.GoString(s)
}
