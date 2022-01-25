package event

import (
	"fmt"
	"os"
	"testing"

	"github.com/LianjiaTech/lightning/common"
)

func TestParseKeyRing(t *testing.T) {
	keys, err := parseKeyRing(common.DevPath + "/test/keyring")
	if err != nil {
		t.Error(err.Error())
	}
	for _, key := range keys {
		fmt.Println(string(key.KeyID), string(key.UserID))
	}
}

func TestDecryptBinlog(t *testing.T) {
	// stdout redirect
	std := os.Stdout
	fd, err := os.Create(common.DevPath + "/test/binlog.decrypted")
	if err != nil {
		t.Error(err.Error())
	}
	os.Stdout = fd

	err = DecryptBinlog(common.DevPath+"/test/binlog.encrypted", common.DevPath+"/test/keyring")
	if err != nil {
		t.Error(err.Error())
	}
	os.Stdout = std
}
