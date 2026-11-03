// The simulator implements only the synthetic KLAP exchanges used by the e2e test.
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
)

const email = "test@example.invalid"
const password = "test-password"

type simulator struct {
	mu            sync.Mutex
	local, remote []byte
}

func (s *simulator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.URL.Path {
	case "/app/handshake1":
		local, err := io.ReadAll(io.LimitReader(r.Body, 1024))
		if err != nil || len(local) != 16 {
			http.Error(w, "invalid handshake", 400)
			return
		}
		s.local = local
		s.remote = bytes.Repeat([]byte{42}, 16)
		_, _ = w.Write(s.remote)
	case "/app/handshake2":
		if len(s.local) != 16 {
			http.Error(w, "missing handshake", 400)
			return
		}
		w.WriteHeader(http.StatusOK)
	case "/app/request":
		if len(s.local) != 16 {
			http.Error(w, "missing handshake", 400)
			return
		}
		seq, err := strconv.ParseInt(r.URL.Query().Get("seq"), 10, 32)
		if err != nil {
			http.Error(w, "invalid sequence", 400)
			return
		}
		emailHash, passwordHash := sha1.Sum([]byte(email)), sha1.Sum([]byte(password))
		auth := sha256.Sum256(append(emailHash[:], passwordHash[:]...))
		seed := append(append(append([]byte{}, s.local...), s.remote...), auth[:]...)
		key := sha256.Sum256(append([]byte("lsk"), seed...))
		ivHash := sha256.Sum256(append([]byte("iv"), seed...))
		iv := make([]byte, 16)
		copy(iv, ivHash[:12])
		binary.BigEndian.PutUint32(iv[12:], uint32(int32(seq)))
		block, err := aes.NewCipher(key[:16])
		if err != nil {
			http.Error(w, "cipher error", 500)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
		if err != nil || len(body) < 48 || (len(body)-32)%16 != 0 {
			http.Error(w, "invalid request", 400)
			return
		}
		plain := make([]byte, len(body)-32)
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, body[32:])
		padding := int(plain[len(plain)-1])
		if padding < 1 || padding > 16 {
			http.Error(w, "invalid padding", 400)
			return
		}
		plain = plain[:len(plain)-padding]
		var request struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(plain, &request) != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		var response []byte
		switch request.Method {
		case "get_device_info":
			response = []byte(`{"error_code":0,"result":{"model":"P110","fw_ver":"test"}}`)
		case "get_energy_usage":
			response = []byte(`{"error_code":0,"result":{"current_power":12500,"today_energy":1000,"month_energy":2500}}`)
		default:
			response = []byte(`{"error_code":-1}`)
		}
		pad := aes.BlockSize - len(response)%aes.BlockSize
		response = append(response, bytes.Repeat([]byte{byte(pad)}, pad)...)
		encrypted := make([]byte, len(response))
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, response)
		_, _ = w.Write(append(make([]byte, 32), encrypted...))
	default:
		http.NotFound(w, r)
	}
}
func main() {
	if err := http.ListenAndServe(":8080", &simulator{}); err != nil {
		panic(err)
	}
}
