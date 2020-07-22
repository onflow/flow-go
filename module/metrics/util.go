package metrics

import (
	"reflect"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
)

const TracerFieldName = "tracer"

func registerAllFields(collector interface{}, registerer prometheus.Registerer) {
	t := reflect.TypeOf(collector)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	v := reflect.ValueOf(collector)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		// Skip the tracer field, not a prometheus field
		field := t.Field(i)
		if field.Name == TracerFieldName {
			continue
		}

		fieldVal := v.FieldByName(field.Name)
		val := getUnexportedField(fieldVal)
		// All other fields must be of type prometheus.Collector
		registerer.MustRegister(val.(prometheus.Collector))
	}
}

func getUnexportedField(field reflect.Value) interface{} {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}
