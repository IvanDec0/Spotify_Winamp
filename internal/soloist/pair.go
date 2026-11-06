package soloist

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// zeroConfHTTPTimeout bounds one ZeroConf HTTP request.
	zeroConfHTTPTimeout = 5 * time.Second
	// authTypeStoredCredentials is
	// AUTHENTICATION_STORED_SPOTIFY_CREDENTIALS from Spotify's protocol.
	authTypeStoredCredentials = 1
)

var (
	zeroConfGenerator = big.NewInt(2)
	// zeroConfPrime is the Diffie-Hellman group go-librespot's dh package and
	// the ZeroConf server use.
	zeroConfPrime = new(big.Int).SetBytes([]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xc9, 0x0f, 0xda, 0xa2, 0x21, 0x68, 0xc2, 0x34, 0xc4, 0xc6,
		0x62, 0x8b, 0x80, 0xdc, 0x1c, 0xd1, 0x29, 0x02, 0x4e, 0x08, 0x8a, 0x67, 0xcc, 0x74, 0x02, 0x0b, 0xbe, 0xa6,
		0x3b, 0x13, 0x9b, 0x22, 0x51, 0x4a, 0x08, 0x79, 0x8e, 0x34, 0x04, 0xdd, 0xef, 0x95, 0x19, 0xb3, 0xcd, 0x3a,
		0x43, 0x1b, 0x30, 0x2b, 0x0a, 0x6d, 0xf2, 0x5f, 0x14, 0x37, 0x4f, 0xe1, 0x35, 0x6d, 0x6d, 0x51, 0xc2, 0x45,
		0xe4, 0x85, 0xb5, 0x76, 0x62, 0x5e, 0x7e, 0xc6, 0xf4, 0x4c, 0x42, 0xe9, 0xa6, 0x3a, 0x36, 0x20, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	})
)

