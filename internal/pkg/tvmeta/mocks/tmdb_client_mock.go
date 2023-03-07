package mocks

// Code generated by http://github.com/gojuno/minimock (dev). DO NOT EDIT.

//go:generate minimock -i github.com/Nikscorp/soap/internal/pkg/tvmeta.tmdbClient -o ./tmdb_client_mock.go -n TmdbClientMock

import (
	"sync"
	mm_atomic "sync/atomic"
	mm_time "time"

	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/gojuno/minimock/v3"
)

// TmdbClientMock implements tvmeta.tmdbClient
type TmdbClientMock struct {
	t minimock.Tester

	funcGetSearchTVShow          func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error)
	inspectFuncGetSearchTVShow   func(query string, urlOptions map[string]string)
	afterGetSearchTVShowCounter  uint64
	beforeGetSearchTVShowCounter uint64
	GetSearchTVShowMock          mTmdbClientMockGetSearchTVShow

	funcGetTVDetails          func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error)
	inspectFuncGetTVDetails   func(id int, urlOptions map[string]string)
	afterGetTVDetailsCounter  uint64
	beforeGetTVDetailsCounter uint64
	GetTVDetailsMock          mTmdbClientMockGetTVDetails

	funcGetTVSeasonDetails          func(id int, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error)
	inspectFuncGetTVSeasonDetails   func(id int, seasonNumber int, urlOptions map[string]string)
	afterGetTVSeasonDetailsCounter  uint64
	beforeGetTVSeasonDetailsCounter uint64
	GetTVSeasonDetailsMock          mTmdbClientMockGetTVSeasonDetails
}

// NewTmdbClientMock returns a mock for tvmeta.tmdbClient
func NewTmdbClientMock(t minimock.Tester) *TmdbClientMock {
	m := &TmdbClientMock{t: t}
	if controller, ok := t.(minimock.MockController); ok {
		controller.RegisterMocker(m)
	}

	m.GetSearchTVShowMock = mTmdbClientMockGetSearchTVShow{mock: m}
	m.GetSearchTVShowMock.callArgs = []*TmdbClientMockGetSearchTVShowParams{}

	m.GetTVDetailsMock = mTmdbClientMockGetTVDetails{mock: m}
	m.GetTVDetailsMock.callArgs = []*TmdbClientMockGetTVDetailsParams{}

	m.GetTVSeasonDetailsMock = mTmdbClientMockGetTVSeasonDetails{mock: m}
	m.GetTVSeasonDetailsMock.callArgs = []*TmdbClientMockGetTVSeasonDetailsParams{}

	return m
}

type mTmdbClientMockGetSearchTVShow struct {
	mock               *TmdbClientMock
	defaultExpectation *TmdbClientMockGetSearchTVShowExpectation
	expectations       []*TmdbClientMockGetSearchTVShowExpectation

	callArgs []*TmdbClientMockGetSearchTVShowParams
	mutex    sync.RWMutex
}

// TmdbClientMockGetSearchTVShowExpectation specifies expectation struct of the tmdbClient.GetSearchTVShow
type TmdbClientMockGetSearchTVShowExpectation struct {
	mock    *TmdbClientMock
	params  *TmdbClientMockGetSearchTVShowParams
	results *TmdbClientMockGetSearchTVShowResults
	Counter uint64
}

// TmdbClientMockGetSearchTVShowParams contains parameters of the tmdbClient.GetSearchTVShow
type TmdbClientMockGetSearchTVShowParams struct {
	query      string
	urlOptions map[string]string
}

// TmdbClientMockGetSearchTVShowResults contains results of the tmdbClient.GetSearchTVShow
type TmdbClientMockGetSearchTVShowResults struct {
	sp1 *tmdb.SearchTVShows
	err error
}

// Expect sets up expected params for tmdbClient.GetSearchTVShow
func (mmGetSearchTVShow *mTmdbClientMockGetSearchTVShow) Expect(query string, urlOptions map[string]string) *mTmdbClientMockGetSearchTVShow {
	if mmGetSearchTVShow.mock.funcGetSearchTVShow != nil {
		mmGetSearchTVShow.mock.t.Fatalf("TmdbClientMock.GetSearchTVShow mock is already set by Set")
	}

	if mmGetSearchTVShow.defaultExpectation == nil {
		mmGetSearchTVShow.defaultExpectation = &TmdbClientMockGetSearchTVShowExpectation{}
	}

	mmGetSearchTVShow.defaultExpectation.params = &TmdbClientMockGetSearchTVShowParams{query, urlOptions}
	for _, e := range mmGetSearchTVShow.expectations {
		if minimock.Equal(e.params, mmGetSearchTVShow.defaultExpectation.params) {
			mmGetSearchTVShow.mock.t.Fatalf("Expectation set by When has same params: %#v", *mmGetSearchTVShow.defaultExpectation.params)
		}
	}

	return mmGetSearchTVShow
}

