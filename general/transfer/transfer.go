package transfer

import (
	"github.com/jfrog/gofrog/parallel"
	coreCommonCommands "github.com/jfrog/jfrog-cli-core/v2/common/commands"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"time"
)

const (
	tasksMaxCapacity = 500000
	uploadChunkSize  = 10
	defaultThreads   = 16
)

func RunTransfer(c *cli.Context) (err error) {
	if c.NArg() != 3 {
		return cliutils.PrintHelpAndReturnError("wrong number of arguments.", c)
	}

	tc, err := getCommandConfig(c)
	if err != nil {
		return err
	}

	log.Output("Running with", tc.threads, "threads")

	srcUpService, err := createSrcRtUserPluginServiceManager(tc.sourceRtDetails)
	if err != nil {
		return err
	}

	cleanStart, err := isCleanStart()
	if err != nil {
		return err
	}
	if cleanStart {
		err = nodeDetection(srcUpService)
		if err != nil {
			return err
		}
	}

	// todo create repo list here (include exclude)
	srcRepos, err := tc.getAllSrcLocalRepositories()
	if err != nil {
		return err
	}

	targetRepos, err := tc.getAllTargetLocalRepositories()
	if err != nil {
		return err
	}

	for _, repo := range *srcRepos {
		exists := verifyRepoExistsInTarget(targetRepos, repo.Key)
		if !exists {
			// TODO log error.
			continue
		}
		for phaseI := 1; phaseI <= numberOfPhases; phaseI++ {
			var newPhase transferPhase
			switch phaseI {
			case 1:
				newPhase = migrationPhase{}
			case 2:
				newPhase = filesDiffPhase{}
			case 3:
				newPhase = propertiesDiffPhase{}
			}
			// TODO handle errors
			skip, err := newPhase.shouldSkipPhase(repo.Key)
			if err != nil {
				log.Error(err)
			}
			if skip {
				continue
			}
			tc.initNewPhase(newPhase, repo.Key, srcUpService)
			err = newPhase.phaseStarted()
			if err != nil {
				log.Error(err)
			}
			err = newPhase.run()
			if err != nil {
				log.Error(err)
			}
			err = newPhase.phaseDone()
			if err != nil {
				log.Error(err)
			}
		}
	}
	return
}

func verifyRepoExistsInTarget(targetRepos *[]services.RepositoryDetails, srcRepoKey string) bool {
	for _, targetRepo := range *targetRepos {
		if targetRepo.Key == srcRepoKey {
			return true
		}
	}
	return false
}

func (tc *transferCommandConfig) initNewPhase(newPhase transferPhase, repoKey string, srcUpService *srcUserPluginService) {
	newPhase.setRepoKey(repoKey)
	newPhase.shouldCheckExistenceInFilestore(tc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tc.sourceRtDetails)
	newPhase.setTargetDetails(tc.targetRtDetails)
	newPhase.setSrcUserPluginService(srcUpService)
}

type transferCommandConfig struct {
	sourceRtDetails           *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	checkExistenceInFilestore bool
	repository                string
	threads                   int
	retries                   int
	retryWaitTimeMilliSecs    int
}

type producerConsumerDetails struct {
	producerConsumer    parallel.Runner
	expectedChan        chan int
	errorsQueue         *clientUtils.ErrorsQueue
	folderTasksCounters []int
	fileTasksCounters   []int
	uploadTokensChan    chan string
}

// todo remove:
/*
	summaryErr := tc.printSummary(tc.repository, time.Since(startTime))
	if summaryErr != nil {
		if err == nil {
			return summaryErr
		}
		log.Error(summaryErr)
	}
*/
func (tc *transferCommandConfig) printSummary(sourceRepo string, timeElapsed time.Duration) error {
	log.Output("Done. Time elapsed:", timeElapsed)
	log.Output("")
	log.Output("Summary:")
	// todo replace:
	// log.Output("total folders:", totalFolderTasks)
	// log.Output("total files:", totalFileTasks)
	// log.Output("total items:", totalFolderTasks+totalFileTasks)

	storageInfo, err := tc.getStorageInfo()
	if err != nil {
		return err
	}

	for _, repo := range storageInfo.RepositoriesSummaryList {
		if repo.RepoKey == sourceRepo {
			log.Output("")
			log.Output("Expected:")
			log.Output("total folders:", repo.FoldersCount)
			log.Output("total files:", repo.FilesCount)
			log.Output("total items:", repo.ItemsCount)
			log.Output("used space:", repo.UsedSpace)
			return nil
		}
	}
	return errorutils.CheckErrorf("could not find repo '%s' at storage info", sourceRepo)
}

func getCommandConfig(c *cli.Context) (tc transferCommandConfig, err error) {
	tc.sourceRtDetails, err = coreCommonCommands.GetConfig(c.Args().Get(0), true)
	if err != nil {
		return tc, err
	}

	tc.targetRtDetails, err = coreCommonCommands.GetConfig(c.Args().Get(1), true)
	if err != nil {
		return tc, err
	}

	tc.checkExistenceInFilestore = c.Bool(cliutils.Filestore)

	// todo is needed:
	tc.repository = c.Args().Get(2)

	tc.threads, err = cliutils.GetThreadsCount(c, 16)
	if err != nil {
		return tc, err
	}

	tc.retries, err = cliutils.GetRetries(c)
	if err != nil {
		return tc, err
	}

	tc.retryWaitTimeMilliSecs, err = cliutils.GetRetryWaitTime(c)
	return
}

func getThreads() int {
	// TODO implement
	return defaultThreads
}
