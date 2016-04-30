package gopter

import (
	"fmt"
	"reflect"
)

// Shrink is a stream of shrinked down values
// Once the result of a shrink is false, it is considered to be exhausted
type Shrink func() (interface{}, bool)

// Filter creates a shrink filtered by a condition
func (s Shrink) Filter(condition func(interface{}) bool) Shrink {
	if condition == nil {
		return s
	}
	return func() (interface{}, bool) {
		value, ok := s()
		for ok && !condition(value) {
			value, ok = s()
		}
		return value, ok
	}
}

// Map creates a shrink by applying a converter to each element of a shrink.
// f: has to be a function with one parameter (matching the generated value) and a single return.
func (s Shrink) Map(f interface{}) Shrink {
	mapperVal := reflect.ValueOf(f)
	mapperType := mapperVal.Type()

	if mapperVal.Kind() != reflect.Func {
		panic(fmt.Sprintf("Param of Map has to be a func, but is %v", mapperType.Kind()))
	}
	if mapperType.NumIn() != 1 {
		panic(fmt.Sprintf("Param of Map has to be a func with one param, but is %v", mapperType.NumIn()))
	}
	if mapperType.NumOut() != 1 {
		panic(fmt.Sprintf("Param of Map has to be a func with one return value, but is %v", mapperType.NumOut()))
	}

	return func() (interface{}, bool) {
		value, ok := s()
		if ok {
			return mapperVal.Call([]reflect.Value{reflect.ValueOf(value)})[0].Interface(), ok
		}
		return nil, false
	}
}

// All collects all shrinks as a slice. Use with care as this might create
// large results depending on the complexity of the shrink
func (s Shrink) All() []interface{} {
	result := make([]interface{}, 0)
	value, ok := s()
	for ok {
		result = append(result, value)
		value, ok = s()
	}
	return result
}

type concatedShrink struct {
	index   int
	shrinks []Shrink
}

func (c *concatedShrink) Next() (interface{}, bool) {
	for c.index < len(c.shrinks) {
		value, ok := c.shrinks[c.index]()
		if ok {
			return value, ok
		}
		c.index++
	}
	return nil, false
}

// ConcatShrinks concats an array of shrinks to a single shrinks
func ConcatShrinks(shrinks ...Shrink) Shrink {
	concated := &concatedShrink{
		index:   0,
		shrinks: shrinks,
	}
	return concated.Next
}

type interleaved struct {
	first          Shrink
	second         Shrink
	firstExhausted bool
	secondExhaused bool
	state          bool
}

func (i *interleaved) Next() (interface{}, bool) {
	for !i.firstExhausted || !i.secondExhaused {
		i.state = !i.state
		if i.state && !i.firstExhausted {
			value, ok := i.first()
			if ok {
				return value, true
			}
			i.firstExhausted = true
		} else if !i.state && !i.secondExhaused {
			value, ok := i.second()
			if ok {
				return value, true
			}
			i.secondExhaused = true
		}
	}
	return nil, false
}

// Interleave this shrink with another
// Both shrinks are expected to produce the same result
func (s Shrink) Interleave(other Shrink) Shrink {
	interleaved := &interleaved{
		first:  s,
		second: other,
	}
	return interleaved.Next
}

// Shrinker creates a shrink for a given value
type Shrinker func(value interface{}) Shrink

type elementShrink struct {
	original      []interface{}
	index         int
	elementShrink Shrink
}

func (e *elementShrink) Next() (interface{}, bool) {
	element, ok := e.elementShrink()
	if !ok {
		return nil, false
	}
	shrinked := make([]interface{}, len(e.original))
	copy(shrinked, e.original)
	shrinked[e.index] = element

	return shrinked, true
}

func CombineShrinker(shrinkers ...Shrinker) Shrinker {
	return func(v interface{}) Shrink {
		values := v.([]interface{})
		shrinks := make([]Shrink, 0, len(values))
		for i, shrinker := range shrinkers {
			if i >= len(values) {
				break
			}
			shrink := &elementShrink{
				original:      values,
				index:         i,
				elementShrink: shrinker(values[i]),
			}
			shrinks = append(shrinks, shrink.Next)
		}
		return ConcatShrinks(shrinks...)
	}
}

var NoShrink = Shrink(func() (interface{}, bool) {
	return nil, false
})

var NoShrinker = Shrinker(func(value interface{}) Shrink {
	return NoShrink
})