// Inspect accepts an inspector function that has same arguments as the tmdbClient.GetSearchTVShow
func (mmGetSearchTVShow *mTmdbClientMockGetSearchTVShow) Inspect(f func(query string, urlOptions map[string]string)) *mTmdbClientMockGetSearchTVShow {
	if mmGetSearchTVShow.mock.inspectFuncGetSearchTVShow != nil {
		mmGetSearchTVShow.mock.t.Fatalf("Inspect function is already set for TmdbClientMock.GetSearchTVShow")
	}

	mmGetSearchTVShow.mock.inspectFuncGetSearchTVShow = f

	return mmGetSearchTVShow
}

// Return sets up results that will be returned by tmdbClient.GetSearchTVShow
func (mmGetSearchTVShow *mTmdbClientMockGetSearchTVShow) Return(sp1 *tmdb.SearchTVShows, err error) *TmdbClientMock {
	if mmGetSearchTVShow.mock.funcGetSearchTVShow != nil {
		mmGetSearchTVShow.mock.t.Fatalf("TmdbClientMock.GetSearchTVShow mock is already set by Set")
	}

	if mmGetSearchTVShow.defaultExpectation == nil {
		mmGetSearchTVShow.defaultExpectation = &TmdbClientMockGetSearchTVShowExpectation{mock: mmGetSearchTVShow.mock}
	}
	mmGetSearchTVShow.defaultExpectation.results = &TmdbClientMockGetSearchTVShowResults{sp1, err}
	return mmGetSearchTVShow.mock
}

// Set uses given function f to mock the tmdbClient.GetSearchTVShow method
func (mmGetSearchTVShow *mTmdbClientMockGetSearchTVShow) Set(f func(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error)) *TmdbClientMock {
	if mmGetSearchTVShow.defaultExpectation != nil {
		mmGetSearchTVShow.mock.t.Fatalf("Default expectation is already set for the tmdbClient.GetSearchTVShow method")
	}

	if len(mmGetSearchTVShow.expectations) > 0 {
		mmGetSearchTVShow.mock.t.Fatalf("Some expectations are already set for the tmdbClient.GetSearchTVShow method")
	}

	mmGetSearchTVShow.mock.funcGetSearchTVShow = f
	return mmGetSearchTVShow.mock
}

// When sets expectation for the tmdbClient.GetSearchTVShow which will trigger the result defined by the following
// Then helper
func (mmGetSearchTVShow *mTmdbClientMockGetSearchTVShow) When(query string, urlOptions map[string]string) *TmdbClientMockGetSearchTVShowExpectation {
	if mmGetSearchTVShow.mock.funcGetSearchTVShow != nil {
		mmGetSearchTVShow.mock.t.Fatalf("TmdbClientMock.GetSearchTVShow mock is already set by Set")
	}

	expectation := &TmdbClientMockGetSearchTVShowExpectation{
		mock:   mmGetSearchTVShow.mock,
		params: &TmdbClientMockGetSearchTVShowParams{query, urlOptions},
	}
	mmGetSearchTVShow.expectations = append(mmGetSearchTVShow.expectations, expectation)
	return expectation
}

// Then sets up tmdbClient.GetSearchTVShow return parameters for the expectation previously defined by the When method
func (e *TmdbClientMockGetSearchTVShowExpectation) Then(sp1 *tmdb.SearchTVShows, err error) *TmdbClientMock {
	e.results = &TmdbClientMockGetSearchTVShowResults{sp1, err}
	return e.mock
}

// GetSearchTVShow implements tvmeta.tmdbClient
func (mmGetSearchTVShow *TmdbClientMock) GetSearchTVShow(query string, urlOptions map[string]string) (sp1 *tmdb.SearchTVShows, err error) {
	mm_atomic.AddUint64(&mmGetSearchTVShow.beforeGetSearchTVShowCounter, 1)
	defer mm_atomic.AddUint64(&mmGetSearchTVShow.afterGetSearchTVShowCounter, 1)

	if mmGetSearchTVShow.inspectFuncGetSearchTVShow != nil {
		mmGetSearchTVShow.inspectFuncGetSearchTVShow(query, urlOptions)
	}

	mm_params := &TmdbClientMockGetSearchTVShowParams{query, urlOptions}

	// Record call args
	mmGetSearchTVShow.GetSearchTVShowMock.mutex.Lock()
	mmGetSearchTVShow.GetSearchTVShowMock.callArgs = append(mmGetSearchTVShow.GetSearchTVShowMock.callArgs, mm_params)
	mmGetSearchTVShow.GetSearchTVShowMock.mutex.Unlock()

	for _, e := range mmGetSearchTVShow.GetSearchTVShowMock.expectations {
		if minimock.Equal(e.params, mm_params) {
			mm_atomic.AddUint64(&e.Counter, 1)
			return e.results.sp1, e.results.err
		}
	}

	if mmGetSearchTVShow.GetSearchTVShowMock.defaultExpectation != nil {
		mm_atomic.AddUint64(&mmGetSearchTVShow.GetSearchTVShowMock.defaultExpectation.Counter, 1)
		mm_want := mmGetSearchTVShow.GetSearchTVShowMock.defaultExpectation.params
		mm_got := TmdbClientMockGetSearchTVShowParams{query, urlOptions}
		if mm_want != nil && !minimock.Equal(*mm_want, mm_got) {
			mmGetSearchTVShow.t.Errorf("TmdbClientMock.GetSearchTVShow got unexpected parameters, want: %#v, got: %#v%s\n", *mm_want, mm_got, minimock.Diff(*mm_want, mm_got))
		}

		mm_results := mmGetSearchTVShow.GetSearchTVShowMock.defaultExpectation.results
		if mm_results == nil {
			mmGetSearchTVShow.t.Fatal("No results are set for the TmdbClientMock.GetSearchTVShow")
		}
		return (*mm_results).sp1, (*mm_results).err
	}
	if mmGetSearchTVShow.funcGetSearchTVShow != nil {
		return mmGetSearchTVShow.funcGetSearchTVShow(query, urlOptions)
	}
	mmGetSearchTVShow.t.Fatalf("Unexpected call to TmdbClientMock.GetSearchTVShow. %v %v", query, urlOptions)
	return
}

