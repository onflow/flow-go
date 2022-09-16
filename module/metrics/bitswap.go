package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BitswapCollector struct {
	peers            *prometheus.GaugeVec
	wantlist         *prometheus.GaugeVec
	blobsReceived    *prometheus.GaugeVec
	dataReceived     *prometheus.GaugeVec
	blobsSent        *prometheus.GaugeVec
	dataSent         *prometheus.GaugeVec
	dupBlobsReceived *prometheus.GaugeVec
	dupDataReceived  *prometheus.GaugeVec
	messagesReceived *prometheus.GaugeVec
}

func NewBitswapCollector() *BitswapCollector {
	bc := &BitswapCollector{
		peers: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "num_peers",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the number of connected peers",
		}, []string{"prefix"}),
		wantlist: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "wantlist_size",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the wantlist size",
		}, []string{"prefix"}),
		blobsReceived: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "blobs_received",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the number of received blobs",
		}, []string{"prefix"}),
		dataReceived: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "data_received",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the amount of data received",
		}, []string{"prefix"}),
		blobsSent: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "blobs_sent",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the number of sent blobs",
		}, []string{"prefix"}),
		dataSent: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "data_sent",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the amount of data sent",
		}, []string{"prefix"}),
		dupBlobsReceived: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "dup_blobs_received",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the number of duplicate blobs received",
		}, []string{"prefix"}),
		dupDataReceived: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "dup_data_received",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the amount of duplicate data received",
		}, []string{"prefix"}),
		messagesReceived: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "messages_received",
			Namespace: namespaceNetwork,
			Subsystem: subsystemBitswap,
			Help:      "the number of messages received",
		}, []string{"prefix"}),
	}

	return bc
}

func (bc *BitswapCollector) Peers(prefix string, n int) {
	bc.peers.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) Wantlist(prefix string, n int) {
	bc.wantlist.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) BlobsReceived(prefix string, n uint64) {
	bc.blobsReceived.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) DataReceived(prefix string, n uint64) {
	bc.dataReceived.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) BlobsSent(prefix string, n uint64) {
	bc.blobsSent.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) DataSent(prefix string, n uint64) {
	bc.dataSent.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) DupBlobsReceived(prefix string, n uint64) {
	bc.dupBlobsReceived.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) DupDataReceived(prefix string, n uint64) {
	bc.dupDataReceived.WithLabelValues(prefix).Set(float64(n))
}

func (bc *BitswapCollector) MessagesReceived(prefix string, n uint64) {
	bc.messagesReceived.WithLabelValues(prefix).Set(float64(n))
}
