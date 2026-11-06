package soloist

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// testDH mirrors go-librespot's Diffie-Hellman keypair. It is the server half
// of the ZeroConf handshake in these tests.
type testDH struct {
	private *big.Int
	public  *big.Int
}

func newTestDH() (*testDH, error) {
	data := make([]byte, 95)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	private := new(big.Int).SetBytes(data)
	return &testDH{private: private, public: new(big.Int).Exp(zeroConfGenerator, private, zeroConfPrime)}, nil
}

func (d *testDH) publicBytes() []byte { return d.public.Bytes() }

func (d *testDH) exchange(remote []byte) []byte {
	key := new(big.Int).SetBytes(remote)
	return new(big.Int).Exp(key, d.private, zeroConfPrime).Bytes()
}

// testInnerDecrypt mirrors ap.ConnectBlob's parsing of the inner blob.
func testInnerDecrypt(deviceID, username string, encoded []byte) (uint64, []byte, error) {
	encrypted := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, err := base64.StdEncoding.Decode(encrypted, encoded)
	if err != nil {
		return 0, nil, err
	}
	encrypted = encrypted[:n]
	if len(encrypted)%aes.BlockSize != 0 || len(encrypted) == 0 {
		return 0, nil, fmt.Errorf("inner blob length %d not a block multiple", len(encrypted))
	}

	secret := sha1.Sum([]byte(deviceID))
	baseKey := pbkdf2SHA1(secret[:], []byte(username), 256, 20)
	sum := sha1.Sum(baseKey)
	key := make([]byte, 24)
	copy(key, sum[:])
	binary.BigEndian.PutUint32(key[20:], 20)
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, nil, err
	}
	plain := make([]byte, len(encrypted))
	for i := 0; i < len(encrypted); i += aes.BlockSize {
		block.Decrypt(plain[i:], encrypted[i:])
	}
	for i := 0; i < len(plain)-16; i++ {
		plain[len(plain)-i-1] ^= plain[len(plain)-i-17]
	}
	r := bytes.NewReader(plain)
	_, _ = r.Seek(1, 0)
	discardLen, _ := binary.ReadUvarint(r)
	_, _ = r.Seek(int64(discardLen), 1)
	_, _ = r.Seek(1, 1)
	authType, _ := binary.ReadUvarint(r)
	_, _ = r.Seek(1, 1)
	authLen, _ := binary.ReadUvarint(r)
	authData := make([]byte, authLen)
	_, _ = r.Read(authData)
	return authType, authData, nil
}

// testOuterDecrypt mirrors the ZeroConf server's handleAddUser decryption.
func testOuterDecrypt(server *testDH, clientPublic, blob []byte) ([]byte, error) {
	shared := server.exchange(clientPublic)
	baseSum := sha1.Sum(shared)
	baseKey := baseSum[:16]

	mac := hmac.New(sha1.New, baseKey)
	mac.Write([]byte("checksum"))
	checksumKey := mac.Sum(nil)

	mac.Reset()
	mac.Write([]byte("encryption"))
	encryptionKey := mac.Sum(nil)[:16]

	if len(blob) < 16+1+20 {
		return nil, errors.New("outer blob too short")
	}
	iv, encrypted, checksum := blob[:16], blob[16:len(blob)-20], blob[len(blob)-20:]
	mac = hmac.New(sha1.New, checksumKey)
	mac.Write(encrypted)
	if !hmac.Equal(mac.Sum(nil), checksum) {
		return nil, errors.New("checksum mismatch")
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(encrypted))
	cipher.NewCTR(block, iv).XORKeyStream(plain, encrypted)
	return plain, nil
}

type zeroConfTestServer struct {
	*httptest.Server
	dh          *testDH
	deviceID    string
	username    string
	credentials []byte

	mu           sync.Mutex
	infoCalls    int
	addUserCalls int

	allowPath       string
	infoStatus      int
	publicKey       string
	addUserHTTP     int
	addUserStatus   int
	addUserRedirect string
	corruptBlob     bool
	onAddUser       func()
}

func newZeroConfTestServer(t *testing.T, deviceID, username string, credentials []byte) *zeroConfTestServer {
	t.Helper()
	dh, err := newTestDH()
	if err != nil {
		t.Fatalf("new test DH: %v", err)
	}
	server := &zeroConfTestServer{
		dh:          dh,
		deviceID:    deviceID,
		username:    username,
		credentials: credentials,
	}
	server.Server = httptest.NewServer(server.handler())
	t.Cleanup(server.Close)
	return server
}

func (s *zeroConfTestServer) port() int {
	return s.Listener.Addr().(*net.TCPAddr).Port
}

