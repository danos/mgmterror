// Copyright (c) 2017-2019, AT&T Intellectual Property. All rights reserved.
//
// SPDX-License-Identifier: MPL-2.0
//
// Useful test functions for validating mgmterrors.  Wraps the management
// errors to allow for different formatting for CLI, RPC over netconf etc.

package errtest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/danos/mgmterror"
)

type ExpMgmtError struct {
	expMsgContents []string
	expPath        string
	expInfo        string
}

func NewExpMgmtError(msgs []string, path, info string) *ExpMgmtError {
	return &ExpMgmtError{
		expMsgContents: msgs, // Actual error should contain all these.
		expPath:        path, // Absolute match
		expInfo:        info, // May be empty
	}
}

// Rough and ready check that all parts of all warnings appear at some point
// in the log
func CheckMgmtErrorsInLog(
	t *testing.T,
	log bytes.Buffer,
	expWarns []*ExpMgmtError,
) {
	logStr := log.String()
	for _, expWarn := range expWarns {
		if !strings.Contains(logStr, expWarn.expPath) {
			t.Fatalf("Syslog doesn't contain path: %s\n", expWarn.expPath)
		}
		for _, msg := range expWarn.expMsgContents {
			if !strings.Contains(logStr, msg) {
				t.Fatalf("Syslog doesn't contain msg: %s\n", msg)
			}
		}
		if len(expWarn.expInfo) > 0 {
			if !strings.Contains(logStr, expWarn.expInfo) {
				t.Fatalf("Syslog doesn't contain info %s\n", expWarn.expInfo)
			}
		}
	}
}

func CheckMgmtErrors(
	t *testing.T,
	expMgmtErrs []*ExpMgmtError,
	actualErrs []error,
) {
	// Check all actual errors were expected.  We assume all actual errors
	// are mgmterror.Formattable - if not then you're using the wrong test
	// function!
	for _, actErr := range actualErrs {
		me, _ := actErr.(mgmterror.Formattable)

		found := false
	loop1:
		for _, expErr := range expMgmtErrs {
			if me.GetPath() != expErr.expPath {
				continue
			}
			if !checkInfoMatchesNonFatal(me, expErr.expInfo) {
				continue
			}
			for _, expMsg := range expErr.expMsgContents {
				if !strings.Contains(me.GetMessage(), expMsg) {
					continue loop1
				}
			}
			found = true
			break
		}
		if !found {
			t.Logf("Expecting: %v\n", expMgmtErrs[0])
			t.Fatalf(
				"Found unexpected error:\n\tPath:\t%s\n\tMsg:\t%s\nInfo:\t%s\n",
				me.GetPath(), me.GetMessage(), me.GetInfo())
			return
		}
	}

	// Now check all expected errors were seen.
	for _, expErr := range expMgmtErrs {
		found := false
	loop2:
		for _, actErr := range actualErrs {
			me, _ := actErr.(mgmterror.Formattable)
			if me.GetPath() != expErr.expPath {
				continue
			}
			if !checkInfoMatchesNonFatal(me, expErr.expInfo) {
				continue
			}
			for _, expMsg := range expErr.expMsgContents {
				if !strings.Contains(me.GetMessage(), expMsg) {
					continue loop2
				}
			}
			found = true
			break
		}
		if !found {
			t.Fatalf(
				"Error not found:\n\tPath:\t%s\n\tMsgs:\t%v\nInfo:\t%s\n",
				expErr.expPath, expErr.expMsgContents, expErr.expInfo)
			return
		}
	}
}

func CheckPath(t *testing.T, err error, expPath string) {
	me, ok := err.(mgmterror.Formattable)
	if !ok {
		t.Fatalf("Error does not meet Formattable interface!")
		return
	}

	if me.GetPath() != expPath {
		t.Fatalf("Path mismatch:\nExp:\t'%s'\nGot:\t'%s'\n",
			expPath, me.GetPath())
	}
}

