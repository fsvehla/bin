package main

import (
	"compress/zlib"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type gitBranch struct {
	name                string
	headCommitTimestamp int64
}

/* Implementation of the sort interface (Len, Swap, Less) for sorting gitBranches () */
type ByTimestampDesc []gitBranch

func (s ByTimestampDesc) Len() int { return len(s) }

func (s ByTimestampDesc) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByTimestampDesc) Less(i, j int) bool {
	return s[i].headCommitTimestamp > s[j].headCommitTimestamp
}

/* Gets the head commit of a given branch name or ref by
shelling out to Git. We need this, because we can’t (yet) read packed objects. */
func gitHeadCommitTimeByShellingOut(branchName string) int64 {
	cmd := exec.Command("git", "log", "-n1", "--format=%at", branchName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		panic(err)
	}

	outputWithoutNL := strings.TrimSpace(string(output))
	int, err := strconv.ParseInt(outputWithoutNL, 10, 64)

	if err != nil {
		panic(err)
	}

	return int
}

func listFilesInDirectory(path string) []string {
	fileInfos, err := ioutil.ReadDir(path)

	if err != nil {
		panic(err)
	}

	names := make([]string, 0)

	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			names = append(names, fileInfo.Name())
		}
	}

	return names
}

/* Git objects are stored in a Path */
func gitObjectPath(sha string) string {
	path := ".git/objects/"
	path += sha[0:2]
	path += "/"
	path += sha[2:]

	return path
}

/* Reads the git object file, which should be a commit and returns a string */
func getGitCommit(sha string) (string, error) {
	path := gitObjectPath(sha)

	fin, err := os.Open(path)

	if err != nil {
		return "", err
	}

	zlibReader, err := zlib.NewReader(fin)

	if err != nil {
		return "", err
	}

	decodedBytes, err := ioutil.ReadAll(zlibReader)

	if err != nil {
		return "", err
	}

	return string(decodedBytes), nil
}

// Returns the path to a path ref, local refs are in root/refs/heads, remote refs in root/refes/remotes/remote-name
func gitPathToBranchRef(name string) string {
	path := ".git/refs/"

	if strings.Contains(name, "/") {
		path += "remotes/"
	} else {
		path += "heads/"
	}

	path += name

	return path
}

/* Reads a ref file and returns the object */
func gitResolveRef(path string) string {
	contents, err := ioutil.ReadFile(gitPathToBranchRef(path))

	if err != nil {
		panic(err)
	}

	asString := string(contents)
	return asString[0 : len(asString)-1]
}

func gitNativeParseCommitTime(blob string) (time.Time, error) {
	lines := strings.Split(blob, "\n")

	for _, line := range lines {
		if len(line) > len("committer") && (line[0:(len("committer"))] == "committer") {
			// committer Ferdinand Svehla <f.svehla@gmail.com> 1353068842 +0100

			fields := strings.Fields(line)
			timeField := fields[len(fields)-2 : len(fields)-1][0]

			timeInt, err := strconv.ParseInt(timeField, 10, 32)

			if err != nil {
				panic(err)
			}

			unixTime := time.Unix(timeInt, 0)
			return unixTime, nil
		}
	}

	return time.Now(), errors.New("Committer not found in commit blob")
}

/* Returns the commit time of the commit at +sha+ */
func getCommitTime(sha string) (time.Time, error, bool) {
	commitOutput, err := getGitCommit(sha)

	if err != nil {
		/* The commit was packed and we need to fall back to git log */
		return time.Unix(gitHeadCommitTimeByShellingOut(sha), 0), nil, true
	}

	rTime, parseErr := gitNativeParseCommitTime(commitOutput)

	if parseErr != nil {
		panic(parseErr)
	}

	return rTime, nil, false
}

/* 8x faster than shelling out to git(1) */
func getBranchesList() []string {
	branchNames := make([]string, 0)

	for _, localBranchName := range listFilesInDirectory("./.git/refs/heads") {
		branchNames = append(branchNames, localBranchName)
	}

	/* TODO: Parse .git/config for a list of remotes */
	for _, originRemoteBranchName := range listFilesInDirectory("./.git/refs/remotes/origin") {
		if originRemoteBranchName[len(originRemoteBranchName)-4:] != "HEAD" {
			branchNames = append(branchNames, "origin/"+originRemoteBranchName)
		}
	}

	return branchNames
}

func GetLatestCommitDatesByShellingOut(branches []string) []gitBranch {
	results := make([]gitBranch, 0)
	queue := make(chan bool)

	for _, branchName := range branches {
		go func(name string) {
			results = append(results, gitBranch{name, gitHeadCommitTimeByShellingOut(name)})
			queue <- true
		}(branchName)
	}

	for i := 0; i < len(branches); i++ {
		<-queue
	}

	return results
}

func main() {
	branchesToLookUp := getBranchesList()
	fmt.Printf("Got %d branches to look up...\n\n", len(branchesToLookUp))

	startTime := time.Now()
	resultBranches := GetLatestCommitDatesByShellingOut(branchesToLookUp)
	elapsed := time.Since(startTime)

	fmt.Printf("Shelling out to git branch took: %q\n", elapsed)

	getHeadShaStart := time.Now()

	nativeQueue := make(chan bool)
	nativeResultBranches := make([]gitBranch, 0)
	fallbacks := 0

	for _, branchName := range branchesToLookUp {
		go func(name string) {
			ref := gitResolveRef(name)
			commitTime, error, neededToFallBackToShellOut := getCommitTime(ref)

			if error != nil {
				panic(error)
			}

			if neededToFallBackToShellOut == true {
				fallbacks += 1
			}

			branch := gitBranch{name, commitTime.Unix()}

			nativeResultBranches = append(nativeResultBranches, branch)

			nativeQueue <- true
		}(branchName)
	}

	for i := 0; i < len(branchesToLookUp); i++ {
		<-nativeQueue
	}

	fmt.Printf("Go version took: %q\n\n", time.Since(getHeadShaStart), len(nativeResultBranches))
	fmt.Printf("WARN: Fallback to shelling out befause of packed objects necessary in %d instances.\n\n", fallbacks)

	/* select results within the last 48 hours */
	hour := int64(60 * 60)
	unixNow := time.Now().Unix()

	branchesUpdatedWithin48hrs := make([]gitBranch, 0)

	for _, branch := range resultBranches {
		if branch.headCommitTimestamp > (unixNow - hour*140) {
			branchesUpdatedWithin48hrs = append(branchesUpdatedWithin48hrs, branch)
		}
	}

	sort.Sort(ByTimestampDesc(branchesUpdatedWithin48hrs))

	lengthOfLongestBranchName := 0

	for _, branch := range branchesUpdatedWithin48hrs {
		if len(branch.name) > lengthOfLongestBranchName {
			lengthOfLongestBranchName = len(branch.name)
		}
	}

	for _, branch := range branchesUpdatedWithin48hrs {
		diffSecs := unixNow - branch.headCommitTimestamp
		diffMins := diffSecs / 60
		diffHours := diffMins / 60
		restMins := diffMins - (diffHours * 60)

		timeTime := time.Unix(branch.headCommitTimestamp, 0)

		fmt.Printf(
			fmt.Sprintf("%%-%ds %%3dh %%02dm -- %%s\n", (lengthOfLongestBranchName+1)),
			branch.name,
			diffHours,
			restMins,
			timeTime.Format(time.RFC1123))
	}
}
