package utils

import (
	"fmt"
	"mogenius-k8s-manager/logger"

	"github.com/go-playground/validator/v10"
	"github.com/mogenius/punq/structs"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

func PrintLogo() {
	fmt.Println("")
	fmt.Println("███╗░░░███╗░█████╗░░██████╗░███████╗███╗░░██╗██╗██╗░░░██╗░██████╗")
	fmt.Println("████╗░████║██╔══██╗██╔════╝░██╔════╝████╗░██║██║██║░░░██║██╔════╝")
	fmt.Println("██╔████╔██║██║░░██║██║░░██╗░█████╗░░██╔██╗██║██║██║░░░██║╚█████╗░")
	fmt.Println("██║╚██╔╝██║██║░░██║██║░░╚██╗██╔══╝░░██║╚████║██║██║░░░██║░╚═══██╗")
	fmt.Println("██║░╚═╝░██║╚█████╔╝╚██████╔╝███████╗██║░╚███║██║╚██████╔╝██████╔╝")
	fmt.Println("╚═╝░░░░░╚═╝░╚════╝░░╚═════╝░╚══════╝╚═╝░░╚══╝╚═╝░╚═════╝░╚═════╝░")
	fmt.Println("")
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
		result.Errors = append(result.Errors, err.Error())
		logger.Log.Error(structs.PrettyPrintString(result))
		return result
	}
	return nil
}
