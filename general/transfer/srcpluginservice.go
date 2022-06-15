package transfer

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"net/http"
)

type srcUserPluginService struct {
	client     *jfroghttpclient.JfrogHttpClient
	artDetails *auth.ServiceDetails
}

func NewSrcUserPluginService(artDetails auth.ServiceDetails, client *jfroghttpclient.JfrogHttpClient) *srcUserPluginService {
	return &srcUserPluginService{artDetails: &artDetails, client: client}
}

func (sup *srcUserPluginService) GetArtifactoryDetails() auth.ServiceDetails {
	return *sup.artDetails
}

func (sup *srcUserPluginService) GetJfrogHttpClient() *jfroghttpclient.JfrogHttpClient {
	return sup.client
}

func (sup *srcUserPluginService) IsDryRun() bool {
	return false
}

func (sup *srcUserPluginService) getUploadChunksStatus(ucStatus UploadChunksStatusBody) (UploadChunksStatusResponse, error) {
	content, err := json.Marshal(ucStatus)
	if err != nil {
		return UploadChunksStatusResponse{}, errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+"getUploadChunksStatus", content, &httpDetails)
	if err != nil {
		return UploadChunksStatusResponse{}, err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return UploadChunksStatusResponse{}, errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
	}

	var statusResponse UploadChunksStatusResponse
	err = json.Unmarshal(body, &statusResponse)
	if err != nil {
		return UploadChunksStatusResponse{}, errorutils.CheckError(err)
	}
	return statusResponse, nil
}

// Uploads a chunk of files. If no error occurred:
// Returns empty string if all files uploaded successfully with checksum deploy.
// Otherwise, returns uuid token to get chunk status with.
func (sup *srcUserPluginService) uploadChunk(chunk UploadChunk) (uuidToken string, err error) {
	content, err := json.Marshal(chunk)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+"uploadChunk", content, &httpDetails)
	if err != nil {
		return "", err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK, http.StatusAccepted); err != nil {
		return "", errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
	}

	if resp.StatusCode == http.StatusOK {
		return "", nil
	}

	var uploadResponse UploadChunkResponse
	err = json.Unmarshal(body, &uploadResponse)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return uploadResponse.UuidToken, nil
}

func uploadChunkAndAddTokenIfNeeded(srcUpService *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string) error {
	uuidToken, err := srcUpService.uploadChunk(chunk)
	if err != nil {
		return err
	}
	if uuidToken != "" {
		// Add token to polling.
		uploadTokensChan <- uuidToken
	} else {
		// TODO increment progress. If needed increment local counter.
	}
	return nil
}

func (sup *srcUserPluginService) storeProperties(repoKey string) error {
	params := map[string]string{"repoKey": repoKey}
	requestFullUrl, err := utils.BuildArtifactoryUrl(sup.GetArtifactoryDetails().GetUrl(), "storeProperties", params)
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(requestFullUrl, nil, &httpDetails)
	if err != nil {
		return err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
	}

	return nil
}

func (sup *srcUserPluginService) cleanStart() (nodeId string, err error) {
	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+"cleanStart", nil, &httpDetails)
	if err != nil {
		return "", err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return "", errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
	}

	var response NodeIdResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return response.NodeId, nil
}

func (sup *srcUserPluginService) handlePropertiesDiff(requestBody HandlePropertiesDiff) (*HandlePropertiesDiffResponse, error) {
	content, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	httpDetails := sup.GetArtifactoryDetails().CreateHttpClientDetails()
	resp, body, err := sup.client.SendPost(sup.GetArtifactoryDetails().GetUrl()+"handlePropertiesDiff", content, &httpDetails)
	if err != nil {
		return nil, err
	}

	if err = errorutils.CheckResponseStatus(resp, http.StatusOK); err != nil {
		return nil, errorutils.CheckError(errorutils.GenerateResponseError(resp.Status, clientutils.IndentJson(body)))
	}

	var result HandlePropertiesDiffResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &result, nil
}