func CheckMsg(t *testing.T, err error, expMsg string) {
	me, ok := err.(mgmterror.Formattable)
	if !ok {
		t.Fatalf("Error does not meet Formattable interface!")
		return
	}

	if me.GetMessage() != expMsg {
		t.Fatalf("Msg mismatch:\nExp:\t'%s'\nGot:\t'%s'\n",
			expMsg, me.GetMessage())
	}
}

func CheckInfo(t *testing.T, err error, expInfoVal string) {
	me, ok := err.(mgmterror.Formattable)
	if !ok {
		t.Fatalf("Error does not meet Formattable interface!")
		return
	}

	if expInfoVal == "" && len(me.GetInfo()) == 0 {
		// Nothing expected, nothing seen.  All clear.
		return
	}
	if expInfoVal == "" && len(me.GetInfo()) > 0 {
		t.Fatalf("Unexpected info value: '%s'\n", me.GetInfo()[0].Value)
		return
	}
	if expInfoVal != "" && len(me.GetInfo()) == 0 {
		t.Fatalf("No info value!\n")
		return
	}

	if me.GetInfo()[0].Value != expInfoVal {
		t.Fatalf("Info value mismatch:\nExp:\t'%s'\nGot:\t'%s'\n",
			expInfoVal, me.GetInfo()[0].Value)
	}
}

func checkInfoMatchesNonFatal(me mgmterror.Formattable, expInfoVal string) bool {
	if len(me.GetInfo()) == 0 {
		// If nothing expected, nothing seen, return ok, otherwise error
		return expInfoVal == ""
	}

	if me.GetInfo()[0].Value != expInfoVal {
		return false
	}
	return true
}

type xpathType int

const (
	xpathMust xpathType = iota
	xpathWhen
)

type TestError struct {
	t         *testing.T
	path      string
	rawMsgs   []string
	cliMsgs   []string
	rpcMsgs   []string
	setMsg    string
	setSuffix string // used when set error doesn't end with 'is not valid'
}

func (te *TestError) CliErrorStrings() []string {

	pathSlice := getPathSlice(te.t, te.path, "generic error")

	retStr := []string{fmt.Sprintf("%s", errpath(pathSlice))}
	return append(retStr, te.cliMsgs...)
}

func (te *TestError) CommitCliErrorStrings() []string {
	return te.cliMsgs
}

func (te *TestError) RpcErrorStrings() []string {
	if len(te.rpcMsgs) == 0 {
		te.t.Fatalf("Test error message has no 'rpcMsgs'")
		return nil
	}

	pathSlice := getPathSlice(te.t, te.path, "rpc error")

	retStr := []string{fmt.Sprintf("%s", errpath(pathSlice))}
	return append(retStr, te.rpcMsgs...)
}

// Standard messages for set errors are:
//
// Configuration path: <path with last/only element in []> is not valid
//
// <setMsg>
//
// !!!DO NOT CHANGE THIS FORMAT WITHOUT CONSULTATION!!!
//
func (te *TestError) SetCliErrorStrings() []string {
	if te.setMsg == "" {
		te.t.Fatalf("Test error message has no 'setmsg'")
		return nil
	}

	pathSlice := getPathSlice(te.t, te.path, "generic error")
	if te.setMsg == noMsgPrinted {
		return []string{fmt.Sprintf("%s %s %s",
			configPathStr, errpath(pathSlice), isNotValidStr),
		}
	}
	if te.setSuffix == "" {
		return []string{fmt.Sprintf("%s %s %s",
			configPathStr, errpath(pathSlice), isNotValidStr),
			te.setMsg,
		}
	}

	return []string{fmt.Sprintf("%s %s %s",
		configPathStr, errpath(pathSlice), te.setSuffix),
		te.setMsg,
	}
}

func (te *TestError) RawErrorStrings() []string {

	retStr := []string{te.path}
	return append(retStr, te.rawMsgs...)
}

func (te *TestError) RawErrorStringsNoPath() []string {

	retStr := []string{}
	return append(retStr, te.rawMsgs...)
}

func NewAccessDeniedError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:    t,
		path: path,
		rawMsgs: []string{"Access to the requested protocol operation " +
			"or data model is denied because authorization failed."},
	}
}

