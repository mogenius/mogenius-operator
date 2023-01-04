package services

import "mogenius-k8s-manager/dtos"

// @Get('build-info')
// public async buildInfo(@Query('buildId') buildId: number): Promise<CiCdPipelineDto[]> {
func BuildInfo(buildId int) []dtos.CiCdPipelineDto {
	// TODO: Implement
	return []dtos.CiCdPipelineDto{}
}

// @Post('build-info-array')
// public async buildInfoArray(@Body('buildIds') buildIds: number[]): Promise<any> {
func BuildInfoArray(buildIds []int) interface{} {
	// TODO: Implement
	return nil
}

// @Get('build-log')
// public async buildLog(@Query('buildId') buildId: number, @Query('logId') logId: number): Promise<NamespaceAzureBuildLogDto> {
func BuildLog(buildId int, logId int) dtos.NamespaceAzureBuildLogDto {
	// TODO: Implement
	return dtos.NamespaceAzureBuildLogDto{}
}
