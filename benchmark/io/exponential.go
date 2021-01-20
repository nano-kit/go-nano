// Copyright (c) nano Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package io

import (
	"math"
	"math/rand"
	"unsafe"
)

// Rander wraps the Rand method.
type Rander interface {
	// Rand returns a random sample drawn from the distribution.
	Rand() float64
}

// Exponential represents the exponential distribution (https://en.wikipedia.org/wiki/Exponential_distribution).
type Exponential struct {
	Rate float64
	Src  rand.Source
}

// Rand returns a random sample drawn from the distribution.
func (e Exponential) Rand() float64 {
	var rnd float64
	if e.Src == nil {
		rnd = rand.ExpFloat64()
	} else {
		rnd = rand.New(e.Src).ExpFloat64()
	}
	return rnd / e.Rate
}

// Mean returns the mean of the probability distribution.
func (e Exponential) Mean() float64 {
	return 1 / e.Rate
}

// Median returns the median of the probability distribution.
func (e Exponential) Median() float64 {
	return math.Ln2 / e.Rate
}

// StdDev returns the standard deviation of the probability distribution.
func (e Exponential) StdDev() float64 {
	return 1 / e.Rate
}

// Variance returns the variance of the probability distribution.
func (e Exponential) Variance() float64 {
	return 1 / (e.Rate * e.Rate)
}

const badPercentile = "distuv: percentile out of bounds"

// Quantile returns the inverse of the cumulative probability distribution.
func (e Exponential) Quantile(p float64) float64 {
	if p < 0 || p > 1 {
		panic(badPercentile)
	}
	return -math.Log(1-p) / e.Rate
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandString effectively generate random string of length n
// (https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go)
func RandString(n int, src rand.Source) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}
