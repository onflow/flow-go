# order
`import "github.com/dapperlabs/flow-go/network/gossip/order"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Order implements a struct that tracks the progress of gossip messages and responses as well as errors. In gossip, orders are created to encapsulate gossip messages once received, after which they are added to the node's queue for handling. 


## <a name="pkg-index">Index</a>
* [func Valid(o *Order) bool](#Valid)
* [type Order](#Order)
  * [func NewAsync(ctx context.Context, msg *messages.GossipMessage) *Order](#NewAsync)
  * [func NewOrder(ctx context.Context, msg *messages.GossipMessage, isSync bool) *Order](#NewOrder)
  * [func NewSync(ctx context.Context, msg *messages.GossipMessage) *Order](#NewSync)
  * [func (o *Order) Done() &lt;-chan struct{}](#Order.Done)
  * [func (o *Order) Fill(resp []byte, err error)](#Order.Fill)
  * [func (o *Order) Result() ([]byte, error)](#Order.Result)
  * [func (o *Order) String() string](#Order.String)


#### <a name="pkg-files">Package files</a>
[order.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go)





## <a name="Valid">func</a> [Valid](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=2149:2174#L85)
``` go
func Valid(o *Order) bool
```
Valid checks if the order is valid. Currently valid is defined as having a non-nil message




## <a name="Order">type</a> [Order](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=276:405#L12)
``` go
type Order struct {
    Msg *messages.GossipMessage
    Ctx context.Context
    // contains filtered or unexported fields
}

```
Order is the evolved version of the deprecated tracker type of Gossip package.
order offers a struct to internally track running gossip messages and their return values







### <a name="NewAsync">func</a> [NewAsync](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=595:665#L27)
``` go
func NewAsync(ctx context.Context, msg *messages.GossipMessage) *Order
```
NewAsync constructs a new async order


### <a name="NewOrder">func</a> [NewOrder](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=747:830#L32)
``` go
func NewOrder(ctx context.Context, msg *messages.GossipMessage, isSync bool) *Order
```
NewOrder returns a new order instance.


### <a name="NewSync">func</a> [NewSync](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=446:515#L22)
``` go
func NewSync(ctx context.Context, msg *messages.GossipMessage) *Order
```
NewSync constructs a new sync Order





### <a name="Order.Done">func</a> (\*Order) [Done](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=1356:1394#L50)
``` go
func (o *Order) Done() <-chan struct{}
```
Done returns a channel. The channel is used to track the return values of a gossip message.




### <a name="Order.Fill">func</a> (\*Order) [Fill](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=1693:1737#L65)
``` go
func (o *Order) Fill(resp []byte, err error)
```
Fill fills the Order's response of the Order's gossip  msg




### <a name="Order.Result">func</a> (\*Order) [Result](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=1563:1603#L60)
``` go
func (o *Order) Result() ([]byte, error)
```
Result returns the current variables which represent the response of the
Order's gossip msg




### <a name="Order.String">func</a> (\*Order) [String](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/order/order.go?s=1944:1975#L80)
``` go
func (o *Order) String() string
```






