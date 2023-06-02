package gojsondiff

import "golang.org/x/exp/constraints"

// TODO: unused
// func min[T constraints.Ordered](x, y T) T {
// 	if x < y {
// 		return x
// 	}
// 	return y
// }

// func max[T constraints.Ordered](x, y T) T {
// 	if x > y {
// 		return x
// 	}
// 	return y
// }

// TODO: use this in lcs and differ
func maxVariadic[T constraints.Ordered](values ...T) (max T) {
	if values != nil {
		max = values[0]
		if len(values) > 1 {
			for _, value := range values[1:] {
				if max < value {
					max = value
				}
			}
		}
	}
	return max
}