// zeroConfHTTPClient never uses a proxy and never follows a redirect: the
// reusable credentials must reach 127.0.0.1 and nowhere else.
var zeroConfHTTPClient = &http.Client{
	Timeout:   zeroConfHTTPTimeout,
	Transport: &http.Transport{Proxy: nil},
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// zeroConfDH is one ephemeral Diffie-Hellman keypair.
type zeroConfDH struct {
	private *big.Int
	public  *big.Int
}

func newZeroConfDH() (*zeroConfDH, error) {
	data := make([]byte, 95)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	private := new(big.Int).SetBytes(data)
	return &zeroConfDH{
		private: private,
		public:  new(big.Int).Exp(zeroConfGenerator, private, zeroConfPrime),
	}, nil
}

func (d *zeroConfDH) publicBytes() []byte { return d.public.Bytes() }

func (d *zeroConfDH) exchange(remote []byte) []byte {
	key := new(big.Int).SetBytes(remote)
	return new(big.Int).Exp(key, d.private, zeroConfPrime).Bytes()
}

// PairWithCredentials sends the account's reusable Spotify credentials to
// Soloist's loopback ZeroConf endpoint.
func PairWithCredentials(ctx context.Context, port int, path, deviceName, username string, credentials []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if port <= 0 || port > 65535 {
		return codedError(CodeProtocol, "the Spotify Soloist ZeroConf port is invalid", nil)
	}
	if strings.TrimSpace(username) == "" || len(credentials) == 0 {
		return codedError(CodeProtocol, "the Spotify library credentials are incomplete", nil)
	}
	if strings.TrimSpace(deviceName) == "" {
		return codedError(CodeProtocol, "the Spotify Soloist device name is missing", nil)
	}
	if path == "" {
		resolved, err := ProbeZeroConf(ctx, port)
		if err != nil {
			return err
		}
		path = resolved
	}

	deviceID, serverPublicKey, err := zeroConfGetInfo(ctx, port, path)
	if err != nil {
		return err
	}

	inner, err := zeroConfInnerBlob(deviceID, username, authTypeStoredCredentials, credentials)
	if err != nil {
		return codedError(CodeProtocol, "cannot build the Spotify Soloist pairing request", nil)
	}
	blob, clientPublic, err := zeroConfOuterBlob(serverPublicKey, inner)
	if err != nil {
		return codedError(CodeProtocol, "cannot build the Spotify Soloist pairing request", nil)
	}

	form := url.Values{}
	form.Set("action", "addUser")
	form.Set("userName", username)
	form.Set("blob", base64.StdEncoding.EncodeToString(blob))
	form.Set("clientKey", base64.StdEncoding.EncodeToString(clientPublic))
	form.Set("deviceName", deviceName)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, zeroConfURL(port, path), strings.NewReader(form.Encode()))
	if err != nil {
		return codedError(CodeProtocol, "cannot build the Spotify Soloist pairing request", nil)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := zeroConfHTTPClient.Do(request)
	if err != nil {
		return codedError(CodeProtocol, "cannot reach the Spotify Soloist pairing endpoint", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))

	if response.StatusCode != http.StatusOK {
		return codedError(CodeProtocol, fmt.Sprintf("Spotify Soloist rejected the pairing request (HTTP %d)", response.StatusCode), nil)
	}
	var result struct {
		Status int `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return codedError(CodeProtocol, "Spotify Soloist returned an invalid pairing response", nil)
	}
	if result.Status != 101 {
		return codedError(CodeProtocol, fmt.Sprintf("Spotify Soloist rejected the pairing request (status %d)", result.Status), nil)
	}
	return nil
}

// ProbeZeroConf returns the first working path (/zc, then /) for the local
// ZeroConf endpoint.
func ProbeZeroConf(ctx context.Context, port int) (path string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for _, candidate := range []string{"/zc", "/"} {
		if err := ctx.Err(); err != nil {
			return "", codedError(CodeProtocol, "the Spotify Soloist ZeroConf probe was cancelled", err)
		}
		if _, _, err := zeroConfGetInfo(ctx, port, candidate); err == nil {
			return candidate, nil
		} else {
			lastErr = err
		}
	}
	return "", codedError(CodeProtocol, "no Spotify Soloist ZeroConf endpoint responded", lastErr)
}

func zeroConfURL(port int, path string) string {
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
}

// zeroConfGetInfo reads the device ID and public key of the local ZeroConf
// endpoint.
func zeroConfGetInfo(ctx context.Context, port int, path string) (deviceID string, publicKey []byte, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, zeroConfURL(port, path)+"?action=getInfo", nil)
	if err != nil {
		return "", nil, codedError(CodeProtocol, "cannot build the Spotify Soloist ZeroConf request", nil)
	}
	response, err := zeroConfHTTPClient.Do(request)
	if err != nil {
		return "", nil, codedError(CodeProtocol, "cannot reach the Spotify Soloist ZeroConf endpoint", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		return "", nil, codedError(CodeProtocol, fmt.Sprintf("Spotify Soloist rejected the ZeroConf request (HTTP %d)", response.StatusCode), nil)
	}
	var info struct {
		Status     int    `json:"status"`
		DeviceID   string `json:"deviceID"`
		PublicKey  string `json:"publicKey"`
		RemoteName string `json:"remoteName"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", nil, codedError(CodeProtocol, "Spotify Soloist returned an invalid ZeroConf response", nil)
	}
	if strings.TrimSpace(info.DeviceID) == "" || strings.TrimSpace(info.PublicKey) == "" {
		return "", nil, codedError(CodeProtocol, "Spotify Soloist did not report its ZeroConf identity", nil)
	}
	publicKey, err = base64.StdEncoding.DecodeString(info.PublicKey)
	if err != nil || len(publicKey) == 0 {
		return "", nil, codedError(CodeProtocol, "Spotify Soloist reported an invalid ZeroConf public key", nil)
	}
	return info.DeviceID, publicKey, nil
}

// zeroConfInnerBlob builds the base64 inner blob that ap.ConnectBlob parses.
func zeroConfInnerBlob(deviceID, username string, authType uint64, authData []byte) ([]byte, error) {
	// Plain structure the server parses:
	// [discard][uvarint discardLen][discard][uvarint authType][discard][uvarint len][authData][padding]
	plain := []byte{0}
	plain = append(plain, 0) // discardLen = 0
	plain = append(plain, 0)
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], authType)
	plain = append(plain, buf[:n]...)
	plain = append(plain, 0)
	n = binary.PutUvarint(buf[:], uint64(len(authData)))
	plain = append(plain, buf[:n]...)
	plain = append(plain, authData...)
	for len(plain)%aes.BlockSize != 0 {
		plain = append(plain, 0)
	}

	// Inverse of the server's tail unscramble: Q[j] = P[j] ^ P[j-16].
	pre := make([]byte, len(plain))
	copy(pre[:aes.BlockSize], plain[:aes.BlockSize])
	for j := aes.BlockSize; j < len(pre); j++ {
		pre[j] = plain[j] ^ pre[j-aes.BlockSize]
	}

	key := zeroConfInnerKey(deviceID, username)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	encrypted := make([]byte, len(pre))
	for i := 0; i < len(pre); i += aes.BlockSize {
		block.Encrypt(encrypted[i:], pre[i:])
	}
	out := make([]byte, base64.StdEncoding.EncodedLen(len(encrypted)))
	base64.StdEncoding.Encode(out, encrypted)
	return out, nil
}

