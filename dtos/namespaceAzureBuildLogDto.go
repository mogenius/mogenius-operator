package dtos

type NamespaceAzureBuildLogDto struct {
	Count int      `json:"count"`
	Value []string `json:"value"`
}

func NamespaceAzureBuildLogDtoExampleData() NamespaceAzureBuildLogDto {
	return NamespaceAzureBuildLogDto{
		Count: 1,
		//Value: []string{"value"},
		// TODO: Warum klappt das nicht? Crash wenn einkommentiert
	}
}
