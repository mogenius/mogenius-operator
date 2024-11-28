package utils_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"testing"
)

func TestUtils(t *testing.T) {
	t.Parallel()
	logManager := logging.NewMockSlogManager(t)
	configModule := config.NewConfig()
	utils.Setup(logManager, configModule)

	testSecret := "myLittleTestSecret ;-)"
	testEncryptionKey := "testEncryptionKey"
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_API_KEY",
		DefaultValue: &testEncryptionKey,
	})

	// Encrypted string "myLittleTestSecret ;-)" using key "testEncryptionKey"
	encryptedTestStringOk := "mj7mD+eJA+MNKU9ftuPdpmxNH0V+8c9Gwk7HXuRhSZZapEsSZyIw+Wz/qitFPzLG"
	// Encrypted string modified and by that invalid
	encryptedTestStringMessedUp := "mj7mD+eJA+MNKU9ftuPdpmxNH0V+8c9GwkXXXXXXXX7HXuRhSZZapEsSZyIw+Wz/qitFPzLG"

	// ENCRYPT
	encryptedStr, err := utils.EncryptString(testEncryptionKey, testSecret)
	assert.AssertT(t, err == nil, "error should be nil", err)
	assert.AssertT(t, encryptedStr != "", "encryptedStr should not be empty")
	t.Log("Encrypted string: ", encryptedStr)

	// DECRYPT
	decryptedStr, err := utils.DecryptString(testEncryptionKey, encryptedStr)
	assert.AssertT(t, err == nil, "error should be nil", err)
	assert.AssertT(t, decryptedStr == testSecret, "decryptedStr should be equal to testSecret", decryptedStr, testSecret)
	t.Log("Decrypted string: ", decryptedStr)

	// IS ENCRYPTED Valid
	isEncrypted := utils.IsEncrypted(encryptedTestStringOk)
	assert.AssertT(t, isEncrypted, "test for encryption in encryptedTestStringOk should be ok")
	t.Logf("String %s is encrypted\n", encryptedTestStringOk)

	// IS ENCRYPTED Invalid
	isEncrypted = utils.IsEncrypted(encryptedTestStringMessedUp)
	assert.AssertT(t, !isEncrypted, "test for encryption in encryptedTestStringMessedUp should fail")
	t.Logf("String %s is not encrypted, which is correct\n", encryptedTestStringMessedUp)
}
