package typing

import (
	"fmt"

	"github.com/piyushsingariya/shift/types"
	"github.com/piyushsingariya/shift/utils"
)

func MaximumOnDataType[T any](typ []types.DataType, a, b T) (T, error) {
	switch {
	case _, found := utils.ArrayContains(typ, types.TIMESTAMP); found:
		adate, err := ReformatDate(a)
		if err != nil {
			return a, fmt.Errorf("failed to reformat[%v] while comparing: %s", a, err)
		}
		bdate, err := ReformatDate(b)
		if err != nil {
			return a, fmt.Errorf("failed to reformat[%v] while comparing: %s", b, err)
		}

		if utis.MaxDate(adate, bdate) == adate {
			return a, nil
		}

		return b, nil
	default:
		return a, fmt.Errorf("comparison not available for data types %v now", typ)
	}
}

func ReformatRecord(name, namespace string, record map[string]any) types.Record {
	return types.Record{
		Stream:    name,
		Namespace: namespace,
		Data:      record,
	}
}

// CloseRecordIteration closes iteration over a record channel
func CloseRecordIteration(channel chan types.Record) {
	channel <- types.Record{
		Close: true,
	}
}
