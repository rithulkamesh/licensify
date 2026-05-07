// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: sdk/go/licensify/types.go — Go SDK types and status/error modeling.

package licensify

type Config struct {
	ServerURL string
	CachePath string
	Logger    Logger
}

type Status struct {
	Code int
}

type Logger interface {
	Debug(msg string, fields map[string]any)
	Info(msg string, fields map[string]any)
	Warn(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}

type ErrorCode int

const (
	OK                  ErrorCode = 0
	ErrInvalidArgument  ErrorCode = 1
	ErrInitialization   ErrorCode = 2
	ErrActivation       ErrorCode = 3
	ErrCheck            ErrorCode = 4
)

type InitializationError struct {
	Message string
}

func (e *InitializationError) Error() string { return e.Message }

type ActivationError struct {
	Code    ErrorCode
	Message string
}

func (e *ActivationError) Error() string { return e.Message }

type CheckError struct {
	Code    ErrorCode
	Message string
}

func (e *CheckError) Error() string { return e.Message }
