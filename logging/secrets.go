package logging

// @bene
// Pointers are only useful here if the unterlying strings change.
// If not, it would be better to use a `[]string`.
// Also, if they aren't changing pointers anymore the secrets can be stored directly as bytes too.
// This would **significantly** optimize the logger.
var secrets []*string

const REDACTED = "***[REDACTED]***"

func AddSecret(secret *string) {
	secrets = append(secrets, secret)
}

func SecretArray() []string {
	var data []string
	for _, secret := range secrets {
		if secret != nil {
			data = append(data, *secret)
		}
	}
	return data
}

func SecretBytesArray() [][]byte {
	var bytes [][]byte
	for _, secret := range secrets {
		if secret != nil {
			bytes = append(bytes, []byte(*secret))
		}
	}
	return bytes
}
