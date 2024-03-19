package metrics

import "github.com/prometheus/client_golang/prometheus"

// MachineAccountCollector implements metric collection for machine accounts.
type MachineAccountCollector struct {
	accountBalance        prometheus.Gauge
	recommendedMinBalance prometheus.Gauge
	misconfigured         prometheus.Gauge
}

func NewMachineAccountCollector(registerer prometheus.Registerer) *MachineAccountCollector {
	accountBalance := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceMachineAcct,
		Name:      "balance",
		Help:      "the last observed balance of this node's machine account, in units of FLOW",
	})
	recommendedMinBalance := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceMachineAcct,
		Name:      "recommended_min_balance",
		Help:      "the recommended minimum balance for this node role; refill the account when the balance reaches this threshold",
	})
	misconfigured := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceMachineAcct,
		Name:      "is_misconfigured",
		Help:      "reported as a non-zero value when a misconfiguration is detected; check logs for further details",
	})
	registerer.MustRegister(accountBalance, recommendedMinBalance, misconfigured)

	collector := &MachineAccountCollector{
		accountBalance:        accountBalance,
		recommendedMinBalance: recommendedMinBalance,
		misconfigured:         misconfigured,
	}
	return collector
}

func (m MachineAccountCollector) AccountBalance(bal float64) {
	m.accountBalance.Set(bal)
}

func (m MachineAccountCollector) RecommendedMinBalance(bal float64) {
	m.recommendedMinBalance.Set(bal)
}

func (m MachineAccountCollector) IsMisconfigured(misconfigured bool) {
	if misconfigured {
		m.misconfigured.Set(1)
	} else {
		m.misconfigured.Set(0)
	}
}
