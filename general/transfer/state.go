package transfer

import (
	"bytes"
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"strconv"
	"time"
)

const requestsNumForNodeDetection = 50

type TransferState struct {
	Repositories []Repository `json:"repositories,omitempty"`
	NodeIds      []string     `json:"nodes,omitempty"`
}

type Repository struct {
	Name           string       `json:"name,omitempty"`
	Migration      PhaseDetails `json:"migration,omitempty"`
	FilesDiff      PhaseDetails `json:"files_diff,omitempty"`
	PropertiesDiff PhaseDetails `json:"properties_diff,omitempty"`
}

type PhaseDetails struct {
	StartedTimestamp string `json:"started_timestamp,omitempty"`
	EndedTimestamp   string `json:"ended_timestamp,omitempty"`
}

type actionOnStateFunc func(state *TransferState)

func getTransferState() (*TransferState, error) {
	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return nil, err
	}
	exists, err := fileutils.IsFileExists(stateFilePath, false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	content, err := fileutils.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	state := new(TransferState)
	err = json.Unmarshal(content, state)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return state, nil
}

func isCleanStart() (bool, error) {
	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return false, err
	}
	return fileutils.IsFileExists(stateFilePath, false)
}

// TODO Implement mutex if more than one thread might run. Assuming no other process runs so no need for lock.
func saveTransferState(state *TransferState) error {
	content, err := state.getContent()
	if err != nil {
		return err
	}

	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(stateFilePath, content, 0600)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

// TODO remove duplication with getContent() @Core
func (ts *TransferState) getContent() ([]byte, error) {
	b, err := json.Marshal(&ts)
	if err != nil {
		return []byte{}, errorutils.CheckError(err)
	}
	var content bytes.Buffer
	err = json.Indent(&content, b, "", "  ")
	if err != nil {
		return []byte{}, errorutils.CheckError(err)
	}
	return content.Bytes(), nil
}

func (ts *TransferState) getRepository(repoKey string) *Repository {
	for i := range ts.Repositories {
		if ts.Repositories[i].Name == repoKey {
			return &ts.Repositories[i]
		}
	}
	repo := Repository{Name: repoKey}
	ts.Repositories = append(ts.Repositories, repo)
	// TODO make sure correct pointer is returned.
	return &repo
}

func doAndSaveState(action actionOnStateFunc) error {
	state, err := getTransferState()
	if err != nil {
		return err
	}

	action(state)

	err = saveTransferState(state)
	if err != nil {
		return err
	}
	return nil
}

func setRepoMigrationStarted(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) {
		repo := state.getRepository(repoKey)
		repo.Migration.StartedTimestamp = convertTimeToRFC3339(startTime)
	}
	return doAndSaveState(action)

}

func setRepoMigrationCompleted(repoKey string) error {
	action := func(state *TransferState) {
		repo := state.getRepository(repoKey)
		repo.Migration.EndedTimestamp = convertTimeToRFC3339(time.Now())
	}
	return doAndSaveState(action)
}

func setFilesDiffHandlingStarted(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) {
		repo := state.getRepository(repoKey)
		repo.FilesDiff.StartedTimestamp = convertTimeToRFC3339(startTime)
	}
	return doAndSaveState(action)
}

func setFilesDiffHandlingCompleted(repoKey string) error {
	action := func(state *TransferState) {
		repo := state.getRepository(repoKey)
		repo.FilesDiff.EndedTimestamp = convertTimeToRFC3339(time.Now())
	}
	return doAndSaveState(action)
}

func setPropsDiffHandlingStarted(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) {
		repo := state.getRepository(repoKey)
		repo.PropertiesDiff.StartedTimestamp = convertTimeToRFC3339(startTime)
	}
	return doAndSaveState(action)
}

func setPropsDiffHandlingCompleted(repoKey string) error {
	action := func(state *TransferState) {
		repo := state.getRepository(repoKey)
		repo.PropertiesDiff.EndedTimestamp = convertTimeToRFC3339(time.Now())
	}
	return doAndSaveState(action)
}

func getRepoFromState(repoKey string) (*Repository, error) {
	state, err := getTransferState()
	if err != nil {
		return nil, err
	}

	return state.getRepository(repoKey), nil
}

func isRepoMigrated(repoKey string) (bool, error) {
	repo, err := getRepoFromState(repoKey)
	if err != nil {
		return false, err
	}
	return repo.Migration.EndedTimestamp != "", nil
}

func getRepoMigratedCompletionTime(repoKey string) (time.Time, error) {
	repo, err := getRepoFromState(repoKey)
	if err != nil {
		return time.Time{}, err
	}
	return convertRFC3339ToTime(repo.Migration.EndedTimestamp)
}

func convertTimeToRFC3339(timeToConvert time.Time) string {
	return timeToConvert.Format(time.RFC3339)
}

func convertRFC3339ToTime(timeToConvert string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeToConvert)
}

func convertTimeToEpochMilliseconds(timeToConvert time.Time) string {
	return strconv.FormatInt(timeToConvert.UnixMilli(), 13)
}

func convertEpochMillisecondsToTime(timeToConvert string) (time.Time, error) {
	timeInt, err := strconv.Atoi(timeToConvert)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(int64(timeInt)), nil
}

// Sends rapid requests to the user plugin and finds all existing nodes in Artifactory.
// Writes all node ids to the state file.
// Also notifies all nodes of a clean start.
func nodeDetection(srcUpService *srcUserPluginService) error {
	var nodeIds []string
requestsLoop:
	for i := 0; i < requestsNumForNodeDetection; i++ {
		curNodeId, err := srcUpService.cleanStart()
		if err != nil {
			return err
		}
		for _, existingNode := range nodeIds {
			if curNodeId == existingNode {
				continue requestsLoop
			}
		}
		nodeIds = append(nodeIds, curNodeId)
	}

	return saveTransferState(&TransferState{NodeIds: nodeIds})
}
