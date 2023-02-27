package dtos

type EventDto struct {
	Type      string      `json:"type" validate:"required"`
	ApiObject interface{} `json:"apiObject"`
}

func EventDtoExampleData() EventDto {
	return EventDto{
		Type:      "MODIFIED",
		ApiObject: "TEST",
	}
}

func CreateEvent(eventType string, apiObject interface{}) EventDto {
	return EventDto{
		Type:      eventType,
		ApiObject: apiObject,
	}
}
