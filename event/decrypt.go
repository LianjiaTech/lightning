package event

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// Reference:
// https://dev.mysql.com/blog-archive/how-to-manually-decrypt-an-encrypted-binary-log-file/
// https://mysql.wisborg.dk/2019/01/28/automatic-decryption-of-mysql-binary-logs-using-python/

// KeyRingXORStr mysql-server: plugin/keyring/common/keyring_key.cc obfuscate_str
const KeyRingXORStr = `*305=Ljt0*!@$Hnm(*-9-w;:`

// EncryptFileHeaderOffset encrypt file header size
const EncryptFileHeaderOffset = 512

type EncryptHeader struct {
	Version   int
	KeyRingID []byte // keyring id
	Key       []byte // key raw bytes
	IV        []byte // 16 bytes
	Password  []byte // 32 bytes
}

/*
KeyRing ID Format:
	|<-----Keyword------|<---------------UUID--------------->|SEQ_NO|
	Keyring key ID for 'binlog.000003' is 'MySQLReplicationKey_d1deace2-24cc-11ea-a1db-0800270a0142_2'
*/

type KeyRing struct {
	Type   []byte
	KeyID  []byte
	UserID []byte
	Key    []byte
}

// parseEncryptHeader parse encrypt binlog header
func parseEncryptHeader(binlog string, keys []KeyRing) (enc EncryptHeader, err error) {
	fd, err := os.Open(binlog)
	if err != nil {
		return enc, err
	}
	defer fd.Close()

	// Check is encrypted binlog file
	bufFileHeader := make([]byte, FileHeaderLength)
	if _, err := io.ReadFull(fd, bufFileHeader); err != nil {
		return enc, err
	}
	if !CheckBinlogFileEncrypt(bufFileHeader) {
		return enc, fmt.Errorf("file not encrypted")
	}

	oneByte := make([]byte, 1)
	// Get the encrypted version
	if _, err = io.ReadFull(fd, oneByte); err != nil {
		return enc, err
	}
	enc.Version = int(oneByte[0])
	if enc.Version != 1 {
		return enc, fmt.Errorf("wrong version: %d", enc.Version)
	}

	// First header field is a TLV: the keyring key ID
	if _, err = io.ReadFull(fd, oneByte); err != nil {
		return enc, err
	}
	fieldType := int(oneByte[0])
	if fieldType != 1 { // First header
		return enc, fmt.Errorf("wrong field type: %d", fieldType)
	}
	if _, err = io.ReadFull(fd, oneByte); err != nil {
		return enc, err
	}
	id := make([]byte, int(oneByte[0]))
	if _, err = io.ReadFull(fd, id); err != nil {
		return enc, err
	}
	enc.KeyRingID = id

	// get tmp key from keyring
	tk := make([]byte, 32)
	for _, key := range keys {
		if bytes.EqualFold(key.KeyID, enc.KeyRingID) {
			tk = key.Key
			break
		}
	}

	// Second header is a TV: the encrypted file password
	if _, err = io.ReadFull(fd, oneByte); err != nil {
		return enc, err
	}
	fieldType = int(oneByte[0])
	if fieldType != 2 { // Second header
		return enc, fmt.Errorf("wrong field type: %d", fieldType)
	}
	pass := make([]byte, 32)
	if _, err = io.ReadFull(fd, pass); err != nil {
		return enc, err
	}
	enc.Password = pass

	// Third header field is a TV: the IV to decrypt the file password
	if _, err = io.ReadFull(fd, oneByte); err != nil {
		return enc, err
	}
	fieldType = int(oneByte[0])
	if fieldType != 3 { // Third header
		return enc, fmt.Errorf("wrong field type: %d", fieldType)
	}
	tiv := make([]byte, 16) // tmp iv
	if _, err = io.ReadFull(fd, tiv); err != nil {
		return enc, err
	}

	// generate key & iv
	enc.Password, err = decryptAESCBC(pass, tk, tiv)
	if err != nil {
		return enc, err
	}

	keyAndIV := sha512.Sum512(enc.Password)
	if len(keyAndIV) < 48 {
		return enc, fmt.Errorf("generate key & iv failed")
	}
	enc.Key = keyAndIV[:32]
	enc.IV = keyAndIV[32:48]
	for i := 8; i < len(enc.IV); i++ {
		enc.IV[i] = 0
	}

	return enc, err
}

