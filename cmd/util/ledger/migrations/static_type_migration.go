package migrations

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/tools/analysis"
)

type StaticTypeMigrationRules map[interpreter.StaticType]interpreter.StaticType

func NewStaticTypeMigrator[T interpreter.StaticType](
	rules StaticTypeMigrationRules,
) func(staticType T) interpreter.StaticType {

	// Returning `nil` form the callback indicates the type wasn't converted.

	if rules == nil {
		return func(original T) interpreter.StaticType {
			return nil
		}
	}

	return func(original T) interpreter.StaticType {
		if replacement, ok := rules[original]; ok {
			return replacement
		}
		return nil
	}
}

func ReadCSVStaticTypeMigrationRules(
	programs analysis.Programs,
	reader io.Reader,
) (
	rules StaticTypeMigrationRules,
	err error,
) {
	rules = make(StaticTypeMigrationRules)

	csvReader := csv.NewReader(reader)

	type ruleRecord struct {
		program          string
		sourceStaticType string
		targetStaticType string
	}

	var ruleRecords []ruleRecord

	for index := 0; ; index++ {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		const expectedRecordLen = 3
		actualRecordLen := len(record)
		if actualRecordLen != expectedRecordLen {
			return nil, fmt.Errorf(
				"error in rule %d: invalid CSV rule length. expected %d, got %d",
				index+1,
				expectedRecordLen,
				actualRecordLen,
			)
		}

		ruleRecords = append(ruleRecords, ruleRecord{
			program:          record[0],
			sourceStaticType: record[1],
			targetStaticType: record[2],
		})
	}

	getLocation := func(index int) common.Location {
		var location common.ScriptLocation
		binary.BigEndian.PutUint64(location[:], uint64(index))
		return location
	}

	for index, ruleRecord := range ruleRecords {
		location := getLocation(index)

		err := programs.Load(
			analysis.NewSimpleConfig(
				analysis.NeedTypes,
				map[common.Location][]byte{
					location: []byte(ruleRecord.program),
				},
				nil,
				nil,
			),
			location,
		)
		if err != nil {
			return nil, err
		}

		program := programs[location]

		checker := program.Checker

		sourceStaticType, err := ParseStaticType(
			ruleRecord.sourceStaticType,
			checker,
		)
		if err != nil {
			return nil, err
		}

		targetStaticType, err := ParseStaticType(
			ruleRecord.targetStaticType,
			checker,
		)
		if err != nil {
			return nil, err
		}

		rules[sourceStaticType] = targetStaticType
	}

	return
}

func ParseStaticType(ty string, checker *sema.Checker) (interpreter.StaticType, error) {
	code := []byte(ty)
	astType, errs := parser.ParseType(nil, code, parser.Config{})
	if len(errs) > 0 {
		return nil, parser.Error{
			Code:   code,
			Errors: errs,
		}
	}

	semaType := checker.ConvertType(astType)

	checkerErr := checker.CheckerError()
	if checkerErr != nil {
		return nil, checkerErr
	}

	return interpreter.ConvertSemaToStaticType(nil, semaType), nil
}
