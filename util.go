package cupaloy

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/pmezard/go-difflib/difflib"
)

var spewConfig = spew.ConfigState{
	Indent:                  "  ",
	SortKeys:                true, // maps should be spewed in a deterministic order
	DisablePointerAddresses: true, // don't spew the addresses of pointers
	DisableCapacities:       true, // don't spew capacities of collections
	SpewKeys:                true, // if unable to sort map keys then spew keys to strings and sort those
}

//go:generate $GOPATH/bin/mockery -output=examples -outpkg=examples_test -testonly -name=TestingT

// TestingT is a subset of the interface testing.TB allowing it to be mocked in tests.
type TestingT interface {
	Helper()
	Failed() bool
	Error(args ...interface{})
	Fatal(args ...interface{})
	Name() string
}

func getNameOfCaller() string {
	pc, _, _, _ := runtime.Caller(2) // first caller is the caller of this function, we want the caller of our caller
	fullPath := runtime.FuncForPC(pc).Name()
	packageFunctionName := filepath.Base(fullPath)

	return strings.Replace(getNameOfFunc(packageFunctionName), ".", "-", -1)
}

func getNameOfFunc(value string) string {
	index := strings.Index(value, ".Test")
	result := value
	if index != -1 {
		result = value[index+1 : len(value)]
	}
	return result
}

func envVariableSet(envVariable string) bool {
	_, varSet := os.LookupEnv(envVariable)
	return varSet
}

func (c *Config) snapshotFilePath(testName string) string {
	return filepath.Join(c.subDirName, testName+c.snapshotFileExtension)
}

// Legacy snapshot format where all items were spewed
func takeV1Snapshot(i ...interface{}) string {
	return spewConfig.Sdump(i...)
}

// New snapshot format where some types are written out raw to the file
func takeSnapshot(i ...interface{}) string {
	snapshot := &bytes.Buffer{}
	for _, v := range i {
		switch vt := v.(type) {
		case string:
			snapshot.WriteString(vt)
			snapshot.WriteString("\n")
		case []byte:
			snapshot.Write(vt)
			snapshot.WriteString("\n")
		default:
			spewConfig.Fdump(snapshot, v)
		}
	}

	return snapshot.String()
}

func (c *Config) readSnapshot(snapshotName string) (string, error) {
	snapshotFile := c.snapshotFilePath(snapshotName)
	buf, err := ioutil.ReadFile(snapshotFile)

	if os.IsNotExist(err) {
		return "", err
	}

	if err != nil {
		return "", err
	}

	return string(buf), nil
}

func (c *Config) updateSnapshot(snapshotName string, prevSnapshot string, snapshot string) error {
	// check that subdirectory exists before writing snapshot
	err := os.MkdirAll(c.subDirName, os.ModePerm)
	if err != nil {
		return errors.New("could not create snapshots directory")
	}

	snapshotFile := c.snapshotFilePath(snapshotName)
	_, err = os.Stat(snapshotFile)
	isNewSnapshot := os.IsNotExist(err)

	err = ioutil.WriteFile(snapshotFile, []byte(snapshot), os.FileMode(0644))
	if err != nil {
		return err
	}

	if !c.failOnUpdate {
		//TODO: should a warning still be printed here?
		return nil
	}

	snapshotDiff := diffSnapshots(prevSnapshot, snapshot)

	if isNewSnapshot {
		return ErrSnapshotCreated{
			Name:     snapshotName,
			Contents: snapshot,
		}
	}

	return ErrSnapshotUpdated{
		Name: snapshotName,
		Diff: snapshotDiff,
	}
}

func diffSnapshots(previous, current string) string {
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(previous),
		B:        difflib.SplitLines(current),
		FromFile: "Previous",
		FromDate: "",
		ToFile:   "Current",
		ToDate:   "",
		Context:  1,
	})

	return diff
}
