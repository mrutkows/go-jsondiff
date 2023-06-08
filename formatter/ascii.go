package formatter

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"

	diff "github.com/mrutkows/go-jsondiff"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

const (
	AsciiSame    = " "
	AsciiAdded   = "+"
	AsciiDeleted = "-"
	AsciiMoved   = "-+"
	Moved        = "=>"
)

// ANSI color variants
const (
	NORMAL = "m"
	BRIGHT = ";1m"
)

var AsciiStyles = map[string]string{
	AsciiDeleted: "30;41", // background red
	AsciiAdded:   "30;42", // background green
	AsciiMoved:   "30;43", // background yellow
}

func NewAsciiFormatter(left interface{}, config AsciiFormatterConfig) *AsciiFormatter {
	return &AsciiFormatter{
		left:   left,
		config: config,
	}
}

type AsciiFormatter struct {
	left                      interface{}
	config                    AsciiFormatterConfig
	buffer                    *bytes.Buffer
	jsonObjectPath            []string
	jsonObjectUnprocessedSize []int
	inArray                   []bool
	line                      *AsciiLine
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
	f.jsonObjectPath = []string{}
	f.jsonObjectUnprocessedSize = []int{}
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

func (f *AsciiFormatter) processArray(array []interface{}, deltas []diff.Delta) (err error) {

	err = f.preProcessArray(array, deltas)
	if err != nil {
		return
	}

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

func (f *AsciiFormatter) preProcessArray(slice []interface{}, deltas []diff.Delta) (err error) {

	postDeltaMap := orderedmap.New[string, interface{}]()

	// initialize the map to pre-delta entries
	for position, value := range slice {
		preDeltaPosition := diff.Name(strconv.Itoa(position))
		entry := diff.NewDisplaced(preDeltaPosition, value, nil) // NOTE: if diff.Displaced NOT used, then we could use float32 as key
		postDeltaMap.Set(preDeltaPosition.String(), entry)
	}

	// process deltas by type... add, delete, moved
	for _, delta := range deltas {

		switch deltaType := delta.(type) {
		case *diff.Added: // Added objects "displace" the existing in the slice order (and those entries after)
			// insert value at "post"
			fmt.Printf("[%T]: PostPosition(): %s\n", delta, deltaType.PostPosition())
			postPosition := deltaType.PostPosition().String()
			oldValue, present := postDeltaMap.Delete(postPosition)
			fmt.Printf(">> a) Deleted [\"%v\"] \n", deltaType.PostPosition())

			if present {
				fmt.Printf("  >> pre-existing value found at position: [\"%v\"] \n", postPosition)
				floatNum, errConvert := strconv.ParseFloat(postPosition, 64)

				if errConvert != nil {
					return errConvert
				}
				floatNum = floatNum + 0.000001
				newPosition := fmt.Sprintf("%f", floatNum)
				fmt.Printf("  >> adding pre-existing value at new position: [\"%v\"] \n", newPosition)
				postDeltaMap.Set(newPosition, oldValue)
			}

		case *diff.Deleted: // Deleted objects still appear in output, so are a
			fmt.Printf("[%T]: PrePosition(): %s\n", delta, deltaType.PrePosition())
		case *diff.Moved:
			// delete value at "pre" (key) insert value at "post" (key)
			// if pre == post then skip (trace message)
			fmt.Printf("[%T]: PostPosition(): %s\n", delta, deltaType.PostPosition())
		case *diff.Displaced:
			fmt.Printf("[%T]: PrePosition(): %s\n", delta, deltaType.PrePosition())
		default:
			err = fmt.Errorf("unknown delta type: [%T]", delta)
		}

	}

	for kv := postDeltaMap.Oldest(); kv != nil; kv = kv.Next() {
		fmt.Printf("[%s]: %v (%T)\n", kv.Key, kv.Value, kv.Value)
	}

	return
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
			f.printRecursive(deltaType.Position.String(), deltaType.Value, AsciiAdded)
		}
	}

	return nil
}