// GetSearchTVShowAfterCounter returns a count of finished TmdbClientMock.GetSearchTVShow invocations
func (mmGetSearchTVShow *TmdbClientMock) GetSearchTVShowAfterCounter() uint64 {
	return mm_atomic.LoadUint64(&mmGetSearchTVShow.afterGetSearchTVShowCounter)
}

// GetSearchTVShowBeforeCounter returns a count of TmdbClientMock.GetSearchTVShow invocations
func (mmGetSearchTVShow *TmdbClientMock) GetSearchTVShowBeforeCounter() uint64 {
	return mm_atomic.LoadUint64(&mmGetSearchTVShow.beforeGetSearchTVShowCounter)
}

// Calls returns a list of arguments used in each call to TmdbClientMock.GetSearchTVShow.
// The list is in the same order as the calls were made (i.e. recent calls have a higher index)
func (mmGetSearchTVShow *mTmdbClientMockGetSearchTVShow) Calls() []*TmdbClientMockGetSearchTVShowParams {
	mmGetSearchTVShow.mutex.RLock()

	argCopy := make([]*TmdbClientMockGetSearchTVShowParams, len(mmGetSearchTVShow.callArgs))
	copy(argCopy, mmGetSearchTVShow.callArgs)

	mmGetSearchTVShow.mutex.RUnlock()

	return argCopy
}

// MinimockGetSearchTVShowDone returns true if the count of the GetSearchTVShow invocations corresponds
// the number of defined expectations
func (m *TmdbClientMock) MinimockGetSearchTVShowDone() bool {
	for _, e := range m.GetSearchTVShowMock.expectations {
		if mm_atomic.LoadUint64(&e.Counter) < 1 {
			return false
		}
	}

	// if default expectation was set then invocations count should be greater than zero
	if m.GetSearchTVShowMock.defaultExpectation != nil && mm_atomic.LoadUint64(&m.afterGetSearchTVShowCounter) < 1 {
		return false
	}
	// if func was set then invocations count should be greater than zero
	if m.funcGetSearchTVShow != nil && mm_atomic.LoadUint64(&m.afterGetSearchTVShowCounter) < 1 {
		return false
	}
	return true
}

// MinimockGetSearchTVShowInspect logs each unmet expectation
func (m *TmdbClientMock) MinimockGetSearchTVShowInspect() {
	for _, e := range m.GetSearchTVShowMock.expectations {
		if mm_atomic.LoadUint64(&e.Counter) < 1 {
			m.t.Errorf("Expected call to TmdbClientMock.GetSearchTVShow with params: %#v", *e.params)
		}
	}

	// if default expectation was set then invocations count should be greater than zero
	if m.GetSearchTVShowMock.defaultExpectation != nil && mm_atomic.LoadUint64(&m.afterGetSearchTVShowCounter) < 1 {
		if m.GetSearchTVShowMock.defaultExpectation.params == nil {
			m.t.Error("Expected call to TmdbClientMock.GetSearchTVShow")
		} else {
			m.t.Errorf("Expected call to TmdbClientMock.GetSearchTVShow with params: %#v", *m.GetSearchTVShowMock.defaultExpectation.params)
		}
	}
	// if func was set then invocations count should be greater than zero
	if m.funcGetSearchTVShow != nil && mm_atomic.LoadUint64(&m.afterGetSearchTVShowCounter) < 1 {
		m.t.Error("Expected call to TmdbClientMock.GetSearchTVShow")
	}
}

type mTmdbClientMockGetTVDetails struct {
	mock               *TmdbClientMock
	defaultExpectation *TmdbClientMockGetTVDetailsExpectation
	expectations       []*TmdbClientMockGetTVDetailsExpectation

	callArgs []*TmdbClientMockGetTVDetailsParams
	mutex    sync.RWMutex
}

