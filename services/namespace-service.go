package services

import "mogenius-k8s-manager/dtos"

// @Post('create')
// public async create(@Body('namespace') namespace: K8sNamespaceDto, @Body('stage') stage: K8sStageDto): Promise<void> {
func CreateNamespace(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto) {
	// TODO: Implement
}

// @Post('delete')
// public async delete(@Body('namespace') namespace: K8sNamespaceDto, @Body('stage') stage: K8sStageDto): Promise<void> {
func DeleteNamespace(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto) {
	// TODO: Implement
}

// @Post('shutdown')
// public async shutdown(@Body('namespaceId') namespaceId: string, @Body('stage') stage: K8sStageDto): Promise<void> {
func ShutdownNamespace(namespaceId string, stage dtos.K8sStageDto) {
	// TODO: Implement
}

// @Post('reboot')
// public async reboot(@Body('namespaceId') namespaceId: string, @Body('stage') stage: K8sStageDto): Promise<void> {
func RebootNamespace(namespaceId string, stage dtos.K8sStageDto) {
	// TODO: Implement
}

// @Get('ingress-state/:state')
// public async setIngressState(
//
//	@Body('namespace') namespace: K8sNamespaceDto,
//	@Body('stage') stage: K8sStageDto,
//	@Param('state') state: IngressState
//
// ): Promise<any> {
func SetIngressState(namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, state string) interface{} {
	// ENABLED = 'ENABLED',
	// DISABLED = 'DISABLED',
	// TRAFFIC_EXCEEDED = 'TRAFFIC_EXCEEDED'
	// TODO: Implement
	return nil
}

// @Get('pod-ids/:namespace')
// public async podIds(@Param('namespace') namespace: string): Promise<any> {
func PodIds(namespace string) interface{} {
	// TODO: Implement
	return nil
}

// @Get('get-cluster-pods')
// public async clusterPods(): Promise<string[]> {
func ClusterPods() []string {
	// TODO: Implement
	return []string{}
}

// @Post('validate-cluster-pods')
// public async validateClusterPods(@Body('dbPodNames') dbPodNames: string[]): Promise<any> {
func ValidateClusterPods(dbPodNames []string) interface{} {
	// TODO: Implement
	return nil
}

// @Post('validate-ports')
// public async validateClusterPorts(@Body('ports') ports: NamespaceServicePortDto[]): Promise<any> {
func ValidateClusterPorts(ports []dtos.NamespaceServicePortDto) interface{} {
	// TODO: Implement
	return nil
}

// @Post('storage-size')
// public async storageSize(@Body('stageIds') stageIds: string[]): Promise<{ [key: string]: number }> {
func StorageSize(stageIds []string) map[string]int {
	// TODO: Implement
	return map[string]int{}
}
