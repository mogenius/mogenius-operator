package utils_test

import (
	"fmt"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestUtilsutils.Pointer(t *testing.T) {
	t.Parallel()
	logManager := interfaces.NewMockSlogManager(t)
	configModule := config.NewConfig()
	utils.Setup(logManager, configModule)

	testSecret := "myLittleTestSecret ;-)"
	testEncryptionKey := "testEncryptionKey"
	configModule.Declare(interfaces.ConfigDeclaration{
		Key:          "MO_API_KEY",
		DefaultValue: &testEncryptionKey,
	})

	// Encrypted string "myLittleTestSecret ;-)" using key "testEncryptionKey"
	encryptedTestStringOk := "mj7mD+eJA+MNKU9ftuPdpmxNH0V+8c9Gwk7HXuRhSZZapEsSZyIw+Wz/qitFPzLG"
	// Encrypted string modified and by that invalid
	encryptedTestStringMessedUp := "mj7mD+eJA+MNKU9ftuPdpmxNH0V+8c9GwkXXXXXXXX7HXuRhSZZapEsSZyIw+Wz/qitFPzLG"

	// ENCRYPT
	encryptedStr, err := utils.EncryptString(testEncryptionKey, testSecret)
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
	decryptedStr, err := utils.DecryptString(testEncryptionKey, encryptedStr)
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
	isEncrypted := utils.IsEncrypted(encryptedTestStringOk)
	if !isEncrypted {
		t.Errorf("Error checking if string is encrypted, which is correct")
	} else {
		fmt.Printf("String %s is encrypted\n", encryptedTestStringOk)
	}

	// IS ENCRYPTED Invalid
	isEncrypted = utils.IsEncrypted(encryptedTestStringMessedUp)
	if isEncrypted {
		t.Errorf("Error checking if string is encrypted")
	} else {
		fmt.Printf("String %s is not encrypted, which is correct\n", encryptedTestStringMessedUp)
	}

}
