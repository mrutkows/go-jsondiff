# go-jsondiff

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Installation

```sh
go get github.com/mrutkows/go-jsondiff
```

---

## JSON Delta format

The JSON formatted output from a comparison is based upon a simplified methodology described originally here:

- [https://github.com/benjamine/jsondiffpatch/blob/master/docs/deltas.md](https://github.com/benjamine/jsondiffpatch/blob/master/docs/deltas.md) for the original description.

### Delta (object) types

#### Added

a value was added, i.e. it was undefined and now has a value.

```
delta = [ newValue ]
```

internal representation:

```golang
// delta.(type) == *diff.Added
type Added struct {
	postDelta         // postDelta{position} where `position` is an interface with a String() method
                      // "PostPosition() interface"
	Value interface{} // added value (i.e., 'newValue')
	                  // "Value()" interface
	similarityCache
}

```

##### Notes: 

- "add" operation has no "preDelta" value, only a "postDelta" value

---

#### Deleted

a value was deleted, i.e. it had a value and is now undefined

```
delta = [ oldValue, 0, 0 ]
```

##### Notes: 

- "delete" operation has no "postDelta" value, only a "preDelta" value

---

#### Modified

a value was replaced by another value

```
delta = [ oldValue, newValue ]
```

---

#### Array (with inner changes)

value is an array, and there are nested changes inside its items

```
delta = {
  _t: 'a',
  index1: innerDelta1,
  index2: innerDelta2,
  index5: innerDelta5
}
```
 
##### Notes: 

- only indices with "inner" deltas are included
- `_t: 'a'`: this tag indicates the delta applies to an array, 
	- if a regular object (or a value type) is found when patching, an error will be thrown

internal representation:

```golang
delta.(type) == *diff.Array
```

---

#### Array Moves

an item was moved to a different position in the same array

```
delta = [ '', destinationIndex, 3]
```
     
##### Notes: 

- '': represents the moved item value (suppressed by default)
- 3: indicates "array move"


##### JSON Delta format example

```diff
 {
   "arr": [
     0: "arr0",
     1: 21,
     2: {
       "num": 1,
-      "str": "pek3f"
+      "str": "changed"
     },
     3: [
       0: 0,
-      1: "1"
+      1: "changed"
     ]
   ],
   "bool": true,
   "num_float": 39.39,
   "num_int": 13,
   "obj": {
     "arr": [
       0: 17,
       1: "str",
       2: {
-        "str": "eafeb"
+        "str": "changed"
       }
     ],
+    "new": "added",
-    "num": 19,
     "obj": {
-      "num": 14,
+      "num": 9999
-      "str": "efj3"
+      "str": "changed"
     },
     "str": "bcded"
   },
   "str": "abcde"
 }
```

When you prefer the delta format of [jsondiffpatch](https://github.com/benjamine/jsondiffpatch), add the `-f delta` option.

```sh
jd -f delta one.json another.json
```

This command shows:

```json
{
  "arr": {
    "2": {
      "str": [
        "pek3f",
        "changed"
      ]
    },
    "3": {
      "1": [
        "1",
        "changed"
      ],
      "_t": "a"
    },
    "_t": "a"
  },
  "obj": {
    "arr": {
      "2": {
        "str": [
          "eafeb",
          "changed"
        ]
      },
      "_t": "a"
    },
    "new": [
      "added"
    ],
    "num": [
      19,
      0,
      0
    ],
    "obj": {
      "num": [
        14,
        9999
      ],
      "str": [
        "efj3",
        "changed"
      ]
    }
  }
}
```