func (f *AsciiFormatter) processArrayOrObjectItem(value interface{}, deltas []diff.Delta, position diff.Position) error {
	matchedDeltas := f.searchDeltas(deltas, position)
	objectKey := position.String()
	numDeltaMatches := len(matchedDeltas)

	//fmt.Printf("BEFORE: f.size[]: %v\n", f.size)

	if numDeltaMatches > 0 {
		for _, matchedDelta := range matchedDeltas {

			switch matchedDeltaType := matchedDelta.(type) {
			case *diff.Object:
				mapObject, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("expected: map[string]interface{}: actual type: (%T)", value)
				}
				f.newLine(AsciiSame)
				f.printKey(objectKey)
				f.print("{")
				f.closeLine()
				f.push(objectKey, len(mapObject), false)
				f.processObject(mapObject, matchedDeltaType.Deltas)
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
				f.printKey(objectKey)
				f.print("[")
				f.closeLine()
				f.push(objectKey, len(interfaceSlice), true)
				f.processArray(interfaceSlice, matchedDeltaType.Deltas)
				f.pop()
				f.newLine(AsciiSame)
				f.print("]")
				f.printComma()
				f.closeLine()

			case *diff.Added:
				f.printRecursive(objectKey, matchedDeltaType.Value, AsciiAdded)
				f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1]++
			case *diff.Modified:
				savedSize := f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1]
				f.printRecursive(objectKey, matchedDeltaType.OldValue, AsciiDeleted)
				f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1] = savedSize
				f.printRecursive(objectKey, matchedDeltaType.NewValue, AsciiAdded)
			case *diff.TextDiff:
				savedSize := f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1]
				f.printRecursive(objectKey, matchedDeltaType.OldValue, AsciiDeleted)
				f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1] = savedSize
				f.printRecursive(objectKey, matchedDeltaType.NewValue, AsciiAdded)
			case *diff.Deleted:
				f.printRecursive(objectKey, matchedDeltaType.Value, AsciiDeleted)
			case *diff.Moved:
				fmt.Printf("processItem(): valueType: [%T], matchedDelta type: [%T]\n", value, matchedDeltaType)
				movedString := fmt.Sprintf("%s%s%s", matchedDeltaType.PrePosition().String(), Moved, matchedDeltaType.PostPosition().String())
				f.printRecursive(movedString, matchedDeltaType.Value, AsciiMoved)

				if _, ok := value.(map[string]interface{}); ok {
					f.printRecursive(objectKey, value, AsciiSame)
				} else {
					fmt.Printf("unexpected type for diff.Move: %T\n", value)
				}

			default:
				err := fmt.Errorf("unknown Delta type [%T] detected", matchedDeltaType)
				return errors.New(err.Error())
			}

		}
	} else {
		f.printRecursive(objectKey, value, AsciiSame)
	}

	//fmt.Printf("AFTER: f.size[]: %v\n", f.size)

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
	f.jsonObjectPath = append(f.jsonObjectPath, name)
	f.jsonObjectUnprocessedSize = append(f.jsonObjectUnprocessedSize, size)
	f.inArray = append(f.inArray, array)
}

func (f *AsciiFormatter) pop() {
	f.jsonObjectPath = f.jsonObjectPath[0 : len(f.jsonObjectPath)-1]
	f.jsonObjectUnprocessedSize = f.jsonObjectUnprocessedSize[0 : len(f.jsonObjectUnprocessedSize)-1]
	f.inArray = f.inArray[0 : len(f.inArray)-1]
}

func (f *AsciiFormatter) addLineWith(marker string, value string) {
	f.line = &AsciiLine{
		marker: marker,
		indent: len(f.jsonObjectPath),
		buffer: bytes.NewBufferString(value),
	}
	f.closeLine()
}

func (f *AsciiFormatter) newLine(marker string) {
	f.line = &AsciiLine{
		marker: marker,
		indent: len(f.jsonObjectPath),
		buffer: bytes.NewBuffer([]byte{}),
	}
}

func (f *AsciiFormatter) closeLine() {
	style, ok := AsciiStyles[f.line.marker]
	if f.config.Coloring && ok {
		f.buffer.WriteString("\x1b[" + style + NORMAL)
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
	// Decrement remaining the (array) length as the action of printing a comma indicates
	// one less remaining object to emit.
	f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1]--
	// As long as we have more elements to output in JSON the array, emit a comma
	if f.jsonObjectUnprocessedSize[len(f.jsonObjectUnprocessedSize)-1] > 0 {
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
