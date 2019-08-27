# General Feedback and Questions

- For all the set-up instructions it would be great to have this aggregated into one source of truth with very explicit (Explain it to me like I'm 5 instructions). Eg. For the instructions for the VSCode extension, state clearly that you need to clone this repo, cd into this folder, etc.
- It would be good to have a format for the examples something to the effect of:
    - Here is how to download all the tools necessary
    - Here's some example code
    - Here's how you can run it
    - Here's some code you can add to this.
- If the aim of Flow is to be accessible (ie. As a developer who knows nothing about Flow, I should be able to pick up Flow quickly and be able to write a Blockchain with it). I think it would be really important to have a very thorough walk through of how to create a blockchain. This should be really ELI5 and a step by step walk through, not just explaining how to do things but also explaining Blockchain concepts.
- The language docs are very good for programming language concepts, however to appeal to the masses, I think it would be really useful to have a very detailed walk through on how to go from absolute zero, to making something, and explaining the concepts along the way. For example, a Javascript developer isn't necessarily going to be familiar with the concept of Interfaces at all. The docs are very good at explaining what an interface is and how to use it in *theory*, which will totally work for some people, but to really appeal to those who learn better by creating something and experimenting with it, it would be great to have this explained by building on a project step by step. Sure, I can go to the docs about interfaces, but then I see code using the term 'resources' and my brain is like 'What? What's a resource??'.
- 

Questions:

- What exactly is a Vault? Is it a type, or just a naming convention?
- What is the benefit to using a Struct over a Resource or a Resource over a Struct?
- Struct syntax - are we returning a Vault?
- What is mean by 'Deployment' in this context. ie. What does it mean to deploy a resource?


Below is a rough idea of how I think you might be able to structure a tutorial around Flow in order to build on each concept and finish with something that a user could write and run themselves and in doing so, learn Blockchain concepts while also learning Flow.

I've borrowed and adapted explanations for these concepts from around the web and combining them with some examples of code from the Flow Language Documentation. I've approached this as someone learning Blockchain for the first time.

---


# Instructions
These instructions are intended to walk you through how to set up an example sandbox to test out and play with the BPL.

[Specific language documentation can be found here](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/language/docs#bamboo-programming-language-documentation)

## Installation

### Install Go
- Download and install [Go 1.12](https://golang.org/doc/install)
- Create your workspace `$GOPATH` directory and update your bash_profile to contain the following:

```bash
export `$GOPATH=$HOME/path-to-your-go-workspace/`
```

It's also a good idea to update your `$PATH` to use third party GO binaries: 

```bash
export PATH="$PATH:$GOPATH/bin"
```

- Test that Go was installed correctly: https://golang.org/doc/install#testing


### Set up the Repo

**Clone the bamboo-node repo**

```bash
git clone git@github.com:dapperlabs/bamboo-node.git
```

**Install the CLI tool**

```bash
cd bamboo-node

rm -r pkg/crypto/relic

git submodule update --init --recursive

brew install cmake

make install-cli

# Test the command
bamboo --help
```

**Install the VSCode Extension**
This will provide syntax highlighting for BPL.

You may need to have the `code` command enabled in VSCode. To do this:

```
Open VSCode

Cmd+Shift+P

Search for 'code' and select: `Shell Command: Install 'code' command in PATH`
```

You should get a little pop up telling you that this has been enabled.

Next, build the language server. This will build the language server and place it in $GOPATH/bin/language-server.

```bash
cd bamboo-node/pkg/language/tools/vscode-extension

GO111MODULE=on go install

npm install

npm install -g vsce

vsce package

code --install-extension bamboo-*.vsix
```

```
Open Visual Studio Code

Go to Preferences → Settings

Search for "Bamboo"
In "Bamboo: Language Server Path", enter the binary path for the language server. If $GOPATH/bin is in your path, you can use language-server, if not use the full path to the binary instead ($GOPATH/bin/language-server).

Navigate to the examples folder and create a new file called `hello-world.bpl`
```

The language mode should be set to Bamboo automatically

A popup should appear "Bamboo language server started"

You can restart this server by running this command in VSCode

```
Cmd+Shift+P
Search for Bamboo
'Bamboo: Restart Language Server`
```

You should get two popups, one saying the server has been stopped, and one saying it has been restarted.


## Write Some Code

We'll start with a Hello World program, just like any other programming language.

Open up your hello-world.bpl and write a Hello World function.

In BPL a function is defined with the keyword `fun`, followed by the name of the declaration, the parameters, the optional return type and then the code that should be executed when the function is called.

Here we have a function called `main` which, when executed succesfully, should log 'HELLO WORLD' to the console.

```bpl
fun main() {
    log("HELLO WORLD")
}
```

## Make it Run

In your console, navigate to the runtime folder and run the following commands:


```
cd pkg/language/runtime
```

And then build and run the executable:

```
go build cmd/main.go
./main examples/hello-world.bpl
```


## Make some transactions

Let's implement a Vault that has a withdraw and deposit method in order to make transactions.
We're going to define an Interface, `Vault` that has a balance with the type `Int`. 

**Define Interface**

```bpl
interface Vault  {
    balance: Int
}
```

**Add balace initializer**

Now we'll add an `init` to Vault to check that the balance is initialized to the initial balance

```bpl
    init(balance: Int) {
        post {
            self.balance == balance:
                "the balance must be initialized to the initial balance"
        }
    }
```

Now we'll add a public function (expressed by `pub`) to be able to withdraw an amount.
The function withdraw takes in an amount and returns a vault.

```bpl
    pub fun withdraw(amount: Int): Vault {

    }
```
Inside the function, we are going to declare Preconditions and Postconditions.

Preconditions have to be true right before the execution of a function.
Here we are making sure that the function 'withdraw' will not run if the withdrawl amount is negative, or if the amount is greater than the amount in the Vault.

Postconditions are part of the function itself and can only occur after a precondition.
Here we are checking if the correct amount has been withdrawn or returned.

**Add Precondition and Postcondition**

```bpl
    pub fun withdraw(amount: Int): Vault {
        pre {
            amount > 0:
                "withdrawal amount must be positive"
            amount <= self.balance:
                "insufficient funds: the amount must be smaller or equal to the balance"
        }
        post {
            self.balance == before(self.balance) - amount:
                "Incorrect amount removed"
            result.balance == amount: "incorrect amount returned"
        }
    }
```

Next we want to add a deposit method. This doesn't need a precondition because there isn't a limitation on depositing. You could deposit any amount.

**Add Deposit Method**

```bpl
    pub fun deposit(vault: Vault) {
        post {
            self.balance == before(self.balance) + vault.balance:
                "the amount must be added to the balance"
        }
    }
```

Now let's instantiate a struct `ExampleVault` 
// Needs a bit more instruciton here on what exactly we're doing with the struct

```bpl
struct ExampleVault: Vault {
    var balance: Int

    init(balance: Int) {
        self.balance = balance
    }

    pub fun withdraw(amount: Int): Vault {
        self.balance = self.balance - amount
        return ExampleVault(balance: amount)
    }

    pub fun deposit(vault: Vault) {
         self.balance = self.balance + vault.balance
    }
}
```

Now we can set up a `main` function to run some withdrawls/deposits

**Withdraw and Deposit**

We'll declare vaultA and vaultB with a set balance each.

Here's a good time to point out the difference between variables and constants in Flow

`let` declares a _constant_ which means that the **name** is constant, not necessarily the value.

`var` declares a _variable_ which can be assigned a new value.

This might seem counter-intuitive for someone who has come frmo a Javascript background, but is the way variables and constants are typically declared in Maching Learning.

For example:
```bpl
// Declare a constant named `a`
//
let a = 1
// Invalid: re-assigning to a constant //
a=2
// Declare a variable named `b`
//
var b = 3
// Assign a new value to the variable named `b` //
b=4
```

```bpl
fun main() {
    let vaultA = ExampleVault(balance: 10)
    let vaultB = ExampleVault(balance: 0)
}
```

Then we'll withdraw 7 from vaultA and assign it to vaultC, and deposit that amount to vaultB

```bpl
let vaultC = vaultA.withdraw(amount: 7)
vaultB.deposit(vault: vaultC)
```

And lastly lets log the balance of each vault:

```bpl
log(vaultA.balance)
log(vaultB.balance)
log(vaultC.balance)
```

**Run the program**

To build and run this program we use the same command as before but with the vault.bpl filename:

```bash
go build cmd/main.go
./main examples/vault.bpl
```

Congratulations! You've written your first Flow programs. Happy Coding!

---


## Accounts

Accounts are a way to store funds. They are essentially public/private key pairs which you use to sign transactions.

In Flow an account would look like this:

```bpl
struct interface Account {
    pub init(at address: Address)
}
```

**Public/Private Keys**
Private and Public keys encode informaiton. 

Public keys convert messages into a format that can't be read. If you only had a public key you would not be able to decode a message.

Private keys are secret keys that are used to decrypt the messages.

[Here](https://www.freecodecamp.org/news/https-explained-with-carrier-pigeons-7029d2193351/) is a really useful analogy that explains private and public keys in the context of HTTPS and carrier pigeons.

**Transactions**

Transactions are objects that get signed by an account(or accounts) and get sent to the chain to interact with it. 


## Create an Account
// TODO: Code example that would create an account


## Create a Transaction
// TODO: Code example that would create a transaction for that account