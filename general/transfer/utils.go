package transfer

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"io/ioutil"
	"sync"
	"time"
)

const waitTimeBetweenChunkStatusSeconds = 30

func createSrcRtUserPluginServiceManager(sourceRtDetails *coreConfig.ServerDetails) (*srcUserPluginService, error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	userPluginService := NewSrcUserPluginService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client())
	return userPluginService, nil
}

func (tc *transferCommandConfig) getStorageInfo() (*artifactoryUtils.StorageInfo, error) {
	serviceManager, err := utils.CreateServiceManager(tc.sourceRtDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}
	return serviceManager.StorageInfo()
}

func (tc *transferCommandConfig) createTargetUploadServiceManager() (*services.UploadService, error) {
	serviceManager, err := utils.CreateServiceManager(tc.targetRtDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	uploadService := services.NewUploadService(serviceManager.Client())
	uploadService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
	uploadService.Threads = serviceManager.GetConfig().GetThreads()
	return uploadService, nil
}

func (tc *transferCommandConfig) createSourceDownloadServiceManager() (*services.DownloadService, error) {
	serviceManager, err := utils.CreateServiceManager(tc.sourceRtDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	downloadService := services.NewDownloadService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client())
	downloadService.Threads = serviceManager.GetConfig().GetThreads()
	return downloadService, nil
}

func (tc *transferCommandConfig) createSourcePropsServiceManager() (*services.PropsService, error) {
	return createPropsServiceManager(tc.sourceRtDetails)
}

func (tc *transferCommandConfig) createTargetPropsServiceManager() (*services.PropsService, error) {
	return createPropsServiceManager(tc.targetRtDetails)
}

func createPropsServiceManager(serverDetails *coreConfig.ServerDetails) (*services.PropsService, error) {
	serviceManager, err := utils.CreateServiceManager(serverDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	propsService := services.NewPropsService(serviceManager.Client())
	propsService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
	return propsService, nil
}

func (tc *transferCommandConfig) getAllSrcLocalRepositories() (*[]services.RepositoryDetails, error) {
	return tc.getAllLocalRepositories(tc.sourceRtDetails)
}

func (tc *transferCommandConfig) getAllTargetLocalRepositories() (*[]services.RepositoryDetails, error) {
	return tc.getAllLocalRepositories(tc.targetRtDetails)
}

func (tc *transferCommandConfig) getAllLocalRepositories(serverDetails *coreConfig.ServerDetails) (*[]services.RepositoryDetails, error) {
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}

	params := services.RepositoriesFilterParams{RepoType: "local"}
	return serviceManager.GetAllRepositoriesFiltered(params)
}

func runAql(sourceRtDetails *coreConfig.ServerDetails, query string) (result *artifactoryUtils.AqlSearchResult, err error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}
	reader, err := serviceManager.Aql(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if reader != nil {
			e := reader.Close()
			if err == nil {
				err = errorutils.CheckError(e)
			}
		}
	}()

	respBody, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	result = &artifactoryUtils.AqlSearchResult{}
	err = json.Unmarshal(respBody, result)
	return result, errorutils.CheckError(err)
}

func createTargetAuth(targetRtDetails *coreConfig.ServerDetails) TargetAuth {
	targetAuth := TargetAuth{TargetArtifactoryUrl: targetRtDetails.ArtifactoryUrl,
		TargetToken: targetRtDetails.AccessToken}
	if targetAuth.TargetToken == "" {
		targetAuth.TargetUsername = targetRtDetails.User
		targetAuth.TargetPassword = targetRtDetails.Password
	}
	return targetAuth
}

// This variable holds the number of upload chunk that was sent to the user plugin to process.
// Together with this mutex, they control the load on the user plugin and couple it to the local number of threads.
var curProcessedUploadChunks = 0
var processedUploadChunksMutex sync.Mutex

func pollUploads(srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool) {
	curTokensBatch := UploadChunksStatusBody{}

	for {
		time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)

		curTokensBatch.fillTokensBatch(uploadTokensChan)

		if len(curTokensBatch.UuidTokens) == 0 {
			if curProcessedUploadChunks != 0 {
				// TODO handle
			}
			select {
			case done := <-doneChan:
				if done {
					return
				}
			default:
			}
			continue
		}

		// Send and handle.
		chunksStatus, err := srcUpService.getUploadChunksStatus(curTokensBatch)
		if err != nil {
			// TODO handle error.
			return
		}
		for _, chunk := range chunksStatus.ChunksStatus {
			switch chunk.Status {
			case InProgress:
				continue
			case Done:
				reduceCurProcessedChunks()
				curTokensBatch.UuidTokens = removeTokenFromBatch(curTokensBatch.UuidTokens, chunk.UuidToken)
				handleFilesOfCompletedChunk(chunk.Files)
			}
		}
	}
}

func incrCurProcessedChunksWhenPossible() bool {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	if curProcessedUploadChunks <= getThreads() {
		curProcessedUploadChunks++
		return true
	}
	return false
}

func reduceCurProcessedChunks() {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	curProcessedUploadChunks--
}

func removeTokenFromBatch(uuidTokens []string, token string) []string {
	for i := 0; i < len(uuidTokens); i++ {
		if token == uuidTokens[i] {
			return append(uuidTokens[:i], uuidTokens[i+1:]...)
		}
	}
	// todo log unexpected.
	return uuidTokens
}

func handleFilesOfCompletedChunk(chunkFiles []FileUploadStatusResponse) {
	for _, file := range chunkFiles {
		switch file.Status {
		case Success:
			// TODO increment progress.
		case Fail:
			// TODO increment progress.
			addToFailuresConsumableFile(file)
		case SkippedLargeProps:
			// TODO increment progress.
			addToSkippedFile(file)
		}
	}
}

func addToFailuresConsumableFile(file FileUploadStatusResponse) {
	// TODO implement
}

func addToSkippedFile(file FileUploadStatusResponse) {
	// TODO implement
}

// Uploads chunk when there is room in queue.
// This is a blocking method.
func uploadChunkWhenPossible(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string) error {
	for {
		// If increment done, this go routine can proceed to upload the chunk. Otherwise, sleep and try again.
		isIncr := incrCurProcessedChunksWhenPossible()
		if !isIncr {
			time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
			continue
		}
		err := uploadChunkAndAddTokenIfNeeded(sup, chunk, uploadTokensChan)
		if err != nil {
			reduceCurProcessedChunks()
		}
		return err
	}
}

func uploadChunkAndAddTokenIfNeeded(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string) error {
	uuidToken, err := sup.uploadChunk(chunk)
	if err != nil {
		return err
	}
	// Empty token is returned if all files were checksum deployed.
	if uuidToken != "" {
		// Add token to polling.
		uploadTokensChan <- uuidToken
	} else {
		// TODO increment progress. If needed increment local counter.
	}
	return nil
}