func decryptAESCBC(src, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	mode := cipher.NewCBCDecrypter(block, iv)
	dst := make([]byte, len(src))
	mode.CryptBlocks(dst, src)
	return dst, err
}

// parseKeyRing parse keyring file
func parseKeyRing(keyring string) ([]KeyRing, error) {

	var keys []KeyRing
	fd, err := os.Open(keyring)
	if err != nil {
		return keys, err
	}
	defer fd.Close()

	body, err := ioutil.ReadAll(fd)
	if err != nil {
		return keys, err
	}

	// Verify the start of the file is "Keyring file version:"
	if string(body[:21]) != `Keyring file version:` {
		return keys, fmt.Errorf("wrong file format, not keyring")
	}

	// Get the keyring version - currently only 2.0 is supported
	if string(body[21:24]) != "2.0" {
		return keys, fmt.Errorf("wrong version of keyring")
	}

	// get keys from keyring
	for offset := 24; offset < len(body); {
		if string(body[offset:offset+3]) == "EOF" {
			break
		}

		k, l := getKeyFromKeyring(body[offset:])
		keys = append(keys, k)
		offset += l
	}

	return keys, err
}

func getKeyFromKeyring(data []byte) (KeyRing, int) {
	var key KeyRing
	totalLength := int(binary.LittleEndian.Uint32(data[:8]))
	keyIDLength := data[8:16]
	keyTypeLength := data[16:24]
	userIDLength := data[24:32]
	keyLength := data[32:40]

	keyIDStart := 40
	keyTypeStart := keyIDStart + int(binary.LittleEndian.Uint32(keyIDLength))
	userIDStart := keyTypeStart + int(binary.LittleEndian.Uint32(keyTypeLength))
	keyStart := userIDStart + int(binary.LittleEndian.Uint32(userIDLength))
	keyEnd := keyStart + int(binary.LittleEndian.Uint32(keyLength))

	key.KeyID = data[keyIDStart:keyTypeStart]
	key.Type = data[keyTypeStart:userIDStart]
	// User ID may be blank in which case the length is zero
	key.UserID = data[userIDStart:keyStart]
	key.Key = data[keyStart:keyEnd]

	// XOR_STR
	for i, v := range key.Key {
		key.Key[i] = v ^ KeyRingXORStr[i%len(KeyRingXORStr)]
	}
	return key, totalLength
}

func initAESCTRStream(binlog, keyring string) (cipher.Stream, error) {
	var stream cipher.Stream
	keys, err := parseKeyRing(keyring)
	if err != nil {
		return stream, err
	}

	header, err := parseEncryptHeader(binlog, keys)
	if err != nil {
		return stream, err
	}

	block, err := aes.NewCipher(header.Key)
	if err != nil {
		return stream, err
	}
	stream = cipher.NewCTR(block, header.IV)
	return stream, err
}

// decryptAESCTR decrypt single encrypted block
func decryptAESCTR(stream cipher.Stream, src []byte) []byte {
	dst := make([]byte, len(src))
	stream.XORKeyStream(dst, src)
	return dst
}

// DecryptBinlog decrypt binlog file with keyring
func DecryptBinlog(orgBinlog, keyring string) error {
	stream, err := initAESCTRStream(orgBinlog, keyring)
	if err != nil {
		return err
	}

	ofd, err := os.Open(orgBinlog)
	if err != nil {
		return err
	}
	defer ofd.Close()
	r := bufio.NewReader(ofd)
	w := bufio.NewWriter(os.Stdout)

	var offset int64
	for {
		data := make([]byte, 32) // BlockSize 32
		n, err := r.Read(data)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// The encrypted binary log headers are 512, so skip those
		offset += int64(n)
		if offset <= EncryptFileHeaderOffset {
			continue
		}
		w.Write(decryptAESCTR(stream, data[:n]))
	}
	return w.Flush()
}
