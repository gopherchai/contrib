package version

import (
	"fmt"
	"os"
)

var (
	version   = "Unknown"
	date      = "Unknown"
	author    = "Unknown"
	gitStatus = ""
	gitCommit = ""

//	service = "Unknown"
)

/*
use example as below:
#!/usr/bin/bash
export TAG=$(git tag -l | head -n 1)
export DATE=$(date +'%FT%T')
export AUTHOR=$(git log --pretty=format:"%an" | head -n 1)
export CUR_PWD=$(pwd)
export currentDir=$(cd $(dirname $0) && pwd)
export gitStatus=$(git status -s | wc -l | head -1)
export gitCommit=$(git log --pretty=oneline -n -1 | head -1 | awk '{print $1}')
LD_FLAGS="-X robot/version.gitCommit=$gitCommit -X robot/version.gitStatus=$gitStatus  -X robot/version.version=$TAG -X robot/version.date=$DATE -X robot/version.author=$AUTHOR -w -s"
echo $LD_FLAGS
go build -ldflags "$LD_FLAGS" -gcflags "-N" -i -o cmd
*/
// Version stdout version description
func Version() {
	fmt.Fprintf(os.Stdout, "commit:%s,status:%s version %s build at %s by %s\n", gitCommit, gitStatus, version, date, author)
}
