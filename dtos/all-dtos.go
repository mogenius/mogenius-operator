package dtos

// const (
// 	ClusterStatusDtoPattern = "ClusterStatusDto"
// )

// type DtoListEntry struct {
// 	Name        string
// 	Pattern     string
// 	ExampleData interface{}
// }

// func All_ExampleData() []DtoListEntry {
// 	all := []DtoListEntry{}
// 	all = append(all, DtoListEntry{Name: "HeartBeat", Pattern: "HeartBeat", ExampleData: nil})
// 	all = append(all, DtoListEntry{Name: "ClusterStatusDto", Pattern: "ClusterStatus", ExampleData: ClusterStatusDtoExmapleData()})
// 	all = append(all, DtoListEntry{Name: "CiCdPipelineDto", Pattern: "CiCdPipelineDtoPattern", ExampleData: CiCdPipelineDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "CiCdPipelineLogEntryDto", Pattern: "CiCdPipelineLogEntryDtoPattern", ExampleData: CiCdPipelineLogEntryDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "CloudflareCustomHostnameDto", Pattern: "CloudflareCustomHostnameDtoPattern", ExampleData: CloudflareCustomHostnameDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "K8sAppDto", Pattern: "K8sAppDtoPattern", ExampleData: K8sAppDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "K8sEnvVarDto", Pattern: "K8sEnvVarDtoPattern", ExampleData: K8sEnvVarDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "K8sNamespaceDto", Pattern: "K8sNamespaceDtoPattern", ExampleData: K8sNamespaceDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "K8sPortsDto", Pattern: "K8sPortsDtoPattern", ExampleData: K8sPortsDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "K8sServiceDto", Pattern: "K8sServiceDtoPattern", ExampleData: K8sServiceDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "K8sServiceSettingsDto", Pattern: "K8sServiceSettingsDtoPattern", ExampleData: K8sServiceSettingsDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "k8sStageDto", Pattern: "k8sStageDtoPattern", ExampleData: K8sStageDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "NamespaceAzureBuildLogDto", Pattern: "NamespaceAzureBuildLogDtoPattern", ExampleData: NamespaceAzureBuildLogDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "NamespaceServiceCnameDto", Pattern: "NamespaceServiceCnameDtoPattern", ExampleData: NamespaceServiceCnameDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "NamespaceServicePortDto", Pattern: "NamespaceServicePortDtoPattern", ExampleData: NamespaceServicePortDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "PersistentFileDto", Pattern: "PersistentFileDtoPattern", ExampleData: PersistentFileDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "PersistentFileRequestDto", Pattern: "PersistentFileRequestDtoPattern", ExampleData: PersistentFileRequestDtoExampleData()})
// 	all = append(all, DtoListEntry{Name: "PersistentFileStatsDto", Pattern: "PersistentFileStatsDtoPattern", ExampleData: PersistentFileStatsDtoExampleData()})
// 	return all
// }

// func DtoListEntryForPattern(pattern string) *DtoListEntry {
// 	for _, dto := range All_ExampleData() {
// 		if dto.Pattern == pattern {
// 			return &dto
// 		}
// 	}
// 	return nil
// }
