/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
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

package examples

const FungibleTokenContractInterface = `
  pub contract interface FungibleToken {

      pub resource interface Provider {

          pub fun withdraw(amount: Int): @Vault
      }

      pub resource interface Receiver {

          pub fun deposit(vault: @Vault)
      }

      pub resource Vault: Provider, Receiver {

          pub balance: Int

          init(balance: Int)
      }

      pub fun absorb(vault: @Vault)

      pub fun sprout(balance: Int): @Vault
  }
`

const ExampleFungibleTokenContract = `
  pub contract ExampleToken: FungibleToken {

     pub resource Vault: FungibleToken.Receiver, FungibleToken.Provider {

         pub var balance: Int

         init(balance: Int) {
             self.balance = balance
         }

         pub fun withdraw(amount: Int): @FungibleToken.Vault {
             self.balance = self.balance - amount
             return <-create Vault(balance: amount)
         }

         pub fun deposit(vault: @FungibleToken.Vault) {
            if let exampleVault <- vault as? @Vault {
                self.balance = self.balance + exampleVault.balance
                destroy exampleVault
            } else {
               destroy vault
               panic("deposited vault is not an ExampleToken.Vault")
            }
         }
     }

     pub fun absorb(vault: @FungibleToken.Vault) {
         destroy vault
     }

     pub fun sprout(balance: Int): @FungibleToken.Vault {
         return <-create Vault(balance: balance)
     }
  }
`
