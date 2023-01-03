package structs

// import (
// 	"encoding/json"
// 	"io"

// 	"github.com/go-playground/validator/v10"
// )

// // id: string;
// //     type: string;
// //     name: string;
// //     startTime: string;
// //     finishTime: string;
// //     state: CiCdPipelineStatusEnum;
// //     result?: CiCdPipelineResultEnum;
// //     resultCode?: string;
// //     workerName?: string;
// //     log?: CiCdPipelineLogEntryDto;

// type CiCdPipelineDto struct {
// 	Type       string `json:"type" validate:"required"`
// 	Name       string `json:"name" validate:"required"`
// 	StartTime  string `json:"startTime" validate:"required"`
// 	FinishTime string `json:"finishTime" validate:"required"`
// 	State      string `json:"state" validate:"required"` // "completed", "inProgress", "pending"
// 	Result     string `json:"result" `                   // "abandoned", "canceled", "failed", "skipped", "succeeded", "succeededWithIssues"
// 	ResultCode string `json:"resultCode"`
// 	WorkerName string `json:"workerName"`
// 	Log        string `json:"log"`
// }

// // causes the struct to be cached (this is faster)
// var validate *validator.Validate

// func init() {
// 	validate = validator.New()
// 	validate.RegisterValidation("state", ValidateState)
// }

// func (dto *CiCdPipelineDto) FromJSON(r io.Reader) error {
// 	e := json.NewDecoder(r)
// 	return e.Decode(dto)
// }

// func (dto *CiCdPipelineDto) Validate() error {
// 	validate.Struct(CiCdPipelineDto{})
// }

// func ValidateState(fl validator.FieldLevel) interface {} {
// 	switch fl.Field().String() {
// 	case "completed", "inProgress", "pending":
// 		return true
// 	}
// 	return false
// }
