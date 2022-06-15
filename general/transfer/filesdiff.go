package transfer

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"time"
)

const searchTimeFramesMinutes = 15

type filesDiffPhase struct {
	repoKey         string
	startTime       time.Time
	srcUpService    *srcUserPluginService
	srcRtDetails    *coreConfig.ServerDetails
	targetRtDetails *coreConfig.ServerDetails
}

func (f filesDiffPhase) getPhaseName() string {
	return "Files Diff Handling Phase"
}

func (f filesDiffPhase) getPhaseNumber() int {
	return 2
}

func (f filesDiffPhase) phaseStarted() error {
	f.startTime = time.Now()
	// TODO notify progress
	return setFilesDiffHandlingStarted(f.repoKey, f.startTime)
}

func (f filesDiffPhase) phaseDone() error {
	// TODO notify progress
	return setFilesDiffHandlingCompleted(f.repoKey)
}

func (f filesDiffPhase) setRepoKey(repoKey string) {
	f.repoKey = repoKey
}

func (f filesDiffPhase) setSrcUserPluginService(service *srcUserPluginService) {
	f.srcUpService = service
}

func (f filesDiffPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	f.srcRtDetails = details
}

func (f filesDiffPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	f.targetRtDetails = details
}

func (f filesDiffPhase) run() error {
	// todo add check if previous diff failed.
	// todo get last migration / diff completion time
	completionTime, err := getRepoMigratedCompletionTime(f.repoKey)
	if err != nil {
		return err
	}

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

		curDiffTimeFrame := completionTime
		for f.startTime.Sub(curDiffTimeFrame) > 0 {
			diffTimeFrameHandler := f.createDiffTimeFrameHandlerFunc(pcDetails)
			_, _ = producerConsumer.AddTaskWithError(diffTimeFrameHandler(timeFrameParams{repoKey: f.repoKey, fromTime: curDiffTimeFrame}), errorsQueue.AddError)
			curDiffTimeFrame = curDiffTimeFrame.Add(searchTimeFramesMinutes * time.Minute)
		}
	}()

	doneChan := make(chan bool, 1)

	go pollUploads(f.srcUpService, uploadTokensChan, doneChan)

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

type diffTimeFrameHandlerFunc func(params timeFrameParams) parallel.TaskFunc

type timeFrameParams struct {
	repoKey  string
	fromTime time.Time
}

func (f filesDiffPhase) createDiffTimeFrameHandlerFunc(pcDetails producerConsumerDetails) diffTimeFrameHandlerFunc {
	return func(params timeFrameParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			err := f.handleTimeFrameFilesDiff(params, logMsgPrefix, pcDetails)
			if err != nil {
				return err
			}
			pcDetails.folderTasksCounters[threadId]++
			return nil
		}
	}
}

func (f filesDiffPhase) handleTimeFrameFilesDiff(params timeFrameParams, logMsgPrefix string, pcDetails producerConsumerDetails) error {
	log.Info(logMsgPrefix + "Searching time frame: '" + "from" + "' to '" + "to" + "'") // todo

	result, err := f.getTimeFrameFilesDiff(params.repoKey, params.fromTime)
	if err != nil {
		return err
	}

	if len(result.Results) == 0 {
		// todo probably just log
		return nil
	}

	curUploadChunk := UploadChunk{TargetAuth: createTargetAuth(f.targetRtDetails)}

	for _, item := range result.Results {
		if item.Name == "." {
			continue
		}
		if item.Type == "folder" {
			// todo ?
		} else {
			curUploadChunk.appendUploadCandidate(item)
			if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
				err := uploadChunkAndAddTokenIfNeeded(f.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
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
		return uploadChunkAndAddTokenIfNeeded(f.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
	}
	return nil
}

func (f filesDiffPhase) getTimeFrameFilesDiff(repoKey string, fromTime time.Time) (result *artifactoryUtils.AqlSearchResult, err error) {
	query := generateDiffAqlQuery(repoKey, fromTime)
	return runAql(f.srcRtDetails, query)
}

func generateDiffAqlQuery(repoKey string, fromTime time.Time) string {
	fromTimestamp := fromTime.Format(time.RFC3339)
	toTimestamp := fromTime.Add(searchTimeFramesMinutes * time.Minute).Format(time.RFC3339)
	items := fmt.Sprintf(`items.find({"type":"any","modified":{"$gte":"%s"},"modified":{"$lt":"%s"},"$or":[{"$and":[{"repo":"%s","path":{"$match":"*"},"name":{"$match":"*"}}]}]})`, repoKey, fromTimestamp, toTimestamp)
	items += `.include("repo","path","name","type")`
	return items
}
