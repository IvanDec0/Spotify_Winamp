package soloist

import "time"

// State is the internal process and protocol state of the Soloist backend.
// The audio adapter maps it to the frontend DTO; it never carries the API key.
type State struct {
	Installed    bool
	BinaryPath   string
	Version      string
	BuiltAt      time.Time
	ExpiresAt    time.Time
	Expired      bool
	DaysLeft     int
	Running      bool
	LoggedIn     bool
	NeedsPairing bool
	Active       bool
	DeviceName   string
	Accounts     []string
	Error        string
	ErrorCode    string
}
