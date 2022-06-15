package transfer

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path"
	"time"
)

type migrationPhase struct {
	repoKey         string
	startTime       time.Time
	srcUpService    *srcUserPluginService
	srcRtDetails    *coreConfig.ServerDetails
	targetRtDetails *coreConfig.ServerDetails
}

func (m migrationPhase) getPhaseName() string {
	return "Migration Phase"
}

func (m migrationPhase) getPhaseNumber() int {
	return 1
}

func (m migrationPhase) phaseStarted() error {
	m.startTime = time.Now()
	err := setRepoMigrationStarted(m.repoKey, m.startTime)
	if err != nil {
		return err
	}

	// TODO notify progress

	return m.srcUpService.storeProperties(m.repoKey)
}

func (m migrationPhase) phaseDone() error {
	// TODO notify progress
	return setRepoMigrationCompleted(m.repoKey)
}

func (m migrationPhase) setRepoKey(repoKey string) {
	m.repoKey = repoKey
}

func (m migrationPhase) setSrcUserPluginService(service *srcUserPluginService) {
	m.srcUpService = service
}

func (m migrationPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	m.srcRtDetails = details
}

func (m migrationPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	m.targetRtDetails = details
}

func (m migrationPhase) run() error {
	producerConsumer := parallel.NewRunner(getThreads(), tasksMaxCapacity, false)
	expectedChan := make(chan int, 1)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	uploadTokensChan := make(chan string, tasksMaxCapacity)

	go func() {
		pcDetails := producerConsumerDetails{
			producerConsumer: producerConsumer,
			expectedChan:     expectedChan,
			errorsQueue:      errorsQueue,
			uploadTokensChan: uploadTokensChan,
		}
		folderHandler := m.createFolderMigrationHandlerFunc(pcDetails)
		_, _ = producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: m.repoKey, relativePath: "."}), errorsQueue.AddError)
	}()

	doneChan := make(chan bool, 1)

	go pollUploads(m.srcUpService, uploadTokensChan, doneChan)

	var runnerErr error
	go func() {
		runnerErr = producerConsumer.DoneWhenAllIdle(15)
		doneChan <- true
	}()
	// Blocked until finish consuming
	producerConsumer.Run()

	if runnerErr != nil {
		return runnerErr
	}
	return errorsQueue.GetError()
}

type folderMigrationHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	repoKey      string
	relativePath string
}

func (m migrationPhase) createFolderMigrationHandlerFunc(pcDetails producerConsumerDetails) folderMigrationHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			err := m.migrateFolder(params, logMsgPrefix, pcDetails)
			if err != nil {
				return err
			}
			pcDetails.folderTasksCounters[threadId]++
			return nil
		}
	}
}

func (m migrationPhase) migrateFolder(params folderParams, logMsgPrefix string, pcDetails producerConsumerDetails) error {
	log.Info(logMsgPrefix+"Visited folder:", path.Join(params.repoKey, params.relativePath))

	result, err := m.getDirectoryContentsAql(params.repoKey, params.relativePath)
	if err != nil {
		return err
	}

	if len(result.Results) == 0 {
		// TODO implement for empty folder
	}

	curUploadChunk := UploadChunk{TargetAuth: createTargetAuth(m.targetRtDetails)}

	for _, item := range result.Results {
		if item.Name == "." {
			continue
		}
		if item.Type == "folder" {
			newRelativePath := item.Name
			if params.relativePath != "." {
				newRelativePath = path.Join(params.relativePath, newRelativePath)
			}
			folderHandler := m.createFolderMigrationHandlerFunc(pcDetails)
			_, _ = pcDetails.producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: newRelativePath}), pcDetails.errorsQueue.AddError)
		} else {
			curUploadChunk.appendUploadCandidate(item)
			if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
				err := uploadChunkAndAddTokenIfNeeded(m.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
				if err != nil {
					return err
				}
				// Empty the uploaded chunk.
				curUploadChunk.UploadCandidates = []FileRepresentation{}
			}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		return uploadChunkAndAddTokenIfNeeded(m.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
	}
	return nil
}

func (m migrationPhase) getDirectoryContentsAql(repoKey, relativePath string) (result *artifactoryUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(repoKey, relativePath)
	return runAql(m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string) string {
	return fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]}).include("repo","path","name")`, repoKey, relativePath)
	// todo removed from include to speed up:
	// ,"created","modified","updated","created_by","modified_by","type","actual_md5","actual_sha1","sha256","size","property","stat"
}