// zeroConfInnerKey derives the 24-byte AES-192 key of the inner blob.
func zeroConfInnerKey(deviceID, username string) []byte {
	secret := sha1.Sum([]byte(deviceID))
	baseKey := pbkdf2SHA1(secret[:], []byte(username), 256, 20)
	sum := sha1.Sum(baseKey)
	key := make([]byte, 24)
	copy(key, sum[:])
	binary.BigEndian.PutUint32(key[20:], 20)
	return key
}

// zeroConfOuterBlob builds the ZeroConf addUser blob for the server public
// key and returns it with the client public key.
func zeroConfOuterBlob(serverPublic, payload []byte) (blob, clientPublic []byte, err error) {
	client, err := newZeroConfDH()
	if err != nil {
		return nil, nil, err
	}
	shared := client.exchange(serverPublic)
	baseSum := sha1.Sum(shared)
	baseKey := baseSum[:16]

	mac := hmac.New(sha1.New, baseKey)
	mac.Write([]byte("checksum"))
	checksumKey := mac.Sum(nil)

	mac.Reset()
	mac.Write([]byte("encryption"))
	encryptionKey := mac.Sum(nil)[:16]

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, nil, err
	}
	encrypted := make([]byte, len(payload))
	cipher.NewCTR(block, iv).XORKeyStream(encrypted, payload)

	mac = hmac.New(sha1.New, checksumKey)
	mac.Write(encrypted)
	checksum := mac.Sum(nil)

	blob = append(append(append([]byte{}, iv...), encrypted...), checksum...)
	return blob, client.publicBytes(), nil
}

// pbkdf2SHA1 derives a key with PBKDF2-HMAC-SHA1. It is local so the module
// needs no new dependency.
func pbkdf2SHA1(password, salt []byte, iterations, keyLen int) []byte {
	const hashLen = sha1.Size
	blocks := (keyLen + hashLen - 1) / hashLen
	derived := make([]byte, 0, blocks*hashLen)
	var counter [4]byte
	for block := 1; block <= blocks; block++ {
		binary.BigEndian.PutUint32(counter[:], uint32(block))
		mac := hmac.New(sha1.New, password)
		mac.Write(salt)
		mac.Write(counter[:])
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for iteration := 1; iteration < iterations; iteration++ {
			mac = hmac.New(sha1.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for index := range t {
				t[index] ^= u[index]
			}
		}
		derived = append(derived, t...)
	}
	return derived[:keyLen]
}
