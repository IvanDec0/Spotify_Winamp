package visualizer

import (
	"math"
	"testing"
)

func sineSamples(frequency float64, amplitude float64, count int) []int16 {
	samples := make([]int16, count)
	for index := range samples {
		value := amplitude * math.Sin(2*math.Pi*frequency*float64(index)/SampleRate)
		samples[index] = int16(math.Round(value))
	}
	return samples
}

func TestAnalyzePutsSineInExpectedBand(t *testing.T) {
	analyzer := NewAnalyzer()
	frame := analyzer.Analyze(sineSamples(440, 20000, WindowSize))

	maximum := 0
	for index, value := range frame.Bands {
		if value > frame.Bands[maximum] {
			maximum = index
		}
	}

	low, high := bandBounds(maximum)
	if 440 < low || 440 >= high {
		t.Fatalf("maximum band %d covers %.1f-%.1f Hz, not 440 Hz", maximum, low, high)
	}
	if frame.Bands[maximum] <= 0 {
		t.Fatal("expected a positive amplitude for a full-scale sine")
	}
}

func TestAnalyzeSilenceIsZero(t *testing.T) {
	analyzer := NewAnalyzer()
	frame := analyzer.Analyze(make([]int16, WindowSize))

	if frame.RMS != 0 {
		t.Fatalf("RMS = %v, want 0", frame.RMS)
	}
	for index, value := range frame.Bands {
		if value != 0 {
			t.Fatalf("band %d = %v, want 0", index, value)
		}
	}
	for index, value := range frame.Waveform {
		if value != 0 {
			t.Fatalf("waveform %d = %v, want 0", index, value)
		}
	}
}

func TestAnalyzeEmptyInputIsZero(t *testing.T) {
	frame := NewAnalyzer().Analyze(nil)

	if frame.RMS != 0 || len(frame.Bands) != BandCount || len(frame.Waveform) != WaveformPoints {
		t.Fatalf("unexpected empty frame: %+v", frame)
	}
	for index, value := range frame.Bands {
		if value != 0 {
			t.Fatalf("band %d = %v, want 0", index, value)
		}
	}
	for index, value := range frame.Waveform {
		if value != 0 {
			t.Fatalf("waveform %d = %v, want 0", index, value)
		}
	}
}

func TestAnalyzeRMSRespondsToAmplitude(t *testing.T) {
	quiet := NewAnalyzer().Analyze(sineSamples(440, 3000, WindowSize))
	loud := NewAnalyzer().Analyze(sineSamples(440, 30000, WindowSize))

	if quiet.RMS <= 0 || loud.RMS <= quiet.RMS {
		t.Fatalf("RMS did not grow with amplitude: quiet=%v loud=%v", quiet.RMS, loud.RMS)
	}
	ratio := loud.RMS / quiet.RMS
	if ratio < 9 || ratio > 11 {
		t.Fatalf("RMS ratio = %.2f, want about 10", ratio)
	}
}

func TestAnalyzeWaveformStaysInBounds(t *testing.T) {
	frame := NewAnalyzer().Analyze(sineSamples(440, 32767, WindowSize))

	for index, value := range frame.Waveform {
		if value < -1 || value > 1 {
			t.Fatalf("waveform %d = %v, want -1..1", index, value)
		}
	}
}

func TestFFTPeakMatchesFrequency(t *testing.T) {
	const bin = 20
	frequency := float64(bin) * SampleRate / WindowSize

	real := make([]float64, WindowSize)
	imag := make([]float64, WindowSize)
	for index := range real {
		real[index] = math.Sin(2 * math.Pi * frequency * float64(index) / SampleRate)
	}
	fft(real, imag)

	peak := 1
	for index := 2; index <= WindowSize/2; index++ {
		if magnitude(real, imag, index) > magnitude(real, imag, peak) {
			peak = index
		}
	}
	if peak < bin-1 || peak > bin+1 {
		t.Fatalf("FFT peak bin = %d, want %d", peak, bin)
	}
}

func magnitude(real, imag []float64, index int) float64 {
	return real[index]*real[index] + imag[index]*imag[index]
}