// TmdbClientMockGetTVDetailsExpectation specifies expectation struct of the tmdbClient.GetTVDetails
type TmdbClientMockGetTVDetailsExpectation struct {
	mock    *TmdbClientMock
	params  *TmdbClientMockGetTVDetailsParams
	results *TmdbClientMockGetTVDetailsResults
	Counter uint64
}

// TmdbClientMockGetTVDetailsParams contains parameters of the tmdbClient.GetTVDetails
type TmdbClientMockGetTVDetailsParams struct {
	id         int
	urlOptions map[string]string
}

// TmdbClientMockGetTVDetailsResults contains results of the tmdbClient.GetTVDetails
type TmdbClientMockGetTVDetailsResults struct {
	tp1 *tmdb.TVDetails
	err error
}

// Expect sets up expected params for tmdbClient.GetTVDetails
func (mmGetTVDetails *mTmdbClientMockGetTVDetails) Expect(id int, urlOptions map[string]string) *mTmdbClientMockGetTVDetails {
	if mmGetTVDetails.mock.funcGetTVDetails != nil {
		mmGetTVDetails.mock.t.Fatalf("TmdbClientMock.GetTVDetails mock is already set by Set")
	}

	if mmGetTVDetails.defaultExpectation == nil {
		mmGetTVDetails.defaultExpectation = &TmdbClientMockGetTVDetailsExpectation{}
	}

	mmGetTVDetails.defaultExpectation.params = &TmdbClientMockGetTVDetailsParams{id, urlOptions}
	for _, e := range mmGetTVDetails.expectations {
		if minimock.Equal(e.params, mmGetTVDetails.defaultExpectation.params) {
			mmGetTVDetails.mock.t.Fatalf("Expectation set by When has same params: %#v", *mmGetTVDetails.defaultExpectation.params)
		}
	}

	return mmGetTVDetails
}

// Inspect accepts an inspector function that has same arguments as the tmdbClient.GetTVDetails
func (mmGetTVDetails *mTmdbClientMockGetTVDetails) Inspect(f func(id int, urlOptions map[string]string)) *mTmdbClientMockGetTVDetails {
	if mmGetTVDetails.mock.inspectFuncGetTVDetails != nil {
		mmGetTVDetails.mock.t.Fatalf("Inspect function is already set for TmdbClientMock.GetTVDetails")
	}

	mmGetTVDetails.mock.inspectFuncGetTVDetails = f

	return mmGetTVDetails
}

// Return sets up results that will be returned by tmdbClient.GetTVDetails
func (mmGetTVDetails *mTmdbClientMockGetTVDetails) Return(tp1 *tmdb.TVDetails, err error) *TmdbClientMock {
	if mmGetTVDetails.mock.funcGetTVDetails != nil {
		mmGetTVDetails.mock.t.Fatalf("TmdbClientMock.GetTVDetails mock is already set by Set")
	}

	if mmGetTVDetails.defaultExpectation == nil {
		mmGetTVDetails.defaultExpectation = &TmdbClientMockGetTVDetailsExpectation{mock: mmGetTVDetails.mock}
	}
	mmGetTVDetails.defaultExpectation.results = &TmdbClientMockGetTVDetailsResults{tp1, err}
	return mmGetTVDetails.mock
}

// Set uses given function f to mock the tmdbClient.GetTVDetails method
func (mmGetTVDetails *mTmdbClientMockGetTVDetails) Set(f func(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error)) *TmdbClientMock {
	if mmGetTVDetails.defaultExpectation != nil {
		mmGetTVDetails.mock.t.Fatalf("Default expectation is already set for the tmdbClient.GetTVDetails method")
	}

	if len(mmGetTVDetails.expectations) > 0 {
		mmGetTVDetails.mock.t.Fatalf("Some expectations are already set for the tmdbClient.GetTVDetails method")
	}

	mmGetTVDetails.mock.funcGetTVDetails = f
	return mmGetTVDetails.mock
}

// When sets expectation for the tmdbClient.GetTVDetails which will trigger the result defined by the following
// Then helper
func (mmGetTVDetails *mTmdbClientMockGetTVDetails) When(id int, urlOptions map[string]string) *TmdbClientMockGetTVDetailsExpectation {
	if mmGetTVDetails.mock.funcGetTVDetails != nil {
		mmGetTVDetails.mock.t.Fatalf("TmdbClientMock.GetTVDetails mock is already set by Set")
	}

	expectation := &TmdbClientMockGetTVDetailsExpectation{
		mock:   mmGetTVDetails.mock,
		params: &TmdbClientMockGetTVDetailsParams{id, urlOptions},
	}
	mmGetTVDetails.expectations = append(mmGetTVDetails.expectations, expectation)
	return expectation
}

// Then sets up tmdbClient.GetTVDetails return parameters for the expectation previously defined by the When method
func (e *TmdbClientMockGetTVDetailsExpectation) Then(tp1 *tmdb.TVDetails, err error) *TmdbClientMock {
	e.results = &TmdbClientMockGetTVDetailsResults{tp1, err}
	return e.mock
}

