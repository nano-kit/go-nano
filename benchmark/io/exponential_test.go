// +build benchmark

package io

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

func TestBasic(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	dist := Exponential{Rate: 10, Src: src}
	x := make([]float64, 20)
	generateSamples(x, dist)
	for _, v := range x {
		t.Logf("%.3f", v)
	}
}

func TestRandString(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i := 0; i < 10; i++ {
		t.Log(RandString(5, src))
	}
}

func TestExponential(t *testing.T) {
	t.Parallel()
	src := rand.New(rand.NewSource(1))
	for i, dist := range []Exponential{
		{Rate: 3, Src: src},
		{Rate: 1.5, Src: src},
		{Rate: 0.9, Src: src},
	} {
		testExponential(t, dist, i)
	}
}

func testExponential(t *testing.T, dist Exponential, i int) {
	const (
		tol = 1e-2
		n   = 3e6
	)
	x := make([]float64, n)
	generateSamples(x, dist)
	sort.Float64s(x)

	checkMean(t, i, x, dist, tol)
	checkVarAndStd(t, i, x, dist, tol)
	checkMedian(t, i, x, dist, tol)
}

func generateSamples(x []float64, r Rander) {
	for i := range x {
		x[i] = r.Rand()
	}
}

type meaner interface {
	Mean() float64
}

type quantiler interface {
	Quantile(float64) float64
}

type medianer interface {
	quantiler
	Median() float64
}

type varStder interface {
	StdDev() float64
	Variance() float64
}

func checkMean(t *testing.T, i int, x []float64, m meaner, tol float64) {
	mean := Mean(x, nil)
	if !EqualWithinAbsOrRel(mean, m.Mean(), tol, tol) {
		t.Errorf("Mean mismatch case %v: want: %v, got: %v", i, mean, m.Mean())
	}
}

func checkMedian(t *testing.T, i int, x []float64, m medianer, tol float64) {
	median := Quantile(0.5, x, nil)
	if !EqualWithinAbsOrRel(median, m.Median(), tol, tol) {
		t.Errorf("Median mismatch case %v: want: %v, got: %v", i, median, m.Median())
	}
}

func checkVarAndStd(t *testing.T, i int, x []float64, v varStder, tol float64) {
	variance := Variance(x, nil)
	if !EqualWithinAbsOrRel(variance, v.Variance(), tol, tol) {
		t.Errorf("Variance mismatch case %v: want: %v, got: %v", i, variance, v.Variance())
	}
	std := math.Sqrt(variance)
	if !EqualWithinAbsOrRel(std, v.StdDev(), tol, tol) {
		t.Errorf("StdDev mismatch case %v: want: %v, got: %v", i, std, v.StdDev())
	}
}

// Mean computes the weighted mean of the data set.
//  sum_i {w_i * x_i} / sum_i {w_i}
// If weights is nil then all of the weights are 1. If weights is not nil, then
// len(x) must equal len(weights).
func Mean(x, weights []float64) float64 {
	if weights == nil {
		return Sum(x) / float64(len(x))
	}
	if len(x) != len(weights) {
		panic("stat: slice length mismatch")
	}
	var (
		sumValues  float64
		sumWeights float64
	)
	for i, w := range weights {
		sumValues += w * x[i]
		sumWeights += w
	}
	return sumValues / sumWeights
}

// Quantile returns the sample of x such that x is greater than or
// equal to the fraction p of samples. The exact behavior is determined by the
// CumulantKind, and p should be a number between 0 and 1. Quantile is theoretically
// the inverse of the CDF function, though it may not be the actual inverse
// for all values p and CumulantKinds.
//
// The x data must be sorted in increasing order. If weights is nil then all
// of the weights are 1. If weights is not nil, then len(x) must equal len(weights).
//
// CumulantKind behaviors:
//  - Empirical: Returns the lowest value q for which q is greater than or equal
//  to the fraction p of samples
//  - LinInterp: Returns the linearly interpolated value
func Quantile(p float64, x, weights []float64) float64 {
	if !(p >= 0 && p <= 1) {
		panic("stat: percentile out of bounds")
	}

	if weights != nil && len(x) != len(weights) {
		panic("stat: slice length mismatch")
	}
	if HasNaN(x) {
		return math.NaN() // This is needed because the algorithm breaks otherwise.
	}
	if !sort.Float64sAreSorted(x) {
		panic("x data are not sorted")
	}

	var sumWeights float64
	if weights == nil {
		sumWeights = float64(len(x))
	} else {
		sumWeights = Sum(weights)
	}
	return empiricalQuantile(p, x, weights, sumWeights)
}

