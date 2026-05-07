// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Rithul Kamesh
// Author: Rithul Kamesh <hi@rithul.dev>
// Description: server/internal/hardware/match.go — Hardware component matching utilities for machine binding.

package hardware

func FuzzyMatch(prev map[string]string, next map[string]string, threshold int) bool {
	if threshold <= 0 {
		threshold = 4
	}
	score := 0
	for k, v := range prev {
		if next[k] == v && v != "" {
			score++
		}
	}
	return score >= threshold
}