// GetTVDetails implements tvmeta.tmdbClient
func (mmGetTVDetails *TmdbClientMock) GetTVDetails(id int, urlOptions map[string]string) (tp1 *tmdb.TVDetails, err error) {
	mm_atomic.AddUint64(&mmGetTVDetails.beforeGetTVDetailsCounter, 1)
	defer mm_atomic.AddUint64(&mmGetTVDetails.afterGetTVDetailsCounter, 1)

	if mmGetTVDetails.inspectFuncGetTVDetails != nil {
		mmGetTVDetails.inspectFuncGetTVDetails(id, urlOptions)
	}

	mm_params := &TmdbClientMockGetTVDetailsParams{id, urlOptions}

	// Record call args
	mmGetTVDetails.GetTVDetailsMock.mutex.Lock()
	mmGetTVDetails.GetTVDetailsMock.callArgs = append(mmGetTVDetails.GetTVDetailsMock.callArgs, mm_params)
	mmGetTVDetails.GetTVDetailsMock.mutex.Unlock()

	for _, e := range mmGetTVDetails.GetTVDetailsMock.expectations {
		if minimock.Equal(e.params, mm_params) {
			mm_atomic.AddUint64(&e.Counter, 1)
			return e.results.tp1, e.results.err
		}
	}

	if mmGetTVDetails.GetTVDetailsMock.defaultExpectation != nil {
		mm_atomic.AddUint64(&mmGetTVDetails.GetTVDetailsMock.defaultExpectation.Counter, 1)
		mm_want := mmGetTVDetails.GetTVDetailsMock.defaultExpectation.params
		mm_got := TmdbClientMockGetTVDetailsParams{id, urlOptions}
		if mm_want != nil && !minimock.Equal(*mm_want, mm_got) {
			mmGetTVDetails.t.Errorf("TmdbClientMock.GetTVDetails got unexpected parameters, want: %#v, got: %#v%s\n", *mm_want, mm_got, minimock.Diff(*mm_want, mm_got))
		}

		mm_results := mmGetTVDetails.GetTVDetailsMock.defaultExpectation.results
		if mm_results == nil {
			mmGetTVDetails.t.Fatal("No results are set for the TmdbClientMock.GetTVDetails")
		}
		return (*mm_results).tp1, (*mm_results).err
	}
	if mmGetTVDetails.funcGetTVDetails != nil {
		return mmGetTVDetails.funcGetTVDetails(id, urlOptions)
	}
	mmGetTVDetails.t.Fatalf("Unexpected call to TmdbClientMock.GetTVDetails. %v %v", id, urlOptions)
	return
}

// GetTVDetailsAfterCounter returns a count of finished TmdbClientMock.GetTVDetails invocations
func (mmGetTVDetails *TmdbClientMock) GetTVDetailsAfterCounter() uint64 {
	return mm_atomic.LoadUint64(&mmGetTVDetails.afterGetTVDetailsCounter)
}

// GetTVDetailsBeforeCounter returns a count of TmdbClientMock.GetTVDetails invocations
func (mmGetTVDetails *TmdbClientMock) GetTVDetailsBeforeCounter() uint64 {
	return mm_atomic.LoadUint64(&mmGetTVDetails.beforeGetTVDetailsCounter)
}

// Calls returns a list of arguments used in each call to TmdbClientMock.GetTVDetails.
// The list is in the same order as the calls were made (i.e. recent calls have a higher index)
func (mmGetTVDetails *mTmdbClientMockGetTVDetails) Calls() []*TmdbClientMockGetTVDetailsParams {
	mmGetTVDetails.mutex.RLock()

	argCopy := make([]*TmdbClientMockGetTVDetailsParams, len(mmGetTVDetails.callArgs))
	copy(argCopy, mmGetTVDetails.callArgs)

	mmGetTVDetails.mutex.RUnlock()

	return argCopy
}

// MinimockGetTVDetailsDone returns true if the count of the GetTVDetails invocations corresponds
// the number of defined expectations
func (m *TmdbClientMock) MinimockGetTVDetailsDone() bool {
	for _, e := range m.GetTVDetailsMock.expectations {
		if mm_atomic.LoadUint64(&e.Counter) < 1 {
			return false
		}
	}

	// if default expectation was set then invocations count should be greater than zero
	if m.GetTVDetailsMock.defaultExpectation != nil && mm_atomic.LoadUint64(&m.afterGetTVDetailsCounter) < 1 {
		return false
	}
	// if func was set then invocations count should be greater than zero
	if m.funcGetTVDetails != nil && mm_atomic.LoadUint64(&m.afterGetTVDetailsCounter) < 1 {
		return false
	}
	return true
}