func NewInterfaceMustExistError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{"Interface must exist"},
		cliMsgs: []string{"Interface must exist"},
	}
}

func NewInvalidNodeError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{"An unexpected element is present"},
		cliMsgs: []string{"Configuration path", "is not valid"},
		rpcMsgs: []string{"is not valid"},
		setMsg:  noMsgPrinted,
	}
}

func NewInvalidNumElementsError(
	t *testing.T,
	path string,
	min, max int,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{fmt.Sprintf(wrongNumElementsFmtStr, min, max)},
		cliMsgs: []string{fmt.Sprintf(wrongNumElementsFmtStr, min, max)},
		setMsg:  noMsgPrinted,
	}
}

func NewInvalidRangeError(
	t *testing.T,
	path string,
	min, max int,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{fmt.Sprintf(wrongRangeFmtStr, min, max)},
		cliMsgs: []string{fmt.Sprintf(wrongRangeFmtStr, min, max)},
		setMsg:  fmt.Sprintf(wrongRangeFmtStr, min, max),
	}
}

func NewInvalidRangeCustomError(
	t *testing.T,
	path string,
	customErr string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{customErr},
		cliMsgs: []string{customErr},
		setMsg:  customErr,
	}
}

func NewInvalidPathError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{fmt.Sprintf("%s: %s", path, pathIsInvalidStr)},
		cliMsgs: []string{"TBD"},
		setMsg:  pathIsInvalidStr,
	}
}

func NewInvalidPatternError(
	t *testing.T,
	path string,
	pattern string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{fmt.Sprintf(mustMatchPatternFmtStr, pattern)},
		cliMsgs: []string{fmt.Sprintf(doesntMatchPatternFmtStr, pattern)},
		setMsg:  fmt.Sprintf(doesntMatchPatternFmtStr, pattern),
	}
}

func NewInvalidPatternCustomError(
	t *testing.T,
	path string,
	customErr string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{customErr},
		cliMsgs: []string{customErr},
		setMsg:  customErr,
	}
}

func NewInvalidTypeError(
	t *testing.T,
	path string,
	typ string,
) *TestError {
	pathSlice := getPathSlice(t, path, "invalid type")
	return &TestError{
		t:    t,
		path: path,
		rawMsgs: []string{fmt.Sprintf(
			wrongTypeFmtStr, pathSlice[len(pathSlice)-1], typ)},
		cliMsgs: []string{fmt.Sprintf(
			wrongTypeFmtStr, pathSlice[len(pathSlice)-1], typ)},
		setMsg: fmt.Sprintf(
			wrongTypeFmtStr, pathSlice[len(pathSlice)-1], typ),
	}
}

func NewInvalidLengthError(
	t *testing.T,
	path string,
	min, max int,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{fmt.Sprintf(wrongLengthFmtStr, min, max)},
		cliMsgs: []string{fmt.Sprintf(wrongLengthFmtStr, min, max)},
		setMsg:  fmt.Sprintf(wrongLengthFmtStr, min, max),
	}
}

func NewInvalidLengthCustomError(
	t *testing.T,
	path string,
	customErr string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{customErr},
		cliMsgs: []string{customErr},
		setMsg:  customErr,
	}
}

func NewLeafrefError(
	t *testing.T,
	path string,
	leafrefPath string,
) *TestError {
	return &TestError{
		t:    t,
		path: path,
		rawMsgs: []string{
			leafrefErrorStr, joinPathWithSpaces(
				getPathSlice(t, leafrefPath, "leafref"))},
		cliMsgs: []string{
			leafrefErrorStr, joinPathWithSpaces(
				getPathSlice(t, leafrefPath, "leafref"))},
	}
}

func NewMissingKeyError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{missingListKeyStr},
		cliMsgs: []string{missingListKeyStr},
		setMsg:  notYetTestedStr,
	}
}

func NewMissingMandatoryNodeError(
	t *testing.T,
	path string,
) *TestError {
	pathSlice := getPathSlice(t, path, "mandatory")
	if len(pathSlice) == 0 {
		t.Fatalf("Cannot have empty path for missing mandatory node error")
		return nil
	}
	return &TestError{
		t:    t,
		path: strings.Join(pathSlice[:len(pathSlice)-1], "/"),
		rawMsgs: []string{
			missingMandatoryStr + " " + pathSlice[len(pathSlice)-1]},
		cliMsgs: []string{
			missingMandatoryStr + " " + pathSlice[len(pathSlice)-1]},
	}
}

