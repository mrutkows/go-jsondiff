package formatter

import (
	"encoding/json"
	"fmt"

	diff "github.com/mrutkows/go-jsondiff"
)

const (
	DeltaDelete   = 0
	DeltaTextDiff = 2
	DeltaMove     = 3
)

func NewDeltaFormatter() *DeltaFormatter {
	return &DeltaFormatter{
		PrintIndent: true,
	}
}

type DeltaFormatter struct {
	PrintIndent bool
}

func (f *DeltaFormatter) Format(diff diff.Diff) (result string, err error) {
	jsonObject, err := f.formatObject(diff.Deltas())
	if err != nil {
		return "", err
	}
	var resultBytes []byte
	if f.PrintIndent {
		resultBytes, err = json.MarshalIndent(jsonObject, "", "  ")
	} else {
		resultBytes, err = json.Marshal(jsonObject)
	}
	if err != nil {
		return "", err
	}

	return string(resultBytes) + "\n", nil
}

func (f *DeltaFormatter) FormatAsJson(diff diff.Diff) (json map[string]interface{}, err error) {
	return f.formatObject(diff.Deltas())
}

func (f *DeltaFormatter) formatObject(deltas []diff.Delta) (deltaJson map[string]interface{}, err error) {
	deltaJson = map[string]interface{}{}
	for _, delta := range deltas {
		switch deltaType := delta.(type) {
		case *diff.Object:
			//d := delta.(*diff.Object)
			deltaJson[deltaType.Position.String()], err = f.formatObject(deltaType.Deltas)
			if err != nil {
				return nil, err
			}
		case *diff.Array:
			//d := delta.(*diff.Array)
			deltaJson[deltaType.Position.String()], err = f.formatArray(deltaType.Deltas)
			if err != nil {
				return nil, err
			}
		case *diff.Added:
			//d := delta.(*diff.Added)
			deltaJson[deltaType.PostPosition().String()] = []interface{}{deltaType.Value}
		case *diff.Modified:
			//d := delta.(*diff.Modified)
			deltaJson[deltaType.PostPosition().String()] = []interface{}{deltaType.OldValue, deltaType.NewValue}
		case *diff.TextDiff:
			//d := delta.(*diff.TextDiff)
			deltaJson[deltaType.PostPosition().String()] = []interface{}{deltaType.DiffString(), 0, DeltaTextDiff}
		case *diff.Deleted:
			//d := delta.(*diff.Deleted)
			deltaJson[deltaType.PrePosition().String()] = []interface{}{deltaType.Value, 0, DeltaDelete}
		case *diff.Moved:
			return nil, fmt.Errorf("delta type '%T' is not supported in objects", deltaType)
		default:
			return nil, fmt.Errorf("unknown Delta type detected: %T", deltaType)
		}
	}
	return
}

func (f *DeltaFormatter) formatArray(deltas []diff.Delta) (deltaJson map[string]interface{}, err error) {
	deltaJson = map[string]interface{}{
		"_t": "a",
	}
	for _, delta := range deltas {
		switch deltaType := delta.(type) {
		case *diff.Object:
			//d := delta.(*diff.Object)
			deltaJson[deltaType.Position.String()], err = f.formatObject(deltaType.Deltas)
			if err != nil {
				return nil, err
			}
		case *diff.Array:
			//d := delta.(*diff.Array)
			deltaJson[deltaType.Position.String()], err = f.formatArray(deltaType.Deltas)
			if err != nil {
				return nil, err
			}
		case *diff.Added:
			//d := delta.(*diff.Added)
			deltaJson[deltaType.PostPosition().String()] = []interface{}{deltaType.Value}
		case *diff.Modified:
			//d := delta.(*diff.Modified)
			deltaJson[deltaType.PostPosition().String()] = []interface{}{deltaType.OldValue, deltaType.NewValue}
		case *diff.TextDiff:
			//d := delta.(*diff.TextDiff)
			deltaJson[deltaType.PostPosition().String()] = []interface{}{deltaType.DiffString(), 0, DeltaTextDiff}
		case *diff.Deleted:
			//d := delta.(*diff.Deleted)
			deltaJson["_"+deltaType.PrePosition().String()] = []interface{}{deltaType.Value, 0, DeltaDelete}
		case *diff.Moved:
			//d := delta.(*diff.Moved)
			deltaJson["_"+deltaType.PrePosition().String()] = []interface{}{"", deltaType.PostPosition(), DeltaMove}
		default:
			return nil, fmt.Errorf("unknown Delta type detected: %T", deltaType)
		}
	}
	return
}
