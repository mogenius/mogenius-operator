package services

import "mogenius-k8s-manager/dtos"

// @Get('storage-stats')
// public async allFiles(): Promise<PersistentFileStatsDto> {
func AllFiles() dtos.PersistentFileStatsDto {
	// TODO: Implement
	return dtos.PersistentFileStatsDto{}
}

// @Post('list')
// public async list(@Body() data: PersistentFileRequestDto): Promise<PersistentFileDto[]> {
func List(data dtos.PersistentFileRequestDto) []dtos.PersistentFileDto {
	// TODO: Implement
	return []dtos.PersistentFileDto{}
}

// @Post('download')
// public async download(@Body() data: PersistentFileRequestDto, @Res() res: Response): Promise<any> {
func Download(data dtos.PersistentFileRequestDto, res interface{}) interface{} {
	// TODO: Implement
	return nil
}

// @Post('upload')
// @UseInterceptors(FileInterceptor('file'))
// public async upload(@Body() data: PersistentFileRequestDto, @UploadedFile() file: Express.Multer.File): Promise<any> {
func Upload(data dtos.PersistentFileRequestDto, file interface{}) interface{} {
	// TODO: Implement
	return nil
}

// @Patch('update')
// @UseInterceptors(FileInterceptor('file'))
// public async update(@Body() data: PersistentFileRequestDto, @UploadedFile() file: Express.Multer.File): Promise<any> {
func Update(data dtos.PersistentFileRequestDto, file interface{}) interface{} {
	// TODO: Implement
	return nil
}

// @Post('create-folder')
// public async create(@Body() data: PersistentFileRequestDto): Promise<boolean> {
func CreateFolder(data dtos.PersistentFileRequestDto) bool {
	// TODO: Implement
	return false
}

// @Post('rename')
// public async rename(@Body() data: PersistentFileRequestDto, @Body('newName') newName: string): Promise<boolean> {
func Rename(data dtos.PersistentFileRequestDto, newName string) bool {
	// TODO: Implement
	return false
}

// @Post('chown')
// public async chown(@Body() data: PersistentFileRequestDto, @Body('uid') uid: number, @Body('gid') gid: number): Promise<boolean | Error> {
func Chown(data dtos.PersistentFileRequestDto, uid int, gid int) interface{} {
	// TODO: Implement
	return nil
}

// @Post('chmod')
// public async chmod(@Body() data: PersistentFileRequestDto, @Body('mode') mode: string): Promise<boolean | Error> {
func Chmod(data dtos.PersistentFileRequestDto, mode string) interface{} {
	// TODO: Implement
	return nil
}

// @Post('delete')
// public async delete(@Body() data: PersistentFileRequestDto): Promise<boolean> {
func Delete(data dtos.PersistentFileRequestDto) bool {
	// TODO: Implement
	return false
}
