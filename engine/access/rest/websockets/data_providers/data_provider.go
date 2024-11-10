package data_providers

type DataProvider interface {
	BaseDataProvider

	Run() error
}
