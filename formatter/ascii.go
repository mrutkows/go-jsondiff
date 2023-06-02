package formatter

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	diff "github.com/mrutkows/go-jsondiff"
)

const (
	AsciiSame    = " "
	AsciiAdded   = "+"
	AsciiDeleted = "-"
	AsciiMoved   = "=>"
)

var AsciiStyles = map[string]string{
	AsciiAdded:   "30;42", // background green
	AsciiDeleted: "30;41", // background red
	AsciiMoved:   "30;42", // background yellow
}

func NewAsciiFormatter(left interface{}, config AsciiFormatterConfig) *AsciiFormatter {
	return &AsciiFormatter{
		left:   left,
		config: config,
	}
}

type AsciiFormatter struct {
	left    interface{}
	config  AsciiFormatterConfig
	buffer  *bytes.Buffer
	path    []string
	size    []int
	inArray []bool
	line    *AsciiLine
}

type AsciiFormatterConfig struct {
	ShowArrayIndex bool
	Coloring       bool
}

var AsciiFormatterDefaultConfig = AsciiFormatterConfig{}

type AsciiLine struct {
	marker string
	indent int
	buffer *bytes.Buffer
}

func (f *AsciiFormatter) Format(diff diff.Diff) (result string, err error) {
	f.buffer = bytes.NewBuffer([]byte{})
	f.path = []string{}
	f.size = []int{}
	f.inArray = []bool{}

	if v, ok := f.left.(map[string]interface{}); ok {
		f.formatObject(v, diff)
	} else if v, ok := f.left.([]interface{}); ok {
		f.formatArray(v, diff)
	} else {
		return "", fmt.Errorf("expected map[string]interface{} or []interface{}, got %T",
			f.left)
	}

	return f.buffer.String(), nil
}

func (f *AsciiFormatter) formatObject(left map[string]interface{}, df diff.Diff) {
	f.addLineWith(AsciiSame, "{")
	f.push("ROOT", len(left), false)
	f.processObject(left, df.Deltas())
	f.pop()
	f.addLineWith(AsciiSame, "}")
}

func (f *AsciiFormatter) formatArray(left []interface{}, df diff.Diff) {
	f.addLineWith(AsciiSame, "[")
	f.push("ROOT", len(left), true)
	f.processArray(left, df.Deltas())
	f.pop()
	f.addLineWith(AsciiSame, "]")
}

func (f *AsciiFormatter) processArray(array []interface{}, deltas []diff.Delta) error {
	patchedIndex := 0
	for index, value := range array {
		f.processArrayOrObjectItem(value, deltas, diff.Index(index))
		patchedIndex++
	}

	// additional Added
	for _, delta := range deltas {
		switch delta.(type) {
		case *diff.Added:
			d := delta.(*diff.Added)
			// skip items already processed
			if int(d.Position.(diff.Index)) < len(array) {
				continue
			}
			f.printRecursive(d.Position.String(), d.Value, AsciiAdded)
		}
	}

	return nil
}

func (f *AsciiFormatter) processObject(object map[string]interface{}, deltas []diff.Delta) error {
	names := sortedKeys(object)
	for _, name := range names {
		value := object[name]
		f.processArrayOrObjectItem(value, deltas, diff.Name(name))
	}

	// Added
	for _, delta := range deltas {
		switch deltaType := delta.(type) {
		case *diff.Added:
			//d := delta.(*diff.Added)
			f.printRecursive(deltaType.Position.String(), deltaType.Value, AsciiAdded)
		}
	}

	return nil
}

func (f *AsciiFormatter) processArrayOrObjectItem(value interface{}, deltas []diff.Delta, position diff.Position) error {
	matchedDeltas := f.searchDeltas(deltas, position)
	positionStr := position.String()
	if len(matchedDeltas) > 0 {
		for _, matchedDelta := range matchedDeltas {

			switch matchedDelta := matchedDelta.(type) {
			case *diff.Object:
				mapObject, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("expected: map[string]interface{}: actual type: (%T)", value)
				}
				f.newLine(AsciiSame)
				f.printKey(positionStr)
				f.print("{")
				f.closeLine()
				f.push(positionStr, len(mapObject), false)
				f.processObject(mapObject, matchedDelta.Deltas)
				f.pop()
				f.newLine(AsciiSame)
				f.print("}")
				f.printComma()
				f.closeLine()

			case *diff.Array:
				interfaceSlice, ok := value.([]interface{})
				if !ok {
					return fmt.Errorf("expected: []interface{}: actual type: (%T)", value)
				}
				f.newLine(AsciiSame)
				f.printKey(positionStr)
				f.print("[")
				f.closeLine()
				f.push(positionStr, len(interfaceSlice), true)
				f.processArray(interfaceSlice, matchedDelta.Deltas)
				f.pop()
				f.newLine(AsciiSame)
				f.print("]")
				f.printComma()
				f.closeLine()

			case *diff.Added:
				f.printRecursive(positionStr, matchedDelta.Value, AsciiAdded)
				f.size[len(f.size)-1]++
			case *diff.Modified:
				savedSize := f.size[len(f.size)-1]
				f.printRecursive(positionStr, matchedDelta.OldValue, AsciiDeleted)
				f.size[len(f.size)-1] = savedSize
				f.printRecursive(positionStr, matchedDelta.NewValue, AsciiAdded)
			case *diff.TextDiff:
				savedSize := f.size[len(f.size)-1]
				f.printRecursive(positionStr, matchedDelta.OldValue, AsciiDeleted)
				f.size[len(f.size)-1] = savedSize
				f.printRecursive(positionStr, matchedDelta.NewValue, AsciiAdded)
			case *diff.Deleted:
				f.printRecursive(positionStr, matchedDelta.Value, AsciiDeleted)
			case *diff.Moved:
				fmt.Printf("processItem(): valueType: [%T], matchedDelta type: [%T]", value, matchedDelta)
				f.printRecursive(matchedDelta.PrePosition().String(), matchedDelta.Value, AsciiMoved)
				//f.printRecursive(matchedDelta.PostPosition().String(), matchedDelta.Value, AsciiAdded)
				//fmt.Printf("processItem(): *diff.Moved: not supported\n")
			default:
				err := fmt.Errorf("unknown Delta type [%T] detected", matchedDelta)
				return errors.New(err.Error())
			}

		}
	} else {
		f.printRecursive(positionStr, value, AsciiSame)
	}

	return nil
}

