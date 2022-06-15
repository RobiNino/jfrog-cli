package transfer

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"io/ioutil"
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

func (tc *transferCommandConfig) getAllLocalRepositories() (*[]services.RepositoryDetails, error) {
	serviceManager, err := utils.CreateServiceManager(tc.sourceRtDetails, -1, 0, false)
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

func pollUploads(srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool) {
	curTokensBatch := UploadChunksStatusBody{}

	for {
		time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)

		curTokensBatch.fillTokensBatch(uploadTokensChan)

		if len(curTokensBatch.UuidTokens) == 0 {
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
			if chunk.Status == InProcess {
				continue
			}
			for _, file := range chunk.Files {
				if file.Status == Success {
					// TODO increment progress.
				} else {
					// TODO increment progress.
					// TODO write failure to file.
				}
			}
		}
	}
}
