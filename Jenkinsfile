node("docker") {
    cleanWs()
    // Subtract repo name from the repo url (https://REPO_NAME/ -> REPO_NAME/)
    withCredentials([string(credentialsId: 'repo21-url', variable: 'REPO21_URL')]) {
        echo "${REPO21_URL}"
        def repo21Name = "${REPO21_URL}".substring(8, "${REPO21_URL}".length())
        env.REPO_NAME_21="$repo21Name"
    }
    def architectures = [
            [pkg: 'jfrog-cli-windows-amd64', goos: 'windows', goarch: 'amd64', fileExtension: '.exe', chocoImage: '${REPO_NAME_21}/jfrog-docker/linuturk/mono-choco'],
            [pkg: 'jfrog-cli-linux-386', goos: 'linux', goarch: '386', fileExtension: '', debianImage: '${REPO_NAME_21}/jfrog-docker/i386/ubuntu:16.04', debianArch: 'i386'],
            [pkg: 'jfrog-cli-linux-amd64', goos: 'linux', goarch: 'amd64', fileExtension: '', debianImage: '${REPO_NAME_21}/jfrog-docker/ubuntu:16.04', debianArch: 'x86_64', rpmImage: 'centos:8'],
            [pkg: 'jfrog-cli-linux-arm64', goos: 'linux', goarch: 'arm64', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-arm', goos: 'linux', goarch: 'arm', fileExtension: ''],
            [pkg: 'jfrog-cli-mac-386', goos: 'darwin', goarch: 'amd64', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-s390x', goos: 'linux', goarch: 's390x', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-ppc64', goos: 'linux', goarch: 'ppc64', fileExtension: ''],
            [pkg: 'jfrog-cli-linux-ppc64le', goos: 'linux', goarch: 'ppc64le', fileExtension: '']
    ]

    cliExecutableName = 'jf'
    identifier = 'v2-jf'

    repo = 'jfrog-cli'
    sh 'rm -rf temp'
    sh 'mkdir temp'
    def goRoot = tool 'go-1.17.2'
    env.GOROOT="$goRoot"
    env.PATH+=":${goRoot}/bin"
    env.GO111MODULE="on"
    env.CI=true

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
        stage('Release jf executable') {
            runRelease(architectures)
        }

        stage('Release jfrog executable') {
            cliExecutableName = 'jfrog'
            identifier = 'v2'
            runRelease(architectures)
        }
    }
}

