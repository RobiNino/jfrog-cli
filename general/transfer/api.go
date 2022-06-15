package transfer

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

type ChunkStatusType string

const (
	Done      ChunkStatusType = "DONE"
	InProcess ChunkStatusType = "IN_PROCESS"
)

type ChunkFileStatusType string

const (
	Success ChunkFileStatusType = "SUCCESS"
	Fail    ChunkFileStatusType = "FAIL"
)

type TargetAuth struct {
	TargetArtifactoryUrl string `json:"target_artifactory_url,omitempty"`
	TargetUsername       string `json:"target_username,omitempty"`
	TargetPassword       string `json:"target_password,omitempty"`
	TargetToken          string `json:"target_token,omitempty"`
}

type HandlePropertiesDiff struct {
	TargetAuth
	RepoKey           string `json:"repo_key,omitempty"`
	StartMilliseconds string `json:"start_milliseconds,omitempty"`
	EndMilliseconds   string `json:"end_milliseconds,omitempty"`
}

type HandlePropertiesDiffResponse struct {
	NodeIdResponse
	PropertiesDelivered json.Number     `json:"properties_delivered,omitempty"`
	PropertiesRemained  json.Number     `json:"properties_remained,omitempty"`
	Status              ChunkStatusType `json:"status,omitempty"`
	Errors              string          `json:"errors,omitempty"`
}

type PropertiesHandlingError struct {
	FileRepresentation
	StatusCode string `json:"status_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type UploadChunk struct {
	TargetAuth
	UploadCandidates []FileRepresentation `json:"upload_candidates,omitempty"`
}

type FileRepresentation struct {
	Repo string `json:"repo,omitempty"`
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
}

type UploadChunkResponse struct {
	NodeIdResponse
	UuidTokenResponse
}

type UploadChunksStatusBody struct {
	UuidTokens []string `json:"uuid_tokens,omitempty"`
}

type UploadChunksStatusResponse struct {
	NodeIdResponse
	ChunksStatus []ChunkStatus `json:"chunks_status,omitempty"`
}

type ChunkStatus struct {
	UuidTokenResponse
	Status ChunkStatusType            `json:"status,omitempty"`
	Files  []FileUploadStatusResponse `json:"files,omitempty"`
}

type FileUploadStatusResponse struct {
	FileRepresentation
	Status     ChunkFileStatusType `json:"status,omitempty"`
	StatusCode string              `json:"status_code,omitempty"`
	Reason     string              `json:"reason,omitempty"`
}

type NodeIdResponse struct {
	NodeId string `json:"node_id,omitempty"`
}

type UuidTokenResponse struct {
	UuidToken string `json:"uuid_token,omitempty"`
}

// Fill tokens batch till full. Return if no new tokens are available.
func (ucs *UploadChunksStatusBody) fillTokensBatch(uploadTokensChan chan string) {
	for len(ucs.UuidTokens) < uploadChunkSize {
		select {
		case token := <-uploadTokensChan:
			ucs.UuidTokens = append(ucs.UuidTokens, token)
		default:
			// No new tokens are waiting.
			return
		}
	}
}

func (uc *UploadChunk) appendUploadCandidate(item utils.ResultItem) {
	uc.UploadCandidates = append(uc.UploadCandidates, FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name})
}
