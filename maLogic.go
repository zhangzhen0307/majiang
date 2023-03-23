package main

import (
	"math/rand"
	"sort"
	"time"
)

const (
	SHUNZI1 = iota
	SHUNZI2
	SHUNZI3
	KEZI1
	KEZI2
	KEZI3
)

func isHu(hands []Card, newCard int, laizi int) bool {
	var nums = make([]int, 0)
	for _, card := range hands {
		nums = append(nums, card.Num)
	}
	if newCard != 0 {
		nums = append(nums, newCard)
	}
	sort.Ints(nums)
	var countValues = make(map[int]int)
	for _, num := range nums {
		if count, ok := countValues[num]; ok {
			countValues[num] = count + 1
		} else {
			countValues[num] = 1
		}
	}
	var lCount = 0
	if v, ok := countValues[laizi]; ok {
		lCount = v
		delete(countValues, laizi)
	}

	if lCount > 1 {
		return false
	}

	for k, v := range countValues {
		var result = false
		if v > 1 {
			var c = copyMap(countValues)
			c[k] -= 2
			c = filterMap(c)
			result = isOkForTriple(c, lCount)
		} else if lCount > 0 {
			var c = copyMap(countValues)
			c[k] -= 1
			c = filterMap(c)
			result = isOkForTriple(c, lCount-1)
		}

		if result {
			return true
		}
	}

	return false
}

func isOkForTriple(countValues map[int]int, lCount int) bool {
	if len(countValues) == 0 {
		return true
	}
	var minKey = 1000
	for k, _ := range countValues {
		if k < minKey {
			minKey = k
		}
	}
	var minValue = countValues[minKey]
	var possibility = make([]int, 0)
	if minKey < DONG {
		var _, ok1 = countValues[minKey+1]
		var _, ok2 = countValues[minKey+2]
		if ok1 && ok2 {
			possibility = append(possibility, SHUNZI1)
		} else if ok1 && lCount > 0 {
			possibility = append(possibility, SHUNZI2)
		} else if ok2 && lCount > 0 {
			possibility = append(possibility, SHUNZI3)
		}

		if minValue >= 3 {
			possibility = append(possibility, KEZI1)
		} else if minValue == 2 && lCount > 0 {
			possibility = append(possibility, KEZI2)
		} else if minValue == 1 && lCount > 1 {
			possibility = append(possibility, KEZI3)
		}
	} else {
		if minValue >= 3 {
			possibility = append(possibility, KEZI1)
		} else if minValue == 2 && lCount > 0 {
			possibility = append(possibility, KEZI2)
		} else if minValue == 1 && lCount > 1 {
			possibility = append(possibility, KEZI3)
		}
	}

	if len(possibility) == 0 {
		return false
	}
	for _, p := range possibility {
		var result = false
		switch p {
		case SHUNZI1:
			var c = copyMap(countValues)
			c[minKey] -= 1
			c[minKey+1] -= 1
			c[minKey+2] -= 1
			c = filterMap(c)
			result = isOkForTriple(c, lCount)
			break
		case SHUNZI2:
			var c = copyMap(countValues)
			c[minKey] -= 1
			c[minKey+1] -= 1
			c = filterMap(c)
			result = isOkForTriple(c, lCount-1)
			break
		case SHUNZI3:
			var c = copyMap(countValues)
			c[minKey] -= 1
			c[minKey+2] -= 1
			c = filterMap(c)
			result = isOkForTriple(c, lCount-1)
			break
		case KEZI1:
			var c = copyMap(countValues)
			c[minKey] -= 3
			c = filterMap(c)
			result = isOkForTriple(c, lCount)
			break
		case KEZI2:
			var c = copyMap(countValues)
			c[minKey] -= 2
			c = filterMap(c)
			result = isOkForTriple(c, lCount-1)
			break
		case KEZI3:
			var c = copyMap(countValues)
			c[minKey] -= 1
			c = filterMap(c)
			result = isOkForTriple(c, lCount-2)
			break
		}
		if result {
			return true
		}
	}
	return false
}

func copyMap(source map[int]int) map[int]int {
	var c = make(map[int]int)
	for k, v := range source {
		c[k] = v
	}
	return c
}

func filterMap(source map[int]int) map[int]int {
	for k, v := range source {
		if v == 0 {
			delete(source, k)
		}
	}
	return source
}

func randCards() ([]Card, int) {
	var cards = make([]Card, 0)
	rand.Seed(time.Now().UnixNano())
	var lcount = 1
	for i := 0; i < 14-lcount; i++ {
		var r1 = rand.Intn(3)
		var r2 = rand.Intn(9) + 1
		var c = Card{Num: r1*100 + r2}
		cards = append(cards, c)
	}
	return cards, lcount
}
