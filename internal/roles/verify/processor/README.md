

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
  * [func (e *EffectsProvider) HasMinStake(*types.ExecutionReceipt) (bool, error)](#EffectsProvider.HasMinStake)
  * [func (e *EffectsProvider) IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error)](#EffectsProvider.IsSealedWithDifferentReceipt)
  * [func (e *EffectsProvider) IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error)](#EffectsProvider.IsValidExecutionReceipt)
  * [func (e *EffectsProvider) Send(*types.ExecutionReceipt, []byte) error](#EffectsProvider.Send)
  * [func (e *EffectsProvider) SlashExpiredReceipt(*types.ExecutionReceipt) error](#EffectsProvider.SlashExpiredReceipt)
  * [func (e *EffectsProvider) SlashInvalidReceipt(*types.ExecutionReceipt, *compute.BlockPartExecutionResult) error](#EffectsProvider.SlashInvalidReceipt)
* [type ReceiptProcessor](#ReceiptProcessor)
  * [func NewReceiptProcessor(effects Effects, rc *ReceiptProcessorConfig, hasher crypto.Hasher) *ReceiptProcessor](#NewReceiptProcessor)
  * [func (p *ReceiptProcessor) Submit(receipt *types.ExecutionReceipt, done chan bool)](#ReceiptProcessor.Submit)
* [type ReceiptProcessorConfig](#ReceiptProcessorConfig)


#### <a name="pkg-files">Package files</a>
[effects_interface.go](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_interface.go) [effects_provider_wip.go](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go) [processor.go](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go)






## <a name="Effects">type</a> [Effects](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_interface.go?s=292:727#L9)
``` go
type Effects interface {
    IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error)
    HasMinStake(*types.ExecutionReceipt) (bool, error)
    IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error)
    Send(*types.ExecutionReceipt, []byte) error
    SlashExpiredReceipt(*types.ExecutionReceipt) error
    SlashInvalidReceipt(*types.ExecutionReceipt, *compute.BlockPartExecutionResult) error
    HandleError(error)
}
```
Effects is an interface for external encapuslated funcs with side-effects to be used in the receipt processor. It follows the template pattern.







### <a name="NewEffectsProvider">func</a> [NewEffectsProvider](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=390:423#L15)
``` go
func NewEffectsProvider() Effects
```




## <a name="EffectsProvider">type</a> [EffectsProvider](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=357:388#L12)
``` go
type EffectsProvider struct {
}

```
EffectsProvider implements the Effects interface.
Note: this is still a WIP and blocked on progress of features outside of the verifier role (gossip layer, stakes, etc').










### <a name="EffectsProvider.HandleError">func</a> (\*EffectsProvider) [HandleError](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=1146:1194#L43)
``` go
func (e *EffectsProvider) HandleError(err error)
```



### <a name="EffectsProvider.HasMinStake">func</a> (\*EffectsProvider) [HasMinStake](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=618:694#L23)
``` go
func (e *EffectsProvider) HasMinStake(*types.ExecutionReceipt) (bool, error)
```



### <a name="EffectsProvider.IsSealedWithDifferentReceipt">func</a> (\*EffectsProvider) [IsSealedWithDifferentReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=718:811#L27)
``` go
func (e *EffectsProvider) IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error)
```



### <a name="EffectsProvider.IsValidExecutionReceipt">func</a> (\*EffectsProvider) [IsValidExecutionReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=456:564#L19)
``` go
func (e *EffectsProvider) IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error)
```



### <a name="EffectsProvider.Send">func</a> (\*EffectsProvider) [Send](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=836:905#L31)
``` go
func (e *EffectsProvider) Send(*types.ExecutionReceipt, []byte) error
```



### <a name="EffectsProvider.SlashExpiredReceipt">func</a> (\*EffectsProvider) [SlashExpiredReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=923:999#L35)
``` go
func (e *EffectsProvider) SlashExpiredReceipt(*types.ExecutionReceipt) error
```



### <a name="EffectsProvider.SlashInvalidReceipt">func</a> (\*EffectsProvider) [SlashInvalidReceipt](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/effects_provider_wip.go?s=1017:1128#L39)
``` go
func (e *EffectsProvider) SlashInvalidReceipt(*types.ExecutionReceipt, *compute.BlockPartExecutionResult) error
```



## <a name="ReceiptProcessor">type</a> [ReceiptProcessor](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=765:893#L17)
``` go
type ReceiptProcessor struct {
    // contains filtered or unexported fields
}

```






### <a name="NewReceiptProcessor">func</a> [NewReceiptProcessor](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=1105:1214#L31)
``` go
func NewReceiptProcessor(effects Effects, rc *ReceiptProcessorConfig, hasher crypto.Hasher) *ReceiptProcessor
```
NewReceiptProcessor returns a new processor instance.
A go routine is initialised and waiting to process new items.





### <a name="ReceiptProcessor.Submit">func</a> (\*ReceiptProcessor) [Submit](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=1624:1706#L45)
``` go
func (p *ReceiptProcessor) Submit(receipt *types.ExecutionReceipt, done chan bool)
```
Submit takes in an ExecutionReceipt to be process async.
The done chan is optional. If caller is not interested to be notified when processing has been completed, nil value should be used for it.




## <a name="ReceiptProcessorConfig">type</a> [ReceiptProcessorConfig](https://github.com/dapperlabs/bamboo-node/tree/master/internal/roles/verify/processor/processor.go?s=3951:4023#L123)
``` go
type ReceiptProcessorConfig struct {
    QueueBuffer int
    CacheBuffer int
}

```
ReceiptProcessorConfig holds the configuration for receipt processor.














- - -
Generated by [godoc2md](http://godoc.org/github.com/lanre-ade/godoc2md)