// MinimockGetTVDetailsInspect logs each unmet expectation
func (m *TmdbClientMock) MinimockGetTVDetailsInspect() {
	for _, e := range m.GetTVDetailsMock.expectations {
		if mm_atomic.LoadUint64(&e.Counter) < 1 {
			m.t.Errorf("Expected call to TmdbClientMock.GetTVDetails with params: %#v", *e.params)
		}
	}

	// if default expectation was set then invocations count should be greater than zero
	if m.GetTVDetailsMock.defaultExpectation != nil && mm_atomic.LoadUint64(&m.afterGetTVDetailsCounter) < 1 {
		if m.GetTVDetailsMock.defaultExpectation.params == nil {
			m.t.Error("Expected call to TmdbClientMock.GetTVDetails")
		} else {
			m.t.Errorf("Expected call to TmdbClientMock.GetTVDetails with params: %#v", *m.GetTVDetailsMock.defaultExpectation.params)
		}
	}
	// if func was set then invocations count should be greater than zero
	if m.funcGetTVDetails != nil && mm_atomic.LoadUint64(&m.afterGetTVDetailsCounter) < 1 {
		m.t.Error("Expected call to TmdbClientMock.GetTVDetails")
	}
}

type mTmdbClientMockGetTVSeasonDetails struct {
	mock               *TmdbClientMock
	defaultExpectation *TmdbClientMockGetTVSeasonDetailsExpectation
	expectations       []*TmdbClientMockGetTVSeasonDetailsExpectation

	callArgs []*TmdbClientMockGetTVSeasonDetailsParams
	mutex    sync.RWMutex
}

// TmdbClientMockGetTVSeasonDetailsExpectation specifies expectation struct of the tmdbClient.GetTVSeasonDetails
type TmdbClientMockGetTVSeasonDetailsExpectation struct {
	mock    *TmdbClientMock
	params  *TmdbClientMockGetTVSeasonDetailsParams
	results *TmdbClientMockGetTVSeasonDetailsResults
	Counter uint64
}

// TmdbClientMockGetTVSeasonDetailsParams contains parameters of the tmdbClient.GetTVSeasonDetails
type TmdbClientMockGetTVSeasonDetailsParams struct {
	id           int
	seasonNumber int
	urlOptions   map[string]string
}

// TmdbClientMockGetTVSeasonDetailsResults contains results of the tmdbClient.GetTVSeasonDetails
type TmdbClientMockGetTVSeasonDetailsResults struct {
	tp1 *tmdb.TVSeasonDetails
	err error
}

// Expect sets up expected params for tmdbClient.GetTVSeasonDetails
func (mmGetTVSeasonDetails *mTmdbClientMockGetTVSeasonDetails) Expect(id int, seasonNumber int, urlOptions map[string]string) *mTmdbClientMockGetTVSeasonDetails {
	if mmGetTVSeasonDetails.mock.funcGetTVSeasonDetails != nil {
		mmGetTVSeasonDetails.mock.t.Fatalf("TmdbClientMock.GetTVSeasonDetails mock is already set by Set")
	}

	if mmGetTVSeasonDetails.defaultExpectation == nil {
		mmGetTVSeasonDetails.defaultExpectation = &TmdbClientMockGetTVSeasonDetailsExpectation{}
	}

	mmGetTVSeasonDetails.defaultExpectation.params = &TmdbClientMockGetTVSeasonDetailsParams{id, seasonNumber, urlOptions}
	for _, e := range mmGetTVSeasonDetails.expectations {
		if minimock.Equal(e.params, mmGetTVSeasonDetails.defaultExpectation.params) {
			mmGetTVSeasonDetails.mock.t.Fatalf("Expectation set by When has same params: %#v", *mmGetTVSeasonDetails.defaultExpectation.params)
		}
	}

	return mmGetTVSeasonDetails
}

// Inspect accepts an inspector function that has same arguments as the tmdbClient.GetTVSeasonDetails
func (mmGetTVSeasonDetails *mTmdbClientMockGetTVSeasonDetails) Inspect(f func(id int, seasonNumber int, urlOptions map[string]string)) *mTmdbClientMockGetTVSeasonDetails {
	if mmGetTVSeasonDetails.mock.inspectFuncGetTVSeasonDetails != nil {
		mmGetTVSeasonDetails.mock.t.Fatalf("Inspect function is already set for TmdbClientMock.GetTVSeasonDetails")
	}

	mmGetTVSeasonDetails.mock.inspectFuncGetTVSeasonDetails = f

	return mmGetTVSeasonDetails
}

// Return sets up results that will be returned by tmdbClient.GetTVSeasonDetails
func (mmGetTVSeasonDetails *mTmdbClientMockGetTVSeasonDetails) Return(tp1 *tmdb.TVSeasonDetails, err error) *TmdbClientMock {
	if mmGetTVSeasonDetails.mock.funcGetTVSeasonDetails != nil {
		mmGetTVSeasonDetails.mock.t.Fatalf("TmdbClientMock.GetTVSeasonDetails mock is already set by Set")
	}

	if mmGetTVSeasonDetails.defaultExpectation == nil {
		mmGetTVSeasonDetails.defaultExpectation = &TmdbClientMockGetTVSeasonDetailsExpectation{mock: mmGetTVSeasonDetails.mock}
	}
	mmGetTVSeasonDetails.defaultExpectation.results = &TmdbClientMockGetTVSeasonDetailsResults{tp1, err}
	return mmGetTVSeasonDetails.mock
}

