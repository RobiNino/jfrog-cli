node("docker") {
    cleanWs()
    def architectures = [
            [pkg: 'jfrog-cli-windows-amd64', goos: 'windows', goarch: 'amd64', fileExtension: '.exe', chocoImage: 'linuturk/mono-choco'],
            [pkg: 'jfrog-cli-linux-386', goos: 'linux', goarch: '386', fileExtension: '', debianImage: 'i386/ubuntu:16.04', debianArch: 'i386'],
            [pkg: 'jfrog-cli-linux-amd64', goos: 'linux', goarch: 'amd64', fileExtension: '', debianImage: 'ubuntu:16.04', debianArch: 'x86_64', rpmImage: 'centos:8'],
            [pkg: 'jfrog-cli-linux-arm64', goos: 'linux', goarch: 'arm64', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-arm', goos: 'linux', goarch: 'arm', fileExtension: ''],
            [pkg: 'jfrog-cli-mac-386', goos: 'darwin', goarch: 'amd64', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-s390x', goos: 'linux', goarch: 's390x', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-ppc64', goos: 'linux', goarch: 'ppc64', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-ppc64le', goos: 'linux', goarch: 'ppc64le', fileExtension: '']
    ]

    cliExecutableName = 'jf'
    basePathInReleases = 'jfrog-cli/v2-jf/'
    majorVersion = 'v2'

    repo = 'jfrog-cli'
    sh 'rm -rf temp'
    sh 'mkdir temp'
    def goRoot = tool 'go-1.17.2'
    env.GOROOT="$goRoot"
    env.PATH+=":${goRoot}/bin"
    env.GO111MODULE="on"
    env.JFROG_CLI_OFFER_CONFIG="false"

    dir('temp') {
        cliWorkspace = pwd()
        sh "echo cliWorkspace=$cliWorkspace"
        stage('Clone JFrog CLI sources') {
            sh 'git clone https://github.com/jfrog/jfrog-cli.git'
            dir("$repo") {
                if (BRANCH?.trim()) {
                    sh "git checkout $BRANCH"
                }
            }
        }

        runRelease()
        if (RELEASE_DEPRECATED_EXECUTABLE_NAME.toBoolean()) {
            cliExecutableName = 'jfrog'
            basePathInReleases = 'jfrog-cli/v2/'
            runRelease()
        }
    }
}

def runRelease() {
    stage('Build JFrog CLI') {
        sh "echo Running release for executable name: '$cliExecutableName'"

        jfrogCliRepoDir = "${cliWorkspace}/${repo}/"
        builderDir = "${cliExecutableName}-builder/"
        sh "mkdir $builderDir"
        builderPath = "${builderDir}/${cliExecutableName}"

        sh 'go version'
        dir("$jfrogCliRepoDir") {
            sh "build/build.sh $cliExecutableName"
        }

        sh "mv $jfrogCliRepoDir/$cliExecutableName $builderDir"
        // Extract CLI version
        version = sh(script: "$builderDir$cliExecutableName -v | tr -d 'jfrog version' | tr -d '\n'", returnStdout: true)
        print "CLI version: $version"
    }
    if ("$EXECUTION_MODE".toString().equals("Publish packages")) {
        stage('Build and Publish Rpm and Deb') {
            buildRpmAndDeb(version, architectures)
        }

        // Download cert files, to be used for signing the Windows executable, packaged by Chocolatey.
        downloadToolsCert()
        stage('Build and Publish Chocolatey') {
            publishChocoPackage(version, jfrogCliRepoDir, architectures)
        }

        stage('Npm Publish') {
            publishNpmPackage(jfrogCliRepoDir)
        }

        stage('Build and Publish Docker Images') {
            buildPublishDockerImages(version, jfrogCliRepoDir)
        }
    } else if ("$EXECUTION_MODE".toString().equals("Build CLI")) {
        downloadToolsCert()
        print "Uploading version $version to releases.jfrog.io"
        uploadCli(architectures)
    }
}

def downloadToolsCert() {
    stage('Download tools cert') {
        // Download the certificate file and key file, used for signing the JFrog CLI binary.
        withCredentials([string(credentialsId: 'download-signing-cert-access-token', variable: 'DOWNLOAD_SIGNING_CERT_ACCESS_TOKEN')]) {
        sh """#!/bin/bash
            $builderPath rt dl installation-files/certificates/jfrog/jfrogltd_signingcer_full.tar.gz --url https://entplus.jfrog.io/artifactory --flat --access-token=$DOWNLOAD_SIGNING_CERT_ACCESS_TOKEN
            """
        }
        sh 'tar -xvzf jfrogltd_signingcer_full.tar.gz'
    }
}

def buildRpmAndDeb(version, architectures) {
    boolean built = false
    withCredentials([file(credentialsId: 'rpm-gpg-key2', variable: 'rpmGpgKeyFile'), string(credentialsId: 'rpm-sign-passphrase', variable: 'rpmSignPassphrase')]) {
        def dirName = "${majorVersion}-${cliExecutableName}"
        def dirPath = "${pwd()}/jfrog-cli/build/deb_rpm/${dirName}/pkg"
        def gpgPassphraseFilePath = "$dirPath/RPM-GPG-PASSPHRASE-jfrog-cli"
        writeFile(file: gpgPassphraseFilePath, text: "$rpmSignPassphrase")

        for (int i = 0; i < architectures.size(); i++) {
            def currentBuild = architectures[i]
            if (currentBuild.debianImage) {
                stage("Build debian ${currentBuild.pkg}") {
                    build(currentBuild.goos, currentBuild.goarch, currentBuild.pkg, $cliExecutableName)
                    dir("$jfrogCliRepoDir") {
                        sh "build/deb_rpm/$dirName/build-scripts/pack.sh -b $cliExecutableName -v $version -f deb --deb-arch $currentBuild.debianArch --deb-build-image $currentBuild.debianImage -t --deb-test-image $currentBuild.debianImage"
                        built = true
                    }
                }
            }
            if (currentBuild.rpmImage) {
                stage("Build rpm ${currentBuild.pkg}") {
                    build(currentBuild.goos, currentBuild.goarch, currentBuild.pkg, $cliExecutableName)
                    dir("$jfrogCliRepoDir") {
                        sh """#!/bin/bash
                            build/deb_rpm/$dirName/build-scripts/pack.sh -b $cliExecutableName -v $version -f rpm --rpm-build-image $currentBuild.rpmImage -t --rpm-test-image $currentBuild.rpmImage --rpm-gpg-key-file /$rpmGpgKeyFile --rpm-gpg-passphrase-file $gpgPassphraseFilePath
                        """
                        built = true
                    }
                }
            }
        }

        if (built) {
            stage("Deploy deb and rpm") {
                withCredentials([string(credentialsId: 'jfrog-cli-automation', variable: 'JFROG_CLI_AUTOMATION_ACCESS_TOKEN')]) {
                    options = "--url https://releases.jfrog.io/artifactory --flat --access-token=$JFROG_CLI_AUTOMATION_ACCESS_TOKEN"
                    def packageName = "jfrog-cli-v2"
                    if ($cliExecutableName == 'jf') {
                        packageName="${packageName}-jf"
                    }
                    sh """#!/bin/bash
                        $builderPath rt u $jfrogCliRepoDir/build/deb_rpm/$dirName/*.i386.deb jfrog-debs/pool/$packageName/ --deb=xenial,bionic,eoan,focal/contrib/i386 $options
                        $builderPath rt u $jfrogCliRepoDir/build/deb_rpm/$dirName/*.x86_64.deb jfrog-debs/pool/$packageName/ --deb=xenial,bionic,eoan,focal/contrib/amd64 $options
                        $builderPath rt u $jfrogCliRepoDir/build/deb_rpm/$dirName/*.rpm jfrog-rpms/$packageName/ $options
                        """
                }
            }
        }
    } 
}

def uploadCli(architectures) {
    stage("Upload getCli and installCli scripts") {
        uploadGetCliToJfrogReleases()
        uploadInstallCliToJfrogReleases()
    }
    for (int i = 0; i < architectures.size(); i++) {
        def currentBuild = architectures[i]
        stage("Build and upload ${currentBuild.pkg}") {
            buildAndUpload(currentBuild.goos, currentBuild.goarch, currentBuild.pkg, currentBuild.fileExtension)
        }
    }
}

def buildPublishDockerImages(version, jfrogCliRepoDir) {
    def images = [
            [dockerFile:'build/docker/slim/Dockerfile', name:'releases-docker.jfrog.io/jfrog/jfrog-cli-v2'],
            [dockerFile:'build/docker/full/Dockerfile', name:'releases-docker.jfrog.io/jfrog/jfrog-cli-full-v2']
    ]
    for (int i = 0; i < images.size(); i++) {
        def currentImage = images[i]
        def imgName = currentImage.name
        if ($cliExecutableName == 'jf') {
            imgName="${imgName}-jf"
        }

        buildDockerImage(imgName, version, currentImage.dockerFile, jfrogCliRepoDir)
        pushDockerImageVersionAndRelease(imgName, version)
    }
}

def buildDockerImage(name, version, dockerFile, jfrogCliRepoDir) {
    dir("$jfrogCliRepoDir") {
        sh """#!/bin/bash
            docker build --build-arg cliExecutableName=$cliExecutableName --tag=$name:$version -f $dockerFile .
        """
    }
}

def pushDockerImageVersionAndRelease(name, version) {
    withCredentials([string(credentialsId: 'jfrog-cli-automation', variable: 'JFROG_CLI_AUTOMATION_ACCESS_TOKEN')]) {
        options = "--url https://releases.jfrog.io/artifactory --access-token=$JFROG_CLI_AUTOMATION_ACCESS_TOKEN"
        sh """#!/bin/bash
            $builderPath rt docker-push $name:$version reg2 $options
            docker tag $name:$version $name:latest
            $builderPath rt docker-push $name:latest reg2 $options
        """
    }
}

def uploadGetCliToJfrogReleases() {
    withCredentials([string(credentialsId: 'jfrog-cli-automation', variable: 'JFROG_CLI_AUTOMATION_ACCESS_TOKEN')]) {
        sh """#!/bin/bash
                $builderPath rt u $jfrogCliRepoDir/build/getcli/$cliExecutableName.sh $basePathInReleases/scripts/getCli.sh --url https://releases.jfrog.io/artifactory/ --access-token=$JFROG_CLI_AUTOMATION_ACCESS_TOKEN --flat
        """
    }   
}

def uploadInstallCliToJfrogReleases() {
    withCredentials([string(credentialsId: 'jfrog-cli-automation', variable: 'JFROG_CLI_AUTOMATION_ACCESS_TOKEN')]) {
        sh """#!/bin/bash
                $builderPath rt u $jfrogCliRepoDir/build/installcli/$cliExecutableName.sh $basePathInReleases/scripts/installCli.sh --url https://releases.jfrog.io/artifactory/ --access-token=$JFROG_CLI_AUTOMATION_ACCESS_TOKEN --flat
        """
    }
}

def uploadBinaryToJfrogReleases(pkg, fileName) {
    withCredentials([string(credentialsId: 'jfrog-cli-automation', variable: 'JFROG_CLI_AUTOMATION_ACCESS_TOKEN')]) {
        sh """#!/bin/bash
                $builderPath rt u $jfrogCliRepoDir/$fileName $basePathInReleases/$version/$pkg/ --url https://releases.jfrog.io/artifactory/ --access-token=$JFROG_CLI_AUTOMATION_ACCESS_TOKEN --flat
        """
    }   
}

def build(goos, goarch, pkg, fileName) {
    dir("${jfrogCliRepoDir}") {
        env.GOOS="$goos"
        env.GOARCH="$goarch"
        sh "build/build.sh $fileName"
        sh "chmod +x $fileName"

        if (goos == 'windows') {
            dir("${cliWorkspace}/certs-dir") {
                // Move the jfrog executable into the 'sign' directory, so that it is signed there.
                sh "mv $jfrogCliRepoDir/$fileName ${jfrogCliRepoDir}build/sign/${fileName}.unsigned"
                // Copy all the certificate files into the 'sign' directory.
                sh "cp -r ./ ${jfrogCliRepoDir}build/sign/"
                // Build and run the docker container, which signs the JFrog CLI binary.
                sh "docker build -t jfrog-cli-sign-tool ${jfrogCliRepoDir}build/sign/"
                def signCmd = "osslsigncode sign -certs workspace/JFrog_Ltd_.crt -key workspace/jfrogltd.key  -n JFrog_CLI -i https://www.jfrog.com/confluence/display/CLI/JFrog+CLI -in workspace/${fileName}.unsigned -out workspace/$fileName"
                sh "docker run -v ${jfrogCliRepoDir}build/sign/:/workspace --rm jfrog-cli-sign-tool $signCmd"
                // Move the JFrog CLI binary from the 'sign' directory, back to its original location.
                sh "mv ${jfrogCliRepoDir}build/sign/$fileName $jfrogCliRepoDir"
            }
        }
    }
}

def buildAndUpload(goos, goarch, pkg, fileExtension) {
    def extension = fileExtension == null ? '' : fileExtension
    def fileName = "$cliExecutableName$fileExtension"

    build(goos, goarch, pkg, fileName)
    uploadBinaryToJfrogReleases(pkg, fileName)
    sh "rm $jfrogCliRepoDir/$fileName"
}

def publishNpmPackage(jfrogCliRepoDir) {
    dir(jfrogCliRepoDir+"build/npm/$majorVersion-$cliExecutableName") {
        withCredentials([string(credentialsId: 'npm-authorization', variable: 'NPM_AUTH_TOKEN')]) {
            sh '''#!/bin/bash
                apt update
                apt install wget -y
                echo "Downloading npm..."
                wget https://nodejs.org/dist/v8.11.1/node-v8.11.1-linux-x64.tar.xz
                tar -xvf node-v8.11.1-linux-x64.tar.xz
                export PATH=$PATH:$PWD/node-v8.11.1-linux-x64/bin/
                echo "//registry.npmjs.org/:_authToken=$NPM_AUTH_TOKEN" > .npmrc
                echo "registry=https://registry.npmjs.org" >> .npmrc
                ./node-v8.11.1-linux-x64/bin/npm publish
            '''
        }
    }
}

def publishChocoPackage(version, jfrogCliRepoDir, architectures) {
    def architecture = architectures.find { it.goos == 'windows' && it.goarch == 'amd64' }
    build(architecture.goos, architecture.goarch, architecture.pkg, "$cliExecutableName.exe")
    dir(jfrogCliRepoDir+'build/chocolatey') {
        withCredentials([string(credentialsId: 'choco-api-key', variable: 'CHOCO_API_KEY')]) {
            sh """#!/bin/bash
                mv $jfrogCliRepoDir/$cliExecutableName.exe $jfrogCliRepoDir/build/chocolatey/tools
                cp $jfrogCliRepoDir/LICENSE $jfrogCliRepoDir/build/chocolatey/tools
                docker run -v \$PWD:/work -w /work $architecture.chocoImage pack version=$version
                docker run -v \$PWD:/work -w /work $architecture.chocoImage push --apiKey \$CHOCO_API_KEY jfrog-cli.${version}.nupkg
            """
        }
    }
}
