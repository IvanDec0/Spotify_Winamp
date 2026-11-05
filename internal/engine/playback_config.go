package engine

// PlaybackConfig is the engine playback and cache configuration that can
// change without replacing the binary, config directory or log path.
type PlaybackConfig struct {
	Bitrate                   int
	CrossfadeDurationMS       int
	NormalisationDisabled     bool
	NormalisationUseAlbumGain bool
	NormalisationPregain      float32
	CacheEnabled              bool
	CacheDir                  string
	CacheSizeLimit            string
	FlacEnabled               bool
	MPRISEnabled              bool
	DeviceIsGroup             bool
	ImageSize                 string
}

// cacheSizeLimits are the size limits the engine accepts. "0" means unlimited;
// an empty value lets the engine use its own default.
var cacheSizeLimits = map[string]bool{
	"":      true,
	"0":     true,
	"512MB": true,
	"1GB":   true,
	"2GB":   true,
	"4GB":   true,
	"8GB":   true,
}

// NormalizePlaybackConfig replaces out-of-range values with the defaults.
func NormalizePlaybackConfig(config PlaybackConfig) PlaybackConfig {
	switch config.Bitrate {
	case 96, 160, 320:
	default:
		config.Bitrate = 320
	}
	if config.CrossfadeDurationMS < 0 || config.CrossfadeDurationMS > 12000 {
		config.CrossfadeDurationMS = 0
	}
	if config.NormalisationPregain < -20 || config.NormalisationPregain > 20 {
		config.NormalisationPregain = 0
	}
	config.ImageSize = normalizeImageSize(config.ImageSize)
	if !cacheSizeLimits[config.CacheSizeLimit] {
		config.CacheSizeLimit = "1GB"
	}
	return config
}

// ApplyPlaybackConfig normalizes and stores playback options for the next
// engine start. It never restarts the running engine or touches its binary,
// config directory, log path or client.
func (r *Supervisor) ApplyPlaybackConfig(config PlaybackConfig) {
	config = NormalizePlaybackConfig(config)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bitrate = config.Bitrate
	r.crossfadeDurationMS = config.CrossfadeDurationMS
	r.normalisationDisabled = config.NormalisationDisabled
	r.normalisationUseAlbumGain = config.NormalisationUseAlbumGain
	r.normalisationPregain = config.NormalisationPregain
	r.cacheEnabled = config.CacheEnabled
	r.cacheDir = config.CacheDir
	r.cacheSizeLimit = config.CacheSizeLimit
	r.flacEnabled = config.FlacEnabled
	r.mprisEnabled = config.MPRISEnabled
	r.deviceIsGroup = config.DeviceIsGroup
	r.imageSize = config.ImageSize
}
