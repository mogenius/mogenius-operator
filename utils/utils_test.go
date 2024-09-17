package utils

import (
	"fmt"
	"testing"
)

func TestUtilsUtils(t *testing.T) {
	testSecret := "myLittleTestSecret ;-)"
	testEncryptionKey := "testEncryptionKey"
	fmt.Println("Secret: " + testSecret)

	encryptedStr, err := EncryptString(testEncryptionKey, testSecret)
	if err != nil {
		t.Errorf("Error encrypting string")
	}
	fmt.Println("Encrypted: " + encryptedStr)
	if encryptedStr == "" {
		t.Errorf("Error encrypting string")
	}

	decryptedStr, err := DecryptString(testEncryptionKey, encryptedStr)
	if err != nil {
		t.Errorf("Error decrypting string")
	}
	fmt.Println("Decrypted: " + decryptedStr)
	if decryptedStr != testSecret || decryptedStr == "" {
		t.Errorf("Error decrypting string")
	}
}
