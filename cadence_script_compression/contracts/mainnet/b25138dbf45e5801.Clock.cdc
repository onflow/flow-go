
/*
A contract to mock time. 

If you want to mock time create an function in your admin that enables the  Clock
The clock will then start at block 0
use the tick method to tick the time. 

```
		//this is used to mock the clock, NB! Should consider removing this before deploying to mainnet?
		pub fun tickClock(_ time: UFix64) {
			pre {
				self.capability != nil: "Cannot use AdminProxy, ste capability first"
			}
			Clock.enable()
			Clock.tick(time)
		}
```

You can then call this from a transaction like this:

```
import YourThing from "../contracts/YouThing.cdc"

transaction(clock: UFix64) {
	prepare(account: AuthAccount) {

		let adminClient=account.borrow<&YourThing.AdminProxy>(from: YourThing.AdminProxyStoragePath)!
		adminClient.tickClock(clock)

	}
}
```

In order to read the mocked time you use the following code in cadence

```
Clock.time()
```

Limitations: 
 - all contracts must live in the same account to (ab)use this trick

*/
pub contract Clock{
	//want to mock time on emulator. 
	access(contract) var fakeClock:UFix64
	access(contract) var enabled:Bool


	access(account) fun tick(_ duration: UFix64) {
		self.fakeClock = self.fakeClock + duration
	}


	access(account) fun enable() {
		self.enabled=true
	}

	//mocking the time! Should probably remove self.fakeClock in mainnet?
	pub fun time() : UFix64 {
		if self.enabled {
			return self.fakeClock 
		}
		return getCurrentBlock().timestamp
	}

	init() {
		self.fakeClock=0.0
		self.enabled=false
	}

}

