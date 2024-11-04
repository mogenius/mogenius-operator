package utils

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

func PrintLogo() {
	fmt.Printf(
		"███╗░░░███╗░█████╗░░██████╗░███████╗███╗░░██╗██╗██╗░░░██╗░██████╗\n" +
			"████╗░████║██╔══██╗██╔════╝░██╔════╝████╗░██║██║██║░░░██║██╔════╝\n" +
			"██╔████╔██║██║░░██║██║░░██╗░█████╗░░██╔██╗██║██║██║░░░██║╚█████╗░\n" +
			"██║╚██╔╝██║██║░░██║██║░░╚██╗██╔══╝░░██║╚████║██║██║░░░██║░╚═══██╗\n" +
			"██║░╚═╝░██║╚█████╔╝╚██████╔╝███████╗██║░╚███║██║╚██████╔╝██████╔╝\n" +
			"╚═╝░░░░░╚═╝░╚════╝░░╚═════╝░╚══════╝╚═╝░░╚══╝╚═╝░╚═════╝░╚═════╝░\n\n",
	)
}

type ValidationError struct {
	Errors []string `json:"errors"`
}

func createEmptyValidationErr() *ValidationError {
	return &ValidationError{
		Errors: []string{},
	}
}

func ValidateJSON(obj interface{}) *ValidationError {
	result := createEmptyValidationErr()
	err := validate.Struct(obj)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldError := range validationErrors {
				errorMessage := fmt.Sprintf("Field '%s' failed validation, Condition: %s", fieldError.Field(), fieldError.Tag())
				result.Errors = append(result.Errors, errorMessage)
			}
		}
		utilsLogger.Error("struct validation failed", "result", result)
		return result
	}
	return nil
}

func FormatJsonTimePretty(jsonTimestamp string) string {
	t, err := time.Parse(time.RFC3339, jsonTimestamp)
	if err != nil {
		utilsLogger.Error("Failed to parse timestamp", "timestamp", jsonTimestamp, "expectedFormat", "RFC3339", "error", err)
		return jsonTimestamp
	}
	return t.Format("2006-01-02 15:04:05")
}
