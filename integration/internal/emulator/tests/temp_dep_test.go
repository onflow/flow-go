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
package tests

import "github.com/btcsuite/btcd/chaincfg/chainhash"

// this is added to resolve the issue with chainhash ambiguous import,
// the code is not used, but it's needed to force go.mod specify and retain chainhash version
// workaround for issue: https://github.com/golang/go/issues/27899
var _ = chainhash.Hash{}
