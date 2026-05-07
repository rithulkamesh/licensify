// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/hardware/match_test.go — Tests for hardware fuzzy-match scoring.

package hardware

import "testing"

func TestFuzzyMatchDefaultThreshold(t *testing.T) {
	prev := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}
	next := prev
	if !FuzzyMatch(prev, next, 0) {
		t.Fatal("expected match with default threshold 4")
	}
	if !FuzzyMatch(prev, next, -1) {
		t.Fatal("negative threshold should default to 4 and match")
	}
}

func TestFuzzyMatchPartial(t *testing.T) {
	prev := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}
	next := map[string]string{"a": "1", "b": "2", "c": "x", "d": "y"}
	if FuzzyMatch(prev, next, 4) {
		t.Fatal("expected no match")
	}
	if !FuzzyMatch(prev, next, 2) {
		t.Fatal("expected match with threshold 2")
	}
}

func TestFuzzyMatchEmptyValuesIgnored(t *testing.T) {
	prev := map[string]string{"a": "", "b": ""}
	next := map[string]string{"a": "", "b": ""}
	if FuzzyMatch(prev, next, 1) {
		t.Fatal("empty values should not score")
	}
}