def runRelease(architectures) {
    stage('Build JFrog CLI') {
        sh "echo Running release for executable name: '$cliExecutableName'"

        jfrogCliRepoDir = "${cliWorkspace}/${repo}/"
        print "jfrogCliRepoDir: $jfrogCliRepoDir"
        builderDir = "${cliExecutableName}-builder/"
        sh "mkdir $builderDir"
        builderPath = "${builderDir}${cliExecutableName}"
        print "Builder path: $builderPath"

        sh 'go version'
        dir("$jfrogCliRepoDir") {
            sh "build/build.sh $cliExecutableName"
        }

        sh "mv $jfrogCliRepoDir/$cliExecutableName $builderDir"
        // Extract CLI version
        version = sh(script: "$builderDir$cliExecutableName -v | tr -d 'jfrog version' | tr -d '\n'", returnStdout: true)
        print "CLI version: $version"
    }
    configRepo21()

    try {
        if ("$EXECUTION_MODE".toString().equals("Publish packages")) {
            stage('Docker Login') {
                dockerLogin()
            }

            stage('Build and Publish Rpm and Debian') {
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
            print "Uploading version $version to Repo21"
            uploadCli(architectures)
            stage("Distribute jfrog-cli to releases") {
                distributeToReleases("jfrog-cli", version, "cli-rbc-spec.json")
            }
        }
    } finally {
        cleanupRepo21()
    }
}

def downloadToolsCert() {
    stage('Download tools cert') {
        // Download the certificate file and key file, used for signing the JFrog CLI binary.
        withCredentials([
            string(credentialsId: 'download-signing-cert-access-token', variable: 'DOWNLOAD_SIGNING_CERT_ACCESS_TOKEN'),
            string(credentialsId: 'repo21-url', variable: 'REPO21_URL')
        ]) {
        sh """#!/bin/bash
            $builderPath rt dl installation-files/certificates/jfrog/jfrogltd_signingcer_full.tar.gz --url $REPO21_URL/artifactory --flat --access-token=$DOWNLOAD_SIGNING_CERT_ACCESS_TOKEN
            """
        }
        sh 'tar -xvzf jfrogltd_signingcer_full.tar.gz'
    }
}

// Config Repo21 as default server.
def configRepo21() {
    withCredentials([
        usernamePassword(credentialsId: 'repo21', usernameVariable: 'REPO21_USER', passwordVariable: 'REPO21_PASSWORD'),
        string(credentialsId: 'repo21-url', variable: 'REPO21_URL')
    ]) {
        sh """#!/bin/bash
            $builderPath c add repo21 --url=$REPO21_URL --user=$REPO21_USER --password=$REPO21_PASSWORD --overwrite
            $builderPath c use repo21
        """
    }
}

def cleanupRepo21() {
    sh """#!/bin/bash
        $builderPath c rm repo21
    """
}

def buildRpmAndDeb(version, architectures) {
    boolean built = false
    withCredentials([file(credentialsId: 'rpm-gpg-key2', variable: 'rpmGpgKeyFile'), string(credentialsId: 'rpm-sign-passphrase', variable: 'rpmSignPassphrase')]) {
        def dirPath = "${pwd()}/jfrog-cli/build/deb_rpm/${identifier}/pkg"
        def gpgPassphraseFilePath = "$dirPath/RPM-GPG-PASSPHRASE-jfrog-cli"
        writeFile(file: gpgPassphraseFilePath, text: "$rpmSignPassphrase")

        for (int i = 0; i < architectures.size(); i++) {
            def currentBuild = architectures[i]
            if (currentBuild.debianImage) {
                stage("Build debian ${currentBuild.pkg}") {
                    build(currentBuild.goos, currentBuild.goarch, currentBuild.pkg, cliExecutableName)
                    dir("$jfrogCliRepoDir") {
                        sh "build/deb_rpm/$identifier/build-scripts/pack.sh -b $cliExecutableName -v $version -f deb --deb-arch $currentBuild.debianArch --deb-build-image $currentBuild.debianImage -t --deb-test-image $currentBuild.debianImage"
                        built = true
                    }
                }
            }
            if (currentBuild.rpmImage) {
                stage("Build rpm ${currentBuild.pkg}") {
                    build(currentBuild.goos, currentBuild.goarch, currentBuild.pkg, cliExecutableName)
                    dir("$jfrogCliRepoDir") {
                        sh """#!/bin/bash
                            build/deb_rpm/$identifier/build-scripts/pack.sh -b $cliExecutableName -v $version -f rpm --rpm-build-image $currentBuild.rpmImage -t --rpm-test-image $currentBuild.rpmImage --rpm-gpg-key-file /$rpmGpgKeyFile --rpm-gpg-passphrase-file $gpgPassphraseFilePath
                        """
                        built = true
                    }
                }
            }
        }

        if (built) {
            stage("Deploy deb and rpm") {
               def packageName = "jfrog-cli-$identifier"
               sh """#!/bin/bash
                        $builderPath rt u $jfrogCliRepoDir/build/deb_rpm/$identifier/*.i386.deb ecosys-jfrog-debs/pool/$packageName/ --deb=xenial,bionic,eoan,focal/contrib/i386 --flat
                        $builderPath rt u $jfrogCliRepoDir/build/deb_rpm/$identifier/*.x86_64.deb ecosys-jfrog-debs/pool/$packageName/ --deb=xenial,bionic,eoan,focal/contrib/amd64 --flat
                        $builderPath rt u $jfrogCliRepoDir/build/deb_rpm/$identifier/*.rpm ecosys-jfrog-rpms/$packageName/ --flat
               """
            }
            stage("Distribute deb-rpm to releases") {
                distributeToReleases("deb-rpm", version, "deb-rpm-rbc-spec.json")
            }
        }
    }
}

def uploadCli(architectures) {
    stage("Upload getCli.sh and installCli.sh") {
        uploadGetCliToJfrogRepo21()
        uploadInstallCliToJfrogRepo21()
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
            [dockerFile:'build/docker/slim/Dockerfile', name:"${REPO_NAME_21}/ecosys-docker-local/jfrog/jfrog-cli-${identifier}"],
            [dockerFile:'build/docker/full/Dockerfile', name:"${REPO_NAME_21}/ecosys-docker-local/jfrog/jfrog-cli-full-${identifier}"]
    ]
    // Build all images
    for (int i = 0; i < images.size(); i++) {
        def currentImage = images[i]
        print "Building and pushing docker image: $currentImage.name"
        buildDockerImage(currentImage.name, version, currentImage.dockerFile, jfrogCliRepoDir)
        pushDockerImageVersion(currentImage.name, version)
    }
    stage("Distribute cli-docker-images to releases") {
        distributeToReleases("cli-docker-images", version, "docker-images-rbc-spec.json")
    }
}

def buildDockerImage(name, version, dockerFile, jfrogCliRepoDir) {
    dir("$jfrogCliRepoDir") {
        sh """#!/bin/bash
            docker build --build-arg cli_executable_name=$cliExecutableName --build-arg repo_name_21=$REPO_NAME_21 --tag=$name:$version -f $dockerFile .
        """
    }
}

def pushDockerImageVersion(name, version) {
        sh """#!/bin/bash
            $builderPath rt docker-push $name:$version ecosys-docker-local
            docker tag $name:$version $name:latest
            $builderPath rt docker-push $name:latest ecosys-docker-local
        """
}

def uploadGetCliToJfrogRepo21() {
    sh """#!/bin/bash
        $builderPath rt u $jfrogCliRepoDir/build/getcli/${cliExecutableName}.sh ecosys-jfrog-cli/$identifier/scripts/getCli.sh --flat
    """
}

def uploadInstallCliToJfrogRepo21() {
    sh """#!/bin/bash
        $builderPath rt u $jfrogCliRepoDir/build/installcli/${cliExecutableName}.sh ecosys-jfrog-cli/$identifier/scripts/installCli.sh --flat
    """
}

def uploadBinaryToJfrogRepo21(pkg, fileName) {
    sh """#!/bin/bash
        $builderPath rt u $jfrogCliRepoDir/$fileName ecosys-jfrog-cli/$identifier/$version/$pkg/ --flat
    """
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
                // Pull the docker container, which signs the JFrog CLI binary.
                // In order to build it locally, run the following command:
                // "docker build -t jfrog-cli-sign-tool ${jfrogCliRepoDir}build/sign/"
                sh """#!/bin/bash
                  $cliWorkspace/$builderPath rt docker-pull ${REPO_NAME_21}/ecosys-docker-local/jfrog-cli-sign-tool ecosys-docker-local
                """
                // Run the pulled image in order to signs the JFrog CLI binary.
                def signCmd = "osslsigncode sign -certs workspace/JFrog_Ltd_.crt -key workspace/jfrogltd.key  -n JFrog_CLI -i https://www.jfrog.com/confluence/display/CLI/JFrog+CLI -in workspace/${fileName}.unsigned -out workspace/$fileName"
                sh "docker run -v ${jfrogCliRepoDir}build/sign/:/workspace --rm ${REPO_NAME_21}/ecosys-docker-local/jfrog-cli-sign-tool $signCmd"
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
    uploadBinaryToJfrogRepo21(pkg, fileName)
    sh "rm $jfrogCliRepoDir/$fileName"
}

def distributeToReleases(stage, version, rbcSpecName) {
    sh """$builderPath ds rbc $stage-rb-$identifier $version --spec=${cliWorkspace}/${repo}/build/release_specs/$rbcSpecName --spec-vars="VERSION=$version;IDENTIFIER=$identifier" --sign"""
    sh "$builderPath ds rbd $stage-rb-$identifier $version --site=releases.jfrog.io"
}

def publishNpmPackage(jfrogCliRepoDir) {
    dir(jfrogCliRepoDir+"build/npm/$identifier") {
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
    build(architecture.goos, architecture.goarch, architecture.pkg, "${cliExecutableName}.exe")
    def packageName = "jfrog-cli"
    if (cliExecutableName == 'jf') {
        packageName="${packageName}-v2-jf"
    }
    print "Choco package name: $packageName"
    dir(jfrogCliRepoDir+"build/chocolatey/$identifier") {
        withCredentials([string(credentialsId: 'choco-api-key', variable: 'CHOCO_API_KEY')]) {
            sh """#!/bin/bash
                mv $jfrogCliRepoDir/${cliExecutableName}.exe $jfrogCliRepoDir/build/chocolatey/tools
                cp $jfrogCliRepoDir/LICENSE $jfrogCliRepoDir/build/chocolatey/tools
                docker run -v \$PWD:/work -w /work $architecture.chocoImage pack version=$version
                docker run -v \$PWD:/work -w /work $architecture.chocoImage push --apiKey \$CHOCO_API_KEY ${packageName}.${version}.nupkg
            """
        }
    }
}

def dockerLogin(){
    withCredentials([
        usernamePassword(credentialsId: 'repo21', usernameVariable: 'REPO21_USER', passwordVariable: 'REPO21_PASSWORD'),
        string(credentialsId: 'repo21-url', variable: 'REPO21_URL')
    ]) {
            sh "docker login $REPO_NAME_21 -u=$REPO21_USER -p=$REPO21_PASSWORD"
       }
}