// Set uses given function f to mock the tmdbClient.GetTVSeasonDetails method
func (mmGetTVSeasonDetails *mTmdbClientMockGetTVSeasonDetails) Set(f func(id int, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error)) *TmdbClientMock {
	if mmGetTVSeasonDetails.defaultExpectation != nil {
		mmGetTVSeasonDetails.mock.t.Fatalf("Default expectation is already set for the tmdbClient.GetTVSeasonDetails method")
	}

	if len(mmGetTVSeasonDetails.expectations) > 0 {
		mmGetTVSeasonDetails.mock.t.Fatalf("Some expectations are already set for the tmdbClient.GetTVSeasonDetails method")
	}

	mmGetTVSeasonDetails.mock.funcGetTVSeasonDetails = f
	return mmGetTVSeasonDetails.mock
}

// When sets expectation for the tmdbClient.GetTVSeasonDetails which will trigger the result defined by the following
// Then helper
func (mmGetTVSeasonDetails *mTmdbClientMockGetTVSeasonDetails) When(id int, seasonNumber int, urlOptions map[string]string) *TmdbClientMockGetTVSeasonDetailsExpectation {
	if mmGetTVSeasonDetails.mock.funcGetTVSeasonDetails != nil {
		mmGetTVSeasonDetails.mock.t.Fatalf("TmdbClientMock.GetTVSeasonDetails mock is already set by Set")
	}

	expectation := &TmdbClientMockGetTVSeasonDetailsExpectation{
		mock:   mmGetTVSeasonDetails.mock,
		params: &TmdbClientMockGetTVSeasonDetailsParams{id, seasonNumber, urlOptions},
	}
	mmGetTVSeasonDetails.expectations = append(mmGetTVSeasonDetails.expectations, expectation)
	return expectation
}

// Then sets up tmdbClient.GetTVSeasonDetails return parameters for the expectation previously defined by the When method
func (e *TmdbClientMockGetTVSeasonDetailsExpectation) Then(tp1 *tmdb.TVSeasonDetails, err error) *TmdbClientMock {
	e.results = &TmdbClientMockGetTVSeasonDetailsResults{tp1, err}
	return e.mock
}

// GetTVSeasonDetails implements tvmeta.tmdbClient
func (mmGetTVSeasonDetails *TmdbClientMock) GetTVSeasonDetails(id int, seasonNumber int, urlOptions map[string]string) (tp1 *tmdb.TVSeasonDetails, err error) {
	mm_atomic.AddUint64(&mmGetTVSeasonDetails.beforeGetTVSeasonDetailsCounter, 1)
	defer mm_atomic.AddUint64(&mmGetTVSeasonDetails.afterGetTVSeasonDetailsCounter, 1)

	if mmGetTVSeasonDetails.inspectFuncGetTVSeasonDetails != nil {
		mmGetTVSeasonDetails.inspectFuncGetTVSeasonDetails(id, seasonNumber, urlOptions)
	}

	mm_params := &TmdbClientMockGetTVSeasonDetailsParams{id, seasonNumber, urlOptions}

	// Record call args
	mmGetTVSeasonDetails.GetTVSeasonDetailsMock.mutex.Lock()
	mmGetTVSeasonDetails.GetTVSeasonDetailsMock.callArgs = append(mmGetTVSeasonDetails.GetTVSeasonDetailsMock.callArgs, mm_params)
	mmGetTVSeasonDetails.GetTVSeasonDetailsMock.mutex.Unlock()

	for _, e := range mmGetTVSeasonDetails.GetTVSeasonDetailsMock.expectations {
		if minimock.Equal(e.params, mm_params) {
			mm_atomic.AddUint64(&e.Counter, 1)
			return e.results.tp1, e.results.err
		}
	}

	if mmGetTVSeasonDetails.GetTVSeasonDetailsMock.defaultExpectation != nil {
		mm_atomic.AddUint64(&mmGetTVSeasonDetails.GetTVSeasonDetailsMock.defaultExpectation.Counter, 1)
		mm_want := mmGetTVSeasonDetails.GetTVSeasonDetailsMock.defaultExpectation.params
		mm_got := TmdbClientMockGetTVSeasonDetailsParams{id, seasonNumber, urlOptions}
		if mm_want != nil && !minimock.Equal(*mm_want, mm_got) {
			mmGetTVSeasonDetails.t.Errorf("TmdbClientMock.GetTVSeasonDetails got unexpected parameters, want: %#v, got: %#v%s\n", *mm_want, mm_got, minimock.Diff(*mm_want, mm_got))
		}

		mm_results := mmGetTVSeasonDetails.GetTVSeasonDetailsMock.defaultExpectation.results
		if mm_results == nil {
			mmGetTVSeasonDetails.t.Fatal("No results are set for the TmdbClientMock.GetTVSeasonDetails")
		}
		return (*mm_results).tp1, (*mm_results).err
	}
	if mmGetTVSeasonDetails.funcGetTVSeasonDetails != nil {
		return mmGetTVSeasonDetails.funcGetTVSeasonDetails(id, seasonNumber, urlOptions)
	}
	mmGetTVSeasonDetails.t.Fatalf("Unexpected call to TmdbClientMock.GetTVSeasonDetails. %v %v %v", id, seasonNumber, urlOptions)
	return
}