func (s *zeroConfTestServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.allowPath != "" && r.URL.Path != s.allowPath {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		switch r.Form.Get("action") {
		case "getInfo":
			s.mu.Lock()
			s.infoCalls++
			infoStatus := s.infoStatus
			publicKey := s.publicKey
			s.mu.Unlock()
			if infoStatus != 0 {
				w.WriteHeader(infoStatus)
				return
			}
			if publicKey == "" {
				publicKey = base64.StdEncoding.EncodeToString(s.dh.publicBytes())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":     101,
				"deviceID":   s.deviceID,
				"publicKey":  publicKey,
				"remoteName": "Cadence",
			})
		case "addUser":
			s.mu.Lock()
			s.addUserCalls++
			addUserHTTP := s.addUserHTTP
			addUserStatus := s.addUserStatus
			addUserRedirect := s.addUserRedirect
			corruptBlob := s.corruptBlob
			onAddUser := s.onAddUser
			s.mu.Unlock()
			if addUserRedirect != "" {
				http.Redirect(w, r, addUserRedirect, http.StatusTemporaryRedirect)
				return
			}
			if addUserHTTP != 0 {
				w.WriteHeader(addUserHTTP)
				return
			}
			if err := s.verifyAddUser(r, corruptBlob); err != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if onAddUser != nil {
				onAddUser()
			}
			if addUserStatus == 0 {
				addUserStatus = 101
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": addUserStatus, "statusString": "OK"})
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *zeroConfTestServer) verifyAddUser(r *http.Request, corruptBlob bool) error {
	if r.Form.Get("userName") != s.username {
		return errors.New("username mismatch")
	}
	if r.Form.Get("deviceName") == "" {
		return errors.New("missing device name")
	}
	blob, err := base64.StdEncoding.DecodeString(r.Form.Get("blob"))
	if err != nil {
		return err
	}
	clientKey, err := base64.StdEncoding.DecodeString(r.Form.Get("clientKey"))
	if err != nil {
		return err
	}
	if corruptBlob && len(blob) > 0 {
		blob[len(blob)-1] ^= 0xff
	}
	inner, err := testOuterDecrypt(s.dh, clientKey, blob)
	if err != nil {
		return err
	}
	authType, data, err := testInnerDecrypt(s.deviceID, s.username, inner)
	if err != nil {
		return err
	}
	if authType != 1 {
		return fmt.Errorf("auth type = %d", authType)
	}
	if !bytes.Equal(data, s.credentials) {
		return errors.New("credentials mismatch")
	}
	return nil
}

func (s *zeroConfTestServer) calls() (info, addUser int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoCalls, s.addUserCalls
}

func TestZeroConfCryptoRoundTrip(t *testing.T) {
	server, err := newTestDH()
	if err != nil {
		t.Fatalf("server DH: %v", err)
	}
	const deviceID = "soloist-device-id-123"
	const username = "31rn6o3egrtllzi7a674vi5johni"
	credentials := make([]byte, 260)
	for index := range credentials {
		credentials[index] = byte(index)
	}

	inner, err := zeroConfInnerBlob(deviceID, username, authTypeStoredCredentials, credentials)
	if err != nil {
		t.Fatalf("inner blob: %v", err)
	}
	blob, clientPublic, err := zeroConfOuterBlob(server.publicBytes(), inner)
	if err != nil {
		t.Fatalf("outer blob: %v", err)
	}
	plain, err := testOuterDecrypt(server, clientPublic, blob)
	if err != nil {
		t.Fatalf("outer decrypt: %v", err)
	}
	authType, data, err := testInnerDecrypt(deviceID, username, plain)
	if err != nil {
		t.Fatalf("inner decrypt: %v", err)
	}
	if authType != 1 {
		t.Fatalf("auth type = %d, want 1", authType)
	}
	if !bytes.Equal(data, credentials) {
		t.Fatalf("credential round trip mismatch: got %d bytes", len(data))
	}
}

func TestPairWithCredentialsSucceeds(t *testing.T) {
	credentials := bytes.Repeat([]byte{0xab}, 260)
	server := newZeroConfTestServer(t, "device-123", "alice", credentials)

	if err := PairWithCredentials(t.Context(), server.port(), "/", "Cadence", "alice", credentials); err != nil {
		t.Fatalf("PairWithCredentials: %v", err)
	}
	infoCalls, addUserCalls := server.calls()
	if infoCalls != 1 {
		t.Fatalf("getInfo calls = %d, want 1", infoCalls)
	}
	if addUserCalls != 1 {
		t.Fatalf("addUser calls = %d, want 1", addUserCalls)
	}
}

func TestPairWithCredentialsProbesWhenPathEmpty(t *testing.T) {
	credentials := bytes.Repeat([]byte{0xcd}, 64)
	server := newZeroConfTestServer(t, "device-456", "bob", credentials)
	server.allowPath = "/zc"

	if err := PairWithCredentials(t.Context(), server.port(), "", "Cadence", "bob", credentials); err != nil {
		t.Fatalf("PairWithCredentials: %v", err)
	}
}

func TestProbeZeroConfPrefersZC(t *testing.T) {
	server := newZeroConfTestServer(t, "device-123", "alice", nil)
	server.allowPath = "/zc"

	path, err := ProbeZeroConf(t.Context(), server.port())
	if err != nil {
		t.Fatalf("ProbeZeroConf: %v", err)
	}
	if path != "/zc" {
		t.Fatalf("path = %q, want /zc", path)
	}
}

func TestProbeZeroConfFallsBackToRoot(t *testing.T) {
	server := newZeroConfTestServer(t, "device-123", "alice", nil)
	server.allowPath = "/"

	path, err := ProbeZeroConf(t.Context(), server.port())
	if err != nil {
		t.Fatalf("ProbeZeroConf: %v", err)
	}
	if path != "/" {
		t.Fatalf("path = %q, want /", path)
	}
}

func TestProbeZeroConfNoEndpoint(t *testing.T) {
	server := newZeroConfTestServer(t, "device-123", "alice", nil)
	server.allowPath = "/missing"

	if _, err := ProbeZeroConf(t.Context(), server.port()); err == nil {
		t.Fatal("expected an error when no ZeroConf path responds")
	}
}

func TestPairWithCredentialsInvalidPublicKey(t *testing.T) {
	credentials := []byte("secret-stored-credentials")
	server := newZeroConfTestServer(t, "device-123", "alice", credentials)
	server.publicKey = "%%%not-base64%%%"

	assertPairError(t, server, credentials, "public key")
}

func TestPairWithCredentialsWrongServerKey(t *testing.T) {
	credentials := []byte("secret-stored-credentials")
	server := newZeroConfTestServer(t, "device-123", "alice", credentials)
	other, err := newTestDH()
	if err != nil {
		t.Fatalf("other DH: %v", err)
	}
	server.publicKey = base64.StdEncoding.EncodeToString(other.publicBytes())

	assertPairError(t, server, credentials, "rejected")
}

func TestPairWithCredentialsBadChecksum(t *testing.T) {
	credentials := []byte("secret-stored-credentials")
	server := newZeroConfTestServer(t, "device-123", "alice", credentials)
	server.corruptBlob = true

	assertPairError(t, server, credentials, "rejected")
}

func TestPairWithCredentialsForbidden(t *testing.T) {
	credentials := []byte("secret-stored-credentials")
	server := newZeroConfTestServer(t, "device-123", "alice", credentials)
	server.addUserHTTP = http.StatusForbidden

	assertPairError(t, server, credentials, "HTTP 403")
}

func TestPairWithCredentialsStatusNot101(t *testing.T) {
	credentials := []byte("secret-stored-credentials")
	server := newZeroConfTestServer(t, "device-123", "alice", credentials)
	server.addUserStatus = 102

	assertPairError(t, server, credentials, "status 102")
}

func TestPairWithCredentialsRejectsIncomplete(t *testing.T) {
	if err := PairWithCredentials(t.Context(), 1, "/", "Cadence", "", []byte("x")); err == nil {
		t.Fatal("expected an error without a username")
	}
	if err := PairWithCredentials(t.Context(), 1, "/", "Cadence", "alice", nil); err == nil {
		t.Fatal("expected an error without credentials")
	}
}

func assertPairError(t *testing.T, server *zeroConfTestServer, credentials []byte, want string) {
	t.Helper()
	err := PairWithCredentials(t.Context(), server.port(), "/", "Cadence", server.username, credentials)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want containing %q", err, want)
	}
	if strings.Contains(err.Error(), string(credentials)) {
		t.Fatalf("error leaks the raw credentials: %v", err)
	}
	if strings.Contains(err.Error(), base64.StdEncoding.EncodeToString(credentials)) {
		t.Fatalf("error leaks the encoded credentials: %v", err)
	}
}

func TestPairWithCredentialsDoesNotFollowRedirects(t *testing.T) {
	credentials := []byte("secret-stored-credentials")
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)

	server := newZeroConfTestServer(t, "device-123", "alice", credentials)
	server.addUserRedirect = target.URL

	assertPairError(t, server, credentials, "HTTP 307")
	if hits.Load() != 0 {
		t.Fatal("the client followed a redirect and exposed the credentials")
	}
}
