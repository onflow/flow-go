/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package emulator

import (
	"fmt"
	"github.com/logrusorgru/aurora"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"
)

func PrintScriptResult(logger *zerolog.Logger, result *ScriptResult) {
	if result.Succeeded() {
		logger.Debug().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("⭐  Script executed")
	} else {
		logger.Warn().
			Str("scriptID", result.ScriptID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("❗  Script reverted")
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR", FlowIdentifierToSDK(result.ScriptID), aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func PrintTransactionResult(logger *zerolog.Logger, result *TransactionResult) {
	if result.Succeeded() {
		logger.Debug().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("⭐  Transaction executed")
	} else {
		logger.Warn().
			Str("txID", result.TransactionID.String()).
			Uint64("computationUsed", result.ComputationUsed).
			Uint64("memoryEstimate", result.MemoryEstimate).
			Msg("❗  Transaction reverted")
	}

	for _, event := range result.Events {
		logger.Debug().Msgf(
			"%s %s",
			logPrefix("EVT", result.TransactionID, aurora.GreenFg),
			event,
		)
	}

	if !result.Succeeded() {
		logger.Warn().Msgf(
			"%s %s",
			logPrefix("ERR", result.TransactionID, aurora.RedFg),
			result.Error.Error(),
		)

		if result.Debug != nil {
			logger.Debug().Fields(result.Debug.Meta).Msgf("%s %s", "❗  Transaction Signature Error", result.Debug.Message)
		}
	}
}

func logPrefix(prefix string, id sdk.Identifier, color aurora.Color) string {
	prefix = aurora.Colorize(prefix, color|aurora.BoldFm).String()
	shortID := fmt.Sprintf("[%s]", id.String()[:6])
	shortID = aurora.Colorize(shortID, aurora.FaintFm).String()
	return fmt.Sprintf("%s %s", prefix, shortID)
}