// GetTVSeasonDetailsAfterCounter returns a count of finished TmdbClientMock.GetTVSeasonDetails invocations
func (mmGetTVSeasonDetails *TmdbClientMock) GetTVSeasonDetailsAfterCounter() uint64 {
	return mm_atomic.LoadUint64(&mmGetTVSeasonDetails.afterGetTVSeasonDetailsCounter)
}

// GetTVSeasonDetailsBeforeCounter returns a count of TmdbClientMock.GetTVSeasonDetails invocations
func (mmGetTVSeasonDetails *TmdbClientMock) GetTVSeasonDetailsBeforeCounter() uint64 {
	return mm_atomic.LoadUint64(&mmGetTVSeasonDetails.beforeGetTVSeasonDetailsCounter)
}

// Calls returns a list of arguments used in each call to TmdbClientMock.GetTVSeasonDetails.
// The list is in the same order as the calls were made (i.e. recent calls have a higher index)
func (mmGetTVSeasonDetails *mTmdbClientMockGetTVSeasonDetails) Calls() []*TmdbClientMockGetTVSeasonDetailsParams {
	mmGetTVSeasonDetails.mutex.RLock()

	argCopy := make([]*TmdbClientMockGetTVSeasonDetailsParams, len(mmGetTVSeasonDetails.callArgs))
	copy(argCopy, mmGetTVSeasonDetails.callArgs)

	mmGetTVSeasonDetails.mutex.RUnlock()

	return argCopy
}

// MinimockGetTVSeasonDetailsDone returns true if the count of the GetTVSeasonDetails invocations corresponds
// the number of defined expectations
func (m *TmdbClientMock) MinimockGetTVSeasonDetailsDone() bool {
	for _, e := range m.GetTVSeasonDetailsMock.expectations {
		if mm_atomic.LoadUint64(&e.Counter) < 1 {
			return false
		}
	}

	// if default expectation was set then invocations count should be greater than zero
	if m.GetTVSeasonDetailsMock.defaultExpectation != nil && mm_atomic.LoadUint64(&m.afterGetTVSeasonDetailsCounter) < 1 {
		return false
	}
	// if func was set then invocations count should be greater than zero
	if m.funcGetTVSeasonDetails != nil && mm_atomic.LoadUint64(&m.afterGetTVSeasonDetailsCounter) < 1 {
		return false
	}
	return true
}

// MinimockGetTVSeasonDetailsInspect logs each unmet expectation
func (m *TmdbClientMock) MinimockGetTVSeasonDetailsInspect() {
	for _, e := range m.GetTVSeasonDetailsMock.expectations {
		if mm_atomic.LoadUint64(&e.Counter) < 1 {
			m.t.Errorf("Expected call to TmdbClientMock.GetTVSeasonDetails with params: %#v", *e.params)
		}
	}

	// if default expectation was set then invocations count should be greater than zero
	if m.GetTVSeasonDetailsMock.defaultExpectation != nil && mm_atomic.LoadUint64(&m.afterGetTVSeasonDetailsCounter) < 1 {
		if m.GetTVSeasonDetailsMock.defaultExpectation.params == nil {
			m.t.Error("Expected call to TmdbClientMock.GetTVSeasonDetails")
		} else {
			m.t.Errorf("Expected call to TmdbClientMock.GetTVSeasonDetails with params: %#v", *m.GetTVSeasonDetailsMock.defaultExpectation.params)
		}
	}
	// if func was set then invocations count should be greater than zero
	if m.funcGetTVSeasonDetails != nil && mm_atomic.LoadUint64(&m.afterGetTVSeasonDetailsCounter) < 1 {
		m.t.Error("Expected call to TmdbClientMock.GetTVSeasonDetails")
	}
}

// MinimockFinish checks that all mocked methods have been called the expected number of times
func (m *TmdbClientMock) MinimockFinish() {
	if !m.minimockDone() {
		m.MinimockGetSearchTVShowInspect()

		m.MinimockGetTVDetailsInspect()

		m.MinimockGetTVSeasonDetailsInspect()
		m.t.FailNow()
	}
}

// MinimockWait waits for all mocked methods to be called the expected number of times
func (m *TmdbClientMock) MinimockWait(timeout mm_time.Duration) {
	timeoutCh := mm_time.After(timeout)
	for {
		if m.minimockDone() {
			return
		}
		select {
		case <-timeoutCh:
			m.MinimockFinish()
			return
		case <-mm_time.After(10 * mm_time.Millisecond):
		}
	}
}

func (m *TmdbClientMock) minimockDone() bool {
	done := true
	return done &&
		m.MinimockGetSearchTVShowDone() &&
		m.MinimockGetTVDetailsDone() &&
		m.MinimockGetTVSeasonDetailsDone()
}
