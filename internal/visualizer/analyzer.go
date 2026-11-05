// Package visualizer turns captured audio samples into compact frames for the
// frontend visualizers. It has no Wails dependency so the math can be tested in
// isolation.
package visualizer

import (
	"math"
	"time"
)

const (
	// SampleRate is the capture rate used by the monitor source.
	SampleRate = 44100
	// WindowSize is the analysis window. Short windows keep the visualizer
	// responsive; 1024 samples is about 23 ms at 44.1 kHz.
	WindowSize = 1024
	// BandCount is the number of log-spaced spectrum bands.
	BandCount = 24
	// WaveformPoints is the number of oscilloscope samples in a frame.
	WaveformPoints = 64

	minFrequency = 30.0
	maxFrequency = 16000.0

	attack  = 0.55
	release = 0.12

	// Quiet content looks flat when the raw FFT amplitude is used directly.
	// A mild gain and a perceptual curve keep low-level music visible while
	// still clamping to the documented 0..1 range.
	bandGain  = 1.6
	bandCurve = 0.65

	int16Scale = 1.0 / 32768.0
)

// Frame is the JSON payload pushed to the webview. All values are normalized:
// RMS and bands are 0..1 and waveform samples are -1..1.
type Frame struct {
	RMS      float64   `json:"rms"`
	Bands    []float64 `json:"bands"`
	Waveform []float64 `json:"waveform"`
	At       int64     `json:"at"`
}

// Analyzer keeps the smoothing state between frames. It is not safe for
// concurrent use; a capture loop owns one instance.
type Analyzer struct {
	window    []float64
	real      []float64
	imag      []float64
	smoothed  []float64
	windowSum float64
}

// NewAnalyzer creates an analyzer with a precomputed Hann window and scratch
// buffers.
func NewAnalyzer() *Analyzer {
	window := make([]float64, WindowSize)
	sum := 0.0
	for index := range window {
		value := 0.5 * (1 - math.Cos(2*math.Pi*float64(index)/float64(WindowSize-1)))
		window[index] = value
		sum += value
	}
	return &Analyzer{
		window:    window,
		real:      make([]float64, WindowSize),
		imag:      make([]float64, WindowSize),
		smoothed:  make([]float64, BandCount),
		windowSum: sum,
	}
}

// Analyze converts signed 16-bit mono samples into a frame. Less than a full
// window is zero-padded; silence produces zeroed bands, waveform and RMS.
func (a *Analyzer) Analyze(samples []int16) Frame {
	frame := Frame{
		Bands:    make([]float64, BandCount),
		Waveform: make([]float64, WaveformPoints),
		At:       time.Now().UnixMilli(),
	}

	count := len(samples)
	if count > WindowSize {
		count = WindowSize
	}

	silent := true
	sumSquares := 0.0
	for index := 0; index < count; index++ {
		value := float64(samples[index]) * int16Scale
		sumSquares += value * value
		if samples[index] != 0 {
			silent = false
		}
	}
	if count > 0 {
		frame.RMS = math.Sqrt(sumSquares / float64(count))
	}

	if silent {
		for index := range a.smoothed {
			a.smoothed[index] = 0
		}
		return frame
	}

	// The oscilloscope averages small groups so high frequencies do not alias
	// into a noisy line.
	segment := WindowSize / WaveformPoints
	for point := 0; point < WaveformPoints; point++ {
		sum := 0.0
		samplesInSegment := 0
		for index := point * segment; index < (point+1)*segment; index++ {
			if index >= count {
				break
			}
			sum += float64(samples[index]) * int16Scale
			samplesInSegment++
		}
		if samplesInSegment > 0 {
			frame.Waveform[point] = clamp(sum/float64(samplesInSegment), -1, 1)
		}
	}

	for index := 0; index < WindowSize; index++ {
		a.real[index] = 0
		a.imag[index] = 0
		if index < count {
			a.real[index] = float64(samples[index]) * int16Scale * a.window[index]
		}
	}
	fft(a.real, a.imag)

	for band := 0; band < BandCount; band++ {
		value := a.bandAmplitude(band)
		value = math.Pow(math.Min(1, value*bandGain), bandCurve)
		previous := a.smoothed[band]
		if value > previous {
			previous += (value - previous) * attack
		} else {
			previous += (value - previous) * release
		}
		a.smoothed[band] = previous
		frame.Bands[band] = previous
	}

	return frame
}

// bandAmplitude normalizes the FFT energy of one log-spaced band to 0..1,
// where 1 is a full-scale sine.
func (a *Analyzer) bandAmplitude(band int) float64 {
	low, high := bandBounds(band)
	start := int(math.Floor(low * float64(WindowSize) / float64(SampleRate)))
	end := int(math.Ceil(high * float64(WindowSize) / float64(SampleRate)))
	if start < 1 {
		start = 1
	}
	if end <= start {
		end = start + 1
	}
	if end > WindowSize/2+1 {
		end = WindowSize/2 + 1
	}

	energy := 0.0
	for bin := start; bin < end && bin <= WindowSize/2; bin++ {
		energy += a.real[bin]*a.real[bin] + a.imag[bin]*a.imag[bin]
	}
	amplitude := math.Sqrt(energy) * 2 / a.windowSum
	return amplitude
}

// bandBounds returns the frequency limits of a band. Bands are log-spaced so
// low frequencies get more visual resolution than high ones.
func bandBounds(band int) (float64, float64) {
	ratio := maxFrequency / minFrequency
	low := minFrequency * math.Pow(ratio, float64(band)/BandCount)
	high := minFrequency * math.Pow(ratio, float64(band+1)/BandCount)
	return low, high
}

// fft is an in-place iterative radix-2 Cooley-Tukey transform.
func fft(real, imag []float64) {
	size := len(real)
	if size < 2 || len(imag) < size || size&(size-1) != 0 {
		return
	}

	for index, reversed := 1, 0; index < size; index++ {
		bit := size >> 1
		for reversed&bit != 0 {
			reversed ^= bit
			bit >>= 1
		}
		reversed ^= bit
		if index < reversed {
			real[index], real[reversed] = real[reversed], real[index]
			imag[index], imag[reversed] = imag[reversed], imag[index]
		}
	}

	for length := 2; length <= size; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		stepReal := math.Cos(angle)
		stepImag := math.Sin(angle)
		for start := 0; start < size; start += length {
			phaseReal := 1.0
			phaseImag := 0.0
			half := length / 2
			for offset := 0; offset < half; offset++ {
				left := start + offset
				right := left + half
				rightReal := real[right]*phaseReal - imag[right]*phaseImag
				rightImag := real[right]*phaseImag + imag[right]*phaseReal

				real[left], real[right] = real[left]+rightReal, real[left]-rightReal
				imag[left], imag[right] = imag[left]+rightImag, imag[left]-rightImag

				nextReal := phaseReal*stepReal - phaseImag*stepImag
				phaseImag = phaseReal*stepImag + phaseImag*stepReal
				phaseReal = nextReal
			}
		}
	}
}

func clamp(value, minimum, maximum float64) float64 {
	if !math.IsNaN(value) && value < minimum {
		return minimum
	}
	if !math.IsNaN(value) && value > maximum {
		return maximum
	}
	return value
}
