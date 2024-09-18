package utils

import (
	"fmt"
	"testing"
)

func TestUtilsUtils(t *testing.T) {
	testSecret := "myLittleTestSecret ;-)"
	testEncryptionKey := "testEncryptionKey"
	encryptedTestStringOk := "mj7mD+eJA+MNKU9ftuPdpmxNH0V+8c9Gwk7HXuRhSZZapEsSZyIw+Wz/qitFPzLG"               // Encrypted string "myLittleTestSecret ;-)" using key "testEncryptionKey"
	encryptedTestStringMessedUp := "mj7mD+eJA+MNKU9ftuPdpmxNH0V+8c9GwkXXXXXXXX7HXuRhSZZapEsSZyIw+Wz/qitFPzLG" // Encrypted string modified and by that invalid
	CONFIG.Kubernetes.ApiKey = testEncryptionKey

	// ENCRYPT
	encryptedStr, err := EncryptString(testEncryptionKey, testSecret)
	if err != nil {
		t.Errorf("Error encrypting string")
	} else {
		if encryptedStr == "" {
			t.Errorf("Error encrypting string")
		} else {
			fmt.Println("Encrypted string: ", encryptedStr)
		}
	}

	// DECRYPT
	decryptedStr, err := DecryptString(testEncryptionKey, encryptedStr)
	if err != nil {
		t.Errorf("Error decrypting string")
	} else {
		if decryptedStr != testSecret || decryptedStr == "" {
			t.Errorf("Error decrypting string")
		} else {
			fmt.Println("Decrypted string: ", decryptedStr)
		}
	}

	// IS ENCRYPTED Valid
	isEncrypted := IsEncrypted(encryptedTestStringOk)
	if !isEncrypted {
		t.Errorf("Error checking if string is encrypted, which is correct")
	} else {
		fmt.Printf("String %s is encrypted\n", encryptedTestStringOk)
	}

	// IS ENCRYPTED Invalid
	isEncrypted = IsEncrypted(encryptedTestStringMessedUp)
	if isEncrypted {
		t.Errorf("Error checking if string is encrypted")
	} else {
		fmt.Printf("String %s is not encrypted, which is correct\n", encryptedTestStringMessedUp)
	}

}
