package upload

import (
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreCommonCommands "github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"
)

const serverID = "transfer"

var m sync.Mutex
var filesUploaded = 0
var serverDetails *config.ServerDetails
var startTime time.Time

const uploadVer = "v4"
const maxLevel = 10
const filesPerDir = 100
const dirsPerLevel = 2
const threads = 64

func RunTransferUploadChunk(c *cli.Context) error {
	sourceRtDetails, err := coreCommonCommands.GetConfig(serverID, false) //c.Args().Get(0)
	if err != nil {
		return err
	}
	serverDetails = sourceRtDetails

	producerConsumer := parallel.NewRunner(threads, 50000000, false)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	expectedChan := make(chan int, 1)
	folderTasksCounters := make([]int, threads)
	fileTasksCounters := make([]int, threads)

	startTime = time.Now()

	go func() {
		err := handleFolder(folderParams{relativeLocation: "generic-robi", level: 0}, producerConsumer, expectedChan, errorsQueue, folderTasksCounters, fileTasksCounters, 1)
		if err != nil {
			log.Error(err)
			return
		}
	}()

	var runnerErr error
	go func() {
		runnerErr = producerConsumer.DoneWhenAllIdle(15)
	}()
	// Blocked until finish consuming
	producerConsumer.Run()

	log.Output("filesUploaded:", filesUploaded)
	log.Output("time elapsed:", time.Since(startTime))
	return nil
}

func incrFilesUploaded(incrBy int) {
	m.Lock()
	filesUploaded += incrBy
	m.Unlock()
}

type folderHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	relativeLocation string
	level            int
}

func createFolderHandlerFunc(producerConsumer parallel.Runner, expectedChan chan int, errorsQueue *clientUtils.ErrorsQueue, folderTasksCounters, fileTasksCounters []int) folderHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			err := handleFolder(params, producerConsumer, expectedChan, errorsQueue, folderTasksCounters, fileTasksCounters, dirsPerLevel)
			if err != nil {
				return err
			}
			folderTasksCounters[threadId]++
			return nil
		}
	}
}

func handleFolder(params folderParams, producerConsumer parallel.Runner, expectedChan chan int, errorsQueue *clientUtils.ErrorsQueue, folderTasksCounters, fileTasksCounters []int, dirsLimit int) (err error) {
	dirsInLevel := 0
	for dirsInLevel <= dirsLimit || params.level >= maxLevel || filesUploaded >= 50000000 {
		dirName := "dir-" + uploadVer + "-" + strconv.Itoa(params.level) + "-" + strconv.Itoa(dirsInLevel)

		folderHandler := createFolderHandlerFunc(producerConsumer, expectedChan, errorsQueue, folderTasksCounters, fileTasksCounters)
		_, _ = producerConsumer.AddTaskWithError(folderHandler(folderParams{relativeLocation: path.Join(params.relativeLocation, dirName), level: params.level + 1}), errorsQueue.AddError)
		dirsInLevel++
	}
	createFilesForDir(params.relativeLocation)
	return nil
}

func createFilesForDir(relativeLocation string) {
	uploadService, err := createUploadServiceManager(serverDetails)
	if err != nil {
		log.Error(err)
		return
	}

	if filesUploaded >= 50000000 {
		return
	}
	filesUploadedInDir := 0
	for filesUploadedInDir < filesPerDir {
		fileName := "file_" + strconv.Itoa(filesUploadedInDir)
		err = deployFile(uploadService, path.Join(relativeLocation, fileName))
		if err != nil {
			log.Error(err)
		} else {
			filesUploadedInDir++
		}
	}
	incrFilesUploaded(filesPerDir)
}

func deployFile(uploadService *services.UploadService, fileRelativePath string) error {
	props := artifactoryUtils.NewProperties()
	props.AddProperty("cur.time", strconv.FormatInt(time.Now().Unix(), 10))
	props.AddProperty("relative.path", fileRelativePath)
	props.AddProperty("constant.prop", "const.value")
	props.AddProperty("long.prop", "veryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryveryverylong")
	props.AddProperty("short.prop", "short")
	props.AddProperty("upload.ver", uploadVer)

	_, targetUrl, err := services.BuildUploadUrls(uploadService.ArtDetails.GetUrl(), fileRelativePath, "", "", props)
	if err != nil {
		return err
	}

	fileDetails := &fileutils.FileDetails{Checksum: entities.Checksum{
		Sha1:   "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		Md5:    "d41d8cd98f00b204e9800998ecf8427e",
		Sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}, Size: 0}

	resp, _, err := uploadService.TryChecksumDeploy(fileDetails, targetUrl, uploadService.ArtDetails.CreateHttpClientDetails())
	if err != nil {
		return err
	}
	// Checksum deploy successful.
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	return errorutils.CheckErrorf("checksum deploy failed")
}

func createUploadServiceManager(serverDetails *config.ServerDetails) (*services.UploadService, error) {
	serviceManager, err := utils.CreateServiceManager(serverDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	uploadService := services.NewUploadService(serviceManager.Client())
	uploadService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
	uploadService.Threads = serviceManager.GetConfig().GetThreads()
	return uploadService, nil
}