func NewNodeDoesntExistError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{nodeDoesntExistStr},
		cliMsgs: []string{nodeDoesntExistStr},
		setMsg:  nodeDoesntExistStr,
	}
}

func NewNodeExistsError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{nodeExistsStr},
		cliMsgs: []string{nodeExistsStr},
		setMsg:  nodeExistsStr,
	}
}

func NewNodeRequiresChildError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{notYetTestedStr},
		cliMsgs: []string{notYetTestedStr},
		setMsg:  nodeRequiresChildStr,
	}
}

func NewNodeRequiresValueError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{notYetTestedStr},
		cliMsgs: []string{notYetTestedStr},
		setMsg:  nodeRequiresValueStr,
	}
}

func NewNonUniquePathsError(
	t *testing.T,
	path string,
	keys []string,
	nonUniqueChildren []string,
) *TestError {
	return &TestError{
		t:    t,
		path: path,
		rawMsgs: []string{
			nonUniqueSetOfPathsStr,
			genChildPathsStr(nonUniqueChildren),
			nonUniqueSetOfKeysStr,
			genKeysStr(keys),
		},
		cliMsgs: []string{
			nonUniqueSetOfPathsStr,
			genChildPathsStr(nonUniqueChildren),
			nonUniqueSetOfKeysStr,
			genKeysStr(keys),
		},
	}
}

func NewPathAmbiguousError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:         t,
		path:      path,
		rawMsgs:   []string{"TBD"},
		cliMsgs:   []string{"TBD"},
		setSuffix: "is ambiguous",
		setMsg:    "Possible completions:",
	}
}

func NewSchemaMismatchError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{"Doesn't match schema"},
		cliMsgs: []string{"TBD"}, // TODO
	}
}

func NewSyntaxError(
	t *testing.T,
	path string,
	scriptErr string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{scriptErr},
		cliMsgs: []string{scriptErr},
	}
}

func NewUnknownElementError(
	t *testing.T,
	path string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{"Doesn't match schema"},
		cliMsgs: []string{"TBD"}, // TODO
		setMsg:  noMsgPrinted,
	}
}

func NewMustCustomError(
	t *testing.T,
	path,
	customError string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{path, customError},
		cliMsgs: []string{
			fmt.Sprintf("[%s]", joinPathWithSpaces(
				getPathSlice(t, path, "must custom"))),
			customError,
		},
	}
}

func NewWhenCustomError(
	t *testing.T,
	path,
	customError string,
) *TestError {
	return &TestError{
		t:       t,
		path:    path,
		rawMsgs: []string{path, customError},
		cliMsgs: []string{
			fmt.Sprintf("[%s]", joinPathWithSpaces(
				getPathSlice(t, path, "when custom"))),
			customError,
		},
	}
}

func NewMustDefaultError(
	t *testing.T,
	path,
	stmt string,
) *TestError {
	return &TestError{
		t:    t,
		path: path,
		rawMsgs: []string{
			path,
			fmt.Sprintf("'must' condition is false: '%s'", stmt),
		},
		cliMsgs: []string{
			fmt.Sprintf("[%s]", joinPathWithSpaces(
				getPathSlice(t, path, "must default"))),
			fmt.Sprintf("'must' condition is false: '%s'", stmt),
		},
	}
}

func NewWhenDefaultError(
	t *testing.T,
	path,
	stmt string,
) *TestError {
	return &TestError{
		t:    t,
		path: path,
		rawMsgs: []string{
			path,
			fmt.Sprintf("'when' condition is false: '%s'", stmt),
		},
		cliMsgs: []string{
			fmt.Sprintf("[%s]", joinPathWithSpaces(
				getPathSlice(t, path, "when default"))),
			fmt.Sprintf("'when' condition is false: '%s'", stmt),
		},
	}
}