func empiricalQuantile(p float64, x, weights []float64, sumWeights float64) float64 {
	var cumsum float64
	fidx := p * sumWeights
	for i := range x {
		if weights == nil {
			cumsum++
		} else {
			cumsum += weights[i]
		}
		if cumsum >= fidx {
			return x[i]
		}
	}
	panic("impossible")
}

// Variance computes the unbiased weighted sample variance:
//  \sum_i w_i (x_i - mean)^2 / (sum_i w_i - 1)
// If weights is nil then all of the weights are 1. If weights is not nil, then
// len(x) must equal len(weights).
// When weights sum to 1 or less, a biased variance estimator should be used.
func Variance(x, weights []float64) float64 {
	_, variance := MeanVariance(x, weights)
	return variance
}

// MeanVariance computes the sample mean and unbiased variance, where the mean and variance are
//  \sum_i w_i * x_i / (sum_i w_i)
//  \sum_i w_i (x_i - mean)^2 / (sum_i w_i - 1)
// respectively.
// If weights is nil then all of the weights are 1. If weights is not nil, then
// len(x) must equal len(weights).
// When weights sum to 1 or less, a biased variance estimator should be used.
func MeanVariance(x, weights []float64) (mean, variance float64) {
	// This uses the corrected two-pass algorithm (1.7), from "Algorithms for computing
	// the sample variance: Analysis and recommendations" by Chan, Tony F., Gene H. Golub,
	// and Randall J. LeVeque.

	// Note that this will panic if the slice lengths do not match.
	mean = Mean(x, weights)
	var (
		ss           float64
		compensation float64
	)
	if weights == nil {
		for _, v := range x {
			d := v - mean
			ss += d * d
			compensation += d
		}
		variance = (ss - compensation*compensation/float64(len(x))) / float64(len(x)-1)
		return mean, variance
	}

	var sumWeights float64
	for i, v := range x {
		w := weights[i]
		d := v - mean
		wd := w * d
		ss += wd * d
		compensation += wd
		sumWeights += w
	}
	variance = (ss - compensation*compensation/sumWeights) / (sumWeights - 1)
	return mean, variance
}

// EqualWithinAbsOrRel returns true when a and b are equal to within
// the absolute or relative tolerances. See EqualWithinAbs and
// EqualWithinRel for details.
func EqualWithinAbsOrRel(a, b, absTol, relTol float64) bool {
	return EqualWithinAbs(a, b, absTol) || EqualWithinRel(a, b, relTol)
}

// minNormalFloat64 is the smallest normal number. For 64 bit IEEE-754
// floats this is 2^{-1022}.
const minNormalFloat64 = 0x1p-1022

// EqualWithinRel returns true when the difference between a and b
// is not greater than tol times the greater absolute value of a and b,
//  abs(a-b) <= tol * max(abs(a), abs(b)).
func EqualWithinRel(a, b, tol float64) bool {
	if a == b {
		return true
	}
	delta := math.Abs(a - b)
	if delta <= minNormalFloat64 {
		return delta <= tol*minNormalFloat64
	}
	// We depend on the division in this relationship to identify
	// infinities (we rely on the NaN to fail the test) otherwise
	// we compare Infs of the same sign and evaluate Infs as equal
	// independent of sign.
	return delta/math.Max(math.Abs(a), math.Abs(b)) <= tol
}

// EqualWithinAbs returns true when a and b have an absolute difference
// not greater than tol.
func EqualWithinAbs(a, b, tol float64) bool {
	return a == b || math.Abs(a-b) <= tol
}

// Sum is
//  var sum float64
//  for i := range x {
//      sum += x[i]
//  }
func Sum(x []float64) float64 {
	var sum float64
	for _, v := range x {
		sum += v
	}
	return sum
}

// HasNaN returns true when the slice s has any values that are NaN and false
// otherwise.
func HasNaN(s []float64) bool {
	for _, v := range s {
		if math.IsNaN(v) {
			return true
		}
	}
	return false
}
