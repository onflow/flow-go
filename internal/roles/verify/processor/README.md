

# processor
`import "github.com/dapperlabs/bamboo-node/internal/roles/verify/processor"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Subdirectories](#pkg-subdirectories)

## <a name="pkg-overview">Overview</a>
Package processor is in charge of the ExecutionReceipt processing flow.
It decides whether a receipt gets discarded/slashed/approved/cached, while relying on external side effects functions to trigger these actions (template pattern).
The package holds a queue of receipts and processes them in FIFO to utilise caching result and not re-validate a validated receipt submitted by another node.
Note that some concurrency optimisation is possible by having a queue-per-block-height without sacrificing any caching potential.




## <a name="pkg-index">Index</a>
* [type Effects](#Effects)
  * [func NewEffectsProvider() Effects](#NewEffectsProvider)
* [type EffectsProvider](#EffectsProvider)
  * [func (e *EffectsProvider) HandleError(err error)](#EffectsProvider.HandleError)
  * [func (e *EffectsProvider) HasMinStake(*ExecutionReceipt) (bool, error)](#EffectsProvider.HasMinStake)
  * [func (e *EffectsProvider) IsSealedWithDifferentReceipt(*ExecutionReceipt) (bool, error)](#EffectsProvider.IsSealedWithDifferentReceipt)
  * [func (e *EffectsProvider) IsValidExecutionReceipt(*ExecutionReceipt) (compute.ValidationResult, error)](#EffectsProvider.IsValidExecutionReceipt)
  * [func (e *EffectsProvider) Send(*ExecutionReceipt, []byte) error](#EffectsProvider.Send)
  * [func (e *EffectsProvider) SlashExpiredReceipt(*ExecutionReceipt) error](#EffectsProvider.SlashExpiredReceipt)
  * [func (e *EffectsProvider) SlashInvalidReceipt(*ExecutionReceipt, *compute.BlockPartExecutionResult) error](#EffectsProvider.SlashInvalidReceipt)
* [type ExecutionReceipt](#ExecutionReceipt)
  * [func (m *ExecutionReceipt) Encode() []byte](#ExecutionReceipt.Encode)
* [type ReceiptProcessor](#ReceiptProcessor)
  * [func NewReceiptProcessor(effects Effects, rc *ReceiptProcessorConfig, hasher crypto.Hasher) *ReceiptProcessor](#NewReceiptProcessor)
  * [func (p *ReceiptProcessor) Submit(receipt *ExecutionReceipt, done chan bool)](#ReceiptProcessor.Submit)
* [type ReceiptProcessorConfig](#ReceiptProcessorConfig)


#### <a name="pkg-files">Package files</a>
[effects_interface.go](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_interface.go) [effects_provider_wip.go](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go) [processor.go](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go)






## <a name="Effects">type</a> [Effects](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_interface.go?s=245:644#L8)
``` go
type Effects interface {
    IsValidExecutionReceipt(*ExecutionReceipt) (compute.ValidationResult, error)
    HasMinStake(*ExecutionReceipt) (bool, error)
    IsSealedWithDifferentReceipt(*ExecutionReceipt) (bool, error)
    Send(*ExecutionReceipt, []byte) error
    SlashExpiredReceipt(*ExecutionReceipt) error
    SlashInvalidReceipt(*ExecutionReceipt, *compute.BlockPartExecutionResult) error
    HandleError(error)
}
```
Effects is an interface for external encapuslated funcs with side-effects to be used in the receipt processor. It follows the template pattern.







### <a name="NewEffectsProvider">func</a> [NewEffectsProvider](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=343:376#L14)
``` go
func NewEffectsProvider() Effects
```




## <a name="EffectsProvider">type</a> [EffectsProvider](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=310:341#L11)
``` go
type EffectsProvider struct {
}

```
EffectsProvider implements the Effects interface.
Note: this is still a WIP and blocked on progress of features outside of the verifier role (gossip layer, stakes, etc').










### <a name="EffectsProvider.HandleError">func</a> (\*EffectsProvider) [HandleError](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=1063:1111#L42)
``` go
func (e *EffectsProvider) HandleError(err error)
```



### <a name="EffectsProvider.HasMinStake">func</a> (\*EffectsProvider) [HasMinStake](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=565:635#L22)
``` go
func (e *EffectsProvider) HasMinStake(*ExecutionReceipt) (bool, error)
```



### <a name="EffectsProvider.IsSealedWithDifferentReceipt">func</a> (\*EffectsProvider) [IsSealedWithDifferentReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=659:746#L26)
``` go
func (e *EffectsProvider) IsSealedWithDifferentReceipt(*ExecutionReceipt) (bool, error)
```



### <a name="EffectsProvider.IsValidExecutionReceipt">func</a> (\*EffectsProvider) [IsValidExecutionReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=409:511#L18)
``` go
func (e *EffectsProvider) IsValidExecutionReceipt(*ExecutionReceipt) (compute.ValidationResult, error)
```



### <a name="EffectsProvider.Send">func</a> (\*EffectsProvider) [Send](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=771:834#L30)
``` go
func (e *EffectsProvider) Send(*ExecutionReceipt, []byte) error
```



### <a name="EffectsProvider.SlashExpiredReceipt">func</a> (\*EffectsProvider) [SlashExpiredReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=852:922#L34)
``` go
func (e *EffectsProvider) SlashExpiredReceipt(*ExecutionReceipt) error
```



### <a name="EffectsProvider.SlashInvalidReceipt">func</a> (\*EffectsProvider) [SlashInvalidReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=940:1045#L38)
``` go
func (e *EffectsProvider) SlashInvalidReceipt(*ExecutionReceipt, *compute.BlockPartExecutionResult) error
```



## <a name="ExecutionReceipt">type</a> [ExecutionReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=743:1003#L18)
``` go
type ExecutionReceipt struct {
    PreviousReceiptHash       crypto.Hash
    BlockHash                 crypto.Hash
    Signatures                []crypto.Signature
    InitialRegisters          compute.Registers
    IntermediateRegistersList []compute.IntermediateRegisters
}

```









### <a name="ExecutionReceipt.Encode">func</a> (\*ExecutionReceipt) [Encode](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=1056:1098#L27)
``` go
func (m *ExecutionReceipt) Encode() []byte
```
Encode implements the crypto.Encoder interface.




## <a name="ReceiptProcessor">type</a> [ReceiptProcessor](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=1220:1348#L36)
``` go
type ReceiptProcessor struct {
    // contains filtered or unexported fields
}

```






### <a name="NewReceiptProcessor">func</a> [NewReceiptProcessor](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=1554:1663#L50)
``` go
func NewReceiptProcessor(effects Effects, rc *ReceiptProcessorConfig, hasher crypto.Hasher) *ReceiptProcessor
```
NewReceiptProcessor returns a new processor instance.
A go routine is initialised and waiting to process new items.





### <a name="ReceiptProcessor.Submit">func</a> (\*ReceiptProcessor) [Submit](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=2073:2149#L64)
``` go
func (p *ReceiptProcessor) Submit(receipt *ExecutionReceipt, done chan bool)
```
Submit takes in an ExecutionReceipt to be process async.
The done chan is optional. If caller is not interested to be notified when processing has been completed, nil value should be used for it.




## <a name="ReceiptProcessorConfig">type</a> [ReceiptProcessorConfig](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=4388:4460#L142)
``` go
type ReceiptProcessorConfig struct {
    QueueBuffer int
    CacheBuffer int
}

```
ReceiptProcessorConfig holds the configuration for receipt processor.














- - -
Generated by [godoc2md](http://godoc.org/github.com/lanre-ade/godoc2md)
