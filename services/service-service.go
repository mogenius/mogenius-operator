package services

import "mogenius-k8s-manager/dtos"

// @Post('create')
// public async create(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto
//
// ): Promise<void | BadRequestException> {
func CreateService(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Post('delete')
// public async delete(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto
//
// ): Promise<void | BadRequestException | Job>
func DeleteService(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Patch('images/:imageName')
// public async setImage(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto,
//	@Param('imageName') imageName: string
//
// ): Promise<void | BadRequestException> {
func SetImage(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto, imageName string) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Get('pod-ids/:namespace/:serviceId')
// public async podIds(@Param('namespace') namespace: string, @Param('serviceId') serviceId: string): Promise<any>
func ServicePodIds(namespace string, serviceId string) interface{} {
	// TODO: Implement
	return nil
}

// @Get('log/:namespace/:podId')
// public async podLog(@Param('namespace') namespace: string, @Param('podId') podId: string): Promise<any>
func PodLog(namespace string, podId string) interface{} {
	// TODO: Implement
	return nil
}

// @Sse('log-stream/:namespace/:podId/:sinceSeconds')
// public podLogStream(
//
//	@Param('namespace') namespace: string,
//	@Param('podId') podId: string,
//	@Param('sinceSeconds') sinceSeconds: number,
//	@Res() res: Response,
//	@Req() req: Request
//
// ): Observable<any> {
func PodLogStream(namespace string, podId string, sinceSeconds int) interface{} {
	// TODO: Implement
	return nil
}

// @Get('resource-status/:resource/:namespace/:name/:statusOnly')
// public podStatus(
//
//	@Param('resource') resource: string,
//	@Param('namespace') namespace: string,
//	@Param('name') name: string,
//	@Param('statusOnly') statusOnly: boolean
//
// ): Promise<any> {
func PodStatus(resource string, namespace string, name string, statusOnly bool) interface{} {
	// TODO: Implement
	return nil
}

// @Post('build')
// public async build(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto,
//	@Body('commitHash') commitHash: string,
//	@Body('commitAuthor') commitAuthor: string,
//	@Body('commitMessage') commitMessage: string
//
// ): Promise<void | BadRequestException> {
func Build(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto, commitHash string, commitAuthor string, commitMessage string) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Post('restart')
// public async restart(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto
//
// ): Promise<void | BadRequestException> {
func Restart(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Post('stop')
// public async stop(
//
//	@Body('namespaceId') namespaceId: string,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto
//
// ): Promise<void | BadRequestException> {
func StopService(namespaceId string, stage dtos.K8sStageDto, service dtos.K8sServiceDto) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Post('start')
// public async start(
//
//	@Body('namespaceId') namespaceId: string,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto
//
// ): Promise<void | BadRequestException> {
func StartService(namespaceId string, stage dtos.K8sStageDto, service dtos.K8sServiceDto) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Post('update-service')
// public async updateService(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Body('service') service: K8sServiceDto
//
// ): Promise<void | BadRequestException> {
func UpdateService(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, service dtos.K8sServiceDto) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Post('spectrum-bind')
// public async bind(
//
//	@Query('k8sNamespaceName') k8sNamespaceName: string,
//	@Query('k8sServiceName') k8sServiceName: string,
//	@Query('externalPort') externalPort: number,
//	@Query('internalPort') internalPort: number,
//	@Query('type') type: NamespaceServicePortBindingTypeEnum,
//	@Query('namespaceId') namespaceId: string
//
// ): Promise<void | BadRequestException> {
func BindSpectrum(k8sNamespaceName string, k8sServiceName string, externalPort int, internalPort int, portType string, namespaceId string) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Delete('spectrum-unbind')
// public async unbind(
//
//	@Query('externalPort') externalPort: number,
//	@Query('type') type: NamespaceServicePortBindingTypeEnum,
//	@Query('namespaceId') namespaceId: string
//
// ): Promise<void | BadRequestException> {
func UnbindSpectrum(externalPort int, portType string, namespaceId string) (interface{}, error) {
	// TODO: Implement
	return nil, nil
}

// @Get('spectrum-configmaps')
// public async spectrumConfigmaps(): Promise<any | BadRequestException> {
func SpectrumConfigmaps() (interface{}, error) {
	// TODO: Implement
	return nil, nil
}