func (f *AsciiFormatter) searchDeltas(deltas []diff.Delta, position diff.Position) (results []diff.Delta) {
	results = make([]diff.Delta, 0)
	for _, delta := range deltas {
		switch deltaType := delta.(type) {
		case diff.PostDelta:
			if deltaType.PostPosition() == position {
				results = append(results, delta)
			}
		case diff.PreDelta:
			if deltaType.PrePosition() == position {
				results = append(results, delta)
			}
		default:
			panic("heh")
		}
	}
	return
}

func (f *AsciiFormatter) push(name string, size int, array bool) {
	f.path = append(f.path, name)
	f.size = append(f.size, size)
	f.inArray = append(f.inArray, array)
}

func (f *AsciiFormatter) pop() {
	f.path = f.path[0 : len(f.path)-1]
	f.size = f.size[0 : len(f.size)-1]
	f.inArray = f.inArray[0 : len(f.inArray)-1]
}

func (f *AsciiFormatter) addLineWith(marker string, value string) {
	f.line = &AsciiLine{
		marker: marker,
		indent: len(f.path),
		buffer: bytes.NewBufferString(value),
	}
	f.closeLine()
}

func (f *AsciiFormatter) newLine(marker string) {
	f.line = &AsciiLine{
		marker: marker,
		indent: len(f.path),
		buffer: bytes.NewBuffer([]byte{}),
	}
}

func (f *AsciiFormatter) closeLine() {
	style, ok := AsciiStyles[f.line.marker]
	if f.config.Coloring && ok {
		f.buffer.WriteString("\x1b[" + style + "m")
	}

	f.buffer.WriteString(f.line.marker)
	for n := 0; n < f.line.indent; n++ {
		f.buffer.WriteString("  ")
	}
	f.buffer.Write(f.line.buffer.Bytes())

	if f.config.Coloring && ok {
		f.buffer.WriteString("\x1b[0m")
	}

	f.buffer.WriteRune('\n')
}

func (f *AsciiFormatter) printKey(name string) {
	if !f.inArray[len(f.inArray)-1] {
		fmt.Fprintf(f.line.buffer, `"%s": `, name)
	} else if f.config.ShowArrayIndex {
		fmt.Fprintf(f.line.buffer, `%s: `, name)
	}
}

func (f *AsciiFormatter) printComma() {
	f.size[len(f.size)-1]--
	if f.size[len(f.size)-1] > 0 {
		f.line.buffer.WriteRune(',')
	}
}

func (f *AsciiFormatter) printValue(value interface{}) {
	switch value.(type) {
	case string:
		fmt.Fprintf(f.line.buffer, `"%s"`, value)
	case nil:
		f.line.buffer.WriteString("null")
	default:
		fmt.Fprintf(f.line.buffer, `%#v`, value)
	}
}

func (f *AsciiFormatter) print(a string) {
	f.line.buffer.WriteString(a)
}

func (f *AsciiFormatter) printRecursive(name string, value interface{}, marker string) {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		f.newLine(marker)
		f.printKey(name)
		f.print("{")
		f.closeLine()
		size := len(typedValue)
		f.push(name, size, false)

		keys := sortedKeys(typedValue)
		for _, key := range keys {
			f.printRecursive(key, typedValue[key], marker)
		}
		f.pop()
		f.newLine(marker)
		f.print("}")
		f.printComma()
		f.closeLine()

	case []interface{}:
		f.newLine(marker)
		f.printKey(name)
		f.print("[")
		f.closeLine()
		size := len(typedValue)
		f.push("", size, true)
		for _, item := range typedValue {
			f.printRecursive("", item, marker)
		}
		f.pop()
		f.newLine(marker)
		f.print("]")
		f.printComma()
		f.closeLine()

	default:
		f.newLine(marker)
		f.printKey(name)
		f.printValue(value)
		f.printComma()
		f.closeLine()
	}
}

func sortedKeys(m map[string]interface{}) (keys []string) {
	keys = make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return
}
