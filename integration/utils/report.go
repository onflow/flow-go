package main

type PerformanceReport struct {
	NumCollNodes    int     // number of collection nodes
	NumConsNodes    int     // number of consensus nodes
	NumExecNodes    int     // number of execution nodes
	NumVerfNodes    int     // number of verification nodes
	NumCollClusters int     // number of collection clusters
	TotalTxSubmited int     // total transaction submited
	TxBatchSize     int     // transaction batch size
	TxFailureRate   float32 // number of failed transactions / total number of transactions
	AvgTTF          int     // Average transaction time to finality (in seconds)
	MedTTF          int     // Median transaction time to finality (in seconds)
	VarTTF          float32 // Variance of transaction time to finality (in seconds)
	AvgTTE          int     // Average transaction time to execution (in seconds)
	MedTTE          int     // Median transaction time to execution (in seconds)
	VarTTE          float32 // Variance of transaction time to execution (in seconds)
	AvgTTS          int     // Average transaction time to seal (in seconds)
	MedTTS          int     // Median transaction time to seal (in seconds)
	VarTTS          float32 // Variance of transaction time to seal (in seconds)
	MaxTPS          float32 // maximum number of transaction per second tested
}

func (pr PerformanceReport) String() {

}

func (pr PerformanceReport) ToFile(filePath string) {

}
